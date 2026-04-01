package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── Sandbox Adapter Tests ──────────────────────────────────────────────────

func TestSandboxAdapter_Name(t *testing.T) {
	s := NewSandboxAdapter()
	assert.Equal(t, "sandbox", s.Name())
}

func TestSandboxAdapter_SupportedCurrencies(t *testing.T) {
	s := NewSandboxAdapter()
	currencies := s.SupportedCurrencies()
	assert.Contains(t, currencies, "USD")
	assert.Contains(t, currencies, "EUR")
	assert.Contains(t, currencies, "USDC")
}

func TestSandboxAdapter_InitializeAndHealthy(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	assert.NoError(t, s.Initialize(ctx, nil))
	assert.NoError(t, s.Healthy(ctx))
}

func TestSandboxAdapter_CreatePaymentIntent(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
		Amount:   aicp.MoneyAmount{Value: "25.00", Currency: "USD"},
		PayerDID: "did:aicp:payer",
		PayeeDID: "did:aicp:payee",
	})
	require.NoError(t, err)

	assert.Contains(t, intent.IntentID, "sandbox_pi_")
	assert.Equal(t, "requires_capture", intent.Status)
	assert.Equal(t, "25.00", intent.Amount.Value)
	assert.Equal(t, "USD", intent.Amount.Currency)
	assert.False(t, intent.CreatedAt.IsZero())
}

func TestSandboxAdapter_CapturePayment(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
		Amount:   aicp.MoneyAmount{Value: "50.00", Currency: "EUR"},
		PayerDID: "did:aicp:payer",
		PayeeDID: "did:aicp:payee",
	})
	require.NoError(t, err)

	capture, err := s.CapturePayment(ctx, intent.IntentID)
	require.NoError(t, err)

	assert.Equal(t, intent.IntentID, capture.IntentID)
	assert.Equal(t, "captured", capture.Status)
	assert.Equal(t, "50.00", capture.Amount.Value)
	assert.False(t, capture.CapturedAt.IsZero())
}

func TestSandboxAdapter_CapturePayment_NotFound(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	_, err := s.CapturePayment(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSandboxAdapter_RefundPayment(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
		Amount:   aicp.MoneyAmount{Value: "30.00", Currency: "GBP"},
		PayerDID: "did:aicp:payer",
		PayeeDID: "did:aicp:payee",
	})
	require.NoError(t, err)

	_, err = s.CapturePayment(ctx, intent.IntentID)
	require.NoError(t, err)

	refund, err := s.RefundPayment(ctx, intent.IntentID, nil)
	require.NoError(t, err)

	assert.Contains(t, refund.RefundID, "sandbox_re_")
	assert.Equal(t, intent.IntentID, refund.IntentID)
	assert.Equal(t, "succeeded", refund.Status)
	assert.Equal(t, "30.00", refund.Amount.Value)
}

func TestSandboxAdapter_RefundPayment_NotFound(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	_, err := s.RefundPayment(ctx, "nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSandboxAdapter_GetPaymentStatus(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
		Amount:   aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
		PayerDID: "did:aicp:payer",
		PayeeDID: "did:aicp:payee",
	})
	require.NoError(t, err)

	status, err := s.GetPaymentStatus(ctx, intent.IntentID)
	require.NoError(t, err)
	assert.Equal(t, "requires_capture", status.Status)
	assert.Nil(t, status.SettledAt)

	// Capture and check again.
	_, err = s.CapturePayment(ctx, intent.IntentID)
	require.NoError(t, err)

	status, err = s.GetPaymentStatus(ctx, intent.IntentID)
	require.NoError(t, err)
	assert.Equal(t, "captured", status.Status)
	assert.NotNil(t, status.SettledAt)
}

func TestSandboxAdapter_GetPaymentStatus_NotFound(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	_, err := s.GetPaymentStatus(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSandboxAdapter_ValidateWebhook(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	event, err := s.ValidateWebhook(ctx, []byte("payload"), "sig")
	require.NoError(t, err)
	assert.Contains(t, event.EventID, "sandbox_evt_")
	assert.Equal(t, "payment_intent.succeeded", event.EventType)
}

func TestSandboxAdapter_FullLifecycle(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	// Create → Capture → Refund.
	intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
		Amount:      aicp.MoneyAmount{Value: "100.00", Currency: "USDC"},
		PayerDID:    "did:aicp:agent1",
		PayeeDID:    "did:aicp:service1",
		Description: "compute services",
	})
	require.NoError(t, err)
	assert.Equal(t, "requires_capture", intent.Status)

	capture, err := s.CapturePayment(ctx, intent.IntentID)
	require.NoError(t, err)
	assert.Equal(t, "captured", capture.Status)

	refund, err := s.RefundPayment(ctx, intent.IntentID, &RefundParams{
		Reason: "service unavailable",
	})
	require.NoError(t, err)
	assert.Equal(t, "succeeded", refund.Status)

	status, err := s.GetPaymentStatus(ctx, intent.IntentID)
	require.NoError(t, err)
	assert.Equal(t, "refunded", status.Status)
}

func TestSandboxAdapter_ConcurrentAccess(t *testing.T) {
	s := NewSandboxAdapter()
	ctx := context.Background()

	// Create multiple intents concurrently.
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			intent, err := s.CreatePaymentIntent(ctx, CreateIntentParams{
				Amount:   aicp.MoneyAmount{Value: "10.00", Currency: "USD"},
				PayerDID: "did:aicp:payer",
				PayeeDID: "did:aicp:payee",
			})
			assert.NoError(t, err)
			assert.NotEmpty(t, intent.IntentID)

			_, err = s.CapturePayment(ctx, intent.IntentID)
			assert.NoError(t, err)
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ─── Default Registry Tests ─────────────────────────────────────────────────

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()

	names := reg.List()
	assert.Contains(t, names, "sandbox")
	assert.Contains(t, names, "stripe")
	assert.Contains(t, names, "paypal")
	assert.Contains(t, names, "usdc")

	// Sandbox should be healthy.
	sandbox, err := reg.Get("sandbox")
	require.NoError(t, err)
	assert.NoError(t, sandbox.Healthy(context.Background()))
}

// ─── generateID Tests ───────────────────────────────────────────────────────

func TestGenerateID_Unique(t *testing.T) {
	ids := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := generateID()
		assert.Len(t, id, 24) // 12 bytes = 24 hex chars
		assert.False(t, ids[id], "generated duplicate ID")
		ids[id] = true
	}
}
