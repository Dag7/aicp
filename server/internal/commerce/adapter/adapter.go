// Package adapter defines the PaymentAdapter interface and adapter registry
// for pluggable payment provider integration (Stripe, PayPal, USDC).
package adapter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// PaymentAdapter is the Strategy interface for payment providers.
// Each adapter (Stripe, PayPal, USDC) implements this interface.
type PaymentAdapter interface {
	// Name returns the unique adapter identifier (e.g., "stripe", "paypal", "usdc").
	Name() string

	// SupportedCurrencies returns the list of supported currency codes.
	SupportedCurrencies() []string

	// Initialize sets up the adapter with provider-specific configuration.
	Initialize(ctx context.Context, config map[string]any) error

	// Healthy checks the adapter's connection to the payment provider.
	Healthy(ctx context.Context) error

	// CreatePaymentIntent creates a payment intent with the given parameters.
	CreatePaymentIntent(ctx context.Context, params CreateIntentParams) (*PaymentIntent, error)

	// CapturePayment captures a previously authorized payment.
	CapturePayment(ctx context.Context, intentID string) (*PaymentCapture, error)

	// RefundPayment refunds a captured payment.
	RefundPayment(ctx context.Context, intentID string, params *RefundParams) (*PaymentRefund, error)

	// GetPaymentStatus checks the current status of a payment.
	GetPaymentStatus(ctx context.Context, intentID string) (*PaymentStatus, error)

	// ValidateWebhook verifies a webhook payload signature from the provider.
	ValidateWebhook(ctx context.Context, payload []byte, sig string) (*WebhookEvent, error)
}

// ─── Parameter Types ────────────────────────────────────────────────────────

// CreateIntentParams holds the parameters for creating a payment intent.
type CreateIntentParams struct {
	Amount         aicp.MoneyAmount `json:"amount"`
	PayerDID       string           `json:"payer_did"`
	PayeeDID       string           `json:"payee_did"`
	Description    string           `json:"description"`
	IdempotencyKey string           `json:"idempotency_key"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// RefundParams holds optional refund parameters.
type RefundParams struct {
	Amount *aicp.MoneyAmount `json:"amount,omitempty"` // nil = full refund
	Reason string            `json:"reason,omitempty"`
}

// ─── Result Types ───────────────────────────────────────────────────────────

// PaymentIntent represents a created payment intent.
type PaymentIntent struct {
	IntentID    string           `json:"intent_id"`
	Status      string           `json:"status"`
	Amount      aicp.MoneyAmount `json:"amount"`
	ProviderRef string           `json:"provider_ref,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

// PaymentCapture represents a captured payment.
type PaymentCapture struct {
	IntentID   string           `json:"intent_id"`
	Status     string           `json:"status"`
	Amount     aicp.MoneyAmount `json:"amount"`
	CapturedAt time.Time        `json:"captured_at"`
}

// PaymentRefund represents a refund.
type PaymentRefund struct {
	RefundID   string           `json:"refund_id"`
	IntentID   string           `json:"intent_id"`
	Status     string           `json:"status"`
	Amount     aicp.MoneyAmount `json:"amount"`
	RefundedAt time.Time        `json:"refunded_at"`
}

// PaymentStatus holds the current status of a payment.
type PaymentStatus struct {
	IntentID   string           `json:"intent_id"`
	Status     string           `json:"status"`
	Amount     aicp.MoneyAmount `json:"amount"`
	SettledAt  *time.Time       `json:"settled_at,omitempty"`
}

// WebhookEvent represents a parsed and verified webhook event.
type WebhookEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	IntentID  string `json:"intent_id"`
	Status    string `json:"status"`
}

// ─── Adapter Registry ───────────────────────────────────────────────────────

// Registry manages registered payment adapters (Factory pattern).
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]PaymentAdapter
}

// NewRegistry creates a new empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]PaymentAdapter),
	}
}

// Register adds an adapter to the registry.
func (r *Registry) Register(adapter PaymentAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Name()] = adapter
}

// Get returns an adapter by name.
func (r *Registry) Get(name string) (PaymentAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("payment adapter %q not registered", name)
	}
	return adapter, nil
}

// List returns all registered adapter names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// HealthCheckAll checks the health of all registered adapters.
func (r *Registry) HealthCheckAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make(map[string]error, len(r.adapters))
	for name, adapter := range r.adapters {
		results[name] = adapter.Healthy(ctx)
	}
	return results
}
