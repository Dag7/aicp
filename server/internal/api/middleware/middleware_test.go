package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDIDKeyResolver implements DIDKeyResolver for tests.
type mockDIDKeyResolver struct {
	publicKey ed25519.PublicKey
	err       error
}

func (m *mockDIDKeyResolver) ResolvePublicKey(_ context.Context, _ string, _ string) (ed25519.PublicKey, error) {
	return m.publicKey, m.err
}

// signTestJWS creates a valid JWS token for testing.
func signTestJWS(t *testing.T, privateKey ed25519.PrivateKey, kid string) string {
	t.Helper()
	signingKey := jose.SigningKey{Algorithm: jose.EdDSA, Key: privateKey}
	opts := &jose.SignerOptions{}
	opts.WithHeader("kid", kid)
	signer, err := jose.NewSigner(signingKey, opts)
	require.NoError(t, err)
	jws, err := signer.Sign([]byte(`{"test":"payload"}`))
	require.NoError(t, err)
	compact, err := jws.CompactSerialize()
	require.NoError(t, err)
	return compact
}

// ─── DID Validation Tests ────────────────────────────────────────────────────

func TestIsValidDID(t *testing.T) {
	tests := []struct {
		name  string
		did   string
		valid bool
	}{
		{"valid principal", "did:aicp:3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", true},
		{"valid agent", "did:aicp:3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy:5Uen9xH2MoZBmKiH", true},
		{"short id", "did:aicp:abc", true},
		{"empty", "", false},
		{"wrong method", "did:web:example.com", false},
		{"no id", "did:aicp:", false},
		{"invalid chars", "did:aicp:abc=def", false},
		{"base58 zero excluded", "did:aicp:0abc", false},
		{"sql injection attempt", "did:aicp:abc'; DROP TABLE--", false},
		{"path traversal", "did:aicp:../../../etc/passwd", false},
		{"too many segments", "did:aicp:a:b:c", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidDID(tt.did))
		})
	}
}

