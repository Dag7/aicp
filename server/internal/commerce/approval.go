package commerce

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/internal/events"
	"github.com/aicp-protocol/aicp/server/internal/identity"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Default approval TTL is 15 minutes.
const defaultApprovalTTL = 15 * time.Minute

// ApprovalStatus represents the state of a pending approval.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalDenied   ApprovalStatus = "denied"
	ApprovalExpired  ApprovalStatus = "expired"
)

// PendingApproval represents a payment that requires principal approval.
type PendingApproval struct {
	ApprovalID   string           `json:"approval_id"`
	AgentDID     string           `json:"agent_did"`
	PrincipalDID string           `json:"principal_did"`
	Amount       aicp.MoneyAmount `json:"amount"`
	Purpose      string           `json:"purpose"`
	TokenJTI     string           `json:"token_jti"`
	Status       ApprovalStatus   `json:"status"`
	DenyReason   string           `json:"deny_reason,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	ExpiresAt    time.Time        `json:"expires_at"`
	DecidedAt    *time.Time       `json:"decided_at,omitempty"`
	DecidedBy    string           `json:"decided_by,omitempty"`
}

// ApprovalService manages the human approval workflow for high-value transactions.
type ApprovalService struct {
	rdb        *redis.Client
	delegation *identity.DelegationVerifier
	emitter    *events.Emitter
	logger     *slog.Logger
	ttl        time.Duration
}

// NewApprovalService creates a new approval service.
func NewApprovalService(
	rdb *redis.Client,
	delegation *identity.DelegationVerifier,
	emitter *events.Emitter,
	logger *slog.Logger,
) *ApprovalService {
	return &ApprovalService{
		rdb:        rdb,
		delegation: delegation,
		emitter:    emitter,
		logger:     logger,
		ttl:        defaultApprovalTTL,
	}
}

// CheckApprovalRequired checks if a payment amount exceeds the RequiresApprovalAbove
// threshold for the given agent. Returns (needsApproval, threshold, error).
func (as *ApprovalService) CheckApprovalRequired(ctx context.Context, agentDID string, amount aicp.MoneyAmount) (bool, *aicp.MoneyAmount, error) {
	spendingAuth, err := as.delegation.VerifySpendingAuthorization(ctx, agentDID)
	if err != nil {
		// If no spending authorization found, no approval threshold is set.
		as.logger.Debug("no spending authorization for approval check", "agent_did", agentDID, "error", err)
		return false, nil, nil
	}

	if spendingAuth.RequiresApprovalAbove == nil {
		return false, nil, nil
	}

	threshold := spendingAuth.RequiresApprovalAbove

	txAmount, ok := new(big.Float).SetString(amount.Value)
	if !ok {
		return false, nil, fmt.Errorf("invalid amount value: %s", amount.Value)
	}

	thresholdVal, ok := new(big.Float).SetString(threshold.Value)
	if !ok {
		return false, nil, fmt.Errorf("invalid threshold value: %s", threshold.Value)
	}

	// Only require approval if currencies match and amount exceeds threshold.
	if amount.Currency != threshold.Currency {
		return false, threshold, nil
	}

	needsApproval := txAmount.Cmp(thresholdVal) > 0
	return needsApproval, threshold, nil
}

// RequestApproval creates a pending approval request and emits an event for the principal.
func (as *ApprovalService) RequestApproval(ctx context.Context, req PendingApproval) (*PendingApproval, error) {
	now := time.Now().UTC()

	req.ApprovalID = fmt.Sprintf("appr_%s", crypto.SHA256Hex([]byte(req.AgentDID + req.TokenJTI + now.String()))[:16])
	req.Status = ApprovalPending
	req.CreatedAt = now
	req.ExpiresAt = now.Add(as.ttl)

	if err := as.save(ctx, &req); err != nil {
		return nil, fmt.Errorf("saving approval: %w", err)
	}

	// Add to the principal's pending set.
	setKey := as.principalSetKey(req.PrincipalDID)
	if err := as.rdb.SAdd(ctx, setKey, req.ApprovalID).Err(); err != nil {
		return nil, fmt.Errorf("adding to principal set: %w", err)
	}
	if err := as.rdb.Expire(ctx, setKey, as.ttl).Err(); err != nil {
		as.logger.Warn("failed to set TTL on principal set", "error", err)
	}

	// Emit approval required event.
	as.emitEvent(aicp.EventPaymentApprovalRequired, req.PrincipalDID, map[string]any{
		"approval_id":   req.ApprovalID,
		"agent_did":     req.AgentDID,
		"principal_did": req.PrincipalDID,
		"amount":        req.Amount,
		"purpose":       req.Purpose,
		"expires_at":    req.ExpiresAt,
	})

	as.logger.Info("approval requested",
		"approval_id", req.ApprovalID,
		"agent_did", req.AgentDID,
		"principal_did", req.PrincipalDID,
		"amount", req.Amount.Value,
		"currency", req.Amount.Currency,
	)

	return &req, nil
}

// Approve marks an approval as approved. Must be called by the principal.
func (as *ApprovalService) Approve(ctx context.Context, approvalID, principalDID string) error {
	approval, err := as.GetApproval(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("getting approval: %w", err)
	}

	if approval.PrincipalDID != principalDID {
		return aicp.ErrForbidden
	}
	if approval.Status != ApprovalPending {
		return fmt.Errorf("approval is not pending: current status is %s", approval.Status)
	}
	if time.Now().After(approval.ExpiresAt) {
		approval.Status = ApprovalExpired
		if err := as.save(ctx, approval); err != nil {
			as.logger.Error("failed to mark approval expired", "error", err, "approval_id", approvalID)
		}
		return aicp.ErrApprovalExpired
	}

	now := time.Now().UTC()
	approval.Status = ApprovalApproved
	approval.DecidedAt = &now
	approval.DecidedBy = principalDID

	if err := as.save(ctx, approval); err != nil {
		return fmt.Errorf("saving approved state: %w", err)
	}

	as.removePrincipalEntry(ctx, approval.PrincipalDID, approvalID)

	as.emitEvent(aicp.EventPaymentApprovalDecided, approval.PrincipalDID, map[string]any{
		"approval_id": approvalID,
		"status":      string(ApprovalApproved),
		"decided_by":  principalDID,
	})

	as.logger.Info("approval approved", "approval_id", approvalID, "decided_by", principalDID)
	return nil
}

// Deny marks an approval as denied.
func (as *ApprovalService) Deny(ctx context.Context, approvalID, principalDID, reason string) error {
	approval, err := as.GetApproval(ctx, approvalID)
	if err != nil {
		return fmt.Errorf("getting approval: %w", err)
	}

	if approval.PrincipalDID != principalDID {
		return aicp.ErrForbidden
	}
	if approval.Status != ApprovalPending {
		return fmt.Errorf("approval is not pending: current status is %s", approval.Status)
	}

	now := time.Now().UTC()
	approval.Status = ApprovalDenied
	approval.DenyReason = reason
	approval.DecidedAt = &now
	approval.DecidedBy = principalDID

	if err := as.save(ctx, approval); err != nil {
		return fmt.Errorf("saving denied state: %w", err)
	}

	as.removePrincipalEntry(ctx, approval.PrincipalDID, approvalID)

	as.emitEvent(aicp.EventPaymentApprovalDecided, approval.PrincipalDID, map[string]any{
		"approval_id": approvalID,
		"status":      string(ApprovalDenied),
		"decided_by":  principalDID,
		"reason":      reason,
	})

	as.logger.Info("approval denied", "approval_id", approvalID, "decided_by", principalDID, "reason", reason)
	return nil
}

// GetApproval retrieves a pending approval by ID.
func (as *ApprovalService) GetApproval(ctx context.Context, approvalID string) (*PendingApproval, error) {
	data, err := as.rdb.Get(ctx, as.approvalKey(approvalID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("getting approval: %w", err)
	}

	var approval PendingApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return nil, fmt.Errorf("unmarshaling approval: %w", err)
	}

	// Mark expired if still pending past the deadline.
	if approval.Status == ApprovalPending && time.Now().After(approval.ExpiresAt) {
		approval.Status = ApprovalExpired
		if err := as.save(ctx, &approval); err != nil {
			as.logger.Error("failed to mark approval expired", "error", err, "approval_id", approvalID)
		}
		as.removePrincipalEntry(ctx, approval.PrincipalDID, approvalID)
	}

	return &approval, nil
}

// ListPendingForPrincipal returns all pending approvals for a principal.
func (as *ApprovalService) ListPendingForPrincipal(ctx context.Context, principalDID string) ([]PendingApproval, error) {
	setKey := as.principalSetKey(principalDID)
	ids, err := as.rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("listing principal approvals: %w", err)
	}

	var pending []PendingApproval
	for _, id := range ids {
		approval, err := as.GetApproval(ctx, id)
		if err != nil {
			// Remove stale entries.
			as.removePrincipalEntry(ctx, principalDID, id)
			continue
		}
		if approval.Status != ApprovalPending {
			as.removePrincipalEntry(ctx, principalDID, id)
			continue
		}
		pending = append(pending, *approval)
	}

	return pending, nil
}

// ─── Internal Helpers ────────────────────────────────────────────────────────

func (as *ApprovalService) save(ctx context.Context, approval *PendingApproval) error {
	data, err := json.Marshal(approval)
	if err != nil {
		return fmt.Errorf("marshaling approval: %w", err)
	}

	ttl := time.Until(approval.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute // Keep briefly for lookups after expiry.
	}

	return as.rdb.Set(ctx, as.approvalKey(approval.ApprovalID), data, ttl).Err()
}

func (as *ApprovalService) approvalKey(approvalID string) string {
	return "aicp:approval:" + approvalID
}

func (as *ApprovalService) principalSetKey(principalDID string) string {
	return "aicp:approvals:principal:" + principalDID
}

func (as *ApprovalService) removePrincipalEntry(ctx context.Context, principalDID, approvalID string) {
	if err := as.rdb.SRem(ctx, as.principalSetKey(principalDID), approvalID).Err(); err != nil {
		as.logger.Warn("failed to remove approval from principal set", "error", err, "approval_id", approvalID)
	}
}

func (as *ApprovalService) emitEvent(eventType aicp.EventType, subject string, data map[string]any) {
	if as.emitter == nil {
		return
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		as.logger.Error("failed to marshal event data", "error", err)
		return
	}

	as.emitter.Emit(aicp.Event{
		EventID:   fmt.Sprintf("evt_%s", crypto.SHA256Hex([]byte(fmt.Sprintf("%s:%s:%d", eventType, subject, time.Now().UnixNano())))[:16]),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Source:    "commerce.approval",
		Subject:   subject,
		Data:      dataJSON,
	})
}
