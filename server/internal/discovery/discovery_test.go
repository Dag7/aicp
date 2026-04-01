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

package discovery

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Mock LedgerBackend ─────────────────────────────────────────────────────

type mockLedgerBackend struct {
	mu          sync.RWMutex
	ansRecords  map[string]*aicp.ANSRecord
	trustScores map[string]*aicp.TrustScore
}

func newMockLedgerBackend() *mockLedgerBackend {
	return &mockLedgerBackend{
		ansRecords:  make(map[string]*aicp.ANSRecord),
		trustScores: make(map[string]*aicp.TrustScore),
	}
}

func (m *mockLedgerBackend) StoreANSRecord(_ context.Context, record aicp.ANSRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ansRecords[record.Service.DID] = &record
	return nil
}

func (m *mockLedgerBackend) GetANSRecord(_ context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.ansRecords[serviceDID]
	if !ok {
		return nil, aicp.ErrNotFound
	}
	return rec, nil
}

func (m *mockLedgerBackend) DeleteANSRecord(_ context.Context, serviceDID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.ansRecords, serviceDID)
	return nil
}

func (m *mockLedgerBackend) SearchANSRecords(_ context.Context, _ string, _ aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	return &aicp.PaginatedResult[aicp.ANSRecord]{}, nil
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

// Remaining stubs.
func (m *mockLedgerBackend) Initialize(context.Context) error { return nil }
func (m *mockLedgerBackend) Close() error                     { return nil }
func (m *mockLedgerBackend) Healthy(context.Context) error    { return nil }
func (m *mockLedgerBackend) AnchorDID(context.Context, string, aicp.DIDDocument, aicp.Proof) (*aicp.AnchorReceipt, error) {
	return nil, nil
}
func (m *mockLedgerBackend) ResolveDID(context.Context, string) (*aicp.DIDResolutionResult, error) {
	return nil, aicp.ErrNotFound
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
func (m *mockLedgerBackend) UpdateTrustScore(context.Context, aicp.TrustScore) error { return nil }
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
func (m *mockLedgerBackend) CheckAndRecordSpending(context.Context, string, aicp.MoneyAmount, string, string) (bool, error) {
	return true, nil
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func validANSRecord(did string) aicp.ANSRecord {
	return aicp.ANSRecord{
		AICPVersion: "1.0",
		Service: aicp.ANSServiceInfo{
			Name:     "Test Service",
			DID:      did,
			Category: []string{"compute"},
		},
		Onboarding: aicp.ANSOnboardingInfo{
			Flows:               []string{"standard"},
			RequiredCredentials: []string{"AICPAgentDelegation"},
		},
		Authentication: aicp.ANSAuthInfo{
			DIDMethods:    []string{"did:aicp"},
			AuthProtocols: []string{"DID-Auth"},
		},
	}
}

// ─── ANS Record Validation Tests ──────────────────────────────────────────────

func TestValidateANSRecord_Valid(t *testing.T) {
	record := validANSRecord("did:aicp:svc1")
	err := validateANSRecord(record)
	assert.NoError(t, err)
}

func TestValidateANSRecord_TableDriven(t *testing.T) {
	base := validANSRecord("did:aicp:svc1")

	tests := []struct {
		name   string
		mutate func(r *aicp.ANSRecord)
		want   string
	}{
		{
			"missing version",
			func(r *aicp.ANSRecord) { r.AICPVersion = "" },
			"aicp_version is required",
		},
		{
			"missing service name",
			func(r *aicp.ANSRecord) { r.Service.Name = "" },
			"service name is required",
		},
		{
			"missing service DID",
			func(r *aicp.ANSRecord) { r.Service.DID = "" },
			"service DID is required",
		},
		{
			"no categories",
			func(r *aicp.ANSRecord) { r.Service.Category = nil },
			"at least one service category",
		},
		{
			"no onboarding flows",
			func(r *aicp.ANSRecord) { r.Onboarding.Flows = nil },
			"at least one onboarding flow",
		},
		{
			"no DID methods",
			func(r *aicp.ANSRecord) { r.Authentication.DIDMethods = nil },
			"at least one DID method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := base // Copy.
			tt.mutate(&r)
			err := validateANSRecord(r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// ─── ANS Manager Tests ───────────────────────────────────────────────────────

func TestANSManager_Register(t *testing.T) {
	backend := newMockLedgerBackend()
	am := NewANSManager(backend)
	ctx := context.Background()

	record := validANSRecord("did:aicp:svc1")
	err := am.Register(ctx, record)
	require.NoError(t, err)

	stored, err := am.Get(ctx, "did:aicp:svc1")
	require.NoError(t, err)
	assert.Equal(t, "Test Service", stored.Service.Name)
}

func TestANSManager_Register_Invalid(t *testing.T) {
	backend := newMockLedgerBackend()
	am := NewANSManager(backend)
	ctx := context.Background()

	err := am.Register(ctx, aicp.ANSRecord{}) // Empty record.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ANS record")
}

func TestANSManager_Get_NotFound(t *testing.T) {
	backend := newMockLedgerBackend()
	am := NewANSManager(backend)
	ctx := context.Background()

	_, err := am.Get(ctx, "did:aicp:nonexistent")
	require.Error(t, err)
}

func TestANSManager_Delete(t *testing.T) {
	backend := newMockLedgerBackend()
	am := NewANSManager(backend)
	ctx := context.Background()

	record := validANSRecord("did:aicp:svc1")
	err := am.Register(ctx, record)
	require.NoError(t, err)

	err = am.Delete(ctx, "did:aicp:svc1")
	require.NoError(t, err)

	_, err = am.Get(ctx, "did:aicp:svc1")
	require.Error(t, err)
}

// ─── ANS Record Builder Tests ────────────────────────────────────────────────

func TestANSRecordBuilder_Defaults(t *testing.T) {
	record := NewANSRecordBuilder("did:aicp:svc", "My Service").Build()

	assert.Equal(t, aicp.ProtocolVersion, record.AICPVersion)
	assert.Equal(t, "did:aicp:svc", record.Service.DID)
	assert.Equal(t, "My Service", record.Service.Name)
	assert.Contains(t, record.Authentication.DIDMethods, "did:aicp")
	assert.Contains(t, record.Authentication.AuthProtocols, "DID-Auth")
	assert.Contains(t, record.Onboarding.Flows, "standard")
}

func TestANSRecordBuilder_FullBuild(t *testing.T) {
	record := NewANSRecordBuilder("did:aicp:svc", "Full Service").
		Description("A test service").
		Categories("compute", "storage").
		DiscoveryTags("ml", "inference").
		Homepage("https://example.com").
		RequiredCredentials("AICPAgentDelegation").
		AcceptedPayments("stripe", "usdc").
		MinTrustScore(200).
		Pricing("tiered", "USD", []aicp.ANSPricingTier{
			{Name: "basic", Rate: "0.01", Description: "Basic tier"},
		}).
		Trust([]string{"aicp-consortium"}, "2024-01-01").
		Endpoints("/onboard", "/api/v1", "/ws", "/health").
		Build()

	assert.Equal(t, "A test service", record.Service.Description)
	assert.Equal(t, []string{"compute", "storage"}, record.Service.Category)
	assert.Equal(t, []string{"ml", "inference"}, record.DiscoveryTags)
	assert.Equal(t, "https://example.com", record.Service.Homepage)
	assert.Equal(t, []string{"AICPAgentDelegation"}, record.Onboarding.RequiredCredentials)
	assert.Equal(t, []string{"stripe", "usdc"}, record.Onboarding.AcceptedPayments)
	assert.Equal(t, 200, record.Onboarding.MinimumTrustScore)
	require.NotNil(t, record.Pricing)
	assert.Equal(t, "tiered", record.Pricing.Model)
	assert.Equal(t, "USD", record.Pricing.Currency)
	assert.Len(t, record.Pricing.Tiers, 1)
	require.NotNil(t, record.Trust)
	assert.Contains(t, record.Trust.VerifiedBy, "aicp-consortium")
	assert.Equal(t, "2024-01-01", record.Trust.EstablishedSince)
	require.NotNil(t, record.Endpoints)
	assert.Equal(t, "/onboard", record.Endpoints.Onboard)
	assert.Equal(t, "/api/v1", record.Endpoints.APIBase)
	assert.Equal(t, "/ws", record.Endpoints.WebSocket)
	assert.Equal(t, "/health", record.Endpoints.Health)
}

func TestANSRecordBuilder_ValidationPasses(t *testing.T) {
	// Builder should produce records that pass validation when categories are set.
	record := NewANSRecordBuilder("did:aicp:svc", "Valid").
		Categories("compute").
		Build()

	err := validateANSRecord(record)
	assert.NoError(t, err)
}

// ─── Ranking Tests ───────────────────────────────────────────────────────────

func TestRankingService_RankResults(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.trustScores["did:aicp:svc1"] = &aicp.TrustScore{DID: "did:aicp:svc1", Score: 800}
	backend.trustScores["did:aicp:svc2"] = &aicp.TrustScore{DID: "did:aicp:svc2", Score: 200}

	rs := NewRankingService(backend)
	ctx := context.Background()

	records := []aicp.ANSRecord{
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:svc2", Name: "Low Trust"}},
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:svc1", Name: "High Trust"}},
	}

	ranked, err := rs.RankResults(ctx, records)
	require.NoError(t, err)
	require.Len(t, ranked, 2)

	// After ranking, order may change based on combined text relevance + trust.
	// svc2 is first in input (higher text relevance), svc1 has higher trust.
	for _, r := range ranked {
		assert.Greater(t, r.Rank, 0.0)
		assert.LessOrEqual(t, r.Rank, 1.0)
	}
}

func TestRankingService_RankResults_Empty(t *testing.T) {
	backend := newMockLedgerBackend()
	rs := NewRankingService(backend)
	ctx := context.Background()

	ranked, err := rs.RankResults(ctx, []aicp.ANSRecord{})
	require.NoError(t, err)
	assert.Empty(t, ranked)
}

func TestRankingService_RankResults_NoTrustScores(t *testing.T) {
	backend := newMockLedgerBackend()
	rs := NewRankingService(backend)
	ctx := context.Background()

	records := []aicp.ANSRecord{
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:a", Name: "A"}},
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:b", Name: "B"}},
	}

	ranked, err := rs.RankResults(ctx, records)
	require.NoError(t, err)
	assert.Len(t, ranked, 2)
	// Without trust scores, text relevance dominates -- first item keeps rank.
	assert.GreaterOrEqual(t, ranked[0].Rank, ranked[1].Rank)
}

// ─── Filter Tests ────────────────────────────────────────────────────────────

func TestFilterByMinTrustScore(t *testing.T) {
	results := []RankedResult{
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "High"}}, TrustScore: 800},
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "Mid"}}, TrustScore: 400},
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "Low"}}, TrustScore: 100},
	}

	filtered := FilterByMinTrustScore(results, 300)
	assert.Len(t, filtered, 2)
	assert.Equal(t, "High", filtered[0].Record.Service.Name)
	assert.Equal(t, "Mid", filtered[1].Record.Service.Name)
}

