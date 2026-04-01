package handlers

import (
	"context"
	"fmt"
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
	resp := aicp.VerifyResponse{DID: req.DID, Valid: true}

	h.resolveDID(ctx, &resp)
	if resp.Valid {
		h.verifyDelegation(ctx, req.DID, &resp)
	}
	h.enrichTrustScore(ctx, req.DID, &resp)
	h.verifyCredentials(ctx, req.Presentation, &resp)

	JSON(w, http.StatusOK, resp)
}

// resolveDID resolves the DID document and checks for deactivation.
func (h *VerifyHandler) resolveDID(ctx context.Context, resp *aicp.VerifyResponse) {
	result, err := h.didMgr.Resolve(ctx, resp.DID)
	if err != nil {
		resp.Valid = false
		resp.Errors = append(resp.Errors, fmt.Sprintf("DID not found: %v", err))
		return
	}
	resp.Document = result.Document
	if result.DocumentMetadata.Deactivated {
		resp.Valid = false
		resp.Errors = append(resp.Errors, "DID is deactivated")
	}
}

// verifyDelegation checks the delegation chain for agent DIDs.
func (h *VerifyHandler) verifyDelegation(ctx context.Context, did string, resp *aicp.VerifyResponse) {
	if !identity.IsAgentDID(did) {
		return
	}
	chain, err := h.delegator.VerifyDelegation(ctx, did)
	if err != nil {
		resp.Errors = append(resp.Errors, fmt.Sprintf("delegation verification failed: %v", err))
		return
	}
	if chain == nil {
		return
	}
	resp.DelegationChain = chain.Chain
	if !chain.Valid {
		resp.Valid = false
		resp.Errors = append(resp.Errors, chain.Errors...)
	}
}

// enrichTrustScore adds the trust score to the response if available.
func (h *VerifyHandler) enrichTrustScore(ctx context.Context, did string, resp *aicp.VerifyResponse) {
	score, err := h.trust.GetScore(ctx, did)
	if err == nil {
		resp.TrustScore = score
	}
}

// verifyCredentials verifies all credentials in the presentation.
func (h *VerifyHandler) verifyCredentials(ctx context.Context, presentation *aicp.VerifiablePresentation, resp *aicp.VerifyResponse) {
	if presentation == nil {
		return
	}
	for _, cred := range presentation.VerifiableCredential {
		resp.CredentialResults = append(resp.CredentialResults, h.verifySingleCredential(ctx, cred))
	}
}

// verifySingleCredential verifies a single credential and returns the result.
func (h *VerifyHandler) verifySingleCredential(ctx context.Context, cred aicp.VerifiableCredential) aicp.CredentialVerification {
	cv := aicp.CredentialVerification{
		CredentialID: cred.ID,
		Valid:        true,
	}
	if len(cred.Type) > 0 {
		cv.Type = cred.Type[0]
	}
	verResult, err := h.credMgr.VerifyCredential(ctx, cred)
	if err != nil {
		cv.Valid = false
		cv.Error = err.Error()
		return cv
	}
	if !verResult.Valid {
		cv.Valid = false
		if len(verResult.Errors) > 0 {
			cv.Error = verResult.Errors[0]
		}
	}
	return cv
}
