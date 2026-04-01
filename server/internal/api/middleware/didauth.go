package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-jose/go-jose/v4"
)

type contextKey string

const (
	// DIDContextKey is the context key for the authenticated DID.
	DIDContextKey contextKey = "aicp_did"

	// didAICPPrefix is the expected prefix for AICP DID key IDs.
	didAICPPrefix = "did:aicp:"
)

// DIDAuth returns middleware that extracts and validates DID-based authentication
// from the Authorization header. It parses the Bearer JWS token, extracts the
// kid header (e.g. "did:aicp:abc123#key-1"), derives the signer's DID, and
// stores it in the request context.
//
// If no Authorization header is present, the request continues unauthenticated --
// individual handlers decide whether authentication is required.
//
// If the header is present but the JWS token is malformed or the kid cannot
// be parsed into a valid DID, the middleware returns 401 Unauthorized.
func DIDAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				// No auth header: continue unauthenticated.
				next.ServeHTTP(w, r)
				return
			}

			// Expected format: Bearer <jws-compact-token>
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeAuthError(w, "malformed Authorization header")
				return
			}

			token := parts[1]
			if token == "" {
				writeAuthError(w, "empty bearer token")
				return
			}

			// Parse the JWS token to extract the kid header.
			// We accept EdDSA (the only algorithm used by AICP).
			parsed, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.EdDSA})
			if err != nil {
				writeAuthError(w, "malformed JWS token")
				return
			}

			if len(parsed.Signatures) == 0 {
				writeAuthError(w, "JWS token contains no signatures")
				return
			}

			kid := parsed.Signatures[0].Header.KeyID
			if kid == "" {
				writeAuthError(w, "JWS token missing kid header")
				return
			}

			// Extract the DID from the kid.
			// kid format: "did:aicp:<id>#key-fragment" → DID is everything before '#'.
			did := extractDIDFromKID(kid)
			if did == "" || !strings.HasPrefix(did, didAICPPrefix) {
				writeAuthError(w, "invalid DID in kid header")
				return
			}

			ctx := context.WithValue(r.Context(), DIDContextKey, did)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractDIDFromKID extracts the DID portion from a key ID.
// Example: "did:aicp:abc123#key-1" → "did:aicp:abc123"
// If there is no '#' fragment, the full kid is returned (it may itself be a DID).
func extractDIDFromKID(kid string) string {
	if idx := strings.IndexByte(kid, '#'); idx > 0 {
		return kid[:idx]
	}
	return kid
}

// writeAuthError writes a 401 Unauthorized JSON error response.
// Uses json.Marshal to prevent JSON injection via untrusted message content.
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	resp := struct {
		Success bool `json:"success"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Success: false,
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Code: "UNAUTHORIZED", Message: message},
	}
	data, _ := json.Marshal(resp)
	_, _ = w.Write(data)
}

// GetAuthDID extracts the authenticated DID from the request context.
func GetAuthDID(ctx context.Context) (string, bool) {
	did, ok := ctx.Value(DIDContextKey).(string)
	return did, ok && did != ""
}

// RequireAuth returns middleware that rejects unauthenticated requests.
func RequireAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := GetAuthDID(r.Context())
			if !ok {
				writeAuthError(w, "authentication required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
