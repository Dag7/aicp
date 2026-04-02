package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/api/middleware"
	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DIDHandler handles DID lifecycle management endpoints.
type DIDHandler struct {
	didMgr    *identity.DIDManager
	principal *identity.PrincipalService
	credMgr   *identity.CredentialManager
}

// NewDIDHandler creates a new DID handler.
func NewDIDHandler(didMgr *identity.DIDManager, principal *identity.PrincipalService, credMgr *identity.CredentialManager) *DIDHandler {
	return &DIDHandler{
		didMgr:    didMgr,
		principal: principal,
		credMgr:   credMgr,
	}
}

// Create handles POST /aicp/v1/did — creates a new principal DID.
// This is a registration endpoint; no authentication required.
func (h *DIDHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req aicp.DIDCreateRequest
	if err := DecodeJSON(r, &req); err != nil {
		// Body is optional for DID creation; proceed with defaults.
		req = aicp.DIDCreateRequest{}
	}

	kp, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate key pair")
		return
	}

	result, err := h.principal.Register(r.Context(), kp)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "registration failed")
		return
	}

	JSON(w, http.StatusCreated, aicp.DIDCreateResponse{
		DID:           result.DID,
		PublicKey:     crypto.PublicKeyMultibase(kp.PublicKey),
		Document:      result.Document,
		AnchorReceipt: result.AnchorReceipt,
	})
}

// CreateAgent handles POST /aicp/v1/did/agent — creates an agent DID under a principal.
// Requires authentication: the caller must be the principal that owns the agent.
func (h *DIDHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	callerDID, _ := middleware.GetAuthDID(r.Context())

	var req aicp.DIDCreateAgentRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.PrincipalDID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "principal_did is required")
		return
	}

	if !middleware.IsValidDID(req.PrincipalDID) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid principal DID format")
		return
	}

	// Enforce ownership: the authenticated DID must be the principal.
	if callerDID != req.PrincipalDID {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you can only create agents under your own principal DID")
		return
	}

	// Resolve the principal to verify it exists and is active.
	principalResult, err := h.didMgr.Resolve(r.Context(), req.PrincipalDID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "principal DID not found")
		return
	}
	if principalResult.DocumentMetadata.Deactivated {
		JSONError(w, http.StatusConflict, "DID_DEACTIVATED", "principal DID is deactivated")
		return
	}

	// Extract the principal's public key from the DID document.
	if len(principalResult.Document.VerificationMethod) == 0 {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "principal has no verification methods")
		return
	}
	principalPubKey, err := crypto.PublicKeyFromMultibase(principalResult.Document.VerificationMethod[0].PublicKeyMultibase)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to decode principal public key")
		return
	}

	// Generate a new agent key pair.
	agentKP, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate agent key pair")
		return
	}

	agentDID := crypto.AgentDID(principalPubKey, agentKP.PublicKey)
	doc := identity.BuildAgentDocument(agentDID, req.PrincipalDID, agentKP.PublicKey)

	// Anchor the agent DID document to the ledger.
	proof := identity.ServerAttestationProof(callerDID)
	receipt, err := h.didMgr.AnchorAgent(r.Context(), agentDID, doc, proof)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to anchor agent DID")
		return
	}

	JSON(w, http.StatusCreated, aicp.DIDCreateAgentResponse{
		AgentDID:      agentDID,
		PublicKey:     crypto.PublicKeyMultibase(agentKP.PublicKey),
		Document:      &doc,
		AnchorReceipt: receipt,
	})
}

// Resolve handles GET /aicp/v1/did/{did} — resolves a DID document.
// No authentication required (DIDs are public).
func (h *DIDHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	did := chi.URLParam(r, "did")
	if did == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "DID is required")
		return
	}

	if !middleware.IsValidDID(did) {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid DID format")
		return
	}

	result, err := h.didMgr.Resolve(r.Context(), did)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "DID not found")
		return
	}

	JSON(w, http.StatusOK, result)
}

// Update handles PUT /aicp/v1/did/{did} — updates a DID document.
// Requires authentication: the caller must be the DID controller.
func (h *DIDHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var req aicp.DIDUpdateRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	// Resolve the existing document to verify ownership.
	existing, err := h.didMgr.Resolve(r.Context(), did)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "DID not found")
		return
	}
	if existing.DocumentMetadata.Deactivated {
		JSONError(w, http.StatusConflict, "DID_DEACTIVATED", "DID is deactivated")
		return
	}

	// Enforce ownership: caller must be the DID itself or its controller.
	if !h.isAuthorized(callerDID, existing.Document) {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you are not authorized to update this DID")
		return
	}

	// Apply requested updates to the existing document.
	updatedDoc := *existing.Document
	if len(req.Service) > 0 {
		updatedDoc.Service = req.Service
	}
	if len(req.VerificationMethod) > 0 {
		updatedDoc.VerificationMethod = req.VerificationMethod
	}

	// Persist the update to the ledger.
	proof := identity.ServerAttestationProof(callerDID)
	receipt, err := h.didMgr.UpdateDocument(r.Context(), did, updatedDoc, proof)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist DID update")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"did":            did,
		"document":       updatedDoc,
		"status":         "updated",
		"anchor_receipt": receipt,
	})
}

// Deactivate handles POST /aicp/v1/did/{did}/deactivate — deactivates a DID.
// Requires authentication: the caller must be the DID controller.
func (h *DIDHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
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

	// Verify the DID exists and is not already deactivated.
	existing, err := h.didMgr.Resolve(r.Context(), did)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "DID not found")
		return
	}
	if existing.DocumentMetadata.Deactivated {
		JSONError(w, http.StatusConflict, "DID_DEACTIVATED", "DID is already deactivated")
		return
	}

	// Enforce ownership: caller must be the DID itself or its controller.
	if !h.isAuthorized(callerDID, existing.Document) {
		JSONError(w, http.StatusForbidden, "FORBIDDEN", "you are not authorized to deactivate this DID")
		return
	}

	// Persist the deactivation to the ledger.
	proof := identity.ServerAttestationProof(callerDID)
	if err := h.didMgr.DeactivateWithProof(r.Context(), did, proof); err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to persist DID deactivation")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"did":    did,
		"status": "deactivated",
	})
}

// isAuthorized checks whether the caller DID is the document owner or controller.
func (h *DIDHandler) isAuthorized(callerDID string, doc *aicp.DIDDocument) bool {
	if callerDID == doc.ID {
		return true
	}
	if doc.Controller != "" && callerDID == doc.Controller {
		return true
	}
	// For agent DIDs, the principal portion of the caller should match the controller.
	if middleware.IsAgentDID(doc.ID) {
		principalDID := middleware.PrincipalFromAgentDID(doc.ID)
		if principalDID != "" && callerDID == principalDID {
			return true
		}
	}
	// Check if caller's DID is a prefix of the target (principal controlling agent).
	if strings.HasPrefix(doc.ID, callerDID+":") {
		return true
	}
	return false
}
