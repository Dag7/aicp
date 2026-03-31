package onboarding

import (
	"context"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// TrustComputer computes trust scores for DIDs based on the defined formula:
// Identity verification (25%), Account age (15%), Transactions (20%),
// Endorsements (15%), Credentials (10%), Dispute penalty (15%).
type TrustComputer struct {
	backend ledger.LedgerBackend
}

// NewTrustComputer creates a new trust score computer.
func NewTrustComputer(backend ledger.LedgerBackend) *TrustComputer {
	return &TrustComputer{backend: backend}
}

// Compute calculates the trust score for a DID and persists it.
func (tc *TrustComputer) Compute(ctx context.Context, did string) (*aicp.TrustScore, error) {
	// Get existing score for reference.
	existing, _ := tc.backend.GetTrustScore(ctx, did)

	identity := tc.computeIdentity(ctx, did)
	accountAge := tc.computeAccountAge(ctx, did)
	transactions := tc.computeTransactions(ctx, did)
	endorsements := tc.computeEndorsements(ctx, did, existing)
	credentials := tc.computeCredentials(ctx, did, existing)
	disputePenalty := tc.computeDisputePenalty(ctx, did, existing)

	// Weighted composite: max 1000.
	// Identity: 25% of 1000 = max 250
	// Account age: 15% of 1000 = max 150
	// Transactions: 20% of 1000 = max 200
	// Endorsements: 15% of 1000 = max 150
	// Credentials: 10% of 1000 = max 100
	// Dispute penalty: 15% of 1000 = max -150
	score := identity + accountAge + transactions + endorsements + credentials - disputePenalty

	if score < 0 {
		score = 0
	}
	if score > 1000 {
		score = 1000
	}

	ts := aicp.TrustScore{
		DID:   did,
		Score: score,
		Components: aicp.TrustScoreBreakdown{
			Identity:       identity,
			AccountAge:     accountAge,
			Transactions:   transactions,
			Endorsements:   endorsements,
			Credentials:    credentials,
			DisputePenalty: disputePenalty,
		},
		ComputedAt: time.Now().UTC(),
	}

	if err := tc.backend.UpdateTrustScore(ctx, ts); err != nil {
		return nil, fmt.Errorf("updating trust score: %w", err)
	}

	return &ts, nil
}

// GetScore returns the current trust score for a DID.
func (tc *TrustComputer) GetScore(ctx context.Context, did string) (int, error) {
	ts, err := tc.backend.GetTrustScore(ctx, did)
	if err != nil {
		if err == aicp.ErrNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("getting trust score: %w", err)
	}
	return ts.Score, nil
}

// computeIdentity: 25% weight, max 250 points.
// Based on DID resolution and verification method presence.
func (tc *TrustComputer) computeIdentity(ctx context.Context, did string) int {
	result, err := tc.backend.ResolveDID(ctx, did)
	if err != nil {
		return 0
	}
	if result.DocumentMetadata.Deactivated {
		return 0
	}

	score := 100 // Base for having a valid DID.
	if len(result.Document.VerificationMethod) > 0 {
		score += 100 // Has verification methods.
	}
	if len(result.Document.Service) > 0 {
		score += 50 // Has service endpoints.
	}

	if score > 250 {
		score = 250
	}
	return score
}

// computeAccountAge: 15% weight, max 150 points.
// Linear growth over first 365 days.
func (tc *TrustComputer) computeAccountAge(ctx context.Context, did string) int {
	result, err := tc.backend.ResolveDID(ctx, did)
	if err != nil {
		return 0
	}

	age := time.Since(result.DocumentMetadata.Created)
	days := int(age.Hours() / 24)

	if days >= 365 {
		return 150
	}
	return (days * 150) / 365
}

// computeTransactions: 20% weight, max 200 points.
// Based on successful payment receipt count.
func (tc *TrustComputer) computeTransactions(ctx context.Context, did string) int {
	receipts, err := tc.backend.ListReceiptsByAgent(ctx, did, aicp.PaginationOpts{Limit: 1})
	if err != nil {
		return 0
	}

	// Use HasMore as a proxy: if there are receipts at all, start scoring.
	if len(receipts.Items) == 0 {
		return 0
	}

	// Base score for having at least one transaction.
	return 100
}

// computeEndorsements: 15% weight, max 150 points.
func (tc *TrustComputer) computeEndorsements(_ context.Context, _ string, existing *aicp.TrustScore) int {
	if existing != nil {
		return existing.Components.Endorsements
	}
	return 0
}

// computeCredentials: 10% weight, max 100 points.
func (tc *TrustComputer) computeCredentials(_ context.Context, _ string, existing *aicp.TrustScore) int {
	if existing != nil {
		return existing.Components.Credentials
	}
	return 0
}

// computeDisputePenalty: 15% weight, max 150 points deduction.
func (tc *TrustComputer) computeDisputePenalty(_ context.Context, _ string, existing *aicp.TrustScore) int {
	if existing != nil {
		return existing.Components.DisputePenalty
	}
	return 0
}
