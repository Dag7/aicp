package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── extractMethod ──────────────────────────────────────────────────────────

func TestExtractMethod_Valid(t *testing.T) {
	tests := []struct {
		did    string
		method string
	}{
		{"did:aicp:abc123", "aicp"},
		{"did:key:z6MkhaXg...", "key"},
		{"did:web:example.com", "web"},
		{"did:example:foo:bar", "example"},
	}
	for _, tc := range tests {
		method, err := extractMethod(tc.did)
		require.NoError(t, err)
		assert.Equal(t, tc.method, method)
	}
}

func TestExtractMethod_Invalid(t *testing.T) {
	tests := []struct {
		did string
	}{
		{"not-a-did"},
		{"did:"},
		{"did::missing"},
		{""},
	}
	for _, tc := range tests {
		_, err := extractMethod(tc.did)
		assert.Error(t, err, "expected error for DID: %s", tc.did)
	}
}

// ─── MultiMethodResolver ────────────────────────────────────────────────────

func TestMultiMethodResolver_RegisterAndResolve(t *testing.T) {
	resolver := NewMultiMethodResolver()
	resolver.Register(NewDIDKeyResolver())

	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	multibase := crypto.PublicKeyMultibase(kp.PublicKey)
	did := "did:key:" + multibase

	result, err := resolver.Resolve(context.Background(), did)
	require.NoError(t, err)
	assert.Equal(t, did, result.Document.ID)
	assert.Equal(t, did, result.Document.Controller)
}

func TestMultiMethodResolver_UnsupportedMethod(t *testing.T) {
	resolver := NewMultiMethodResolver()

	_, err := resolver.Resolve(context.Background(), "did:unknown:abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported DID method")
}

func TestMultiMethodResolver_Methods(t *testing.T) {
	resolver := NewMultiMethodResolver()
	assert.Empty(t, resolver.Methods())

	resolver.Register(NewDIDKeyResolver())
	assert.ElementsMatch(t, []string{"key"}, resolver.Methods())

	resolver.Register(NewDIDWebResolver())
	methods := resolver.Methods()
	assert.Len(t, methods, 2)
	assert.Contains(t, methods, "key")
	assert.Contains(t, methods, "web")
}

func TestMultiMethodResolver_OverwriteMethod(t *testing.T) {
	resolver := NewMultiMethodResolver()
	resolver.Register(NewDIDKeyResolver())
	resolver.Register(NewDIDKeyResolver()) // Register again
	assert.Len(t, resolver.Methods(), 1)
}

// ─── DIDKeyResolver ─────────────────────────────────────────────────────────

func TestDIDKeyResolver_Method(t *testing.T) {
	r := NewDIDKeyResolver()
	assert.Equal(t, "key", r.Method())
}

func TestDIDKeyResolver_Resolve(t *testing.T) {
	r := NewDIDKeyResolver()
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	multibase := crypto.PublicKeyMultibase(kp.PublicKey)
	did := "did:key:" + multibase

	result, err := r.Resolve(context.Background(), did)
	require.NoError(t, err)

	// Verify document structure.
	doc := result.Document
	assert.Equal(t, did, doc.ID)
	assert.Equal(t, did, doc.Controller)
	assert.Len(t, doc.VerificationMethod, 1)
	assert.Equal(t, "Ed25519VerificationKey2020", doc.VerificationMethod[0].Type)
	assert.Equal(t, did+"#"+multibase, doc.VerificationMethod[0].ID)
	assert.Contains(t, doc.Authentication, did+"#"+multibase)
	assert.Contains(t, doc.AssertionMethod, did+"#"+multibase)

	// Verify metadata.
	assert.NotNil(t, result.DocumentMetadata)
	assert.Equal(t, 1, result.DocumentMetadata.VersionID)
	assert.False(t, result.DocumentMetadata.Deactivated)
}

func TestDIDKeyResolver_Resolve_InvalidPrefix(t *testing.T) {
	r := NewDIDKeyResolver()
	_, err := r.Resolve(context.Background(), "did:aicp:abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid did:key format")
}

func TestDIDKeyResolver_Resolve_InvalidMultibase(t *testing.T) {
	r := NewDIDKeyResolver()
	_, err := r.Resolve(context.Background(), "did:key:notvalidmultibase")
	assert.Error(t, err)
}

