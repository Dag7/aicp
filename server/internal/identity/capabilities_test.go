package identity

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── matchesSingleCapability ────────────────────────────────────────────────

func TestMatchesSingleCapability_ExactMatch(t *testing.T) {
	assert.True(t, matchesSingleCapability("payment:read", aicp.CapabilityPaymentRead))
	assert.True(t, matchesSingleCapability("payment:execute", aicp.CapabilityPaymentExecute))
	assert.True(t, matchesSingleCapability("did:create", aicp.CapabilityDIDCreate))
}

func TestMatchesSingleCapability_NoMatch(t *testing.T) {
	assert.False(t, matchesSingleCapability("payment:read", aicp.CapabilityPaymentExecute))
	assert.False(t, matchesSingleCapability("did:create", aicp.CapabilityCredentialIssue))
}

func TestMatchesSingleCapability_GlobalWildcard(t *testing.T) {
	assert.True(t, matchesSingleCapability("*", aicp.CapabilityPaymentRead))
	assert.True(t, matchesSingleCapability("*", aicp.CapabilityDIDCreate))
	assert.True(t, matchesSingleCapability("*", aicp.CapabilityOnboard))
}

func TestMatchesSingleCapability_HierarchicalWildcard(t *testing.T) {
	assert.True(t, matchesSingleCapability("payment:*", aicp.CapabilityPaymentRead))
	assert.True(t, matchesSingleCapability("payment:*", aicp.CapabilityPaymentExecute))
	assert.True(t, matchesSingleCapability("payment:*", aicp.CapabilityPaymentApprove))
	assert.False(t, matchesSingleCapability("payment:*", aicp.CapabilityDIDCreate))
	assert.False(t, matchesSingleCapability("did:*", aicp.CapabilityPaymentRead))
}

func TestMatchesSingleCapability_CategoryMatch(t *testing.T) {
	assert.True(t, matchesSingleCapability("payment", aicp.CapabilityPaymentRead))
	assert.True(t, matchesSingleCapability("payment", aicp.CapabilityPaymentExecute))
	assert.True(t, matchesSingleCapability("did", aicp.CapabilityDIDCreate))
	assert.False(t, matchesSingleCapability("did", aicp.CapabilityPaymentRead))
}

func TestMatchesSingleCapability_SimpleCapability(t *testing.T) {
	// Capabilities without ":" like "onboard" and "subscribe".
	assert.True(t, matchesSingleCapability("onboard", aicp.CapabilityOnboard))
	assert.True(t, matchesSingleCapability("subscribe", aicp.CapabilitySubscribe))
	assert.False(t, matchesSingleCapability("onboard", aicp.CapabilitySubscribe))
}

// ─── matchesCapability ──────────────────────────────────────────────────────

func TestMatchesCapability_AnyMatch(t *testing.T) {
	granted := []string{"payment:read", "did:create"}
	assert.True(t, matchesCapability(granted, aicp.CapabilityPaymentRead))
	assert.True(t, matchesCapability(granted, aicp.CapabilityDIDCreate))
	assert.False(t, matchesCapability(granted, aicp.CapabilityPaymentExecute))
}

func TestMatchesCapability_Empty(t *testing.T) {
	assert.False(t, matchesCapability(nil, aicp.CapabilityPaymentRead))
	assert.False(t, matchesCapability([]string{}, aicp.CapabilityPaymentRead))
}

func TestMatchesCapability_WildcardInList(t *testing.T) {
	granted := []string{"*"}
	assert.True(t, matchesCapability(granted, aicp.CapabilityPaymentRead))
	assert.True(t, matchesCapability(granted, aicp.CapabilityOnboard))
}

// ─── CapabilityChecker (integration with DelegationVerifier) ────────────────

