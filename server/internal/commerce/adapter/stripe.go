package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// StripeAdapter implements PaymentAdapter for Stripe.
type StripeAdapter struct {
	apiKey    string
	webhookSecret string
	initialized   bool
}

// NewStripeAdapter creates a new Stripe adapter.
func NewStripeAdapter() *StripeAdapter {
	return &StripeAdapter{}
}

func (s *StripeAdapter) Name() string { return "stripe" }

func (s *StripeAdapter) SupportedCurrencies() []string {
	return []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF"}
}

func (s *StripeAdapter) Initialize(ctx context.Context, config map[string]any) error {
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("stripe: api_key is required")
	}
	s.apiKey = apiKey

	if ws, ok := config["webhook_secret"].(string); ok {
		s.webhookSecret = ws
	}

	s.initialized = true
	return nil
}

func (s *StripeAdapter) Healthy(ctx context.Context) error {
	if !s.initialized {
		return fmt.Errorf("stripe adapter not initialized")
	}
	// In production, this would call the Stripe API to verify connectivity.
	return nil
}

func (s *StripeAdapter) CreatePaymentIntent(ctx context.Context, params CreateIntentParams) (*PaymentIntent, error) {
	if !s.initialized {
		return nil, fmt.Errorf("stripe adapter not initialized")
	}

	// In production, this calls:
	//   stripe.PaymentIntents.New(&stripe.PaymentIntentParams{...})
	// For now, we return a structured response that matches the Stripe API shape.
	intentID := fmt.Sprintf("pi_%s", generateID())

	return &PaymentIntent{
		IntentID:    intentID,
		Status:      "requires_capture",
		Amount:      params.Amount,
		ProviderRef: intentID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (s *StripeAdapter) CapturePayment(ctx context.Context, intentID string) (*PaymentCapture, error) {
	if !s.initialized {
		return nil, fmt.Errorf("stripe adapter not initialized")
	}

	// stripe.PaymentIntents.Capture(intentID, nil)
	return &PaymentCapture{
		IntentID:   intentID,
		Status:     "captured",
		Amount:     aicp.MoneyAmount{}, // Populated from Stripe response.
		CapturedAt: time.Now().UTC(),
	}, nil
}

func (s *StripeAdapter) RefundPayment(ctx context.Context, intentID string, params *RefundParams) (*PaymentRefund, error) {
	if !s.initialized {
		return nil, fmt.Errorf("stripe adapter not initialized")
	}

	// stripe.Refunds.New(&stripe.RefundParams{PaymentIntent: &intentID, ...})
	refundID := fmt.Sprintf("re_%s", generateID())

	return &PaymentRefund{
		RefundID:   refundID,
		IntentID:   intentID,
		Status:     "succeeded",
		RefundedAt: time.Now().UTC(),
	}, nil
}

func (s *StripeAdapter) GetPaymentStatus(ctx context.Context, intentID string) (*PaymentStatus, error) {
	if !s.initialized {
		return nil, fmt.Errorf("stripe adapter not initialized")
	}

	// stripe.PaymentIntents.Get(intentID, nil)
	return &PaymentStatus{
		IntentID: intentID,
		Status:   "succeeded",
	}, nil
}

func (s *StripeAdapter) ValidateWebhook(ctx context.Context, payload []byte, sig string) (*WebhookEvent, error) {
	if !s.initialized {
		return nil, fmt.Errorf("stripe adapter not initialized")
	}
	if s.webhookSecret == "" {
		return nil, fmt.Errorf("stripe webhook secret not configured")
	}

	// webhook.ConstructEvent(payload, sig, s.webhookSecret)
	return &WebhookEvent{
		EventID:   fmt.Sprintf("evt_%s", generateID()),
		EventType: "payment_intent.succeeded",
	}, nil
}

// Compile-time interface check.
var _ PaymentAdapter = (*StripeAdapter)(nil)