func TestFilterByMinTrustScore_AllFiltered(t *testing.T) {
	results := []RankedResult{
		{TrustScore: 10},
		{TrustScore: 20},
	}

	filtered := FilterByMinTrustScore(results, 100)
	assert.Empty(t, filtered)
}

func TestFilterByMinTrustScore_NoneFiltered(t *testing.T) {
	results := []RankedResult{
		{TrustScore: 500},
		{TrustScore: 600},
	}

	filtered := FilterByMinTrustScore(results, 100)
	assert.Len(t, filtered, 2)
}

func TestFilterByCategory(t *testing.T) {
	results := []RankedResult{
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "A", Category: []string{"compute", "ml"}}}},
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "B", Category: []string{"storage"}}}},
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "C", Category: []string{"compute"}}}},
	}

	filtered := FilterByCategory(results, []string{"compute"})
	assert.Len(t, filtered, 2)
	assert.Equal(t, "A", filtered[0].Record.Service.Name)
	assert.Equal(t, "C", filtered[1].Record.Service.Name)
}

func TestFilterByCategory_EmptyCategories(t *testing.T) {
	results := []RankedResult{
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "A"}}},
	}

	filtered := FilterByCategory(results, nil)
	assert.Len(t, filtered, 1, "empty filter should return all results")
}

