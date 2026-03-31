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

package commerce

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/commerce/adapter"
	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Mock LedgerBackend ─────────────────────────────────────────────────────

type mockLedgerBackend struct {
	mu             sync.RWMutex
	usedTokens     map[string]bool
	dailySpending  map[string]string // "did:currency" -> amount
	monthlySpending map[string]string
}

func newMockLedgerBackend() *mockLedgerBackend {
	return &mockLedgerBackend{
		usedTokens:      make(map[string]bool),
		dailySpending:   make(map[string]string),
		monthlySpending: make(map[string]string),
	}
}

func (m *mockLedgerBackend) IsTokenUsed(_ context.Context, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.usedTokens[jti], nil
}

func (m *mockLedgerBackend) MarkTokenUsed(_ context.Context, jti string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usedTokens[jti] = true
	return nil
}

func (m *mockLedgerBackend) GetDailySpending(_ context.Context, did string, currency string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := did + ":" + currency
	v, ok := m.dailySpending[key]
	if !ok {
		return "0", nil
	}
	return v, nil
}

func (m *mockLedgerBackend) GetMonthlySpending(_ context.Context, did string, currency string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := did + ":" + currency
	v, ok := m.monthlySpending[key]
	if !ok {
		return "0", nil
	}
	return v, nil
}

func (m *mockLedgerBackend) RecordSpending(_ context.Context, _ string, _ aicp.MoneyAmount) error {
	return nil
}

// Remaining stubs to satisfy the LedgerBackend interface.
func (m *mockLedgerBackend) Initialize(context.Context) error                 { return nil }
func (m *mockLedgerBackend) Close() error                                     { return nil }
func (m *mockLedgerBackend) Healthy(context.Context) error                    { return nil }
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
func (m *mockLedgerBackend) DeleteANSRecord(context.Context, string) error         { return nil }
func (m *mockLedgerBackend) RegisterValidator(context.Context, aicp.Validator) error { return nil }
func (m *mockLedgerBackend) RemoveValidator(context.Context, string) error           { return nil }
func (m *mockLedgerBackend) ListValidators(context.Context) ([]aicp.Validator, error) {
	return nil, nil
}
func (m *mockLedgerBackend) GetValidator(context.Context, string) (*aicp.Validator, error) {
	return nil, aicp.ErrNotFound
}

// ─── Token Issuer Tests ──────────────────────────────────────────────────────

func TestTokenIssuer_Issue(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:        "did:aicp:agent123",
		ServiceDID:      "did:aicp:service456",
		Amount:          aicp.MoneyAmount{Value: "25.00", Currency: "USD"},
		Purpose:         string(aicp.PurposeOnboarding),
		ServiceCategory: "compute",
	})
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Token)
	assert.True(t, strings.HasPrefix(resp.TokenID, "pt_"))
	assert.True(t, resp.ExpiresAt.After(time.Now()))
	// JWS compact serialization has 3 dot-separated parts.
	parts := strings.Split(resp.Token, ".")
	assert.Len(t, parts, 3)
}

func TestTokenIssuer_Issue_DifferentTokenIDs(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	req := aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	}

	resp1, err := issuer.Issue(ctx, req)
	require.NoError(t, err)
	resp2, err := issuer.Issue(ctx, req)
	require.NoError(t, err)

	assert.NotEqual(t, resp1.TokenID, resp2.TokenID, "each token must have a unique JTI")
	assert.NotEqual(t, resp1.Token, resp2.Token)
}

func TestTokenIssuer_Issue_MaxAmount(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	maxAmt := aicp.MoneyAmount{Value: "100.00", Currency: "USD"}
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "50.00", Currency: "USD"},
		MaxAmount:  &maxAmt,
		Purpose:    string(aicp.PurposeOneTime),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

// ─── Token Validator Tests ───────────────────────────────────────────────────

