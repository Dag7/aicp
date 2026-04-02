package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	"github.com/aicp-protocol/aicp/server/internal/crypto"
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

// Issue handles POST /aicp/v1/credentials/issue — issues a verifiable credential.
// Requires authentication: the caller must be the issuer.
func (h *CredentialHandler) Issue(w http.ResponseWriter, r *http.Request) {
	callerDID, _ := middleware.GetAuthDID(r.Context())

	var req aicp.CredentialIssueRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.IssuerDID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "issuer_did is required")
		return
	}
	if req.SubjectDID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "subject_did is required")
		return
	}
	if req.CredentialType == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "credential_type is required")
		return
	}

	if !middleware.IsValidDID(req.IssuerDID) || !middleware.IsValidDID(req.SubjectDID) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid DID format")
		return
	}

	// Enforce: the authenticated caller must be the issuer.
	if callerDID != req.IssuerDID {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you can only issue credentials as yourself")
		return
	}

	// Build the credential subject from the request claims.
	subject := map[string]any{
		"id":   req.SubjectDID,
		"type": req.CredentialType,
	}
	for k, v := range req.Claims {
		subject[k] = v
	}

	// Generate a deterministic credential ID.
	credID := aicp.CredentialIDPrefix + crypto.SHA256Hex([]byte(fmt.Sprintf("%s:%s:%s", req.IssuerDID, req.SubjectDID, req.CredentialType)))[:16]

	cred, err := h.credMgr.IssueCredential(r.Context(), identity.IssueParams{
		CredentialID: credID,
		Types:        []string{"VerifiableCredential", req.CredentialType},
		IssuerDID:    req.IssuerDID,
		Subject:      subject,
	})
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "credential issuance failed")
		return
	}

	JSON(w, http.StatusCreated, aicp.CredentialIssueResponse{
		Credential: cred,
	})
}

// Revoke handles POST /aicp/v1/credentials/{id}/revoke.
// Requires authentication: the caller must be the credential issuer.
func (h *CredentialHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	callerDID, _ := middleware.GetAuthDID(r.Context())

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

	if err := h.credMgr.RevokeCredentialByID(r.Context(), credentialID, callerDID); err != nil {
		if errors.Is(err, aicp.ErrForbidden) {
			JSONError(w, http.StatusForbidden, "FORBIDDEN", "only the credential issuer can revoke")
			return
		}
		JSONError(w, http.StatusInternalServerError, "REVOCATION_FAILED", "credential revocation failed")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"credential_id": credentialID,
		"status":        "revoked",
	})
}
