package handlers

import (
	"math/big"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	"github.com/aicp-protocol/aicp/server/internal/commerce"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// amountPattern validates payment amount format: positive decimal, max 15 digits + 8 decimals.
var amountPattern = regexp.MustCompile(`^\d{1,15}(\.\d{1,8})?$`)

// validCurrencies are the supported ISO 4217 currency codes and crypto.
var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true, "JPY": true, "CAD": true,
	"AUD": true, "CHF": true, "CNY": true, "USDC": true, "USDT": true,
}

// PaymentHandler handles commerce endpoints.
type PaymentHandler struct {
	issuer                *commerce.TokenIssuer
	executor              *commerce.PaymentExecutor
	issuerPubKeyMultibase string
}

// NewPaymentHandler creates a new payment handler.
func NewPaymentHandler(issuer *commerce.TokenIssuer, executor *commerce.PaymentExecutor, issuerPubKeyMultibase string) *PaymentHandler {
	return &PaymentHandler{
		issuer:                issuer,
		executor:              executor,
		issuerPubKeyMultibase: issuerPubKeyMultibase,
	}
}

// RequestToken handles POST /aicp/v1/payment/token.
func (h *PaymentHandler) RequestToken(w http.ResponseWriter, r *http.Request) {
	var req aicp.PaymentTokenRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.AgentDID == "" || req.ServiceDID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent_did and service_did are required")
		return
	}

	if !middleware.IsValidDID(req.AgentDID) || !middleware.IsValidDID(req.ServiceDID) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid DID format")
		return
	}

	if err := validateAmount(req.Amount); err != "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}

	if len(req.Purpose) > 256 {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "purpose must be 256 characters or fewer")
		return
	}

	resp, err := h.issuer.Issue(r.Context(), req)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "TOKEN_ERROR", "failed to issue payment token")
		return
	}

	JSON(w, http.StatusCreated, resp)
}

// Execute handles POST /aicp/v1/payment/execute.
func (h *PaymentHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req aicp.PaymentExecuteRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Token == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "token is required")
		return
	}

	resp, err := h.executor.Execute(r.Context(), req.Token, h.issuerPubKeyMultibase)
	if err != nil {
		ProtocolErrorResponse(w, err)
		return
	}

	JSON(w, http.StatusOK, resp)
}

// Status handles GET /aicp/v1/payment/status/{id}.
func (h *PaymentHandler) Status(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "payment ID is required")
		return
	}

	resp, err := h.executor.GetStatus(r.Context(), paymentID, "")
	if err != nil {
		ProtocolErrorResponse(w, err)
		return
	}

	JSON(w, http.StatusOK, resp)
}

// validateAmount validates a payment amount for format, positivity, and currency.
// Returns an error message string, or empty string if valid.
func validateAmount(amount aicp.MoneyAmount) string {
	if amount.Value == "" || amount.Currency == "" {
		return "amount value and currency are required"
	}

	if !amountPattern.MatchString(amount.Value) {
		return "amount value must be a positive decimal number (max 15 digits, 8 decimal places)"
	}

	// Verify the value is actually positive (not "0" or "0.00").
	val, ok := new(big.Float).SetString(amount.Value)
	if !ok || val.Sign() <= 0 {
		return "amount value must be greater than zero"
	}

	if !validCurrencies[amount.Currency] {
		return "unsupported currency; use ISO 4217 code (USD, EUR, GBP, etc.) or USDC"
	}

	return ""
}
