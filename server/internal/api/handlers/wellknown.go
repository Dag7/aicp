package handlers

import (
	"net/http"

	"github.com/aicp-protocol/aicp/server/internal/discovery"
)

// WellKnownHandler handles GET /.well-known/aicp.json.
type WellKnownHandler struct {
	wellKnown  *discovery.WellKnownService
	serviceDID string
}

// NewWellKnownHandler creates a new well-known handler.
func NewWellKnownHandler(wk *discovery.WellKnownService, serviceDID string) *WellKnownHandler {
	return &WellKnownHandler{wellKnown: wk, serviceDID: serviceDID}
}

// Handle responds with the /.well-known/aicp.json for this service.
func (h *WellKnownHandler) Handle(w http.ResponseWriter, r *http.Request) {
	data, err := h.wellKnown.Generate(r.Context(), h.serviceDID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "ANS record not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