func TestTokenValidator_Validate_ValidToken(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	backend := newMockLedgerBackend()
	validator := NewTokenValidator(backend)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	})
	require.NoError(t, err)

	pubKeyMultibase := crypto.PublicKeyMultibase(kp.PublicKey)
	result, err := validator.Validate(ctx, resp.Token, pubKeyMultibase)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.NotNil(t, result.Claims)
	assert.Equal(t, "did:aicp:agent", result.Claims.Subject)
	assert.Equal(t, "did:aicp:svc", result.Claims.Audience)
	assert.Equal(t, "10.00", result.Claims.PaymentDetails.Amount.Value)
}

func TestTokenValidator_Validate_WrongKey(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)
	wrongKP, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	backend := newMockLedgerBackend()
	validator := NewTokenValidator(backend)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	})
	require.NoError(t, err)

	wrongMultibase := crypto.PublicKeyMultibase(wrongKP.PublicKey)
	result, err := validator.Validate(ctx, resp.Token, wrongMultibase)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "signature verification failed")
}

func TestTokenValidator_Validate_ExpiredToken(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	// Issue with 0 TTL to make it already expired.
	issuer, err := NewTokenIssuer(kp, -1*time.Second)
	require.NoError(t, err)

	backend := newMockLedgerBackend()
	validator := NewTokenValidator(backend)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	})
	require.NoError(t, err)

	pubKeyMultibase := crypto.PublicKeyMultibase(kp.PublicKey)
	result, err := validator.Validate(ctx, resp.Token, pubKeyMultibase)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "expired")
}

func TestTokenValidator_Validate_ReplayDetection(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	backend := newMockLedgerBackend()
	validator := NewTokenValidator(backend)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	})
	require.NoError(t, err)

	pubKeyMultibase := crypto.PublicKeyMultibase(kp.PublicKey)

	// First validation should succeed.
	result, err := validator.Validate(ctx, resp.Token, pubKeyMultibase)
	require.NoError(t, err)
	assert.True(t, result.Valid)

	// Mark the token as used.
	err = validator.MarkUsed(ctx, result.Claims.JTI)
	require.NoError(t, err)

	// Second validation should detect replay.
	result2, err := validator.Validate(ctx, resp.Token, pubKeyMultibase)
	require.NoError(t, err)
	assert.False(t, result2.Valid)
	assert.Contains(t, result2.Errors[0], "replay")
}

func TestTokenValidator_Validate_InvalidMultibase(t *testing.T) {
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	issuer, err := NewTokenIssuer(kp, 15*time.Minute)
	require.NoError(t, err)

	backend := newMockLedgerBackend()
	validator := NewTokenValidator(backend)

	ctx := context.Background()
	resp, err := issuer.Issue(ctx, aicp.PaymentTokenRequest{
		AgentDID:   "did:aicp:agent",
		ServiceDID: "did:aicp:svc",
		Amount:     aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		Purpose:    string(aicp.PurposeUsage),
	})
	require.NoError(t, err)

	result, err := validator.Validate(ctx, resp.Token, "invalid-not-multibase")
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors[0], "invalid issuer key")
}

// ─── Adapter Registry Tests ──────────────────────────────────────────────────

// mockAdapter implements adapter.PaymentAdapter for testing the registry.
type mockAdapter struct {
	name       string
	currencies []string
	healthy    bool
}

func (m *mockAdapter) Name() string                 { return m.name }
func (m *mockAdapter) SupportedCurrencies() []string { return m.currencies }
func (m *mockAdapter) Initialize(_ context.Context, _ map[string]any) error {
	return nil
}
func (m *mockAdapter) Healthy(_ context.Context) error {
	if !m.healthy {
		return assert.AnError
	}
	return nil
}
func (m *mockAdapter) CreatePaymentIntent(_ context.Context, _ adapter.CreateIntentParams) (*adapter.PaymentIntent, error) {
	return &adapter.PaymentIntent{IntentID: "intent-mock", Status: "created"}, nil
}
func (m *mockAdapter) CapturePayment(_ context.Context, _ string) (*adapter.PaymentCapture, error) {
	return &adapter.PaymentCapture{Status: "captured"}, nil
}
func (m *mockAdapter) RefundPayment(_ context.Context, _ string, _ *adapter.RefundParams) (*adapter.PaymentRefund, error) {
	return &adapter.PaymentRefund{Status: "refunded"}, nil
}
func (m *mockAdapter) GetPaymentStatus(_ context.Context, _ string) (*adapter.PaymentStatus, error) {
	return &adapter.PaymentStatus{Status: "settled"}, nil
}
func (m *mockAdapter) ValidateWebhook(_ context.Context, _ []byte, _ string) (*adapter.WebhookEvent, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := adapter.NewRegistry()

	reg.Register(&mockAdapter{name: "stripe", currencies: []string{"USD"}, healthy: true})

	a, err := reg.Get("stripe")
	require.NoError(t, err)
	assert.Equal(t, "stripe", a.Name())
}

