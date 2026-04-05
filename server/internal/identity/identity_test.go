// Copyright 2024 AICP Protocol Contributors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package identity

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Mock LedgerBackend ─────────────────────────────────────────────────────
// Minimal in-memory stub that satisfies the methods used by the identity package.

type mockLedgerBackend struct {
	mu          sync.RWMutex
	dids        map[string]*aicp.DIDResolutionResult
	credentials map[string]*aicp.VerifiableCredential
	revoked     map[string]bool
	agents      map[string][]string // principalDID -> []agentDID
}

func newMockLedgerBackend() *mockLedgerBackend {
	return &mockLedgerBackend{
		dids:        make(map[string]*aicp.DIDResolutionResult),
		credentials: make(map[string]*aicp.VerifiableCredential),
		revoked:     make(map[string]bool),
		agents:      make(map[string][]string),
	}
}

func (m *mockLedgerBackend) AnchorDID(_ context.Context, did string, doc aicp.DIDDocument, _ aicp.Proof) (*aicp.AnchorReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.dids[did]; exists {
		return nil, aicp.ErrAlreadyExists
	}
	m.dids[did] = &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
			VersionID: 1,
		},
	}
	// Track agents under their controller.
	if doc.Controller != did {
		m.agents[doc.Controller] = append(m.agents[doc.Controller], did)
	}
	return &aicp.AnchorReceipt{
		DID:          did,
		BlockNumber:  1,
		EntryHash:    crypto.SHA256Hex([]byte(did)),
		DocumentHash: crypto.SHA256Hex([]byte(did + ":doc")),
		AnchoredAt:   time.Now().UTC(),
	}, nil
}

func (m *mockLedgerBackend) ResolveDID(_ context.Context, did string) (*aicp.DIDResolutionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result, ok := m.dids[did]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	return result, nil
}

func (m *mockLedgerBackend) UpdateDIDDocument(_ context.Context, did string, doc aicp.DIDDocument, _ aicp.Proof) (*aicp.AnchorReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.dids[did]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	existing.Document = &doc
	existing.DocumentMetadata.Updated = time.Now().UTC()
	existing.DocumentMetadata.VersionID = doc.VersionID
	return &aicp.AnchorReceipt{
		DID:          did,
		BlockNumber:  2,
		EntryHash:    crypto.SHA256Hex([]byte(did + ":updated")),
		DocumentHash: crypto.SHA256Hex([]byte(did + ":doc:v2")),
		AnchoredAt:   time.Now().UTC(),
	}, nil
}

func (m *mockLedgerBackend) DeactivateDID(_ context.Context, did string, _ aicp.Proof) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	result, ok := m.dids[did]
	if !ok {
		return aicp.ErrNotFound
	}
	result.DocumentMetadata.Deactivated = true
	return nil
}

func (m *mockLedgerBackend) ListAgentDIDs(_ context.Context, principalDID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[principalDID], nil
}

func (m *mockLedgerBackend) StoreCredential(_ context.Context, cred aicp.VerifiableCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.credentials[cred.ID] = &cred
	return nil
}

func (m *mockLedgerBackend) GetCredential(_ context.Context, credentialID string) (*aicp.VerifiableCredential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cred, ok := m.credentials[credentialID]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	return cred, nil
}

func (m *mockLedgerBackend) RevokeCredential(_ context.Context, credentialID string, _ aicp.Proof) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revoked[credentialID] = true
	return nil
}

func (m *mockLedgerBackend) IsCredentialRevoked(_ context.Context, credentialID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revoked[credentialID], nil
}

