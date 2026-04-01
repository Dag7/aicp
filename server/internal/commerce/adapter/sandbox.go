package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// SandboxAdapter implements PaymentAdapter as an in-memory sandbox
// for development and testing. It requires no external services or
// API keys and automatically tracks payment intents in memory.
type SandboxAdapter struct {
	mu      sync.RWMutex
	intents map[string]*sandboxIntent
}

type sandboxIntent struct {
	ID        string
	Amount    aicp.MoneyAmount
	PayerDID  string
	PayeeDID  string
	Status    string
	CreatedAt time.Time
	SettledAt *time.Time
}

// NewSandboxAdapter creates a pre-initialized sandbox adapter.
func NewSandboxAdapter() *SandboxAdapter {
	return &SandboxAdapter{
		intents: make(map[string]*sandboxIntent),
	}
}

func (s *SandboxAdapter) Name() string { return "sandbox" }

func (s *SandboxAdapter) SupportedCurrencies() []string {
	return []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "USDC"}
}

func (s *SandboxAdapter) Initialize(_ context.Context, _ map[string]any) error {
	return nil // Always succeeds — no external config needed.
}

func (s *SandboxAdapter) Healthy(_ context.Context) error {
	return nil // Always healthy.
}

func (s *SandboxAdapter) CreatePaymentIntent(_ context.Context, params CreateIntentParams) (*PaymentIntent, error) {
	intentID := fmt.Sprintf("sandbox_pi_%s", generateID())

	s.mu.Lock()
	s.intents[intentID] = &sandboxIntent{
		ID:        intentID,
		Amount:    params.Amount,
		PayerDID:  params.PayerDID,
		PayeeDID:  params.PayeeDID,
		Status:    "requires_capture",
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Unlock()

	return &PaymentIntent{
		IntentID:    intentID,
		Status:      "requires_capture",
		Amount:      params.Amount,
		ProviderRef: intentID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (s *SandboxAdapter) CapturePayment(_ context.Context, intentID string) (*PaymentCapture, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return nil, fmt.Errorf("sandbox: intent %q not found", intentID)
	}

	now := time.Now().UTC()
	intent.Status = "captured"
	intent.SettledAt = &now

	return &PaymentCapture{
		IntentID:   intentID,
		Status:     "captured",
		Amount:     intent.Amount,
		CapturedAt: now,
	}, nil
}

func (s *SandboxAdapter) RefundPayment(_ context.Context, intentID string, _ *RefundParams) (*PaymentRefund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return nil, fmt.Errorf("sandbox: intent %q not found", intentID)
	}

	intent.Status = "refunded"

	return &PaymentRefund{
		RefundID:   fmt.Sprintf("sandbox_re_%s", generateID()),
		IntentID:   intentID,
		Status:     "succeeded",
		Amount:     intent.Amount,
		RefundedAt: time.Now().UTC(),
	}, nil
}

func (s *SandboxAdapter) GetPaymentStatus(_ context.Context, intentID string) (*PaymentStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return nil, fmt.Errorf("sandbox: intent %q not found", intentID)
	}

	return &PaymentStatus{
		IntentID:  intentID,
		Status:    intent.Status,
		Amount:    intent.Amount,
		SettledAt: intent.SettledAt,
	}, nil
}

func (s *SandboxAdapter) ValidateWebhook(_ context.Context, _ []byte, _ string) (*WebhookEvent, error) {
	return &WebhookEvent{
		EventID:   fmt.Sprintf("sandbox_evt_%s", generateID()),
		EventType: "payment_intent.succeeded",
	}, nil
}

// Compile-time interface check.
var _ PaymentAdapter = (*SandboxAdapter)(nil)