func TestRegistry_Get_NotRegistered(t *testing.T) {
	reg := adapter.NewRegistry()

	_, err := reg.Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")
}

func TestRegistry_List(t *testing.T) {
	reg := adapter.NewRegistry()

	reg.Register(&mockAdapter{name: "stripe", healthy: true})
	reg.Register(&mockAdapter{name: "paypal", healthy: true})
	reg.Register(&mockAdapter{name: "usdc", healthy: true})

	names := reg.List()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "stripe")
	assert.Contains(t, names, "paypal")
	assert.Contains(t, names, "usdc")
}

func TestRegistry_List_Empty(t *testing.T) {
	reg := adapter.NewRegistry()
	names := reg.List()
	assert.Empty(t, names)
}

func TestRegistry_HealthCheckAll(t *testing.T) {
	reg := adapter.NewRegistry()

	reg.Register(&mockAdapter{name: "stripe", healthy: true})
	reg.Register(&mockAdapter{name: "paypal", healthy: false})

	ctx := context.Background()
	results := reg.HealthCheckAll(ctx)

	assert.NoError(t, results["stripe"])
	assert.Error(t, results["paypal"])
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	reg := adapter.NewRegistry()

	reg.Register(&mockAdapter{name: "stripe", currencies: []string{"USD"}, healthy: true})
	reg.Register(&mockAdapter{name: "stripe", currencies: []string{"USD", "EUR"}, healthy: true})

	a, err := reg.Get("stripe")
	require.NoError(t, err)
	assert.Len(t, a.SupportedCurrencies(), 2, "second registration should overwrite first")
}

// ─── Spending Tracker Tests ──────────────────────────────────────────────────

