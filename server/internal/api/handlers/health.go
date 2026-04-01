package handlers

import (
	"net/http"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	backend ledger.LedgerBackend
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(backend ledger.LedgerBackend) *HealthHandler {
	return &HealthHandler{backend: backend}
}

// Health returns basic liveness status.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": aicp.ProtocolName,
		"version": aicp.ProtocolVersion,
	})
}

// Ready checks database connectivity for readiness.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.backend.Healthy(r.Context()); err != nil {
		JSONError(w, http.StatusServiceUnavailable, "NOT_READY", "database unavailable")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
