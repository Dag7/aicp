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

package onboarding

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Mock LedgerBackend ─────────────────────────────────────────────────────

type mockLedgerBackend struct {
	mu          sync.RWMutex
	dids        map[string]*aicp.DIDResolutionResult
	trustScores map[string]*aicp.TrustScore
	receipts    map[string][]aicp.SignedPaymentReceipt
}

func newMockLedgerBackend() *mockLedgerBackend {
	return &mockLedgerBackend{
		dids:        make(map[string]*aicp.DIDResolutionResult),
		trustScores: make(map[string]*aicp.TrustScore),
		receipts:    make(map[string][]aicp.SignedPaymentReceipt),
	}
}

func (m *mockLedgerBackend) seedDID(did string, doc *aicp.DIDDocument, created time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dids[did] = &aicp.DIDResolutionResult{
		Document: doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   created,
			Updated:   created,
			VersionID: 1,
		},
	}
}

func (m *mockLedgerBackend) seedTrustScore(did string, ts aicp.TrustScore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trustScores[did] = &ts
}

// Interface implementations used by TrustComputer.

func (m *mockLedgerBackend) ResolveDID(_ context.Context, did string) (*aicp.DIDResolutionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.dids[did]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	return r, nil
}

func (m *mockLedgerBackend) GetTrustScore(_ context.Context, did string) (*aicp.TrustScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ts, ok := m.trustScores[did]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	return ts, nil
}

func (m *mockLedgerBackend) UpdateTrustScore(_ context.Context, ts aicp.TrustScore) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trustScores[ts.DID] = &ts
	return nil
}

func (m *mockLedgerBackend) ListReceiptsByAgent(_ context.Context, did string, _ aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.SignedPaymentReceipt], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := m.receipts[did]
	return &aicp.PaginatedResult[aicp.SignedPaymentReceipt]{
		Items:   items,
		HasMore: false,
	}, nil
}

// Remaining LedgerBackend stubs.
func (m *mockLedgerBackend) Initialize(context.Context) error                 { return nil }
func (m *mockLedgerBackend) Close() error                                     { return nil }
func (m *mockLedgerBackend) Healthy(context.Context) error                    { return nil }
func (m *mockLedgerBackend) AnchorDID(context.Context, string, aicp.DIDDocument, aicp.Proof) (*aicp.AnchorReceipt, error) {
	return nil, nil
}
func (m *mockLedgerBackend) UpdateDIDDocument(context.Context, string, aicp.DIDDocument, aicp.Proof) (*aicp.AnchorReceipt, error) {
	return nil, nil
}
func (m *mockLedgerBackend) DeactivateDID(context.Context, string, aicp.Proof) error { return nil }
func (m *mockLedgerBackend) ListAgentDIDs(context.Context, string) ([]string, error) { return nil, nil }
func (m *mockLedgerBackend) StoreCredential(context.Context, aicp.VerifiableCredential) error {
	return nil
}
func (m *mockLedgerBackend) GetCredential(context.Context, string) (*aicp.VerifiableCredential, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) RevokeCredential(context.Context, string, aicp.Proof) error { return nil }
func (m *mockLedgerBackend) IsCredentialRevoked(context.Context, string) (bool, error) {
	return false, nil
}
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
func (m *mockLedgerBackend) IsTokenUsed(context.Context, string) (bool, error)  { return false, nil }
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
func (m *mockLedgerBackend) StoreANSRecord(context.Context, aicp.ANSRecord) error    { return nil }
func (m *mockLedgerBackend) GetANSRecord(context.Context, string) (*aicp.ANSRecord, error) {
	return nil, aicp.ErrNotFound
}
func (m *mockLedgerBackend) SearchANSRecords(context.Context, string, aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	return &aicp.PaginatedResult[aicp.ANSRecord]{}, nil
}
func (m *mockLedgerBackend) DeleteANSRecord(context.Context, string) error         { return nil }
func (m *mockLedgerBackend) RegisterValidator(context.Context, aicp.Validator) error { return nil }
func (m *mockLedgerBackend) RemoveValidator(context.Context, string) error           { return nil }
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

