package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// SessionManager manages onboarding sessions backed by Redis.
type SessionManager struct {
	redis      *redis.Client
	sessionTTL time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(rdb *redis.Client, sessionTTL time.Duration) *SessionManager {
	return &SessionManager{
		redis:      rdb,
		sessionTTL: sessionTTL,
	}
}

// Create starts a new onboarding session.
func (sm *SessionManager) Create(ctx context.Context, agentDID, serviceDID string) (*aicp.OnboardingState, error) {
	if agentDID == "" || serviceDID == "" {
		return nil, fmt.Errorf("agentDID and serviceDID are required")
	}
	now := time.Now().UTC()
	sessionID := fmt.Sprintf("sess_%s", crypto.SHA256Hex([]byte(agentDID + serviceDID + now.String()))[:16])

	state := &aicp.OnboardingState{
		SessionID:  sessionID,
		Phase:      aicp.PhaseInitiated,
		AgentDID:   agentDID,
		ServiceDID: serviceDID,
		StartedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  now.Add(sm.sessionTTL),
	}

	if err := sm.save(ctx, state); err != nil {
		return nil, fmt.Errorf("saving session: %w", err)
	}

	return state, nil
}

// Get retrieves a session by ID.
func (sm *SessionManager) Get(ctx context.Context, sessionID string) (*aicp.OnboardingState, error) {
	key := sm.key(sessionID)
	data, err := sm.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, aicp.ErrNotFound
		}
		return nil, fmt.Errorf("getting session: %w", err)
	}

	var state aicp.OnboardingState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	return &state, nil
}

// Update saves an updated session state.
func (sm *SessionManager) Update(ctx context.Context, state *aicp.OnboardingState) error {
	return sm.save(ctx, state)
}

// Delete removes a session.
func (sm *SessionManager) Delete(ctx context.Context, sessionID string) error {
	return sm.redis.Del(ctx, sm.key(sessionID)).Err()
}

func (sm *SessionManager) save(ctx context.Context, state *aicp.OnboardingState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	ttl := time.Until(state.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute // Keep expired sessions briefly for error reporting.
	}

	return sm.redis.Set(ctx, sm.key(state.SessionID), data, ttl).Err()
}

func (sm *SessionManager) key(sessionID string) string {
	return "aicp:session:" + sessionID
}
