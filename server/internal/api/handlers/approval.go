package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	"github.com/aicp-protocol/aicp/server/internal/commerce"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ApprovalHandler handles payment approval endpoints.
type ApprovalHandler struct {
	approvals *commerce.ApprovalService
}

// NewApprovalHandler creates a new approval handler.
func NewApprovalHandler(approvals *commerce.ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{approvals: approvals}
}

// GetApproval handles GET /aicp/v1/payment/approval/{id}.
func (h *ApprovalHandler) GetApproval(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "id")
	if approvalID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "approval ID is required")
		return
	}

	approval, err := h.approvals.GetApproval(r.Context(), approvalID)
	if err != nil {
		if errors.Is(err, aicp.ErrNotFound) {
			JSONError(w, http.StatusNotFound, "APPROVAL_NOT_FOUND", "approval not found")
			return
		}
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get approval")
		return
	}

	JSON(w, http.StatusOK, toApprovalResponse(approval))
}

// Approve handles POST /aicp/v1/payment/approval/{id}/approve.
func (h *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "id")
	if approvalID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "approval ID is required")
		return
	}

	principalDID, ok := middleware.GetAuthDID(r.Context())
	if !ok {
		JSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	if err := h.approvals.Approve(r.Context(), approvalID, principalDID); err != nil {
		h.handleApprovalError(w, err)
		return
	}

	approval, err := h.approvals.GetApproval(r.Context(), approvalID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "approved but failed to retrieve status")
		return
	}

	JSON(w, http.StatusOK, toApprovalResponse(approval))
}

// Deny handles POST /aicp/v1/payment/approval/{id}/deny.
func (h *ApprovalHandler) Deny(w http.ResponseWriter, r *http.Request) {
	approvalID := chi.URLParam(r, "id")
	if approvalID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "approval ID is required")
		return
	}

	principalDID, ok := middleware.GetAuthDID(r.Context())
	if !ok {
		JSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req aicp.ApprovalDecisionRequest
	if err := DecodeJSON(r, &req); err != nil {
		// Body is optional for deny; proceed with empty reason.
		req = aicp.ApprovalDecisionRequest{}
	}

	if err := h.approvals.Deny(r.Context(), approvalID, principalDID, req.Reason); err != nil {
		h.handleApprovalError(w, err)
		return
	}

	approval, err := h.approvals.GetApproval(r.Context(), approvalID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "denied but failed to retrieve status")
		return
	}

	JSON(w, http.StatusOK, toApprovalResponse(approval))
}

// ListPending handles GET /aicp/v1/payment/approvals/pending.
func (h *ApprovalHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	principalDID, ok := middleware.GetAuthDID(r.Context())
	if !ok {
		JSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	pending, err := h.approvals.ListPendingForPrincipal(r.Context(), principalDID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list pending approvals")
		return
	}

	results := make([]aicp.ApprovalResponse, 0, len(pending))
	for i := range pending {
		results = append(results, toApprovalResponse(&pending[i]))
	}

	JSON(w, http.StatusOK, results)
}

func (h *ApprovalHandler) handleApprovalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, aicp.ErrNotFound):
		JSONError(w, http.StatusNotFound, "APPROVAL_NOT_FOUND", "approval not found")
	case errors.Is(err, aicp.ErrForbidden):
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you are not authorized to decide this approval")
	case errors.Is(err, aicp.ErrApprovalExpired):
		JSONError(w, http.StatusGone, "APPROVAL_EXPIRED", "the approval has expired")
	default:
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
	}
}

func toApprovalResponse(pa *commerce.PendingApproval) aicp.ApprovalResponse {
	return aicp.ApprovalResponse{
		ApprovalID:   pa.ApprovalID,
		AgentDID:     pa.AgentDID,
		PrincipalDID: pa.PrincipalDID,
		Amount:       pa.Amount,
		Purpose:      pa.Purpose,
		Status:       string(pa.Status),
		CreatedAt:    pa.CreatedAt,
		ExpiresAt:    pa.ExpiresAt,
		DecidedAt:    pa.DecidedAt,
		DecidedBy:    pa.DecidedBy,
	}
}