func TestFilterByCategory_NoMatch(t *testing.T) {
	results := []RankedResult{
		{Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Category: []string{"storage"}}}},
	}

	filtered := FilterByCategory(results, []string{"compute"})
	assert.Empty(t, filtered)
}

// ─── FormatSearchResponse Tests ──────────────────────────────────────────────

func TestFormatSearchResponse_BasicPagination(t *testing.T) {
	results := make([]RankedResult, 5)
	for i := range results {
		results[i] = RankedResult{
			Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "Svc"}},
		}
	}

	page := FormatSearchResponse(results, 2, 0)
	assert.Len(t, page.Items, 2)
	assert.True(t, page.HasMore)
	assert.Equal(t, "2", page.NextCursor)
}

func TestFormatSearchResponse_LastPage(t *testing.T) {
	results := make([]RankedResult, 5)
	for i := range results {
		results[i] = RankedResult{
			Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "Svc"}},
		}
	}

	page := FormatSearchResponse(results, 3, 3)
	assert.Len(t, page.Items, 2)
	assert.False(t, page.HasMore)
	assert.Empty(t, page.NextCursor)
}

func TestFormatSearchResponse_DefaultLimit(t *testing.T) {
	results := make([]RankedResult, 5)
	for i := range results {
		results[i] = RankedResult{
			Record: aicp.ANSRecord{Service: aicp.ANSServiceInfo{Name: "Svc"}},
		}
	}

	page := FormatSearchResponse(results, 0, 0) // limit=0 should default to 20.
	assert.Len(t, page.Items, 5)
	assert.False(t, page.HasMore)
}

func TestFormatSearchResponse_Empty(t *testing.T) {
	page := FormatSearchResponse([]RankedResult{}, 10, 0)
	assert.Empty(t, page.Items)
	assert.False(t, page.HasMore)
}
