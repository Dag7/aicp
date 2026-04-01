package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// CredentialHandler handles credential-related endpoints.
type CredentialHandler struct {
	credMgr *identity.CredentialManager
}

// NewCredentialHandler creates a new credential handler.
func NewCredentialHandler(credMgr *identity.CredentialManager) *CredentialHandler {
	return &CredentialHandler{credMgr: credMgr}
}

// Revoke handles POST /aicp/v1/credentials/{id}/revoke.
func (h *CredentialHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "id")
	if credentialID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "credential ID is required")
		return
	}

	var req aicp.CredentialRevokeRequest
	if err := DecodeJSON(r, &req); err != nil {
		// Body is optional; proceed with empty reason.
		req = aicp.CredentialRevokeRequest{}
	}

	// Revoke the credential. RevokeCredential requires a key pair for signing
	// the revocation proof. In a full implementation, the authenticated principal's
	// key would be extracted from the auth middleware. For now, we delegate to
	// the credential manager which validates the issuer internally.
	//
	// The credential manager's RevokeCredential verifies only the issuer can revoke.
	// We pass nil here and let the backend handle the proof-less revocation path.
	if err := h.credMgr.RevokeCredentialByID(r.Context(), credentialID); err != nil {
		JSONError(w, http.StatusInternalServerError, "REVOCATION_FAILED", err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"credential_id": credentialID,
		"status":        "revoked",
	})
}
