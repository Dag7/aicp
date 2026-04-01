package identity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

func setupOIDCBridge(t *testing.T) (*OIDCBridge, string) {
	t.Helper()
	backend := newMockLedgerBackend()

	// Create a principal DID.
	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)

	doc := aicp.DIDDocument{
		ID:         principalDID,
		Controller: principalDID,
		VerificationMethod: []aicp.VerificationMethod{
			{
				ID:                 principalDID + "#key-1",
				Type:               "Ed25519VerificationKey2020",
				Controller:         principalDID,
				PublicKeyMultibase: crypto.PublicKeyMultibase(principalKP.PublicKey),
			},
		},
		Created:   time.Now().UTC(),
		Updated:   time.Now().UTC(),
		VersionID: 1,
	}
	backend.dids[principalDID] = &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   doc.Created,
			Updated:   doc.Updated,
			VersionID: 1,
		},
	}

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	didMgr := NewDIDManager(backend)
	bridge := NewOIDCBridge(didMgr, issuerKP, "https://idp.aicp.example.com")

	return bridge, principalDID
}

func TestOIDCBridge_IssueIDToken(t *testing.T) {
	bridge, did := setupOIDCBridge(t)

	resp, err := bridge.IssueIDToken(context.Background(), did, "https://app.example.com", "test-nonce", 5*time.Minute)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.IDToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, 300, resp.ExpiresIn)
}

func TestOIDCBridge_IssueAndVerifyIDToken(t *testing.T) {
	bridge, did := setupOIDCBridge(t)

	resp, err := bridge.IssueIDToken(context.Background(), did, "https://app.example.com", "nonce-123", 5*time.Minute)
	require.NoError(t, err)

	claims, err := bridge.VerifyIDToken(resp.IDToken)
	require.NoError(t, err)

	assert.Equal(t, "https://idp.aicp.example.com", claims.Issuer)
	assert.Equal(t, did, claims.Subject)
	assert.Equal(t, "https://app.example.com", claims.Audience)
	assert.Equal(t, "nonce-123", claims.Nonce)
	assert.Equal(t, did, claims.DID)
	assert.Equal(t, "aicp", claims.DIDMethod)
	assert.Equal(t, "Ed25519", claims.KeyType)
	assert.NotEmpty(t, claims.PublicKeyHash)
}

func TestOIDCBridge_VerifyIDToken_Expired(t *testing.T) {
	bridge, did := setupOIDCBridge(t)

	// Issue a token with very short TTL that will already be expired.
	resp, err := bridge.IssueIDToken(context.Background(), did, "https://app.example.com", "", -1*time.Second)
	require.NoError(t, err)

	_, err = bridge.VerifyIDToken(resp.IDToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestOIDCBridge_VerifyIDToken_InvalidToken(t *testing.T) {
	bridge, _ := setupOIDCBridge(t)

	_, err := bridge.VerifyIDToken("not-a-valid-jws-token")
	assert.Error(t, err)
}

func TestOIDCBridge_VerifyIDToken_WrongIssuer(t *testing.T) {
	backend := newMockLedgerBackend()
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	did := crypto.PrincipalDID(kp.PublicKey)

	doc := aicp.DIDDocument{
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{
				ID:                 did + "#key-1",
				Type:               "Ed25519VerificationKey2020",
				Controller:         did,
				PublicKeyMultibase: crypto.PublicKeyMultibase(kp.PublicKey),
			},
		},
		Created:   time.Now().UTC(),
		Updated:   time.Now().UTC(),
		VersionID: 1,
	}
	backend.dids[did] = &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   doc.Created,
			Updated:   doc.Updated,
			VersionID: 1,
		},
	}

	issuerKP, _ := crypto.GenerateEd25519KeyPair()
	didMgr := NewDIDManager(backend)

	// Issue with one issuer URL.
	bridge1 := NewOIDCBridge(didMgr, issuerKP, "https://issuer1.example.com")
	resp, err := bridge1.IssueIDToken(context.Background(), did, "aud", "", 5*time.Minute)
	require.NoError(t, err)

	// Verify with a different issuer URL.
	bridge2 := NewOIDCBridge(didMgr, issuerKP, "https://issuer2.example.com")
	_, err = bridge2.VerifyIDToken(resp.IDToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "issuer mismatch")
}