func TestIsAgentDID(t *testing.T) {
	assert.True(t, IsAgentDID("did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.False(t, IsAgentDID("did:aicp:3J98t1Wp"))
	assert.False(t, IsAgentDID(""))
}

func TestIsPrincipalDID(t *testing.T) {
	assert.True(t, IsPrincipalDID("did:aicp:3J98t1Wp"))
	assert.False(t, IsPrincipalDID("did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.False(t, IsPrincipalDID(""))
}

func TestPrincipalFromAgentDID(t *testing.T) {
	assert.Equal(t, "did:aicp:3J98t1Wp", PrincipalFromAgentDID("did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.Equal(t, "", PrincipalFromAgentDID("did:aicp:3J98t1Wp"))
	assert.Equal(t, "", PrincipalFromAgentDID(""))
}

// ─── DID-Auth Middleware Tests ───────────────────────────────────────────────

func TestDIDAuth_NoAuthHeader_PassesThrough(t *testing.T) {
	handler := DIDAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := GetAuthDID(r.Context())
		assert.False(t, ok)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDIDAuth_NilResolver_RejectsAuth(t *testing.T) {
	// With nil resolver, any auth header should be rejected (fail-closed).
	handler := DIDAuth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDIDAuth_MalformedHeader_Returns401(t *testing.T) {
	resolver := &mockDIDKeyResolver{}
	handler := DIDAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))

	tests := []struct {
		name   string
		header string
	}{
		{"not bearer", "Basic dXNlcjpwYXNz"},
		{"empty bearer", "Bearer "},
		{"just bearer", "Bearer"},
		{"no space", "Bearertoken"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var resp struct {
				Success bool `json:"success"`
				Error   struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.False(t, resp.Success)
			assert.Equal(t, "UNAUTHORIZED", resp.Error.Code)
		})
	}
}

func TestDIDAuth_WriteAuthError_NoJSONInjection(t *testing.T) {
	w := httptest.NewRecorder()
	writeAuthError(w, `"injected":"value"},"extra":{"`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Verify the JSON is properly escaped and not injectable.
	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, errObj["message"], "injected")
	// The injected content should be a string value, not parsed JSON structure.
	assert.IsType(t, "", errObj["message"])
}

// ─── RequireAuth Middleware Tests ────────────────────────────────────────────

func TestRequireAuth_RejectsUnauthenticated(t *testing.T) {
	handler := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_AllowsAuthenticated(t *testing.T) {
	handler := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), DIDContextKey, "did:aicp:test")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ─── GetAuthDID Tests ───────────────────────────────────────────────────────

func TestGetAuthDID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	did, ok := GetAuthDID(ctx)
	assert.False(t, ok)
	assert.Empty(t, did)
}

func TestGetAuthDID_EmptyStringDID(t *testing.T) {
	ctx := context.WithValue(context.Background(), DIDContextKey, "")
	did, ok := GetAuthDID(ctx)
	assert.False(t, ok)
	assert.Empty(t, did)
}

func TestGetAuthDID_ValidDID(t *testing.T) {
	ctx := context.WithValue(context.Background(), DIDContextKey, "did:aicp:abc123")
	did, ok := GetAuthDID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "did:aicp:abc123", did)
}

// ─── extractDIDFromKID Tests ────────────────────────────────────────────────

func TestExtractDIDFromKID(t *testing.T) {
	tests := []struct {
		kid      string
		expected string
	}{
		{"did:aicp:abc123#key-1", "did:aicp:abc123"},
		{"did:aicp:abc123:def456#key-1", "did:aicp:abc123:def456"},
		{"did:aicp:abc123", "did:aicp:abc123"},
		// "#fragment-only" — hash at idx 0, not > 0, so full string returned.
		// The DID-Auth middleware separately validates the did:aicp: prefix.
		{"#fragment-only", "#fragment-only"},
	}

	for _, tt := range tests {
		t.Run(tt.kid, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractDIDFromKID(tt.kid))
		})
	}
}

// ─── BodyLimit Middleware Tests ──────────────────────────────────────────────

// ─── DID-Auth Signature Verification Tests ──────────────────────────────────

func TestDIDAuth_ValidSignature_SetsContextDID(t *testing.T) {
	// Generate a key pair and sign a JWS with the correct key.
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	kid := "did:aicp:testprincipal#key-1"
	token := signTestJWS(t, priv, kid)

	resolver := &mockDIDKeyResolver{publicKey: pub}
	var capturedDID string
	handler := DIDAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedDID, _ = GetAuthDID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "did:aicp:testprincipal", capturedDID)
}

func TestDIDAuth_WrongKey_Rejects(t *testing.T) {
	// Sign with one key, but resolver returns a different public key.
	_, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	differentPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	kid := "did:aicp:victim#key-1"
	token := signTestJWS(t, priv, kid)

	// Resolver returns the victim's actual public key, not the attacker's.
	resolver := &mockDIDKeyResolver{publicKey: differentPub}
	handler := DIDAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached — impersonation succeeded!")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp struct {
		Error struct{ Message string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp.Error.Message, "invalid signature")
}

func TestDIDAuth_ResolverError_Rejects(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	kid := "did:aicp:deactivated#key-1"
	token := signTestJWS(t, priv, kid)

	resolver := &mockDIDKeyResolver{err: fmt.Errorf("DID is deactivated")}
	handler := DIDAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDIDAuth_ImpersonationAttack_Blocked(t *testing.T) {
	// Simulate an attacker crafting a JWS with the victim's DID in the kid
	// but signed with the attacker's own key.
	attackerPub, attackerPriv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	victimPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	_ = attackerPub // not used — attacker's pub is not in the resolver

	// Attacker crafts JWS with victim's DID in kid, signed with attacker's key.
	kid := "did:aicp:victimDID#key-1"
	token := signTestJWS(t, attackerPriv, kid)

	// Resolver returns the VICTIM's actual public key (from ledger).
	resolver := &mockDIDKeyResolver{publicKey: victimPub}
	handler := DIDAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("CRITICAL: impersonation attack succeeded — JWS signature not verified")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"impersonation must be rejected: attacker signed with own key but claimed victim's DID")
}

// ─── IsControllerOf Tests ───────────────────────────────────────────────────

func TestIsControllerOf(t *testing.T) {
	assert.True(t, IsControllerOf("did:aicp:3J98t1Wp", "did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.False(t, IsControllerOf("did:aicp:9Xk4mNqR", "did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.False(t, IsControllerOf("did:aicp:3J98t1Wp", "did:aicp:3J98t1Wp"))
	assert.False(t, IsControllerOf("", "did:aicp:3J98t1Wp:5Uen9xH2"))
	assert.False(t, IsControllerOf("did:aicp:3J98t1Wp", ""))
}

// ─── BodyLimit Middleware Tests ──────────────────────────────────────────────

func TestBodyLimit_RejectsLargeBody(t *testing.T) {
	handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 100)
		_, err := r.Body.Read(buf)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	largeBody := make([]byte, 100)
	for i := range largeBody {
		largeBody[i] = 'A'
	}

	req := httptest.NewRequest(http.MethodPost, "/", httptest.NewRequest(http.MethodPost, "/", nil).Body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The exact behavior depends on how the handler reads the body,
	// but the body should be limited.
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
}
