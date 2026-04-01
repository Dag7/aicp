package identity

import (
	"context"
	"fmt"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// CapabilityEnforcer validates that an agent has the required capabilities
// for an operation, based on their delegation credential.
type CapabilityEnforcer struct {
	verifier *DelegationVerifier
}

// NewCapabilityEnforcer creates a new capability enforcer.
func NewCapabilityEnforcer(verifier *DelegationVerifier) *CapabilityEnforcer {
	return &CapabilityEnforcer{verifier: verifier}
}

// Enforce checks that the agent's delegation credential includes the required capability.
// Returns nil if allowed, ErrCapabilityDenied if not.
func (ce *CapabilityEnforcer) Enforce(ctx context.Context, agentDID, requiredCapability string) error {
	chain, err := ce.verifier.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return fmt.Errorf("verifying delegation: %w", err)
	}
	if !chain.Valid {
		return fmt.Errorf("invalid delegation chain for %s: %v", agentDID, chain.Errors)
	}

	for _, cap := range chain.Capabilities {
		if cap == requiredCapability || cap == "*" {
			return nil
		}
	}

	return fmt.Errorf("%w: agent %s lacks capability %q", aicp.ErrCapabilityDenied, agentDID, requiredCapability)
}

// EnforceAny checks that the agent has at least one of the required capabilities.
// Returns nil if any match, ErrCapabilityDenied if none do.
func (ce *CapabilityEnforcer) EnforceAny(ctx context.Context, agentDID string, capabilities ...string) error {
	chain, err := ce.verifier.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return fmt.Errorf("verifying delegation: %w", err)
	}
	if !chain.Valid {
		return fmt.Errorf("invalid delegation chain for %s: %v", agentDID, chain.Errors)
	}

	for _, agentCap := range chain.Capabilities {
		if agentCap == "*" {
			return nil
		}
		for _, required := range capabilities {
			if agentCap == required {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: agent %s lacks any of capabilities %v", aicp.ErrCapabilityDenied, agentDID, capabilities)
}
