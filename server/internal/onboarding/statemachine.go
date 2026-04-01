// Package onboarding implements Layer 3 of the AICP protocol: the 4-phase
// onboarding state machine for agent-to-service registration.
package onboarding

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Transition defines a valid state machine transition.
type Transition struct {
	From aicp.OnboardingPhase
	To   aicp.OnboardingPhase
}

// validTransitions defines the allowed state transitions.
var validTransitions = map[Transition]bool{
	{aicp.PhaseInitiated, aicp.PhaseIdentityPresented}:              true,
	{aicp.PhaseIdentityPresented, aicp.PhaseCapabilitiesNegotiated}: true,
	{aicp.PhaseCapabilitiesNegotiated, aicp.PhasePaymentAuthorized}: true,
	{aicp.PhasePaymentAuthorized, aicp.PhaseProvisioned}:            true,
	// Any phase can transition to failed.
	{aicp.PhaseInitiated, aicp.PhaseFailed}:              true,
	{aicp.PhaseIdentityPresented, aicp.PhaseFailed}:      true,
	{aicp.PhaseCapabilitiesNegotiated, aicp.PhaseFailed}: true,
	{aicp.PhasePaymentAuthorized, aicp.PhaseFailed}:      true,
}

// Flow manages a single onboarding session's state transitions with guards.
type Flow struct {
	state      *aicp.OnboardingState
	sessionTTL time.Duration
	guards     []Guard
}

// Guard is a function that validates a transition before allowing it.
type Guard func(ctx context.Context, from, to aicp.OnboardingPhase, state *aicp.OnboardingState) error

// NewFlow creates a new onboarding flow for a given session.
func NewFlow(state *aicp.OnboardingState, sessionTTL time.Duration, guards ...Guard) *Flow {
	return &Flow{
		state:      state,
		sessionTTL: sessionTTL,
		guards:     guards,
	}
}

// State returns the current onboarding state.
func (f *Flow) State() *aicp.OnboardingState {
	return f.state
}

// Transition attempts to move the state machine to the next phase.
func (f *Flow) Transition(ctx context.Context, to aicp.OnboardingPhase) error {
	// Check expiry.
	if time.Now().After(f.state.ExpiresAt) {
		f.state.Phase = aicp.PhaseFailed
		f.state.FailedReason = "session expired"
		return fmt.Errorf("onboarding session has expired")
	}

	// Validate transition.
	t := Transition{From: f.state.Phase, To: to}
	if !validTransitions[t] {
		return fmt.Errorf("invalid transition from %s to %s", f.state.Phase, to)
	}

	// Run guards.
	for _, guard := range f.guards {
		if err := guard(ctx, f.state.Phase, to, f.state); err != nil {
			return fmt.Errorf("guard failed: %w", err)
		}
	}

	f.state.Phase = to
	f.state.UpdatedAt = time.Now().UTC()
	return nil
}

// Fail transitions the flow to the failed state with a reason.
func (f *Flow) Fail(reason string) {
	f.state.Phase = aicp.PhaseFailed
	f.state.FailedReason = reason
	f.state.UpdatedAt = time.Now().UTC()
}

// IsComplete returns true if the flow has reached the provisioned state.
func (f *Flow) IsComplete() bool {
	return f.state.Phase == aicp.PhaseProvisioned
}

// IsFailed returns true if the flow has failed.
func (f *Flow) IsFailed() bool {
	return f.state.Phase == aicp.PhaseFailed
}

// IsExpired returns true if the session has expired.
func (f *Flow) IsExpired() bool {
	return time.Now().After(f.state.ExpiresAt)
}

// ─── Built-in Guards ────────────────────────────────────────────────────────

// MinTrustScoreGuard rejects transitions if the agent's trust score is below the minimum.
func MinTrustScoreGuard(minScore int, getScore func(ctx context.Context, did string) (int, error)) Guard {
	return func(ctx context.Context, from, to aicp.OnboardingPhase, state *aicp.OnboardingState) error {
		// Only check on identity presentation.
		if to != aicp.PhaseIdentityPresented {
			return nil
		}
		score, err := getScore(ctx, state.AgentDID)
		if err != nil {
			return fmt.Errorf("getting trust score: %w", err)
		}
		if score < minScore {
			return fmt.Errorf("trust score %d is below minimum %d", score, minScore)
		}
		return nil
	}
}
