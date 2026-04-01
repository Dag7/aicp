package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/onboarding"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// TrustHandler handles trust score endpoints.
type TrustHandler struct {
	trust *onboarding.TrustComputer
}

// NewTrustHandler creates a new trust handler.
func NewTrustHandler(trust *onboarding.TrustComputer) *TrustHandler {
	return &TrustHandler{trust: trust}
}

// GetScore handles GET /aicp/v1/trust/{did} — returns the trust score for a DID.
func (h *TrustHandler) GetScore(w http.ResponseWriter, r *http.Request) {
	did := chi.URLParam(r, "did")
	if did == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "DID is required")
		return
	}

	ts, err := h.trust.Compute(r.Context(), did)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, aicp.TrustScoreResponse{
		DID:        ts.DID,
		Score:      ts.Score,
		Components: ts.Components,
		ComputedAt: ts.ComputedAt,
	})
}