func TestDIDKeyResolver_Deterministic(t *testing.T) {
	r := NewDIDKeyResolver()
	kp, err := crypto.GenerateEd25519KeyPair()
	require.NoError(t, err)

	did := "did:key:" + crypto.PublicKeyMultibase(kp.PublicKey)

	res1, err := r.Resolve(context.Background(), did)
	require.NoError(t, err)
	res2, err := r.Resolve(context.Background(), did)
	require.NoError(t, err)

	assert.Equal(t, res1.Document.ID, res2.Document.ID)
	assert.Equal(t, res1.Document.VerificationMethod[0].PublicKeyMultibase,
		res2.Document.VerificationMethod[0].PublicKeyMultibase)
}

// ─── DIDWebResolver ─────────────────────────────────────────────────────────

func TestDIDWebResolver_Method(t *testing.T) {
	r := NewDIDWebResolver()
	assert.Equal(t, "web", r.Method())
}

func TestDIDWebResolver_Resolve(t *testing.T) {
	// Set up a test HTTP server serving a DID document.
	doc := aicp.DIDDocument{
		Context:    []string{"https://www.w3.org/ns/did/v1"},
		ID:         "", // Will be set below after we know the server URL.
		Controller: "did:web:placeholder",
		VersionID:  1,
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer server.Close()

	// Construct the did:web from the server address.
	// httptest TLS servers use localhost:port — we need to work with the URL directly.
	// Since did:web requires HTTPS and domain matching, we test the URL conversion separately.
	r := &DIDWebResolver{client: server.Client()}

	// The server URL is like https://127.0.0.1:PORT
	// For this test we directly call with a crafted DID that maps to the server.
	// This validates the HTTP fetching logic even though domain won't match perfectly.
	host := strings.TrimPrefix(server.URL, "https://")
	encodedHost := strings.ReplaceAll(host, ":", "%3A")
	did := "did:web:" + encodedHost
	doc.ID = did
	doc.Controller = did

	result, err := r.Resolve(context.Background(), did)
	require.NoError(t, err)
	assert.Equal(t, did, result.Document.ID)
}

func TestDIDWebResolver_Resolve_NotFound(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	r := &DIDWebResolver{client: server.Client()}
	host := strings.TrimPrefix(server.URL, "https://")
	encodedHost := strings.ReplaceAll(host, ":", "%3A")
	did := "did:web:" + encodedHost

	_, err := r.Resolve(context.Background(), did)
	assert.Error(t, err)
	assert.ErrorIs(t, err, aicp.ErrNotFound)
}

func TestDIDWebResolver_Resolve_InvalidPrefix(t *testing.T) {
	r := NewDIDWebResolver()
	_, err := r.Resolve(context.Background(), "did:aicp:abc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid did:web format")
}

func TestDIDWebResolver_Resolve_IDMismatch(t *testing.T) {
	doc := aicp.DIDDocument{
		ID:        "did:web:wrong.com",
		VersionID: 1,
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer server.Close()

	r := &DIDWebResolver{client: server.Client()}
	host := strings.TrimPrefix(server.URL, "https://")
	encodedHost := strings.ReplaceAll(host, ":", "%3A")
	did := "did:web:" + encodedHost

	_, err := r.Resolve(context.Background(), did)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

// ─── didWebToURL ────────────────────────────────────────────────────────────

func TestDidWebToURL(t *testing.T) {
	tests := []struct {
		did string
		url string
	}{
		{"did:web:example.com", "https://example.com/.well-known/did.json"},
		{"did:web:example.com:path:to", "https://example.com/path/to/did.json"},
		{"did:web:example.com%3A8080", "https://example.com:8080/.well-known/did.json"},
		{"did:web:example.com:users:alice", "https://example.com/users/alice/did.json"},
	}
	for _, tc := range tests {
		url, err := didWebToURL(tc.did)
		require.NoError(t, err, "DID: %s", tc.did)
		assert.Equal(t, tc.url, url, "DID: %s", tc.did)
	}
}

func TestDidWebToURL_MissingDomain(t *testing.T) {
	_, err := didWebToURL("did:web:")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing domain")
}
