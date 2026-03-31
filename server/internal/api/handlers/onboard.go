package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/internal/onboarding"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// OnboardHandler handles the 4-phase onboarding endpoints.
type OnboardHandler struct {
	sessions  *onboarding.SessionManager
	challenge *onboarding.ChallengeService
	didMgr    *identity.DIDManager
	trust     *onboarding.TrustComputer
	rateLimiter *onboarding.RateLimiter
}

// NewOnboardHandler creates a new onboarding handler.
func NewOnboardHandler(
	sessions *onboarding.SessionManager,
	challenge *onboarding.ChallengeService,
	didMgr *identity.DIDManager,
	trust *onboarding.TrustComputer,
	rateLimiter *onboarding.RateLimiter,
) *OnboardHandler {
	return &OnboardHandler{
		sessions:    sessions,
		challenge:   challenge,
		didMgr:      didMgr,
		trust:       trust,
		rateLimiter: rateLimiter,
	}
}

// Initiate handles POST /aicp/v1/onboard — Phase 1: Identity Presentation.
func (h *OnboardHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req aicp.OnboardInitRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.AgentDID == "" {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent_did is required")
		return
	}

	// Rate limit check.
	if h.rateLimiter != nil {
		if err := h.rateLimiter.CheckDID(r.Context(), req.AgentDID); err != nil {
			JSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many onboarding attempts")
			return
		}
		if err := h.rateLimiter.CheckIP(r.Context(), r.RemoteAddr); err != nil {
			JSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many onboarding attempts")
			return
		}
	}

	// Create session.
	state, err := h.sessions.Create(r.Context(), req.AgentDID, "")
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create session")
		return
	}

	// Issue DID-Auth challenge.
	challenge, err := h.challenge.Issue(r.Context(), state.SessionID)
	if err != nil {
		JSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to issue challenge")
		return
	}

	if h.rateLimiter != nil {
		h.rateLimiter.RecordAttempt(r.Context(), req.AgentDID, r.RemoteAddr)
	}

	JSON(w, http.StatusCreated, aicp.OnboardInitResponse{
		SessionID: state.SessionID,
		State:     *state,
		Challenge: challenge,
	})
}

// Negotiate handles POST /aicp/v1/onboard/{id}/negotiate — Phase 2.
func (h *OnboardHandler) Negotiate(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	var req aicp.NegotiateRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	state, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	// Verify DID-Auth challenge if provided.
	if req.ChallengeResponse != nil {
		if err := h.challenge.Verify(r.Context(), sessionID, req.ChallengeResponse.Challenge); err != nil {
			JSONError(w, http.StatusUnauthorized, "AUTH_FAILED", "challenge verification failed")
			return
		}
	}

	// Transition state.
	flow := onboarding.NewFlow(state, 0)
	if err := flow.Transition(r.Context(), aicp.PhaseIdentityPresented); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}
	if err := flow.Transition(r.Context(), aicp.PhaseCapabilitiesNegotiated); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	// Determine approved capabilities.
	approved := make([]string, 0, len(req.OfferedCapabilities))
	for _, cap := range req.OfferedCapabilities {
		approved = append(approved, cap.Name)
	}

	h.sessions.Update(r.Context(), state)

	JSON(w, http.StatusOK, aicp.NegotiateResponse{
		State:                *state,
		ApprovedCapabilities: approved,
	})
}

// Authorize handles POST /aicp/v1/onboard/{id}/authorize — Phase 3.
func (h *OnboardHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	var req aicp.AuthorizeRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	state, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	flow := onboarding.NewFlow(state, 0)
	if err := flow.Transition(r.Context(), aicp.PhasePaymentAuthorized); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	h.sessions.Update(r.Context(), state)

	JSON(w, http.StatusOK, aicp.AuthorizeResponse{
		State:      *state,
		Authorized: true,
	})
}

// Provision handles POST /aicp/v1/onboard/{id}/provision — Phase 4.
func (h *OnboardHandler) Provision(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	state, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return
	}

	flow := onboarding.NewFlow(state, 0)
	if err := flow.Transition(r.Context(), aicp.PhaseProvisioned); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	h.sessions.Update(r.Context(), state)

	JSON(w, http.StatusOK, aicp.ProvisionResponse{
		State:     *state,
		AccountID: state.SessionID,
	})
}
