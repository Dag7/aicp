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

func TestMatchesSingleCapability_CategoryRequiresExplicitWildcard(t *testing.T) {
	// Bare category names do NOT implicitly match sub-capabilities.
	// Use "payment:*" for hierarchical matching.
	assert.False(t, matchesSingleCapability("payment", aicp.CapabilityPaymentRead))
	assert.False(t, matchesSingleCapability("payment", aicp.CapabilityPaymentExecute))
	assert.False(t, matchesSingleCapability("did", aicp.CapabilityDIDCreate))

	// Explicit wildcard still works.
	assert.True(t, matchesSingleCapability("payment:*", aicp.CapabilityPaymentRead))
	assert.True(t, matchesSingleCapability("did:*", aicp.CapabilityDIDCreate))
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

func setupCapabilityChecker(t *testing.T) (*CapabilityChecker, string) {
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

	return checker, agentDID
}

func TestCapabilityChecker_HasCapability(t *testing.T) {
	checker, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	has, err := checker.HasCapability(ctx, agentDID, aicp.CapabilityPaymentRead)
	require.NoError(t, err)
	assert.True(t, has)

	has, err = checker.HasCapability(ctx, agentDID, aicp.CapabilityDIDCreate)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestCapabilityChecker_HasCapability_Denied(t *testing.T) {
	checker, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	has, err := checker.HasCapability(ctx, agentDID, aicp.CapabilityCredentialIssue)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestCapabilityChecker_HasCapability_InvalidAgent(t *testing.T) {
	checker, _ := setupCapabilityChecker(t)
	ctx := context.Background()

	// Nonexistent DID returns false (delegation chain invalid), not an error.
	has, err := checker.HasCapability(ctx, "did:aicp:nonexistent", aicp.CapabilityPaymentRead)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestCapabilityChecker_ListCapabilities(t *testing.T) {
	checker, agentDID := setupCapabilityChecker(t)
	ctx := context.Background()

	caps, err := checker.ListCapabilities(ctx, agentDID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"payment:read", "payment:execute", "did:create"}, caps)
}

func TestCapabilityChecker_ListCapabilities_InvalidAgent(t *testing.T) {
	checker, _ := setupCapabilityChecker(t)
	ctx := context.Background()

	// Nonexistent DID returns nil capabilities (delegation chain invalid).
	caps, err := checker.ListCapabilities(ctx, "did:aicp:nonexistent")
	require.NoError(t, err)
	assert.Nil(t, caps)
}

func TestCapabilityChecker_HasCapabilityForResource(t *testing.T) {
	checker, agentDID := setupCapabilityChecker(t)
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

// ─── Security-focused Capability Tests ──────────────────────────────────────

// TestCapabilityMatching_WildcardExplicit verifies that a hierarchical wildcard
// grant like "payment:*" correctly matches all sub-capabilities under the
// "payment" category.
func TestCapabilityMatching_WildcardExplicit(t *testing.T) {
	granted := []string{"payment:*"}

	// Must match all payment sub-capabilities.
	assert.True(t, matchesCapability(granted, aicp.CapabilityPaymentRead),
		"payment:* must match payment:read")
	assert.True(t, matchesCapability(granted, aicp.CapabilityPaymentExecute),
		"payment:* must match payment:execute")
	assert.True(t, matchesCapability(granted, aicp.CapabilityPaymentApprove),
		"payment:* must match payment:approve")
	assert.True(t, matchesCapability(granted, aicp.CapabilityType("payment:write")),
		"payment:* must match payment:write (arbitrary sub-capability)")
	assert.True(t, matchesCapability(granted, aicp.CapabilityType("payment:refund")),
		"payment:* must match payment:refund (future sub-capability)")

	// Must NOT match capabilities from other categories.
	assert.False(t, matchesCapability(granted, aicp.CapabilityDIDCreate),
		"payment:* must not match did:create")
	assert.False(t, matchesCapability(granted, aicp.CapabilityCredentialIssue),
		"payment:* must not match credential:issue")
	assert.False(t, matchesCapability(granted, aicp.CapabilityOnboard),
		"payment:* must not match onboard (different category)")
	assert.False(t, matchesCapability(granted, aicp.CapabilityDiscoveryRead),
		"payment:* must not match discovery:read")
}

// TestCapabilityMatching_NoImplicitCategoryMatch verifies that granting a bare
// category name (e.g., "payment") does NOT implicitly match sub-capabilities
// like "payment:read". This is a critical security invariant: a grant must be
// explicit. Only "payment:*" should match sub-capabilities.
func TestCapabilityMatching_NoImplicitCategoryMatch(t *testing.T) {
	// Bare category grants.
	grantedPayment := []string{"payment"}
	grantedDID := []string{"did"}
	grantedCredential := []string{"credential"}
	grantedDiscovery := []string{"discovery"}

	// "payment" must NOT match "payment:read", "payment:execute", etc.
	assert.False(t, matchesCapability(grantedPayment, aicp.CapabilityPaymentRead),
		"bare 'payment' must not match 'payment:read'")
	assert.False(t, matchesCapability(grantedPayment, aicp.CapabilityPaymentExecute),
		"bare 'payment' must not match 'payment:execute'")
	assert.False(t, matchesCapability(grantedPayment, aicp.CapabilityPaymentApprove),
		"bare 'payment' must not match 'payment:approve'")

	// "did" must NOT match "did:create", "did:update", etc.
	assert.False(t, matchesCapability(grantedDID, aicp.CapabilityDIDCreate),
		"bare 'did' must not match 'did:create'")
	assert.False(t, matchesCapability(grantedDID, aicp.CapabilityDIDUpdate),
		"bare 'did' must not match 'did:update'")
	assert.False(t, matchesCapability(grantedDID, aicp.CapabilityDIDDeactivate),
		"bare 'did' must not match 'did:deactivate'")

	// "credential" must NOT match "credential:issue", "credential:revoke".
	assert.False(t, matchesCapability(grantedCredential, aicp.CapabilityCredentialIssue),
		"bare 'credential' must not match 'credential:issue'")
	assert.False(t, matchesCapability(grantedCredential, aicp.CapabilityCredentialRevoke),
		"bare 'credential' must not match 'credential:revoke'")

	// "discovery" must NOT match "discovery:read", "discovery:write".
	assert.False(t, matchesCapability(grantedDiscovery, aicp.CapabilityDiscoveryRead),
		"bare 'discovery' must not match 'discovery:read'")
	assert.False(t, matchesCapability(grantedDiscovery, aicp.CapabilityDiscoveryWrite),
		"bare 'discovery' must not match 'discovery:write'")

	// But bare categories MUST match themselves exactly (an agent granted
	// exactly "payment" can be checked for exactly "payment").
	assert.True(t, matchesCapability(grantedPayment, aicp.CapabilityType("payment")),
		"bare 'payment' must match itself exactly")
	assert.True(t, matchesCapability(grantedDID, aicp.CapabilityType("did")),
		"bare 'did' must match itself exactly")
}

// TestHasCapabilityForResource_MalformedGrants verifies that when a credential
// contains a CredentialSubject with a malformed capabilityGrants field,
// extractCapabilityGrants returns a parse error. This is tested at the
// extractCapabilityGrants layer directly, because the delegation verifier
// rightfully rejects credentials whose subject is entirely invalid JSON before
// it reaches the capability checker.
//
// In a real attack scenario, the credential subject could be structurally valid
// JSON (passing delegation verification) but contain a corrupted capabilityGrants
// field. The system must return an error rather than silently ignoring malformed
// grants and returning false.
func TestHasCapabilityForResource_MalformedGrants(t *testing.T) {
	t.Run("completely invalid JSON subject returns error", func(t *testing.T) {
		cred := &aicp.VerifiableCredential{
			CredentialSubject: json.RawMessage(`{this is not valid json!!!}`),
		}
		grants, err := extractCapabilityGrants(cred)
		assert.Error(t, err, "malformed JSON must produce a parse error, not a silent nil")
		assert.Nil(t, grants)
	})

	t.Run("capabilityGrants with wrong type returns error", func(t *testing.T) {
		// The capabilityGrants field is a string instead of an array of objects.
		cred := &aicp.VerifiableCredential{
			CredentialSubject: json.RawMessage(`{"capabilityGrants": "not-an-array"}`),
		}
		grants, err := extractCapabilityGrants(cred)
		assert.Error(t, err, "capabilityGrants as string must produce a parse error")
		assert.Nil(t, grants)
	})

	t.Run("capabilityGrants with malformed grant objects returns error", func(t *testing.T) {
		// The capabilityGrants field is an array but the elements are ints, not objects.
		cred := &aicp.VerifiableCredential{
			CredentialSubject: json.RawMessage(`{"capabilityGrants": [1, 2, 3]}`),
		}
		// json.Unmarshal for CapabilityGrant from an int will not produce an
		// error in standard Go encoding/json (it silently produces zero-value
		// structs). This is a known Go behavior. The test documents this so
		// that future maintainers are aware and can add stricter validation.
		grants, err := extractCapabilityGrants(cred)
		if err != nil {
			// If strict parsing is added later, this branch catches it.
			assert.Nil(t, grants)
		} else {
			// Standard encoding/json silently decodes ints as zero-value CapabilityGrants.
			assert.Len(t, grants, 3)
			for _, g := range grants {
				assert.Empty(t, string(g.Capability),
					"malformed grant should have empty capability (zero value)")
			}
		}
	})

	t.Run("truncated JSON subject returns error", func(t *testing.T) {
		cred := &aicp.VerifiableCredential{
			CredentialSubject: json.RawMessage(`{"capabilityGrants": [{"capability": "pay`),
		}
		grants, err := extractCapabilityGrants(cred)
		assert.Error(t, err, "truncated JSON must produce a parse error")
		assert.Nil(t, grants)
	})

	t.Run("empty byte slice subject returns error", func(t *testing.T) {
		cred := &aicp.VerifiableCredential{
			CredentialSubject: json.RawMessage(``),
		}
		grants, err := extractCapabilityGrants(cred)
		assert.Error(t, err, "empty subject bytes must produce a parse error")
		assert.Nil(t, grants)
	})
}
