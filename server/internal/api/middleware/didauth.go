package middleware

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	// DIDContextKey is the context key for the authenticated DID.
	DIDContextKey contextKey = "aicp_did"
)

// DIDAuth returns middleware that extracts and validates DID-based authentication
// from the Authorization header (Bearer token with DID-Auth proof).
func DIDAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Expected format: Bearer <did-auth-token>
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]

			// Extract the DID from the token.
			// In a full implementation, this would verify the JWS token
			// and extract the issuer DID from the claims.
			// For now, we store the raw token; handlers that need auth
			// will perform full verification.
			ctx := context.WithValue(r.Context(), DIDContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAuthDID extracts the authenticated DID from the request context.
func GetAuthDID(ctx context.Context) (string, bool) {
	did, ok := ctx.Value(DIDContextKey).(string)
	return did, ok
}

// RequireAuth returns middleware that rejects unauthenticated requests.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := GetAuthDID(r.Context())
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
