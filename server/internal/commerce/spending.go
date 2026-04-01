package commerce

import (
	"context"
	"fmt"
	"math/big"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// parseBigFloat parses a string into a *big.Float, returning zero on failure.
func parseBigFloat(s string) *big.Float {
	val, ok := new(big.Float).SetString(s)
	if !ok {
		return new(big.Float)
	}
	return val
}

// SpendingTracker enforces per-transaction, daily, and monthly spending limits.
type SpendingTracker struct {
	backend ledger.LedgerBackend
}

// NewSpendingTracker creates a new spending tracker.
func NewSpendingTracker(backend ledger.LedgerBackend) *SpendingTracker {
	return &SpendingTracker{backend: backend}
}

// CheckLimits verifies that the given amount doesn't exceed spending limits.
// Returns true if the payment is allowed.
func (st *SpendingTracker) CheckLimits(ctx context.Context, did string, amount aicp.MoneyAmount) (bool, error) {
	// Check daily limit.
	dailyStr, err := st.backend.GetDailySpending(ctx, did, amount.Currency)
	if err != nil {
		return false, fmt.Errorf("getting daily spending: %w", err)
	}

	daily := parseBigFloat(dailyStr)

	txAmount, ok := new(big.Float).SetString(amount.Value)
	if !ok {
		return false, fmt.Errorf("invalid amount value: %s", amount.Value)
	}

	newDaily := new(big.Float).Add(daily, txAmount)

	// Default daily limit (can be overridden per-agent via spending authorization).
	defaultDailyLimit := parseBigFloat(aicp.DefaultDailySpendingLimit)
	if newDaily.Cmp(defaultDailyLimit) > 0 {
		return false, nil
	}

	// Check monthly limit.
	monthlyStr, err := st.backend.GetMonthlySpending(ctx, did, amount.Currency)
	if err != nil {
		return false, fmt.Errorf("getting monthly spending: %w", err)
	}

	monthly := parseBigFloat(monthlyStr)
	newMonthly := new(big.Float).Add(monthly, txAmount)

	// Default monthly limit.
	defaultMonthlyLimit := parseBigFloat(aicp.DefaultMonthlySpendingLimit)
	if newMonthly.Cmp(defaultMonthlyLimit) > 0 {
		return false, nil
	}

	return true, nil
}

// CheckCustomLimits verifies spending against custom per-agent limits from
// a SpendingAuthorization credential.
func (st *SpendingTracker) CheckCustomLimits(ctx context.Context, did string, amount aicp.MoneyAmount, limits aicp.SpendingLimits) (bool, error) {
	// Check per-transaction limit.
	txAmount, ok := new(big.Float).SetString(amount.Value)
	if !ok {
		return false, fmt.Errorf("invalid amount: %s", amount.Value)
	}

	perTxLimit, ok := new(big.Float).SetString(limits.PerTransaction.Value)
	if !ok {
		return false, fmt.Errorf("invalid per-transaction limit: %s", limits.PerTransaction.Value)
	}
	if txAmount.Cmp(perTxLimit) > 0 {
		return false, nil
	}

	// Check daily limit.
	dailyStr, err := st.backend.GetDailySpending(ctx, did, amount.Currency)
	if err != nil {
		return false, fmt.Errorf("getting daily spending: %w", err)
	}
	daily := parseBigFloat(dailyStr)
	newDaily := new(big.Float).Add(daily, txAmount)

	dailyLimit, ok := new(big.Float).SetString(limits.Daily.Value)
	if !ok {
		return false, fmt.Errorf("invalid daily limit: %s", limits.Daily.Value)
	}
	if newDaily.Cmp(dailyLimit) > 0 {
		return false, nil
	}

	// Check monthly limit.
	monthlyStr, err := st.backend.GetMonthlySpending(ctx, did, amount.Currency)
	if err != nil {
		return false, fmt.Errorf("getting monthly spending: %w", err)
	}
	monthly := parseBigFloat(monthlyStr)
	newMonthly := new(big.Float).Add(monthly, txAmount)

	monthlyLimit, ok := new(big.Float).SetString(limits.Monthly.Value)
	if !ok {
		return false, fmt.Errorf("invalid monthly limit: %s", limits.Monthly.Value)
	}
	if newMonthly.Cmp(monthlyLimit) > 0 {
		return false, nil
	}

	return true, nil
}

// RecordSpending records a spending event.
func (st *SpendingTracker) RecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount) error {
	return st.backend.RecordSpending(ctx, did, amount)
}

// AtomicCheckAndRecordSpending atomically verifies that the given amount does
// not exceed spending limits and records it in a single operation. This
// eliminates the TOCTOU race between separate CheckLimits/RecordSpending calls
// where concurrent requests could both pass the check before either records.
// Returns true if the spending was within limits and recorded.
func (st *SpendingTracker) AtomicCheckAndRecordSpending(ctx context.Context, did string, amount aicp.MoneyAmount) (bool, error) {
	return st.backend.CheckAndRecordSpending(
		ctx, did, amount,
		aicp.DefaultDailySpendingLimit,
		aicp.DefaultMonthlySpendingLimit,
	)
}

// AtomicCheckAndRecordCustomSpending atomically verifies that the given amount
// does not exceed custom per-agent spending limits and records it in a single
// operation. The per-transaction limit is checked locally since it does not
// require coordination with the backend.
func (st *SpendingTracker) AtomicCheckAndRecordCustomSpending(ctx context.Context, did string, amount aicp.MoneyAmount, limits aicp.SpendingLimits) (bool, error) {
	// Per-transaction limit is stateless — check it before hitting the backend.
	txAmount, ok := new(big.Float).SetString(amount.Value)
	if !ok {
		return false, fmt.Errorf("invalid amount: %s", amount.Value)
	}
	perTxLimit, ok := new(big.Float).SetString(limits.PerTransaction.Value)
	if !ok {
		return false, fmt.Errorf("invalid per-transaction limit: %s", limits.PerTransaction.Value)
	}
	if txAmount.Cmp(perTxLimit) > 0 {
		return false, nil
	}

	return st.backend.CheckAndRecordSpending(
		ctx, did, amount,
		limits.Daily.Value,
		limits.Monthly.Value,
	)
}

// GetDailySummary returns the current daily spending total.
func (st *SpendingTracker) GetDailySummary(ctx context.Context, did, currency string) (string, error) {
	return st.backend.GetDailySpending(ctx, did, currency)
}

// GetMonthlySummary returns the current monthly spending total.
func (st *SpendingTracker) GetMonthlySummary(ctx context.Context, did, currency string) (string, error) {
	return st.backend.GetMonthlySpending(ctx, did, currency)
}