func TestOIDCBridge_IssueIDToken_DeactivatedDID(t *testing.T) {
	backend := newMockLedgerBackend()
	kp, _ := crypto.GenerateEd25519KeyPair()
	did := crypto.PrincipalDID(kp.PublicKey)

	backend.dids[did] = &aicp.DIDResolutionResult{
		Document: &aicp.DIDDocument{ID: did},
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Deactivated: true,
		},
	}

	issuerKP, _ := crypto.GenerateEd25519KeyPair()
	didMgr := NewDIDManager(backend)
	bridge := NewOIDCBridge(didMgr, issuerKP, "https://idp.example.com")

	_, err := bridge.IssueIDToken(context.Background(), did, "aud", "", 5*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deactivated")
}

func TestOIDCBridge_IssueIDToken_NonexistentDID(t *testing.T) {
	backend := newMockLedgerBackend()
	issuerKP, _ := crypto.GenerateEd25519KeyPair()
	didMgr := NewDIDManager(backend)
	bridge := NewOIDCBridge(didMgr, issuerKP, "https://idp.example.com")

	_, err := bridge.IssueIDToken(context.Background(), "did:aicp:nonexistent", "aud", "", 5*time.Minute)
	assert.Error(t, err)
}

func TestOIDCBridge_AgentDID_PrincipalInClaims(t *testing.T) {
	backend := newMockLedgerBackend()

	principalKP, _ := crypto.GenerateEd25519KeyPair()
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)
	backend.dids[principalDID] = &aicp.DIDResolutionResult{
		Document:         &aicp.DIDDocument{ID: principalDID, Controller: principalDID},
		DocumentMetadata: &aicp.DIDDocumentMetadata{VersionID: 1},
	}

	agentKP, _ := crypto.GenerateEd25519KeyPair()
	agentDID := crypto.AgentDID(principalKP.PublicKey, agentKP.PublicKey)
	backend.dids[agentDID] = &aicp.DIDResolutionResult{
		Document: &aicp.DIDDocument{
			ID:            agentDID,
			Controller:    principalDID,
			AICPPrincipal: principalDID,
			VerificationMethod: []aicp.VerificationMethod{
				{
					ID:                 agentDID + "#key-1",
					Type:               "Ed25519VerificationKey2020",
					PublicKeyMultibase: crypto.PublicKeyMultibase(agentKP.PublicKey),
				},
			},
		},
		DocumentMetadata: &aicp.DIDDocumentMetadata{VersionID: 1},
	}

	issuerKP, _ := crypto.GenerateEd25519KeyPair()
	didMgr := NewDIDManager(backend)
	bridge := NewOIDCBridge(didMgr, issuerKP, "https://idp.example.com")

	resp, err := bridge.IssueIDToken(context.Background(), agentDID, "aud", "", 5*time.Minute)
	require.NoError(t, err)

	claims, err := bridge.VerifyIDToken(resp.IDToken)
	require.NoError(t, err)
	assert.Equal(t, agentDID, claims.Subject)
	assert.Equal(t, principalDID, claims.Principal)
}

// ─── OIDC Discovery & JWKS ─────────────────────────────────────────────────

func TestOIDCBridge_OIDCDiscovery(t *testing.T) {
	bridge, _ := setupOIDCBridge(t)
	disc := bridge.OIDCDiscovery()

	assert.Equal(t, "https://idp.aicp.example.com", disc["issuer"])
	assert.Contains(t, disc["jwks_uri"], ".well-known/jwks.json")
	assert.Contains(t, disc["id_token_signing_alg_values_supported"], "EdDSA")

	claims, ok := disc["claims_supported"].([]string)
	assert.True(t, ok)
	assert.Contains(t, claims, "aicp_did")
	assert.Contains(t, claims, "sub")
}

func TestOIDCBridge_JWKS(t *testing.T) {
	bridge, _ := setupOIDCBridge(t)
	jwks := bridge.JWKS()

	keys, ok := jwks["keys"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, keys, 1)

	key := keys[0]
	assert.Equal(t, "OKP", key["kty"])
	assert.Equal(t, "Ed25519", key["crv"])
	assert.Equal(t, "sig", key["use"])
	assert.NotEmpty(t, key["kid"])
	assert.NotEmpty(t, key["x"])
}