// ─── Helper ──────────────────────────────────────────────────────────────────

func newTestState(phase aicp.OnboardingPhase) *aicp.OnboardingState {
	now := time.Now().UTC()
	return &aicp.OnboardingState{
		SessionID:  "session-001",
		Phase:      phase,
		AgentDID:   "did:aicp:agent123",
		ServiceDID: "did:aicp:service456",
		StartedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  now.Add(30 * time.Minute),
	}
}

// ─── State Machine Tests ─────────────────────────────────────────────────────

func TestFlow_HappyPath(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)
	flow := NewFlow(state, 30*time.Minute)

	// initiated -> identity_presented
	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.NoError(t, err)
	assert.Equal(t, aicp.PhaseIdentityPresented, flow.State().Phase)

	// identity_presented -> capabilities_negotiated
	err = flow.Transition(ctx, aicp.PhaseCapabilitiesNegotiated)
	require.NoError(t, err)
	assert.Equal(t, aicp.PhaseCapabilitiesNegotiated, flow.State().Phase)

	// capabilities_negotiated -> payment_authorized
	err = flow.Transition(ctx, aicp.PhasePaymentAuthorized)
	require.NoError(t, err)
	assert.Equal(t, aicp.PhasePaymentAuthorized, flow.State().Phase)

	// payment_authorized -> provisioned
	err = flow.Transition(ctx, aicp.PhaseProvisioned)
	require.NoError(t, err)
	assert.Equal(t, aicp.PhaseProvisioned, flow.State().Phase)

	assert.True(t, flow.IsComplete())
	assert.False(t, flow.IsFailed())
}

func TestFlow_InvalidTransition_SkipPhase(t *testing.T) {
	tests := []struct {
		name string
		from aicp.OnboardingPhase
		to   aicp.OnboardingPhase
	}{
		{"skip identity -> capabilities", aicp.PhaseInitiated, aicp.PhaseCapabilitiesNegotiated},
		{"skip identity -> payment", aicp.PhaseInitiated, aicp.PhasePaymentAuthorized},
		{"skip identity -> provisioned", aicp.PhaseInitiated, aicp.PhaseProvisioned},
		{"skip capabilities -> provisioned", aicp.PhaseIdentityPresented, aicp.PhasePaymentAuthorized},
		{"skip payment -> provisioned from capabilities", aicp.PhaseCapabilitiesNegotiated, aicp.PhaseProvisioned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			state := newTestState(tt.from)
			flow := NewFlow(state, 30*time.Minute)

			err := flow.Transition(ctx, tt.to)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid transition")
		})
	}
}

func TestFlow_TransitionToFailed_FromAnyPhase(t *testing.T) {
	phases := []aicp.OnboardingPhase{
		aicp.PhaseInitiated,
		aicp.PhaseIdentityPresented,
		aicp.PhaseCapabilitiesNegotiated,
		aicp.PhasePaymentAuthorized,
	}

	for _, phase := range phases {
		t.Run(string(phase)+"_to_failed", func(t *testing.T) {
			ctx := context.Background()
			state := newTestState(phase)
			flow := NewFlow(state, 30*time.Minute)

			err := flow.Transition(ctx, aicp.PhaseFailed)
			require.NoError(t, err)
			assert.Equal(t, aicp.PhaseFailed, flow.State().Phase)
			assert.True(t, flow.IsFailed())
		})
	}
}

func TestFlow_Fail(t *testing.T) {
	state := newTestState(aicp.PhaseIdentityPresented)
	flow := NewFlow(state, 30*time.Minute)

	flow.Fail("identity verification failed")

	assert.True(t, flow.IsFailed())
	assert.Equal(t, "identity verification failed", flow.State().FailedReason)
}

func TestFlow_ExpiredSession(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)
	state.ExpiresAt = time.Now().Add(-1 * time.Second) // Already expired.
	flow := NewFlow(state, 30*time.Minute)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
	assert.Equal(t, aicp.PhaseFailed, flow.State().Phase)
}

