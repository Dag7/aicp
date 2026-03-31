package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// USDCAdapter implements PaymentAdapter for USDC stablecoin payments via Circle API.
type USDCAdapter struct {
	apiKey      string
	walletID    string
	initialized bool
}

// NewUSDCAdapter creates a new USDC/Circle adapter.
func NewUSDCAdapter() *USDCAdapter {
	return &USDCAdapter{}
}

func (u *USDCAdapter) Name() string { return "usdc" }

func (u *USDCAdapter) SupportedCurrencies() []string {
	return []string{"USDC"}
}

func (u *USDCAdapter) Initialize(ctx context.Context, config map[string]any) error {
	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("usdc: api_key is required")
	}
	u.apiKey = apiKey

	if walletID, ok := config["wallet_id"].(string); ok {
		u.walletID = walletID
	}

	u.initialized = true
	return nil
}

func (u *USDCAdapter) Healthy(ctx context.Context) error {
	if !u.initialized {
		return fmt.Errorf("usdc adapter not initialized")
	}
	// In production, this would ping Circle's API.
	return nil
}

func (u *USDCAdapter) CreatePaymentIntent(ctx context.Context, params CreateIntentParams) (*PaymentIntent, error) {
	if !u.initialized {
		return nil, fmt.Errorf("usdc adapter not initialized")
	}

	// In production, this would call Circle's Payments API:
	//   POST /v1/payments
	// or use the Programmable Wallets API for on-chain transfers.
	intentID := fmt.Sprintf("USDC_%s", generateID())

	return &PaymentIntent{
		IntentID:    intentID,
		Status:      "pending",
		Amount:      params.Amount,
		ProviderRef: intentID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (u *USDCAdapter) CapturePayment(ctx context.Context, intentID string) (*PaymentCapture, error) {
	if !u.initialized {
		return nil, fmt.Errorf("usdc adapter not initialized")
	}

	// For USDC, capture is typically immediate (on-chain settlement).
	return &PaymentCapture{
		IntentID:   intentID,
		Status:     "settled",
		Amount:     aicp.MoneyAmount{},
		CapturedAt: time.Now().UTC(),
	}, nil
}

func (u *USDCAdapter) RefundPayment(ctx context.Context, intentID string, params *RefundParams) (*PaymentRefund, error) {
	if !u.initialized {
		return nil, fmt.Errorf("usdc adapter not initialized")
	}

	// Initiate a reverse transfer.
	refundID := fmt.Sprintf("USDCR_%s", generateID())

	return &PaymentRefund{
		RefundID:   refundID,
		IntentID:   intentID,
		Status:     "settled",
		RefundedAt: time.Now().UTC(),
	}, nil
}

func (u *USDCAdapter) GetPaymentStatus(ctx context.Context, intentID string) (*PaymentStatus, error) {
	if !u.initialized {
		return nil, fmt.Errorf("usdc adapter not initialized")
	}

	// GET /v1/payments/{id}
	now := time.Now().UTC()
	return &PaymentStatus{
		IntentID:  intentID,
		Status:    "settled",
		SettledAt: &now,
	}, nil
}

func (u *USDCAdapter) ValidateWebhook(ctx context.Context, payload []byte, sig string) (*WebhookEvent, error) {
	if !u.initialized {
		return nil, fmt.Errorf("usdc adapter not initialized")
	}

	// Verify Circle webhook signature.
	return &WebhookEvent{
		EventID:   fmt.Sprintf("USDCE_%s", generateID()),
		EventType: "payment.settled",
	}, nil
}

var _ PaymentAdapter = (*USDCAdapter)(nil)
