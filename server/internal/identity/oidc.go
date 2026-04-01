package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
)

// OIDCBridge maps AICP DIDs to OpenID Connect compatible identity tokens,
// enabling integration with traditional web services that expect OIDC identity.
type OIDCBridge struct {
	manager  *DIDManager
	issuerKP *crypto.KeyPair
	issuer   string // OIDC issuer URL, e.g., "https://idp.aicp.example.com"
}

// NewOIDCBridge creates a new OIDC bridge.
func NewOIDCBridge(manager *DIDManager, issuerKP *crypto.KeyPair, issuer string) *OIDCBridge {
	return &OIDCBridge{
		manager:  manager,
		issuerKP: issuerKP,
		issuer:   issuer,
	}
}

// OIDCClaims represents the OIDC-compatible claims derived from an AICP DID.
type OIDCClaims struct {
	// Standard OIDC claims
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`             // The DID
	Audience  string `json:"aud,omitempty"`    // Relying party
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	Nonce     string `json:"nonce,omitempty"`

	// AICP-specific claims
	DID           string   `json:"aicp_did"`
	DIDMethod     string   `json:"aicp_did_method"`
	Principal     string   `json:"aicp_principal,omitempty"`
	Capabilities  []string `json:"aicp_capabilities,omitempty"`
	TrustScore    int      `json:"aicp_trust_score,omitempty"`
	KeyType       string   `json:"aicp_key_type"`
	PublicKeyHash string   `json:"aicp_public_key_hash,omitempty"`
}

// OIDCTokenResponse is returned when exchanging a DID proof for an OIDC token.
type OIDCTokenResponse struct {
	IDToken   string `json:"id_token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// IssueIDToken creates an OIDC-compatible ID token from a verified AICP DID.
// The caller must have already verified the DID's proof of control.
func (ob *OIDCBridge) IssueIDToken(ctx context.Context, did string, audience string, nonce string, ttl time.Duration) (*OIDCTokenResponse, error) {
	// Resolve the DID to get the document.
	result, err := ob.manager.Resolve(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("resolving DID for OIDC: %w", err)
	}
	if result.DocumentMetadata.Deactivated {
		return nil, fmt.Errorf("DID is deactivated")
	}

	now := time.Now().UTC()
	claims := OIDCClaims{
		Issuer:    ob.issuer,
		Subject:   did,
		Audience:  audience,
		ExpiresAt: now.Add(ttl).Unix(),
		IssuedAt:  now.Unix(),
		Nonce:     nonce,
		DID:       did,
		DIDMethod: didMethod(did),
		KeyType:   "Ed25519",
	}

	// Add principal if this is an agent DID.
	if result.Document.AICPPrincipal != "" {
		claims.Principal = result.Document.AICPPrincipal
	} else if result.Document.Controller != did {
		claims.Principal = result.Document.Controller
	}

	// Add public key hash for the first verification method.
	if len(result.Document.VerificationMethod) > 0 {
		vm := result.Document.VerificationMethod[0]
		claims.PublicKeyHash = crypto.SHA256Hex([]byte(vm.PublicKeyMultibase))[:16]
	}

	// Sign the token.
	token, err := ob.signToken(claims)
	if err != nil {
		return nil, fmt.Errorf("signing OIDC token: %w", err)
	}

	return &OIDCTokenResponse{
		IDToken:   token,
		TokenType: "Bearer",
		ExpiresIn: int(ttl.Seconds()),
	}, nil
}

// OIDCDiscovery returns the OIDC discovery document (.well-known/openid-configuration).
func (ob *OIDCBridge) OIDCDiscovery() map[string]any {
	return map[string]any{
		"issuer":                                ob.issuer,
		"authorization_endpoint":                ob.issuer + "/authorize",
		"token_endpoint":                        ob.issuer + "/token",
		"jwks_uri":                              ob.issuer + "/.well-known/jwks.json",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"EdDSA"},
		"claims_supported": []string{
			"iss", "sub", "aud", "exp", "iat", "nonce",
			"aicp_did", "aicp_did_method", "aicp_principal",
			"aicp_capabilities", "aicp_trust_score", "aicp_key_type",
		},
	}
}

// JWKS returns the JSON Web Key Set containing the issuer's public key.
func (ob *OIDCBridge) JWKS() map[string]any {
	pubKey := ob.issuerKP.PublicKey
	multibase := crypto.PublicKeyMultibase(ed25519.PublicKey(pubKey))

	return map[string]any{
		"keys": []map[string]any{
			{
				"kty": "OKP",
				"crv": "Ed25519",
				"use": "sig",
				"kid": crypto.SHA256Hex(pubKey)[:16],
				"x":   multibase,
			},
		},
	}
}

// signToken signs the OIDC claims using the issuer's Ed25519 key via JWS.
func (ob *OIDCBridge) signToken(claims OIDCClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshaling claims: %w", err)
	}

	return crypto.SignJWS(ob.issuerKP.PrivateKey, payload, "JWT")
}

// VerifyIDToken verifies an AICP OIDC ID token and returns the claims.
func (ob *OIDCBridge) VerifyIDToken(token string) (*OIDCClaims, error) {
	payload, err := crypto.VerifyJWS(ob.issuerKP.PublicKey, token)
	if err != nil {
		return nil, fmt.Errorf("verifying OIDC token signature: %w", err)
	}

	var claims OIDCClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("decoding OIDC claims: %w", err)
	}

	// Check expiration.
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("OIDC token expired")
	}

	// Verify issuer.
	if claims.Issuer != ob.issuer {
		return nil, fmt.Errorf("OIDC issuer mismatch: got %s, expected %s", claims.Issuer, ob.issuer)
	}

	return &claims, nil
}

// didMethod extracts the DID method from a DID string (e.g., "aicp" from "did:aicp:abc").
// Returns "unknown" for malformed DIDs. Use extractMethod for strict parsing with errors.
func didMethod(did string) string {
	parts := strings.SplitN(did, ":", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return "unknown"
}