// Remaining LedgerBackend interface methods -- not used by identity tests, stubbed out.
func (m *mockLedgerBackend) Initialize(context.Context) error { return nil }
func (m *mockLedgerBackend) Close() error                     { return nil }
func (m *mockLedgerBackend) Healthy(context.Context) error    { return nil }
func (m *mockLedgerBackend) GetRevocationList(context.Context, string) (*aicp.RevocationList, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) CreateRevocationList(context.Context, aicp.RevocationList) error {
	return nil
}
func (m *mockLedgerBackend) UpdateRevocationList(context.Context, aicp.RevocationList) error {
	return nil
}
func (m *mockLedgerBackend) StoreReceipt(context.Context, aicp.SignedPaymentReceipt) error {
	return nil
}
func (m *mockLedgerBackend) GetReceipt(context.Context, string) (*aicp.SignedPaymentReceipt, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) ListReceiptsByAgent(context.Context, string, aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error) {
	return &aicp.PaginatedResult[aicp.SignedPaymentReceipt]{}, nil
}
func (m *mockLedgerBackend) IsTokenUsed(context.Context, string) (bool, error)   { return false, nil }
func (m *mockLedgerBackend) MarkTokenUsed(context.Context, string) error         { return nil }
func (m *mockLedgerBackend) AppendEntry(context.Context, aicp.LedgerEntry) error { return nil }
func (m *mockLedgerBackend) GetPendingEntries(context.Context) ([]aicp.LedgerEntry, error) {
	return nil, nil
}
func (m *mockLedgerBackend) StoreBlock(context.Context, aicp.Block) error { return nil }
func (m *mockLedgerBackend) GetLatestBlock(context.Context) (*aicp.Block, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetBlock(context.Context, uint64) (*aicp.Block, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetTrustScore(context.Context, string) (*aicp.TrustScore, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) UpdateTrustScore(context.Context, aicp.TrustScore) error { return nil }
func (m *mockLedgerBackend) StoreANSRecord(context.Context, aicp.ANSRecord) error    { return nil }
func (m *mockLedgerBackend) GetANSRecord(context.Context, string) (*aicp.ANSRecord, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) SearchANSRecords(context.Context, string, aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	return &aicp.PaginatedResult[aicp.ANSRecord]{}, nil
}
func (m *mockLedgerBackend) DeleteANSRecord(context.Context, string) error { return nil }
func (m *mockLedgerBackend) RegisterValidator(context.Context, aicp.Validator) error {
	return nil
}
func (m *mockLedgerBackend) RemoveValidator(context.Context, string) error { return nil }
func (m *mockLedgerBackend) ListValidators(context.Context) ([]aicp.Validator, error) {
	return nil, nil
}
func (m *mockLedgerBackend) GetValidator(context.Context, string) (*aicp.Validator, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) GetDailySpending(context.Context, string, string) (string, error) {
	return "0", nil
}
func (m *mockLedgerBackend) GetMonthlySpending(context.Context, string, string) (string, error) {
	return "0", nil
}
func (m *mockLedgerBackend) RecordSpending(context.Context, string, aicp.MoneyAmount) error {
	return nil
}
func (m *mockLedgerBackend) CheckAndRecordSpending(context.Context, string, aicp.MoneyAmount, string, string) (bool, error) {
	return true, nil
}

// ─── ParseDID Tests ──────────────────────────────────────────────────────────

func TestParseDID_ValidPrincipal(t *testing.T) {
	method, principalID, agentID, err := ParseDID("did:aicp:abc123")
	require.NoError(t, err)
	assert.Equal(t, "aicp", method)
	assert.Equal(t, "abc123", principalID)
	assert.Empty(t, agentID)
}

func TestParseDID_ValidAgent(t *testing.T) {
	method, principalID, agentID, err := ParseDID("did:aicp:abc123:def456")
	require.NoError(t, err)
	assert.Equal(t, "aicp", method)
	assert.Equal(t, "abc123", principalID)
	assert.Equal(t, "def456", agentID)
}

func TestParseDID_TableDrivenErrors(t *testing.T) {
	tests := []struct {
		name string
		did  string
		want string
	}{
		{"empty string", "", "must start with 'did:'"},
		{"missing method", "did:", "insufficient parts"},
		{"wrong method", "did:web:example.com", "unsupported DID method"},
		{"empty principal", "did:aicp:", "empty principal identifier"},
		{"empty agent", "did:aicp:abc:", "empty agent identifier"},
		{"too many parts", "did:aicp:a:b:c", "too many parts"},
		{"no did prefix", "aicp:abc", "must start with 'did:'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := ParseDID(tt.did)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestIsPrincipalDID(t *testing.T) {
	assert.True(t, IsPrincipalDID("did:aicp:abc123"))
	assert.False(t, IsPrincipalDID("did:aicp:abc123:def456"))
	assert.False(t, IsPrincipalDID("invalid"))
}

func TestIsAgentDID(t *testing.T) {
	assert.False(t, IsAgentDID("did:aicp:abc123"))
	assert.True(t, IsAgentDID("did:aicp:abc123:def456"))
	assert.False(t, IsAgentDID("invalid"))
}

func TestPrincipalDIDFromAgent(t *testing.T) {
	t.Run("extracts principal from agent DID", func(t *testing.T) {
		principalDID, err := PrincipalDIDFromAgent("did:aicp:abc123:def456")
		require.NoError(t, err)
		assert.Equal(t, "did:aicp:abc123", principalDID)
	})

	t.Run("returns principal DID unchanged", func(t *testing.T) {
		principalDID, err := PrincipalDIDFromAgent("did:aicp:abc123")
		require.NoError(t, err)
		assert.Equal(t, "did:aicp:abc123", principalDID)
	})

	t.Run("returns error for invalid DID", func(t *testing.T) {
		_, err := PrincipalDIDFromAgent("invalid")
		require.Error(t, err)
	})
}

func TestValidateDID(t *testing.T) {
	assert.NoError(t, ValidateDID("did:aicp:abc123"))
	assert.NoError(t, ValidateDID("did:aicp:abc123:def456"))
	assert.Error(t, ValidateDID(""))
	assert.Error(t, ValidateDID("did:web:example"))
}

// ─── DID Document Builder Tests ──────────────────────────────────────────────

func TestBuildPrincipalDocument(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	did := crypto.PrincipalDID(kp.PublicKey)
	doc := BuildPrincipalDocument(did, kp.PublicKey)

	assert.Equal(t, did, doc.ID)
	assert.Equal(t, did, doc.Controller)
	assert.Contains(t, doc.Context, "https://www.w3.org/ns/did/v1")
	assert.Contains(t, doc.Context, "https://aicp-protocol.org/ns/v1")
	assert.Len(t, doc.VerificationMethod, 1)
	assert.Equal(t, "Ed25519VerificationKey2020", doc.VerificationMethod[0].Type)
	assert.Equal(t, did+"#key-1", doc.VerificationMethod[0].ID)
	assert.Equal(t, did, doc.VerificationMethod[0].Controller)
	assert.Contains(t, doc.Authentication, did+"#key-1")
	assert.Contains(t, doc.AssertionMethod, did+"#key-1")
	assert.Contains(t, doc.CapabilityDelegation, did+"#key-1")
	assert.Equal(t, 1, doc.VersionID)
	assert.False(t, doc.Created.IsZero())
	assert.False(t, doc.Updated.IsZero())
}

func TestBuildAgentDocument(t *testing.T) {
	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	principalDID := crypto.PrincipalDID(principalKP.PublicKey)
	agentDID := crypto.AgentDID(principalKP.PublicKey, agentKP.PublicKey)

	doc := BuildAgentDocument(agentDID, principalDID, agentKP.PublicKey)

	assert.Equal(t, agentDID, doc.ID)
	assert.Equal(t, principalDID, doc.Controller)
	assert.Equal(t, principalDID, doc.AICPPrincipal)
	assert.Len(t, doc.VerificationMethod, 1)
	assert.Equal(t, agentDID+"#key-1", doc.VerificationMethod[0].ID)
	assert.Contains(t, doc.Authentication, agentDID+"#key-1")
	assert.Empty(t, doc.CapabilityDelegation, "agent documents should not have capability delegation")
	assert.Equal(t, 1, doc.VersionID)
}

func TestAddServiceEndpoint(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	did := crypto.PrincipalDID(kp.PublicKey)
	doc := BuildPrincipalDocument(did, kp.PublicKey)

	assert.Empty(t, doc.Service)

	AddServiceEndpoint(&doc, "#api", "AgentService", "https://example.com/api")

	require.Len(t, doc.Service, 1)
	assert.Equal(t, "#api", doc.Service[0].ID)
	assert.Equal(t, "AgentService", doc.Service[0].Type)
	assert.Equal(t, "https://example.com/api", doc.Service[0].ServiceEndpoint)
}

func TestAddVerificationMethod(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	did := crypto.PrincipalDID(kp.PublicKey)
	doc := BuildPrincipalDocument(did, kp.PublicKey)

	require.Len(t, doc.VerificationMethod, 1)

	rotatedKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	AddVerificationMethod(&doc, did+"#key-2", rotatedKP.PublicKey, did)

	require.Len(t, doc.VerificationMethod, 2)
	assert.Equal(t, did+"#key-2", doc.VerificationMethod[1].ID)
}

// ─── DID Manager Tests ───────────────────────────────────────────────────────

func TestDIDManager_CreatePrincipal(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	ctx := context.Background()
	doc, receipt, err := dm.CreatePrincipal(ctx, kp)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(doc.ID, DIDMethodPrefix))
	assert.Equal(t, doc.ID, doc.Controller)
	assert.NotNil(t, receipt)
	assert.Equal(t, doc.ID, receipt.DID)
	assert.NotEmpty(t, receipt.EntryHash)
}

func TestDIDManager_CreateAgent(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	// Must create principal first.
	_, _, err = dm.CreatePrincipal(ctx, principalKP)
	require.NoError(t, err)

	doc, receipt, err := dm.CreateAgent(ctx, principalKP, agentKP)
	require.NoError(t, err)

	principalDID := crypto.PrincipalDID(principalKP.PublicKey)
	assert.Equal(t, principalDID, doc.Controller)
	assert.True(t, IsAgentDID(doc.ID))
	assert.NotNil(t, receipt)
}

func TestDIDManager_CreateAgent_PrincipalNotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	_, _, err = dm.CreateAgent(ctx, principalKP, agentKP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolving principal DID")
}

func TestDIDManager_Resolve(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	doc, _, err := dm.CreatePrincipal(ctx, kp)
	require.NoError(t, err)

	result, err := dm.Resolve(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, result.Document.ID)
	assert.False(t, result.DocumentMetadata.Deactivated)
}

func TestDIDManager_Resolve_NotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	_, err := dm.Resolve(ctx, "did:aicp:nonexistent")
	require.Error(t, err)
}

func TestDIDManager_Deactivate(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	doc, _, err := dm.CreatePrincipal(ctx, kp)
	require.NoError(t, err)

	err = dm.Deactivate(ctx, doc.ID, kp)
	require.NoError(t, err)

	result, err := dm.Resolve(ctx, doc.ID)
	require.NoError(t, err)
	assert.True(t, result.DocumentMetadata.Deactivated)
}

func TestDIDManager_Deactivate_AlreadyDeactivated(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	doc, _, err := dm.CreatePrincipal(ctx, kp)
	require.NoError(t, err)

	err = dm.Deactivate(ctx, doc.ID, kp)
	require.NoError(t, err)

	err = dm.Deactivate(ctx, doc.ID, kp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already deactivated")
}

func TestDIDManager_Deactivate_WrongSigner(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	otherKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	doc, _, err := dm.CreatePrincipal(ctx, kp)
	require.NoError(t, err)

	err = dm.Deactivate(ctx, doc.ID, otherKP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")
}

func TestDIDManager_ListAgents(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	ctx := context.Background()

	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, principalKP)
	require.NoError(t, err)

	principalDID := crypto.PrincipalDID(principalKP.PublicKey)

	// Create two agents under the principal.
	for i := 0; i < 2; i++ {
		agentKP, err := crypto.GenerateEd25519KeyPair()
		require.NoError(t, err)
		_, _, err = dm.CreateAgent(ctx, principalKP, agentKP)
		require.NoError(t, err)
	}

	agents, err := dm.ListAgents(ctx, principalDID)
	require.NoError(t, err)
	assert.Len(t, agents, 2)
}

// ─── Credential Manager Tests ────────────────────────────────────────────────

func TestCredentialManager_IssueCredential(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	issuerDID := crypto.PrincipalDID(issuerKP.PublicKey)

	subject := map[string]string{
		"id":   "did:aicp:agent123",
		"type": "AICPAgentDelegation",
	}

	cred, err := cm.IssueCredential(ctx, IssueParams{
		CredentialID: "cred-001",
		Types:        []string{"VerifiableCredential", "AICPAgentDelegation"},
		IssuerDID:    issuerDID,
		IssuerKey:    issuerKP,
		Subject:      subject,
	})
	require.NoError(t, err)

	assert.Equal(t, "cred-001", cred.ID)
	assert.Equal(t, issuerDID, cred.Issuer)
	assert.Contains(t, cred.Type, "VerifiableCredential")
	assert.Contains(t, cred.Type, "AICPAgentDelegation")
	assert.NotNil(t, cred.Proof)
	assert.Equal(t, "Ed25519Signature2020", cred.Proof.Type)
	assert.NotEmpty(t, cred.Proof.ProofValue)
}

func TestCredentialManager_IssueCredentialWithExpiration(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	expires := time.Now().Add(24 * time.Hour)
	cred, err := cm.IssueCredential(ctx, IssueParams{
		CredentialID:   "cred-exp",
		Types:          []string{"VerifiableCredential"},
		IssuerDID:      crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:      issuerKP,
		Subject:        map[string]string{"test": "data"},
		ExpirationDate: &expires,
	})
	require.NoError(t, err)
	require.NotNil(t, cred.ExpirationDate)
	assert.WithinDuration(t, expires, *cred.ExpirationDate, time.Second)
}

func TestCredentialManager_GetCredential(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	_, err = cm.IssueCredential(ctx, IssueParams{
		CredentialID: "cred-get-test",
		Types:        []string{"VerifiableCredential"},
		IssuerDID:    crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:    issuerKP,
		Subject:      map[string]string{"id": "test"},
	})
	require.NoError(t, err)

	retrieved, err := cm.GetCredential(ctx, "cred-get-test")
	require.NoError(t, err)
	assert.Equal(t, "cred-get-test", retrieved.ID)
}

func TestCredentialManager_GetCredential_NotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	_, err := cm.GetCredential(ctx, "nonexistent")
	require.Error(t, err)
}

func TestCredentialManager_VerifyCredential_Valid(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	cred, err := cm.IssueCredential(ctx, IssueParams{
		CredentialID: "cred-verify",
		Types:        []string{"VerifiableCredential"},
		IssuerDID:    crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:    issuerKP,
		Subject:      map[string]string{"id": "test"},
	})
	require.NoError(t, err)

	result, err := cm.VerifyCredential(ctx, *cred)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestCredentialManager_VerifyCredential_Expired(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	pastTime := time.Now().Add(-1 * time.Hour)
	cred, err := cm.IssueCredential(ctx, IssueParams{
		CredentialID:   "cred-expired",
		Types:          []string{"VerifiableCredential"},
		IssuerDID:      crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:      issuerKP,
		Subject:        map[string]string{"id": "test"},
		ExpirationDate: &pastTime,
	})
	require.NoError(t, err)

	result, err := cm.VerifyCredential(ctx, *cred)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "expired")
}

func TestCredentialManager_VerifyCredential_NoProof(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	cred := aicp.VerifiableCredential{
		ID:     "cred-no-proof",
		Issuer: "did:aicp:issuer",
		Type:   []string{"VerifiableCredential"},
		Proof:  nil,
	}

	result, err := cm.VerifyCredential(ctx, cred)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "no proof")
}

func TestCredentialManager_RevokeCredential(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	cred, err := cm.IssueCredential(ctx, IssueParams{
		CredentialID: "cred-revoke",
		Types:        []string{"VerifiableCredential"},
		IssuerDID:    crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:    issuerKP,
		Subject:      map[string]string{"id": "test"},
	})
	require.NoError(t, err)

	err = cm.RevokeCredential(ctx, cred.ID, issuerKP)
	require.NoError(t, err)

	// Verify it is now revoked in the backend.
	revoked, err := backend.IsCredentialRevoked(ctx, cred.ID)
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestCredentialManager_RevokeCredential_NotIssuer(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	ctx := context.Background()

	issuerKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	_, _, err = dm.CreatePrincipal(ctx, issuerKP)
	require.NoError(t, err)

	_, err = cm.IssueCredential(ctx, IssueParams{
		CredentialID: "cred-revoke-deny",
		Types:        []string{"VerifiableCredential"},
		IssuerDID:    crypto.PrincipalDID(issuerKP.PublicKey),
		IssuerKey:    issuerKP,
		Subject:      map[string]string{"id": "test"},
	})
	require.NoError(t, err)

	otherKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	err = cm.RevokeCredential(ctx, "cred-revoke-deny", otherKP)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only the issuer")
}

// ─── Delegation Verifier Tests ───────────────────────────────────────────────

func TestDelegationVerifier_VerifyDelegation_Valid(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	dv := NewDelegationVerifier(backend, dm, cm)
	ctx := context.Background()

	principalKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	_, _, err = dm.CreatePrincipal(ctx, principalKP)
	require.NoError(t, err)
	agentDoc, _, err := dm.CreateAgent(ctx, principalKP, agentKP)
	require.NoError(t, err)

	chain, err := dv.VerifyDelegation(ctx, agentDoc.ID)
	require.NoError(t, err)
	assert.True(t, chain.Valid)
	assert.Equal(t, agentDoc.ID, chain.AgentDID)
	principalDID := crypto.PrincipalDID(principalKP.PublicKey)
	assert.Equal(t, principalDID, chain.PrincipalDID)
	assert.Len(t, chain.Chain, 2)
}

func TestDelegationVerifier_VerifyDelegation_AgentNotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	dm := NewDIDManager(backend)
	cm := NewCredentialManager(backend, dm)
	dv := NewDelegationVerifier(backend, dm, cm)
	ctx := context.Background()

	chain, err := dv.VerifyDelegation(ctx, "did:aicp:nonexistent:agent")
	require.NoError(t, err)
	assert.False(t, chain.Valid)
	assert.NotEmpty(t, chain.Errors)
}

// ─── DID Format Integration Test ─────────────────────────────────────────────

func TestDID_FormatConsistency(t *testing.T) {
	// Verify that DIDs produced by the crypto package pass the identity parser.
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	principalDID := crypto.PrincipalDID(kp.PublicKey)
	assert.True(t, IsPrincipalDID(principalDID))
	assert.NoError(t, ValidateDID(principalDID))

	agentKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	agentDID := crypto.AgentDID(kp.PublicKey, agentKP.PublicKey)
	assert.True(t, IsAgentDID(agentDID))
	assert.NoError(t, ValidateDID(agentDID))
}

func TestBuildPrincipalDocument_MultibaseKey(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	did := crypto.PrincipalDID(kp.PublicKey)
	doc := BuildPrincipalDocument(did, kp.PublicKey)

	// The public key stored in the doc should round-trip via multibase decode.
	mb := doc.VerificationMethod[0].PublicKeyMultibase
	decoded, err := crypto.PublicKeyFromMultibase(mb)
	require.NoError(t, err)
	assert.Equal(t, kp.PublicKey, decoded)
}