func TestFlow_IsExpired(t *testing.T) {
	t.Run("not expired", func(t *testing.T) {
		state := newTestState(aicp.PhaseInitiated)
		flow := NewFlow(state, 30*time.Minute)
		assert.False(t, flow.IsExpired())
	})

	t.Run("expired", func(t *testing.T) {
		state := newTestState(aicp.PhaseInitiated)
		state.ExpiresAt = time.Now().Add(-1 * time.Second)
		flow := NewFlow(state, 30*time.Minute)
		assert.True(t, flow.IsExpired())
	})
}

func TestFlow_IsComplete(t *testing.T) {
	t.Run("not complete", func(t *testing.T) {
		state := newTestState(aicp.PhaseInitiated)
		flow := NewFlow(state, 30*time.Minute)
		assert.False(t, flow.IsComplete())
	})

	t.Run("complete", func(t *testing.T) {
		state := newTestState(aicp.PhaseProvisioned)
		flow := NewFlow(state, 30*time.Minute)
		assert.True(t, flow.IsComplete())
	})
}

func TestFlow_TransitionUpdatesTimestamp(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)
	originalUpdatedAt := state.UpdatedAt
	flow := NewFlow(state, 30*time.Minute)

	// Small sleep to ensure timestamp differs.
	time.Sleep(1 * time.Millisecond)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.NoError(t, err)
	assert.True(t, flow.State().UpdatedAt.After(originalUpdatedAt) || flow.State().UpdatedAt.Equal(originalUpdatedAt))
}

// ─── Guard Tests ─────────────────────────────────────────────────────────────

func TestFlow_GuardBlocks(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)

	blockingGuard := func(_ context.Context, _, _ aicp.OnboardingPhase, _ *aicp.OnboardingState) error {
		return fmt.Errorf("guard: blocked by policy")
	}

	flow := NewFlow(state, 30*time.Minute, blockingGuard)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guard failed")
	// Phase should not have changed.
	assert.Equal(t, aicp.PhaseInitiated, flow.State().Phase)
}

func TestFlow_GuardPasses(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)

	passingGuard := func(_ context.Context, _, _ aicp.OnboardingPhase, _ *aicp.OnboardingState) error {
		return nil
	}

	flow := NewFlow(state, 30*time.Minute, passingGuard)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.NoError(t, err)
	assert.Equal(t, aicp.PhaseIdentityPresented, flow.State().Phase)
}

func TestMinTrustScoreGuard_Allows(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)

	getScore := func(_ context.Context, _ string) (int, error) {
		return 500, nil
	}

	guard := MinTrustScoreGuard(100, getScore)
	flow := NewFlow(state, 30*time.Minute, guard)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.NoError(t, err)
}

func TestMinTrustScoreGuard_Blocks(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseInitiated)

	getScore := func(_ context.Context, _ string) (int, error) {
		return 50, nil
	}

	guard := MinTrustScoreGuard(100, getScore)
	flow := NewFlow(state, 30*time.Minute, guard)

	err := flow.Transition(ctx, aicp.PhaseIdentityPresented)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trust score")
}

func TestMinTrustScoreGuard_OnlyChecksIdentityPhase(t *testing.T) {
	ctx := context.Background()
	state := newTestState(aicp.PhaseIdentityPresented)

	getScore := func(_ context.Context, _ string) (int, error) {
		return 0, nil // Would fail if checked.
	}

	guard := MinTrustScoreGuard(100, getScore)
	flow := NewFlow(state, 30*time.Minute, guard)

	// Transition to capabilities_negotiated should not check trust score.
	err := flow.Transition(ctx, aicp.PhaseCapabilitiesNegotiated)
	require.NoError(t, err)
}

// ─── Trust Computer Tests ────────────────────────────────────────────────────

