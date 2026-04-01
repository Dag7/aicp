package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// rateLimitScript is an atomic Lua script for rate limiting.
// It increments the counter and sets the expiry in a single atomic operation,
// preventing the race condition where a crash between INCR and EXPIRE leaves
// a key without TTL (persisting forever).
var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
	redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// RateLimit returns middleware that enforces per-IP rate limiting via Redis.
// Uses an atomic Lua script to prevent INCR/EXPIRE race conditions.
// Returns standard rate limit headers per draft-ietf-httpapi-ratelimit-headers.
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	windowSecs := int(window.Seconds())

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ip := r.RemoteAddr

			key := fmt.Sprintf("aicp:ratelimit:api:%s", ip)
			count, err := rateLimitScript.Run(ctx, rdb, []string{key}, windowSecs).Int64()
			if err != nil {
				// If Redis is down, allow the request through (fail-open).
				next.ServeHTTP(w, r)
				return
			}

			remaining := int64(maxRequests) - count
			if remaining < 0 {
				remaining = 0
			}

			// Set rate limit headers on every response.
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(maxRequests))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("RateLimit-Policy", fmt.Sprintf("%d;w=%d", maxRequests, windowSecs))

			if count > int64(maxRequests) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(windowSecs))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprintf(w, `{"success":false,"error":{"code":"RATE_LIMITED","message":"rate limit exceeded"}}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
