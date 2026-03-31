package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	
	"github.com/aicp-protocol/aicp/server/internal/commerce"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// PaymentHandler handles commerce endpoints.
type PaymentHandler struct {
	issuer   *commerce.TokenIssuer
	executor *commerce.PaymentExecutor
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
	if req.Amount.Value == "" || req.Amount.Currency == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "amount value and currency are required")
		return
	}

	resp, err := h.issuer.Issue(r.Context(), req)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "TOKEN_ERROR", err.Error())
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
