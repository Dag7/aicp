package onboarding

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	rateLimitDIDKeyPrefix = "aicp:ratelimit:onboard:did:"
	rateLimitIPKeyPrefix  = "aicp:ratelimit:onboard:ip:"
	anomalySessionPrefix  = "aicp:anomaly:sessions:"
	maxSessionsPerWindow  = 10
)

// RateLimiter provides Redis-backed rate limiting for onboarding attempts.
type RateLimiter struct {
	redis     *redis.Client
	maxPerDID int
	maxPerIP  int
	window    time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(rdb *redis.Client, maxPerDID, maxPerIP int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:     rdb,
		maxPerDID: maxPerDID,
		maxPerIP:  maxPerIP,
		window:    window,
	}
}

// CheckDID checks if a DID has exceeded the onboarding rate limit.
func (rl *RateLimiter) CheckDID(ctx context.Context, did string) error {
	key := rateLimitDIDKeyPrefix + did
	return rl.check(ctx, key, rl.maxPerDID)
}

// CheckIP checks if an IP address has exceeded the onboarding rate limit.
func (rl *RateLimiter) CheckIP(ctx context.Context, ip string) error {
	key := rateLimitIPKeyPrefix + ip
	return rl.check(ctx, key, rl.maxPerIP)
}

// RecordAttempt increments the rate limit counter for a DID and IP.
func (rl *RateLimiter) RecordAttempt(ctx context.Context, did, ip string) error {
	pipe := rl.redis.Pipeline()

	didKey := rateLimitDIDKeyPrefix + did
	pipe.Incr(ctx, didKey)
	pipe.Expire(ctx, didKey, rl.window)

	ipKey := rateLimitIPKeyPrefix + ip
	pipe.Incr(ctx, ipKey)
	pipe.Expire(ctx, ipKey, rl.window)

	_, err := pipe.Exec(ctx)
	return err
}

func (rl *RateLimiter) check(ctx context.Context, key string, limit int) error {
	count, err := rl.redis.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("checking rate limit: %w", err)
	}
	if count >= limit {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
}

// AnomalyDetector performs basic anomaly detection on onboarding requests.
type AnomalyDetector struct {
	redis *redis.Client
}

// NewAnomalyDetector creates a new anomaly detector.
func NewAnomalyDetector(rdb *redis.Client) *AnomalyDetector {
	return &AnomalyDetector{redis: rdb}
}

// Check evaluates an onboarding request for suspicious patterns.
// Returns nil if the request is allowed, or an error describing the anomaly.
// Redis errors are treated as non-blocking — the request proceeds.
func (ad *AnomalyDetector) Check(ctx context.Context, did, ip string) error {
	recentKey := anomalySessionPrefix + did
	count, err := ad.redis.Incr(ctx, recentKey).Result()
	if err != nil {
		// Don't block the request on Redis errors; anomaly detection is best-effort.
		return nil
	}
	if count == 1 {
		if err := ad.redis.Expire(ctx, recentKey, time.Minute).Err(); err != nil {
			// Non-critical — the key will remain without TTL but won't cause harm.
			_ = err
		}
	}

	if count > maxSessionsPerWindow {
		return fmt.Errorf("anomaly detected: excessive session creation from DID %s", did)
	}

	return nil
}
