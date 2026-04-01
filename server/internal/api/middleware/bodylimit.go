package middleware

import (
	"net/http"
)

// MaxBodySize is the default maximum request body size (1 MB).
const MaxBodySize int64 = 1 << 20

// BodyLimit returns middleware that restricts request body size.
// This prevents clients from sending excessively large payloads that could
// exhaust server memory. Returns 413 Payload Too Large on violation.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
