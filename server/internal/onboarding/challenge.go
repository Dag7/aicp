package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	challengeTTL    = 5 * time.Minute
	challengePrefix = "aicp:challenge:"
)

// ChallengeService manages DID-Auth challenge-response flows.
type ChallengeService struct {
	redis  *redis.Client
	domain string
}

// NewChallengeService creates a new challenge service.
func NewChallengeService(rdb *redis.Client, domain string) *ChallengeService {
	return &ChallengeService{
		redis:  rdb,
		domain: domain,
	}
}

// Issue creates a new DID-Auth challenge for the given session.
func (cs *ChallengeService) Issue(ctx context.Context, sessionID string) (*aicp.DIDAuthChallenge, error) {
	challengeBytes := make([]byte, 32)
	if _, err := rand.Read(challengeBytes); err != nil {
		return nil, fmt.Errorf("generating challenge: %w", err)
	}

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	now := time.Now().UTC()
	challenge := &aicp.DIDAuthChallenge{
		Type:      aicp.ChallengeTypeDIDAuth,
		Challenge: hex.EncodeToString(challengeBytes),
		Domain:    cs.domain,
		Nonce:     hex.EncodeToString(nonceBytes),
		IssuedAt:  now,
		ExpiresAt: now.Add(challengeTTL),
	}

	// Store the challenge for later verification.
	key := challengePrefix + sessionID
	if err := cs.redis.Set(ctx, key, challenge.Challenge, challengeTTL).Err(); err != nil {
		return nil, fmt.Errorf("storing challenge: %w", err)
	}

	return challenge, nil
}

// Verify checks that the provided challenge value matches the stored one.
func (cs *ChallengeService) Verify(ctx context.Context, sessionID, challengeValue string) error {
	key := challengePrefix + sessionID
	stored, err := cs.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("challenge not found or expired")
		}
		return fmt.Errorf("retrieving challenge: %w", err)
	}

	if stored != challengeValue {
		return fmt.Errorf("challenge mismatch")
	}

	// Delete after successful verification (one-time use).
	cs.redis.Del(ctx, key)
	return nil
}
