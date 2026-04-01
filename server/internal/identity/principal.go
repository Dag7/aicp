package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// PrincipalService handles principal registration and management,
// including the initial trust score setup.
type PrincipalService struct {
	didMgr  *DIDManager
	credMgr *CredentialManager
	backend ledger.LedgerBackend
}

// NewPrincipalService creates a new principal service.
func NewPrincipalService(didMgr *DIDManager, credMgr *CredentialManager, backend ledger.LedgerBackend) *PrincipalService {
	return &PrincipalService{
		didMgr:  didMgr,
		credMgr: credMgr,
		backend: backend,
	}
}

// RegistrationResult holds the outcome of a principal registration.
type RegistrationResult struct {
	DID           string              `json:"did"`
	Document      *aicp.DIDDocument   `json:"document"`
	AnchorReceipt *aicp.AnchorReceipt `json:"anchor_receipt"`
	TrustScore    *aicp.TrustScore    `json:"trust_score"`
}

// Register creates a new principal identity with an initial trust score.
func (ps *PrincipalService) Register(ctx context.Context, kp *crypto.KeyPair) (*RegistrationResult, error) {
	// Create the principal DID.
	doc, receipt, err := ps.didMgr.CreatePrincipal(ctx, kp)
	if err != nil {
		return nil, fmt.Errorf("creating principal: %w", err)
	}

	did := doc.ID

	// Initialize trust score.
	initialScore := aicp.TrustScore{
		DID:   did,
		Score: 100, // Base score for identity verification.
		Components: aicp.TrustScoreBreakdown{
			Identity:       100,
			AccountAge:     0,
			Transactions:   0,
			Endorsements:   0,
			Credentials:    0,
			DisputePenalty: 0,
		},
		ComputedAt: time.Now().UTC(),
	}

	if err := ps.backend.UpdateTrustScore(ctx, initialScore); err != nil {
		return nil, fmt.Errorf("initializing trust score: %w", err)
	}

	return &RegistrationResult{
		DID:           did,
		Document:      doc,
		AnchorReceipt: receipt,
		TrustScore:    &initialScore,
	}, nil
}

// RegisterAgent creates a new agent under a principal and issues delegation credentials.
func (ps *PrincipalService) RegisterAgent(
	ctx context.Context,
	principalKP *crypto.KeyPair,
	agentKP *crypto.KeyPair,
	capabilities []string,
	constraints aicp.DelegationConstraints,
) (*AgentRegistrationResult, error) {
	// Create the agent DID.
	doc, receipt, err := ps.didMgr.CreateAgent(ctx, principalKP, agentKP)
	if err != nil {
		return nil, fmt.Errorf("creating agent: %w", err)
	}

	agentDID := doc.ID

	// Issue delegation credential.
	delegationCred, err := ps.credMgr.IssueDelegationCredential(
		ctx, principalKP, agentDID, capabilities, constraints, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("issuing delegation credential: %w", err)
	}

	// Update agent's DID document with delegation credential reference.
	doc.AICPDelegationCred = delegationCred.ID
	_, err = ps.didMgr.Update(ctx, agentDID, *doc, principalKP)
	if err != nil {
		return nil, fmt.Errorf("updating agent DID document: %w", err)
	}

	// Initialize agent trust score.
	agentScore := aicp.TrustScore{
		DID:   agentDID,
		Score: 50, // Agents start with lower score than principals.
		Components: aicp.TrustScoreBreakdown{
			Identity:       50,
			AccountAge:     0,
			Transactions:   0,
			Endorsements:   0,
			Credentials:    0,
			DisputePenalty: 0,
		},
		ComputedAt: time.Now().UTC(),
	}

	if err := ps.backend.UpdateTrustScore(ctx, agentScore); err != nil {
		return nil, fmt.Errorf("initializing agent trust score: %w", err)
	}

	return &AgentRegistrationResult{
		AgentDID:       agentDID,
		Document:       doc,
		AnchorReceipt:  receipt,
		DelegationCred: delegationCred,
		TrustScore:     &agentScore,
	}, nil
}

// AgentRegistrationResult holds the outcome of an agent registration.
type AgentRegistrationResult struct {
	AgentDID       string                     `json:"agent_did"`
	Document       *aicp.DIDDocument          `json:"document"`
	AnchorReceipt  *aicp.AnchorReceipt        `json:"anchor_receipt"`
	DelegationCred *aicp.VerifiableCredential `json:"delegation_credential"`
	TrustScore     *aicp.TrustScore           `json:"trust_score"`
}