func setupCapabilityChecker(t *testing.T) (*CapabilityChecker, *mockLedgerBackend, string) {
	t.Helper()
	ctx := context.Background()
	backend := newMockLedgerBackend()
	didMgr := NewDIDManager(backend)
	credMgr := NewCredentialManager(backend, didMgr)

	// Create principal via DIDManager (properly anchored).
	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	principalDoc, _, err := didMgr.CreatePrincipal(ctx, principalKP)
	require.NoError(t, err)
	principalDID := principalDoc.ID

	// Create agent via DIDManager.
	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentDoc, _, err := didMgr.CreateAgent(ctx, principalKP, agentKP)
	require.NoError(t, err)
	agentDID := agentDoc.ID

	// Issue a properly signed delegation credential with capabilities.
	delegationSubject := aicp.AgentDelegationSubject{
		ID:                    agentDID,
		DelegatedCapabilities: []string{"payment:read", "payment:execute", "did:create"},
		Principal:             principalDID,
	}
	cred, err := credMgr.IssueCredential(ctx, IssueParams{
		CredentialID: "urn:uuid:test-delegation-cred",
		Types:        []string{"VerifiableCredential", "AgentDelegationCredential"},
		IssuerDID:    principalDID,
		IssuerKey:    principalKP,
		Subject:      delegationSubject,
	})
	require.NoError(t, err)

	// Store the credential and update the agent doc to reference it.
	backend.credentials[cred.ID] = cred
	agentResult := backend.dids[agentDID]
	agentResult.Document.AICPDelegationCred = cred.ID

	verifier := NewDelegationVerifier(backend, didMgr, credMgr)
	checker := NewCapabilityChecker(verifier)

	return checker, backend, agentDID
}

func TestCapabilityChecker_HasCapability(t *testing.T) {
	checker, _, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	has, err := checker.HasCapability(ctx, agentDID, aicp.CapabilityPaymentRead)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = checker.HasCapability(ctx, agentDID, aicp.CapabilityDIDCreate)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestCapabilityChecker_HasCapability_Denied(t *testing.T) {
	checker, _, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	has, err := checker.HasCapability(ctx, agentDID, aicp.CapabilityCredentialIssue)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestCapabilityChecker_HasCapability_InvalidAgent(t *testing.T) {
	checker, _, _ := setupCapabilityChecker(t)
	ctx := context.Background()

	// Nonexistent DID returns false (delegation chain invalid), not an error.
	has, err := checker.HasCapability(ctx, "did:aicp:nonexistent", aicp.CapabilityPaymentRead)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestCapabilityChecker_ListCapabilities(t *testing.T) {
	checker, _, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	caps, err := checker.ListCapabilities(ctx, agentDID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"payment:read", "payment:execute", "did:create"}, caps)
}

func TestCapabilityChecker_ListCapabilities_InvalidAgent(t *testing.T) {
	checker, _, _ := setupCapabilityChecker(t)
	ctx := context.Background()

	// Nonexistent DID returns nil capabilities (delegation chain invalid).
	caps, err := checker.ListCapabilities(ctx, "did:aicp:nonexistent")
	require.NoError(t, err)
	assert.Nil(t, caps)
}

func TestCapabilityChecker_HasCapabilityForResource(t *testing.T) {
	checker, _, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	// Should match via simple capability list (backward compatible).
	has, err := checker.HasCapabilityForResource(ctx, agentDID, aicp.CapabilityPaymentRead, "any-resource")
	require.NoError(t, err)
	assert.True(t, has)
}

// ─── extractCapabilityGrants ────────────────────────────────────────────────

func TestExtractCapabilityGrants_Nil(t *testing.T) {
	grants, err := extractCapabilityGrants(nil)
	assert.NoError(t, err)
	assert.Nil(t, grants)
}

func TestExtractCapabilityGrants_WithGrants(t *testing.T) {
	subject := struct {
		CapabilityGrants []aicp.CapabilityGrant `json:"capabilityGrants"`
	}{
		CapabilityGrants: []aicp.CapabilityGrant{
			{Capability: aicp.CapabilityPaymentRead, Resource: "svc-123"},
			{Capability: aicp.CapabilityPaymentExecute, Resource: "*"},
		},
	}
	subjectBytes, _ := json.Marshal(subject)

	cred := &aicp.VerifiableCredential{
		CredentialSubject: subjectBytes,
	}
	grants, err := extractCapabilityGrants(cred)
	require.NoError(t, err)
	assert.Len(t, grants, 2)
	assert.Equal(t, aicp.CapabilityPaymentRead, grants[0].Capability)
	assert.Equal(t, "svc-123", grants[0].Resource)
}

func TestExtractCapabilityGrants_NoGrants(t *testing.T) {
	subject := map[string]string{"other": "data"}
	subjectBytes, _ := json.Marshal(subject)

	cred := &aicp.VerifiableCredential{
		CredentialSubject: subjectBytes,
	}
	grants, err := extractCapabilityGrants(cred)
	require.NoError(t, err)
	assert.Empty(t, grants)
}