func TestTrustComputer_Compute_NewDID(t *testing.T) {
	backend := newMockLedgerBackend()

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	did := crypto.PrincipalDID(kp.PublicKey)

	doc := &aicp.DIDDocument{
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{ID: did + "#key-1", Type: "Ed25519VerificationKey2020"},
		},
	}
	backend.seedDID(did, doc, time.Now().Add(-30*24*time.Hour)) // 30 days old

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.Compute(ctx, did)
	require.NoError(t, err)

	assert.Equal(t, did, score.DID)
	assert.Greater(t, score.Score, 0)
	assert.LessOrEqual(t, score.Score, 1000)
	// Identity: 100 (base) + 100 (has VM) = 200.
	assert.Equal(t, 200, score.Components.Identity)
	// Account age: 30 days out of 365 -> (30 * 150) / 365 = 12.
	assert.Greater(t, score.Components.AccountAge, 0)
	assert.Less(t, score.Components.AccountAge, 150)
}

func TestTrustComputer_Compute_MaxIdentityWithServices(t *testing.T) {
	backend := newMockLedgerBackend()
	did := "did:aicp:maxid"

	doc := &aicp.DIDDocument{
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{ID: did + "#key-1"},
		},
		Service: []aicp.ServiceEndpoint{
			{ID: "#api", Type: "AgentService"},
		},
	}
	backend.seedDID(did, doc, time.Now().Add(-365*24*time.Hour))

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.Compute(ctx, did)
	require.NoError(t, err)
	// 100 (base) + 100 (VM) + 50 (service) = 250, capped at 250.
	assert.Equal(t, 250, score.Components.Identity)
	// 365 days old -> max 150.
	assert.Equal(t, 150, score.Components.AccountAge)
}

func TestTrustComputer_Compute_ScoreBounds(t *testing.T) {
	backend := newMockLedgerBackend()
	did := "did:aicp:bounds"
	doc := &aicp.DIDDocument{ID: did, Controller: did}
	backend.seedDID(did, doc, time.Now())

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.Compute(ctx, did)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, score.Score, 0)
	assert.LessOrEqual(t, score.Score, 1000)
}

func TestTrustComputer_Compute_DeactivatedDID(t *testing.T) {
	backend := newMockLedgerBackend()
	did := "did:aicp:deactivated"
	doc := &aicp.DIDDocument{ID: did, Controller: did}
	backend.seedDID(did, doc, time.Now())
	backend.dids[did].DocumentMetadata.Deactivated = true

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.Compute(ctx, did)
	require.NoError(t, err)
	assert.Equal(t, 0, score.Components.Identity)
}

func TestTrustComputer_Compute_PreservesExistingComponents(t *testing.T) {
	backend := newMockLedgerBackend()
	did := "did:aicp:existing"
	doc := &aicp.DIDDocument{
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{ID: did + "#key-1"},
		},
	}
	backend.seedDID(did, doc, time.Now().Add(-10*24*time.Hour))

	// Seed an existing trust score with endorsement and credential data.
	backend.seedTrustScore(did, aicp.TrustScore{
		DID:   did,
		Score: 500,
		Components: aicp.TrustScoreBreakdown{
			Endorsements: 75,
			Credentials:  50,
		},
	})

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.Compute(ctx, did)
	require.NoError(t, err)
	// Endorsements and credentials should carry forward from existing score.
	assert.Equal(t, 75, score.Components.Endorsements)
	assert.Equal(t, 50, score.Components.Credentials)
}

func TestTrustComputer_GetScore_NotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.GetScore(ctx, "did:aicp:nonexistent")
	require.NoError(t, err)
	assert.Equal(t, 0, score) // Default for missing score.
}

func TestTrustComputer_GetScore_Exists(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.seedTrustScore("did:aicp:scored", aicp.TrustScore{
		DID:   "did:aicp:scored",
		Score: 750,
	})

	tc := NewTrustComputer(backend)
	ctx := context.Background()

	score, err := tc.GetScore(ctx, "did:aicp:scored")
	require.NoError(t, err)
	assert.Equal(t, 750, score)
}
