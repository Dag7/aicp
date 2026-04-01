package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit returns middleware that enforces per-IP rate limiting via Redis.
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ip := r.RemoteAddr

			key := fmt.Sprintf("aicp:ratelimit:api:%s", ip)
			count, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				// If Redis is down, allow the request through.
				next.ServeHTTP(w, r)
				return
			}

			if count == 1 {
				rdb.Expire(ctx, key, window)
			}

			if count > int64(maxRequests) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprintf(w, `{"success":false,"error":{"code":"RATE_LIMITED","message":"rate limit exceeded"}}`)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
