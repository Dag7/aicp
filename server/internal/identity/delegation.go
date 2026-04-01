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
	AgentDID     string                     `json:"agent_did"`
	PrincipalDID string                     `json:"principal_did"`
	Chain        []string                   `json:"chain"`
	Valid        bool                       `json:"valid"`
	Capabilities []string                   `json:"capabilities,omitempty"`
	Credential   *aicp.VerifiableCredential `json:"credential,omitempty"`
	Errors       []string                   `json:"errors,omitempty"`
}

// VerifyDelegation verifies that an agent DID has a valid delegation chain
// back to its principal, including checking the delegation credential.
func (dv *DelegationVerifier) VerifyDelegation(ctx context.Context, agentDID string) (*DelegationChain, error) {
	result := &DelegationChain{
		AgentDID: agentDID,
		Valid:    true,
	}

	agentResult, err := dv.resolveAndValidate(ctx, agentDID, "agent")
	if err != nil {
		return invalidChain(result, err.Error()), nil
	}

	principalDID := agentResult.Document.AICPPrincipal
	if principalDID == "" {
		principalDID = agentResult.Document.Controller
	}
	result.PrincipalDID = principalDID
	result.Chain = []string{agentDID, principalDID}

	if _, err := dv.resolveAndValidate(ctx, principalDID, "principal"); err != nil {
		return invalidChain(result, err.Error()), nil
	}

	if agentResult.Document.Controller != principalDID {
		return invalidChain(result, "agent is not controlled by the claimed principal"), nil
	}

	if agentResult.Document.AICPDelegationCred != "" {
		if err := dv.verifyDelegationCredential(ctx, agentDID, agentResult.Document.AICPDelegationCred, result); err != nil {
			return invalidChain(result, err.Error()), nil
		}
	}

	return result, nil
}

// resolveAndValidate resolves a DID and checks it is active.
func (dv *DelegationVerifier) resolveAndValidate(ctx context.Context, did, role string) (*aicp.DIDResolutionResult, error) {
	res, err := dv.didMgr.Resolve(ctx, did)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s DID: %w", role, err)
	}
	if res.Document == nil || res.DocumentMetadata == nil {
		return nil, fmt.Errorf("incomplete resolution result for %s DID", role)
	}
	if res.DocumentMetadata.Deactivated {
		return nil, fmt.Errorf("%s DID is deactivated", role)
	}
	return res, nil
}

// verifyDelegationCredential retrieves, validates, and extracts capabilities from a delegation credential.
func (dv *DelegationVerifier) verifyDelegationCredential(ctx context.Context, agentDID, credID string, result *DelegationChain) error {
	cred, err := dv.backend.GetCredential(ctx, credID)
	if err != nil {
		return fmt.Errorf("cannot retrieve delegation credential: %w", err)
	}

	verResult, err := dv.credMgr.VerifyCredential(ctx, *cred)
	if err != nil {
		return fmt.Errorf("credential verification error: %w", err)
	}
	if !verResult.Valid {
		return fmt.Errorf("credential invalid: %v", verResult.Errors)
	}

	// Verify the credential's subject matches the agent being checked.
	var subject aicp.AgentDelegationSubject
	if err := json.Unmarshal(cred.CredentialSubject, &subject); err != nil {
		return fmt.Errorf("cannot parse delegation subject: %w", err)
	}
	if subject.ID != "" && subject.ID != agentDID {
		return fmt.Errorf("credential subject %s does not match agent %s", subject.ID, agentDID)
	}

	result.Credential = cred
	result.Capabilities = subject.DelegatedCapabilities
	return nil
}

// invalidChain marks a delegation chain as invalid with the given error message.
func invalidChain(chain *DelegationChain, msg string) *DelegationChain {
	chain.Valid = false
	chain.Errors = append(chain.Errors, msg)
	return chain
}

// VerifySpendingAuthorization verifies that an agent has valid spending authorization
// and returns the spending limits.
func (dv *DelegationVerifier) VerifySpendingAuthorization(ctx context.Context, agentDID string) (*aicp.SpendingAuthorizationSubject, error) {
	chain, err := dv.VerifyDelegation(ctx, agentDID)
	if err != nil {
		return nil, fmt.Errorf("verifying delegation: %w", err)
	}
	if !chain.Valid {
		return nil, fmt.Errorf("invalid delegation chain: %v", chain.Errors)
	}

	agentResult, err := dv.didMgr.Resolve(ctx, agentDID)
	if err != nil {
		return nil, fmt.Errorf("resolving agent DID: %w", err)
	}

	for _, svc := range agentResult.Document.Service {
		if svc.Type != "AICPSpendingAuthorization" {
			continue
		}
		subject, err := dv.verifySpendingCredential(ctx, svc.ServiceEndpoint)
		if err != nil {
			continue
		}
		return subject, nil
	}

	return nil, fmt.Errorf("no valid spending authorization found for agent %s", agentDID)
}

// verifySpendingCredential retrieves and validates a spending credential.
func (dv *DelegationVerifier) verifySpendingCredential(ctx context.Context, credID string) (*aicp.SpendingAuthorizationSubject, error) {
	cred, err := dv.backend.GetCredential(ctx, credID)
	if err != nil {
		return nil, err
	}

	verResult, err := dv.credMgr.VerifyCredential(ctx, *cred)
	if err != nil || !verResult.Valid {
		return nil, fmt.Errorf("credential invalid")
	}

	var subject aicp.SpendingAuthorizationSubject
	if err := json.Unmarshal(cred.CredentialSubject, &subject); err != nil {
		return nil, err
	}
	return &subject, nil
}
