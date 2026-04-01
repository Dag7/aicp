package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// CapabilityChecker verifies whether an agent DID has a specific capability
// by inspecting its delegation credential chain.
type CapabilityChecker struct {
	verifier *DelegationVerifier
}

// NewCapabilityChecker creates a new capability checker.
func NewCapabilityChecker(verifier *DelegationVerifier) *CapabilityChecker {
	return &CapabilityChecker{verifier: verifier}
}

// HasCapability checks if the agent has the specified capability, accounting
// for wildcard grants and hierarchical matching (e.g., "payment:*" matches "payment:read").
func (cc *CapabilityChecker) HasCapability(ctx context.Context, agentDID string, required aicp.CapabilityType) (bool, error) {
	chain, err := cc.verifier.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return false, fmt.Errorf("verifying delegation: %w", err)
	}

	if !chain.Valid {
		return false, nil
	}

	return matchesCapability(chain.Capabilities, required), nil
}

// HasCapabilityForResource checks if the agent has a specific capability
// scoped to a resource, inspecting CapabilityGrant conditions.
func (cc *CapabilityChecker) HasCapabilityForResource(ctx context.Context, agentDID string, required aicp.CapabilityType, resource string) (bool, error) {
	chain, err := cc.verifier.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return false, fmt.Errorf("verifying delegation: %w", err)
	}

	if !chain.Valid {
		return false, nil
	}

	// Check simple string capabilities first (backward compatible).
	if matchesCapability(chain.Capabilities, required) {
		return true, nil
	}

	// Check fine-grained CapabilityGrant if present in credential subject.
	grants, err := extractCapabilityGrants(chain.Credential)
	if err != nil {
		return false, nil // Gracefully degrade on parse failure.
	}

	for _, grant := range grants {
		if !matchesSingleCapability(string(grant.Capability), required) {
			continue
		}
		// If grant has a resource scope, it must match.
		if grant.Resource != "" && grant.Resource != resource && grant.Resource != "*" {
			continue
		}
		return true, nil
	}

	return false, nil
}

// ListCapabilities returns all capabilities granted to an agent.
func (cc *CapabilityChecker) ListCapabilities(ctx context.Context, agentDID string) ([]string, error) {
	chain, err := cc.verifier.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return nil, fmt.Errorf("verifying delegation: %w", err)
	}

	if !chain.Valid {
		return nil, nil
	}

	return chain.Capabilities, nil
}

// matchesCapability checks if any of the granted capabilities satisfy the required one.
func matchesCapability(granted []string, required aicp.CapabilityType) bool {
	for _, cap := range granted {
		if matchesSingleCapability(cap, required) {
			return true
		}
	}
	return false
}

// matchesSingleCapability checks if a single granted capability matches the required one.
// Supports: exact match, wildcard "*", and hierarchical prefix matching (e.g., "payment:*").
func matchesSingleCapability(granted string, required aicp.CapabilityType) bool {
	req := string(required)

	// Exact match.
	if granted == req {
		return true
	}

	// Global wildcard.
	if granted == "*" {
		return true
	}

	// Hierarchical wildcard: "payment:*" matches "payment:read", "payment:execute", etc.
	if strings.HasSuffix(granted, ":*") {
		prefix := strings.TrimSuffix(granted, ":*")
		if strings.HasPrefix(req, prefix+":") {
			return true
		}
	}

	// Category match: "payment" matches "payment:read", etc.
	if strings.HasPrefix(req, granted+":") {
		return true
	}

	return false
}

// extractCapabilityGrants parses fine-grained CapabilityGrant objects from a
// delegation credential's subject, if present.
func extractCapabilityGrants(cred *aicp.VerifiableCredential) ([]aicp.CapabilityGrant, error) {
	if cred == nil {
		return nil, nil
	}

	// CredentialSubject is json.RawMessage — unmarshal directly.
	var subject struct {
		CapabilityGrants []aicp.CapabilityGrant `json:"capabilityGrants"`
	}

	if err := json.Unmarshal(cred.CredentialSubject, &subject); err != nil {
		return nil, err
	}

	return subject.CapabilityGrants, nil
}