func TestSpendingTracker_CheckLimits_WithinDefaults(t *testing.T) {
	backend := newMockLedgerBackend()
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	allowed, err := st.CheckLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "100.00", Currency: "USD"})
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestSpendingTracker_CheckLimits_ExceedsDailyDefault(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.dailySpending["did:aicp:agent:USD"] = "9950"
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	// 9950 + 100 = 10050 > 10000 daily limit.
	allowed, err := st.CheckLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "100.00", Currency: "USD"})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestSpendingTracker_CheckLimits_ExceedsMonthlyDefault(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.monthlySpending["did:aicp:agent:USD"] = "99950"
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	// 99950 + 100 = 100050 > 100000 monthly limit.
	allowed, err := st.CheckLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "100.00", Currency: "USD"})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestSpendingTracker_CheckLimits_InvalidAmount(t *testing.T) {
	backend := newMockLedgerBackend()
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	_, err := st.CheckLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "not-a-number", Currency: "USD"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestSpendingTracker_CheckCustomLimits_WithinLimits(t *testing.T) {
	backend := newMockLedgerBackend()
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	limits := aicp.SpendingLimits{
		PerTransaction: aicp.MoneyAmount{Value: "500", Currency: "USD"},
		Daily:          aicp.MoneyAmount{Value: "5000", Currency: "USD"},
		Monthly:        aicp.MoneyAmount{Value: "50000", Currency: "USD"},
	}

	allowed, err := st.CheckCustomLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "100", Currency: "USD"}, limits)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestSpendingTracker_CheckCustomLimits_ExceedsPerTransaction(t *testing.T) {
	backend := newMockLedgerBackend()
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	limits := aicp.SpendingLimits{
		PerTransaction: aicp.MoneyAmount{Value: "50", Currency: "USD"},
		Daily:          aicp.MoneyAmount{Value: "5000", Currency: "USD"},
		Monthly:        aicp.MoneyAmount{Value: "50000", Currency: "USD"},
	}

	allowed, err := st.CheckCustomLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "100", Currency: "USD"}, limits)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestSpendingTracker_CheckCustomLimits_ExceedsDailyCustom(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.dailySpending["did:aicp:agent:USD"] = "4900"
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	limits := aicp.SpendingLimits{
		PerTransaction: aicp.MoneyAmount{Value: "500", Currency: "USD"},
		Daily:          aicp.MoneyAmount{Value: "5000", Currency: "USD"},
		Monthly:        aicp.MoneyAmount{Value: "50000", Currency: "USD"},
	}

	// 4900 + 200 = 5100 > 5000 custom daily limit.
	allowed, err := st.CheckCustomLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "200", Currency: "USD"}, limits)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestSpendingTracker_CheckCustomLimits_ExceedsMonthlyCustom(t *testing.T) {
	backend := newMockLedgerBackend()
	backend.monthlySpending["did:aicp:agent:USD"] = "49900"
	st := NewSpendingTracker(backend)
	ctx := context.Background()

	limits := aicp.SpendingLimits{
		PerTransaction: aicp.MoneyAmount{Value: "500", Currency: "USD"},
		Daily:          aicp.MoneyAmount{Value: "50000", Currency: "USD"},
		Monthly:        aicp.MoneyAmount{Value: "50000", Currency: "USD"},
	}

	// 49900 + 200 = 50100 > 50000 custom monthly limit.
	allowed, err := st.CheckCustomLimits(ctx, "did:aicp:agent", aicp.MoneyAmount{Value: "200", Currency: "USD"}, limits)
	require.NoError(t, err)
	assert.False(t, allowed)
}

// ─── Receipt Generator Tests ─────────────────────────────────────────────────

func TestReceiptGenerator_Generate(t *testing.T) {
	rg := NewReceiptGenerator()
	ctx := context.Background()

	claims := &aicp.PaymentTokenClaims{
		JTI:      "pt_test123",
		Subject:  "did:aicp:payer",
		Audience: "did:aicp:payee",
		PaymentDetails: aicp.PaymentTokenDetails{
			Amount: aicp.MoneyAmount{Value: "25.00", Currency: "USD"},
		},
	}

	receipt, err := rg.Generate(ctx, claims, "intent_abc", "settled")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(receipt.ReceiptID, "rcpt_"))
	assert.Equal(t, "pt_test123", receipt.TokenJTI)
	assert.Equal(t, "did:aicp:payer", receipt.PayerDID)
	assert.Equal(t, "did:aicp:payee", receipt.PayeeDID)
	assert.Equal(t, "25.00", receipt.Amount.Value)
	assert.Equal(t, "USD", receipt.Amount.Currency)
	assert.Equal(t, "settled", receipt.Status)
	assert.NotEmpty(t, receipt.ReceiptHash)
	assert.Len(t, receipt.ReceiptHash, 64) // SHA-256 hex.
}

func TestReceiptGenerator_DeterministicHash(t *testing.T) {
	rg := NewReceiptGenerator()
	ctx := context.Background()

	claims := &aicp.PaymentTokenClaims{
		JTI:      "pt_deterministic",
		Subject:  "did:aicp:payer",
		Audience: "did:aicp:payee",
		PaymentDetails: aicp.PaymentTokenDetails{
			Amount: aicp.MoneyAmount{Value: "10.00", Currency: "EUR"},
		},
	}

	r1, err := rg.Generate(ctx, claims, "intent_1", "settled")
	require.NoError(t, err)
	r2, err := rg.Generate(ctx, claims, "intent_1", "settled")
	require.NoError(t, err)

	assert.Equal(t, r1.ReceiptHash, r2.ReceiptHash, "same inputs must produce same hash")
	assert.Equal(t, r1.ReceiptID, r2.ReceiptID)
}
