package handlers

import (
	"net/http"

	
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/internal/onboarding"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// VerifyHandler handles POST /aicp/v1/verify.
type VerifyHandler struct {
	didMgr    *identity.DIDManager
	credMgr   *identity.CredentialManager
	delegator *identity.DelegationVerifier
	trust     *onboarding.TrustComputer
}

// NewVerifyHandler creates a new verify handler.
func NewVerifyHandler(
	didMgr *identity.DIDManager,
	credMgr *identity.CredentialManager,
	delegator *identity.DelegationVerifier,
	trust *onboarding.TrustComputer,
) *VerifyHandler {
	return &VerifyHandler{
		didMgr:    didMgr,
		credMgr:   credMgr,
		delegator: delegator,
		trust:     trust,
	}
}

// Handle processes a verification request.
func (h *VerifyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var req aicp.VerifyRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.DID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "did is required")
		return
	}

	ctx := r.Context()
	resp := aicp.VerifyResponse{
		DID:   req.DID,
		Valid: true,
	}

	// Resolve the DID.
	result, err := h.didMgr.Resolve(ctx, req.DID)
	if err != nil {
		resp.Valid = false
		resp.Errors = append(resp.Errors, "DID not found: "+err.Error())
		JSON(w, http.StatusOK, resp)
		return
	}

	resp.Document = result.Document

	if result.DocumentMetadata.Deactivated {
		resp.Valid = false
		resp.Errors = append(resp.Errors, "DID is deactivated")
		JSON(w, http.StatusOK, resp)
		return
	}

	// Check delegation chain for agent DIDs.
	if identity.IsAgentDID(req.DID) {
		chain, err := h.delegator.VerifyDelegation(ctx, req.DID)
		if err != nil {
			resp.Errors = append(resp.Errors, "delegation verification failed: "+err.Error())
		} else if chain != nil {
			resp.DelegationChain = chain.Chain
			if !chain.Valid {
				resp.Valid = false
				resp.Errors = append(resp.Errors, chain.Errors...)
			}
		}
	}

	// Get trust score.
	score, err := h.trust.GetScore(ctx, req.DID)
	if err == nil {
		resp.TrustScore = score
	}

	// Verify presentation credentials if provided.
	if req.Presentation != nil {
		for _, cred := range req.Presentation.VerifiableCredential {
			verResult, err := h.credMgr.VerifyCredential(ctx, cred)
			credVerification := aicp.CredentialVerification{
				CredentialID: cred.ID,
				Valid:        true,
			}
			if len(cred.Type) > 0 {
				credVerification.Type = cred.Type[0]
			}
			if err != nil {
				credVerification.Valid = false
				credVerification.Error = err.Error()
			} else if !verResult.Valid {
				credVerification.Valid = false
				if len(verResult.Errors) > 0 {
					credVerification.Error = verResult.Errors[0]
				}
			}
			resp.CredentialResults = append(resp.CredentialResults, credVerification)
		}
	}

	JSON(w, http.StatusOK, resp)
}
