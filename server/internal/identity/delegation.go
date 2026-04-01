package identity

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DelegationVerifier verifies the delegation chain from an agent back to its principal.
type DelegationVerifier struct {
	backend ledger.LedgerBackend
	didMgr  *DIDManager
	credMgr *CredentialManager
}

// NewDelegationVerifier creates a new delegation verifier.
func NewDelegationVerifier(backend ledger.LedgerBackend, didMgr *DIDManager, credMgr *CredentialManager) *DelegationVerifier {
	return &DelegationVerifier{
		backend: backend,
		didMgr:  didMgr,
		credMgr: credMgr,
	}
}

// DelegationChain is the result of verifying an agent's delegation.
type DelegationChain struct {
	AgentDID      string                    `json:"agent_did"`
	PrincipalDID  string                    `json:"principal_did"`
	Chain         []string                  `json:"chain"`
	Valid         bool                      `json:"valid"`
	Capabilities  []string                  `json:"capabilities,omitempty"`
	Credential    *aicp.VerifiableCredential `json:"credential,omitempty"`
	Errors        []string                  `json:"errors,omitempty"`
}

// VerifyDelegation verifies that an agent DID has a valid delegation chain
// back to its principal, including checking the delegation credential.
func (dv *DelegationVerifier) VerifyDelegation(ctx context.Context, agentDID string) (*DelegationChain, error) {
	result := &DelegationChain{
		AgentDID: agentDID,
		Valid:    true,
	}

	// Resolve the agent's DID document.
	agentResult, err := dv.didMgr.Resolve(ctx, agentDID)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot resolve agent DID: %v", err))
		return result, nil
	}
	if agentResult.DocumentMetadata.Deactivated {
		result.Valid = false
		result.Errors = append(result.Errors, "agent DID is deactivated")
		return result, nil
	}

	// Extract the principal DID from the agent's document.
	principalDID := agentResult.Document.AICPPrincipal
	if principalDID == "" {
		principalDID = agentResult.Document.Controller
	}
	result.PrincipalDID = principalDID
	result.Chain = []string{agentDID, principalDID}

	// Verify the principal exists and is active.
	principalResult, err := dv.didMgr.Resolve(ctx, principalDID)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("cannot resolve principal DID: %v", err))
		return result, nil
	}
	if principalResult.DocumentMetadata.Deactivated {
		result.Valid = false
		result.Errors = append(result.Errors, "principal DID is deactivated")
		return result, nil
	}

	// Verify the agent is controlled by the principal.
	if agentResult.Document.Controller != principalDID {
		result.Valid = false
		result.Errors = append(result.Errors, "agent is not controlled by the claimed principal")
		return result, nil
	}

	// If there's a delegation credential reference, verify it.
	if agentResult.Document.AICPDelegationCred != "" {
		cred, err := dv.backend.GetCredential(ctx, agentResult.Document.AICPDelegationCred)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("cannot retrieve delegation credential: %v", err))
			return result, nil
		}

		// Verify the credential.
		verResult, err := dv.credMgr.VerifyCredential(ctx, *cred)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("credential verification error: %v", err))
			return result, nil
		}
		if !verResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, verResult.Errors...)
			return result, nil
		}

		// Store the credential for fine-grained capability inspection.
		result.Credential = cred

		// Extract capabilities from the delegation credential.
		var subject aicp.AgentDelegationSubject
		if err := json.Unmarshal(cred.CredentialSubject, &subject); err == nil {
			result.Capabilities = subject.DelegatedCapabilities
		}
	}

	return result, nil
}

// VerifySpendingAuthorization verifies that an agent has valid spending authorization
// and returns the spending limits.
func (dv *DelegationVerifier) VerifySpendingAuthorization(ctx context.Context, agentDID string) (*aicp.SpendingAuthorizationSubject, error) {
	// First verify delegation.
	chain, err := dv.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return nil, fmt.Errorf("verifying delegation: %w", err)
	}
	if !chain.Valid {
		return nil, fmt.Errorf("invalid delegation chain: %v", chain.Errors)
	}

	// Resolve agent to find spending credential reference.
	agentResult, err := dv.didMgr.Resolve(ctx, agentDID)
	if err != nil {
		return nil, fmt.Errorf("resolving agent DID: %w", err)
	}

	// Look for spending authorization in service endpoints.
	for _, svc := range agentResult.Document.Service {
		if svc.Type == "AICPSpendingAuthorization" {
			cred, err := dv.backend.GetCredential(ctx, svc.ServiceEndpoint)
			if err != nil {
				continue
			}

			verResult, err := dv.credMgr.VerifyCredential(ctx, *cred)
			if err != nil || !verResult.Valid {
				continue
			}

			var subject aicp.SpendingAuthorizationSubject
			if err := json.Unmarshal(cred.CredentialSubject, &subject); err != nil {
				continue
			}

			return &subject, nil
		}
	}

	return nil, fmt.Errorf("no valid spending authorization found for agent %s", agentDID)
}
