package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aicp-protocol/aicp/server/internal/onboarding"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// OnboardHandler handles the 4-phase onboarding endpoints.
type OnboardHandler struct {
	sessions    *onboarding.SessionManager
	challenge   *onboarding.ChallengeService
	trust       *onboarding.TrustComputer
	rateLimiter *onboarding.RateLimiter
	serviceDID  string
}

// NewOnboardHandler creates a new onboarding handler.
func NewOnboardHandler(
	sessions *onboarding.SessionManager,
	challenge *onboarding.ChallengeService,
	trust *onboarding.TrustComputer,
	rateLimiter *onboarding.RateLimiter,
	serviceDID string,
) *OnboardHandler {
	return &OnboardHandler{
		sessions:    sessions,
		challenge:   challenge,
		trust:       trust,
		rateLimiter: rateLimiter,
		serviceDID:  serviceDID,
	}
}

// checkRateLimits verifies both DID-based and IP-based rate limits.
func (h *OnboardHandler) checkRateLimits(ctx context.Context, did, remoteAddr string) error {
	if h.rateLimiter == nil {
		return nil
	}
	if err := h.rateLimiter.CheckDID(ctx, did); err != nil {
		return err
	}
	return h.rateLimiter.CheckIP(ctx, remoteAddr)
}

// getSession retrieves an onboarding session by URL param, writing an error response on failure.
func (h *OnboardHandler) getSession(w http.ResponseWriter, r *http.Request) (*aicp.OnboardingState, bool) {
	sessionID := chi.URLParam(r, "id")
	state, err := h.sessions.Get(r.Context(), sessionID)
	if err != nil {
		JSONError(w, http.StatusNotFound, "NOT_FOUND", "session not found")
		return nil, false
	}
	return state, true
}

// transitionTo advances the onboarding state through the given phases in order.
func (h *OnboardHandler) transitionTo(ctx context.Context, state *aicp.OnboardingState, phases ...aicp.OnboardingPhase) error {
	flow := onboarding.NewFlow(state, 0)
	for _, phase := range phases {
		if err := flow.Transition(ctx, phase); err != nil {
			return err
		}
	}
	return nil
}

// persistSession saves the updated onboarding state, logging any error.
func (h *OnboardHandler) persistSession(ctx context.Context, sessionID string, state *aicp.OnboardingState) {
	if err := h.sessions.Update(ctx, state); err != nil {
		slog.Error("failed to update onboarding session", "session_id", sessionID, "phase", state.Phase, "error", err)
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
	if err := h.checkRateLimits(r.Context(), req.AgentDID, r.RemoteAddr); err != nil {
		JSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many onboarding attempts")
		return
	}

	// Create session.
	state, err := h.sessions.Create(r.Context(), req.AgentDID, h.serviceDID)
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
		if err := h.rateLimiter.RecordAttempt(r.Context(), req.AgentDID, r.RemoteAddr); err != nil {
			slog.Error("failed to record rate limit attempt", "agent_did", req.AgentDID, "error", err)
		}
	}

	JSON(w, http.StatusCreated, aicp.OnboardInitResponse{
		SessionID: state.SessionID,
		State:     *state,
		Challenge: challenge,
	})
}

// Negotiate handles POST /aicp/v1/onboard/{id}/negotiate — Phase 2.
func (h *OnboardHandler) Negotiate(w http.ResponseWriter, r *http.Request) {
	state, ok := h.getSession(w, r)
	if !ok {
		return
	}

	var req aicp.NegotiateRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	// Verify DID-Auth challenge if provided.
	if req.ChallengeResponse != nil {
		if err := h.challenge.Verify(r.Context(), state.SessionID, req.ChallengeResponse.Challenge); err != nil {
			JSONError(w, http.StatusUnauthorized, "AUTH_FAILED", "challenge verification failed")
			return
		}
	}

	if err := h.transitionTo(r.Context(), state, aicp.PhaseIdentityPresented, aicp.PhaseCapabilitiesNegotiated); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	// Determine approved capabilities.
	approved := make([]string, 0, len(req.OfferedCapabilities))
	for _, cap := range req.OfferedCapabilities {
		approved = append(approved, cap.Name)
	}

	h.persistSession(r.Context(), state.SessionID, state)

	JSON(w, http.StatusOK, aicp.NegotiateResponse{
		State:                *state,
		ApprovedCapabilities: approved,
	})
}

// Authorize handles POST /aicp/v1/onboard/{id}/authorize — Phase 3.
func (h *OnboardHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	state, ok := h.getSession(w, r)
	if !ok {
		return
	}

	var req aicp.AuthorizeRequest
	if err := DecodeJSON(r, &req); err != nil {
		JSONError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if err := h.transitionTo(r.Context(), state, aicp.PhasePaymentAuthorized); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	h.persistSession(r.Context(), state.SessionID, state)

	JSON(w, http.StatusOK, aicp.AuthorizeResponse{
		State:      *state,
		Authorized: true,
	})
}

// Provision handles POST /aicp/v1/onboard/{id}/provision — Phase 4.
func (h *OnboardHandler) Provision(w http.ResponseWriter, r *http.Request) {
	state, ok := h.getSession(w, r)
	if !ok {
		return
	}

	if err := h.transitionTo(r.Context(), state, aicp.PhaseProvisioned); err != nil {
		JSONError(w, http.StatusConflict, "INVALID_TRANSITION", err.Error())
		return
	}

	h.persistSession(r.Context(), state.SessionID, state)

	JSON(w, http.StatusOK, aicp.ProvisionResponse{
		State:     *state,
		AccountID: state.SessionID,
	})
}
