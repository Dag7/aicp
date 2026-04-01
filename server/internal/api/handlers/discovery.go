package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	"github.com/aicp-protocol/aicp/server/internal/discovery"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DiscoveryHandler handles ANS discovery endpoints.
type DiscoveryHandler struct {
	ansMgr *discovery.ANSManager
	search *discovery.SearchService
}

// NewDiscoveryHandler creates a new discovery handler.
func NewDiscoveryHandler(ansMgr *discovery.ANSManager, search *discovery.SearchService) *DiscoveryHandler {
	return &DiscoveryHandler{
		ansMgr: ansMgr,
		search: search,
	}
}

// Register handles POST /aicp/v1/discover/register — registers an ANS record.
// Requires authentication: the caller must own the DID being registered.
func (h *DiscoveryHandler) Register(w http.ResponseWriter, r *http.Request) {
	callerDID, _ := middleware.GetAuthDID(r.Context())

	var req aicp.DiscoveryRegisterRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Service.DID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "service DID is required")
		return
	}
	if req.Service.Name == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "service name is required")
		return
	}

	if !middleware.IsValidDID(req.Service.DID) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid service DID format")
		return
	}

	// Enforce ownership: caller must own the service DID or be its principal.
	if callerDID != req.Service.DID && !isControllerOf(callerDID, req.Service.DID) {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you can only register services for your own DID")
		return
	}

	if err := h.ansMgr.Register(r.Context(), req.ANSRecord); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	JSON(w, http.StatusCreated, map[string]any{
		"service_did": req.Service.DID,
		"status":      "registered",
	})
}

// Search handles GET /aicp/v1/discover/search — searches ANS records by capability.
// No authentication required (discovery is public).
func (h *DiscoveryHandler) Search(w http.ResponseWriter, r *http.Request) {
	capability := r.URL.Query().Get("capability")
	if capability == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "capability query parameter is required")
		return
	}

	// Validate capability length to prevent abuse.
	if len(capability) > 256 {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "capability query too long")
		return
	}

	limit := aicp.DefaultPageLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= aicp.MaxPageLimit {
			limit = parsed
		}
	}

	cursor := r.URL.Query().Get("cursor")

	opts := aicp.PaginationOpts{
		Limit:  limit,
		Cursor: cursor,
	}

	result, err := h.search.Search(r.Context(), capability, opts)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}

	JSON(w, http.StatusOK, aicp.DiscoverySearchResponse{
		Items:      result.Items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// Get handles GET /aicp/v1/discover/{did} — retrieves a specific ANS record.
// No authentication required (discovery is public).
func (h *DiscoveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	did := chi.URLParam(r, "did")
	if did == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "DID is required")
		return
	}

	if !middleware.IsValidDID(did) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid DID format")
		return
	}

	record, err := h.ansMgr.Get(r.Context(), did)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "ANS record not found")
		return
	}

	JSON(w, http.StatusOK, record)
}

// Delete handles DELETE /aicp/v1/discover/{did} — removes an ANS record.
// Requires authentication: the caller must own the service DID.
func (h *DiscoveryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	callerDID, _ := middleware.GetAuthDID(r.Context())

	did := chi.URLParam(r, "did")
	if did == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "DID is required")
		return
	}

	if !middleware.IsValidDID(did) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid DID format")
		return
	}

	// Enforce ownership: caller must own the DID or be its principal.
	if callerDID != did && !isControllerOf(callerDID, did) {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own service records")
		return
	}

	if err := h.ansMgr.Delete(r.Context(), did); err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete record")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"service_did": did,
		"status":      "deleted",
	})
}

// isControllerOf returns true if callerDID is the principal of targetDID.
func isControllerOf(callerDID, targetDID string) bool {
	principalDID := middleware.PrincipalFromAgentDID(targetDID)
	return principalDID != "" && callerDID == principalDID
}
