package commerce

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/commerce/adapter"
	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const defaultPaymentAdapter = "sandbox"

// PaymentExecutor orchestrates the full payment lifecycle:
// token validation -> spending limit check -> payment intent -> capture -> receipt.
type PaymentExecutor struct {
	registry   *adapter.Registry
	validator  *TokenValidator
	spending   *SpendingTracker
	receipts   *ReceiptGenerator
	backend    ledger.LedgerBackend
	logger     *slog.Logger
}

// NewPaymentExecutor creates a new payment executor.
func NewPaymentExecutor(
	registry *adapter.Registry,
	validator *TokenValidator,
	spending *SpendingTracker,
	receipts *ReceiptGenerator,
	backend ledger.LedgerBackend,
	logger *slog.Logger,
) *PaymentExecutor {
	return &PaymentExecutor{
		registry:  registry,
		validator: validator,
		spending:  spending,
		receipts:  receipts,
		backend:   backend,
		logger:    logger,
	}
}

// Execute processes a payment token through the full payment pipeline.
func (pe *PaymentExecutor) Execute(ctx context.Context, token string, issuerPubKeyMultibase string) (*aicp.PaymentExecuteResponse, error) {
	claims, err := pe.validateToken(ctx, token, issuerPubKeyMultibase)
	if err != nil {
		return nil, err
	}
	if err := pe.enforceSpendingLimits(ctx, claims); err != nil {
		return nil, err
	}
	if err := pe.validator.MarkUsed(ctx, claims.JTI); err != nil {
		return nil, fmt.Errorf("marking token used: %w", err)
	}
	intent, capture, err := pe.createAndCapturePayment(ctx, claims)
	if err != nil {
		return nil, err
	}
	pe.recordSpending(ctx, claims)
	receipt, err := pe.generateAndStoreReceipt(ctx, claims, intent.IntentID, capture.Status)
	if err != nil {
		return nil, err
	}
	pe.logger.Info("payment executed",
		"payment_id", intent.IntentID, "receipt_id", receipt.ReceiptID,
		"payer", claims.Subject, "payee", claims.Audience,
		"amount", claims.PaymentDetails.Amount.Value, "currency", claims.PaymentDetails.Amount.Currency,
	)
	return &aicp.PaymentExecuteResponse{
		PaymentID: intent.IntentID, Status: capture.Status,
		Amount: claims.PaymentDetails.Amount, ReceiptID: receipt.ReceiptID,
		ReceiptHash: receipt.ReceiptHash,
	}, nil
}

func (pe *PaymentExecutor) validateToken(ctx context.Context, token, issuerPubKeyMultibase string) (*aicp.PaymentTokenClaims, error) {
	validation, err := pe.validator.Validate(ctx, token, issuerPubKeyMultibase)
	if err != nil {
		return nil, fmt.Errorf("validating token: %w", err)
	}
	if !validation.Valid {
		return nil, fmt.Errorf("invalid payment token: %v", validation.Errors)
	}
	return validation.Claims, nil
}

func (pe *PaymentExecutor) enforceSpendingLimits(ctx context.Context, claims *aicp.PaymentTokenClaims) error {
	allowed, err := pe.spending.CheckLimits(ctx, claims.Subject, claims.PaymentDetails.Amount)
	if err != nil {
		return fmt.Errorf("checking spending limits: %w", err)
	}
	if !allowed {
		return aicp.ErrSpendingLimitExceeded
	}
	return nil
}

func (pe *PaymentExecutor) resolveAdapter(adapterID string) (adapter.PaymentAdapter, error) {
	name := adapterID
	if name == "" {
		name = defaultPaymentAdapter
	}
	pa, err := pe.registry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("getting payment adapter: %w", err)
	}
	return pa, nil
}

func (pe *PaymentExecutor) createAndCapturePayment(ctx context.Context, claims *aicp.PaymentTokenClaims) (*adapter.PaymentIntent, *adapter.PaymentCapture, error) {
	pa, err := pe.resolveAdapter(claims.PaymentDetails.AdapterID)
	if err != nil {
		return nil, nil, err
	}
	intent, err := pa.CreatePaymentIntent(ctx, adapter.CreateIntentParams{
		Amount: claims.PaymentDetails.Amount, PayerDID: claims.Subject,
		PayeeDID: claims.Audience,
		Description: fmt.Sprintf("AICP payment: %s", claims.PaymentDetails.Purpose),
		IdempotencyKey: claims.PaymentDetails.IdempotencyKey,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("creating payment intent: %w", err)
	}
	capture, err := pa.CapturePayment(ctx, intent.IntentID)
	if err != nil {
		return nil, nil, fmt.Errorf("capturing payment: %w", err)
	}
	return intent, capture, nil
}

func (pe *PaymentExecutor) recordSpending(ctx context.Context, claims *aicp.PaymentTokenClaims) {
	if err := pe.spending.RecordSpending(ctx, claims.Subject, claims.PaymentDetails.Amount); err != nil {
		pe.logger.Error("failed to record spending", "error", err, "did", claims.Subject)
	}
}

func (pe *PaymentExecutor) generateAndStoreReceipt(ctx context.Context, claims *aicp.PaymentTokenClaims, paymentID, status string) (*aicp.SignedPaymentReceipt, error) {
	receipt, err := pe.receipts.Generate(ctx, claims, paymentID, status)
	if err != nil {
		return nil, fmt.Errorf("generating receipt: %w", err)
	}
	if err := pe.backend.StoreReceipt(ctx, *receipt); err != nil {
		return nil, fmt.Errorf("storing receipt: %w", err)
	}
	return receipt, nil
}

// GetStatus returns the status of a payment by its ID.
func (pe *PaymentExecutor) GetStatus(ctx context.Context, paymentID string, adapterName string) (*aicp.PaymentStatusResponse, error) {
	if adapterName == "" {
		adapterName = defaultPaymentAdapter
	}
	pa, err := pe.resolveAdapter(adapterName)
	if err != nil {
		return nil, err
	}

	status, err := pa.GetPaymentStatus(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("getting payment status: %w", err)
	}

	return &aicp.PaymentStatusResponse{
		PaymentID: paymentID,
		Status:    status.Status,
		Amount:    status.Amount,
		Adapter:   adapterName,
		CreatedAt: time.Now().UTC(),
		SettledAt: status.SettledAt,
	}, nil
}

// ReceiptGenerator creates cryptographic payment receipts.
type ReceiptGenerator struct{}

// NewReceiptGenerator creates a new receipt generator.
func NewReceiptGenerator() *ReceiptGenerator {
	return &ReceiptGenerator{}
}

// Generate creates a signed payment receipt from validated claims.
func (rg *ReceiptGenerator) Generate(ctx context.Context, claims *aicp.PaymentTokenClaims, paymentID, status string) (*aicp.SignedPaymentReceipt, error) {
	receiptID := fmt.Sprintf("rcpt_%s", crypto.SHA256Hex([]byte(claims.JTI+paymentID))[:16])

	hashInput := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		receiptID, claims.JTI, claims.Subject, claims.Audience,
		claims.PaymentDetails.Amount.Value, claims.PaymentDetails.Amount.Currency,
	)
	receiptHash := crypto.SHA256Hex([]byte(hashInput))

	return &aicp.SignedPaymentReceipt{
		ReceiptID:   receiptID,
		TokenJTI:    claims.JTI,
		PayerDID:    claims.Subject,
		PayeeDID:    claims.Audience,
		Amount:      claims.PaymentDetails.Amount,
		ReceiptHash: receiptHash,
		Status:      status,
		SettledAt:   time.Now().UTC(),
	}, nil
}
