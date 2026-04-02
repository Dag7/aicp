package middleware

import (
	"context"
	"crypto/ed25519"
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

// DIDKeyResolver resolves a DID to the Ed25519 public key for JWS signature verification.
// Implementations must return the public key associated with the given keyID
// (e.g. "did:aicp:abc123#key-1") from the resolved DID document.
type DIDKeyResolver interface {
	ResolvePublicKey(ctx context.Context, did string, keyID string) (ed25519.PublicKey, error)
}

// DIDAuth returns middleware that authenticates requests via DID-based JWS Bearer tokens.
// It parses the JWS, extracts the kid header, resolves the signer's public key from
// the DID document, and **cryptographically verifies the JWS signature**. Only after
// successful verification is the DID stored in the request context.
//
// If resolver is nil, the middleware rejects all authenticated requests (fail-closed).
//
// If no Authorization header is present, the request continues unauthenticated --
// individual handlers decide whether authentication is required.
func DIDAuth(resolver DIDKeyResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				// No auth header: continue unauthenticated.
				next.ServeHTTP(w, r)
				return
			}

			// Fail-closed: if no resolver is configured, reject all auth attempts.
			if resolver == nil {
				writeAuthError(w, "authentication not configured")
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

			// Resolve the public key from the DID document and verify the JWS signature.
			pubKey, err := resolver.ResolvePublicKey(r.Context(), did, kid)
			if err != nil {
				writeAuthError(w, "failed to resolve signer identity")
				return
			}

			// Cryptographically verify the JWS signature against the resolved public key.
			if _, err := parsed.Verify(pubKey); err != nil {
				writeAuthError(w, "invalid signature")
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
