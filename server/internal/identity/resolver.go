// Package identity - resolver.go provides a pluggable DID resolution framework
// supporting multiple DID methods (did:aicp, did:key, did:web) via the Strategy pattern.
package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// DIDMethodResolver is the Strategy interface for resolving DIDs of a specific method.
// Each DID method (aicp, key, web) implements this interface.
type DIDMethodResolver interface {
	// Method returns the DID method name (e.g., "aicp", "key", "web").
	Method() string

	// Resolve resolves a DID and returns the DID Document.
	Resolve(ctx context.Context, did string) (*aicp.DIDResolutionResult, error)
}

// MultiMethodResolver chains multiple DID method resolvers. It inspects the DID
// method prefix and delegates to the appropriate resolver (Strategy pattern).
type MultiMethodResolver struct {
	mu        sync.RWMutex
	resolvers map[string]DIDMethodResolver
}

// NewMultiMethodResolver creates a resolver with no methods registered.
func NewMultiMethodResolver() *MultiMethodResolver {
	return &MultiMethodResolver{
		resolvers: make(map[string]DIDMethodResolver),
	}
}

// Register adds a DID method resolver. Overwrites if the method already exists.
func (r *MultiMethodResolver) Register(resolver DIDMethodResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolvers[resolver.Method()] = resolver
}

// Resolve parses the DID method prefix and delegates to the appropriate resolver.
func (r *MultiMethodResolver) Resolve(ctx context.Context, did string) (*aicp.DIDResolutionResult, error) {
	method, err := extractMethod(did)
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	resolver, ok := r.resolvers[method]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unsupported DID method: %s", method)
	}

	return resolver.Resolve(ctx, did)
}

// Methods returns a list of all registered DID methods.
func (r *MultiMethodResolver) Methods() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	methods := make([]string, 0, len(r.resolvers))
	for m := range r.resolvers {
		methods = append(methods, m)
	}
	return methods
}

// ─── did:key Resolver ───────────────────────────────────────────────────────

// DIDKeyResolver resolves did:key DIDs. did:key is a stateless method where
// the DID Document is derived entirely from the public key encoded in the DID.
// Format: did:key:z6Mk... (multibase-encoded Ed25519 public key)
type DIDKeyResolver struct{}

// NewDIDKeyResolver creates a new did:key resolver.
func NewDIDKeyResolver() *DIDKeyResolver {
	return &DIDKeyResolver{}
}

func (r *DIDKeyResolver) Method() string { return "key" }

func (r *DIDKeyResolver) Resolve(_ context.Context, did string) (*aicp.DIDResolutionResult, error) {
	if !strings.HasPrefix(did, "did:key:") {
		return nil, fmt.Errorf("invalid did:key format: %s", did)
	}

	multibaseKey := did[len("did:key:"):]
	pubKey, err := crypto.PublicKeyFromMultibase(multibaseKey)
	if err != nil {
		return nil, fmt.Errorf("decoding did:key public key: %w", err)
	}

	// Build the DID Document from the public key.
	keyID := did + "#" + multibaseKey
	now := time.Now().UTC()

	doc := aicp.DIDDocument{
		Context: []string{
			"https://www.w3.org/ns/did/v1",
			"https://w3id.org/security/suites/ed25519-2020/v1",
		},
		ID:         did,
		Controller: did,
		VerificationMethod: []aicp.VerificationMethod{
			{
				ID:                 keyID,
				Type:               "Ed25519VerificationKey2020",
				Controller:         did,
				PublicKeyMultibase: crypto.PublicKeyMultibase(ed25519.PublicKey(pubKey)),
			},
		},
		Authentication:  []string{keyID},
		AssertionMethod: []string{keyID},
		Created:         now,
		Updated:         now,
		VersionID:       1,
	}

	return &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   now,
			Updated:   now,
			VersionID: 1,
		},
	}, nil
}

// ─── did:web Resolver ───────────────────────────────────────────────────────

// DIDWebResolver resolves did:web DIDs by fetching the DID Document from the
// domain's .well-known path. Format: did:web:example.com -> https://example.com/.well-known/did.json
// did:web:example.com:path:to -> https://example.com/path/to/did.json
type DIDWebResolver struct {
	client *http.Client
}

// NewDIDWebResolver creates a new did:web resolver with a default HTTP client.
func NewDIDWebResolver() *DIDWebResolver {
	return &DIDWebResolver{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *DIDWebResolver) Method() string { return "web" }

func (r *DIDWebResolver) Resolve(ctx context.Context, did string) (*aicp.DIDResolutionResult, error) {
	if !strings.HasPrefix(did, "did:web:") {
		return nil, fmt.Errorf("invalid did:web format: %s", did)
	}

	url, err := didWebToURL(did)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating did:web request: %w", err)
	}
	req.Header.Set("Accept", "application/did+ld+json, application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching did:web document from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, aicp.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return nil, fmt.Errorf("reading did:web document: %w", err)
	}

	var doc aicp.DIDDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("decoding did:web document: %w", err)
	}

	// Verify the document ID matches the requested DID.
	if doc.ID != did {
		return nil, fmt.Errorf("did:web document ID mismatch: got %s, expected %s", doc.ID, did)
	}

	return &aicp.DIDResolutionResult{
		Document: &doc,
		DocumentMetadata: &aicp.DIDDocumentMetadata{
			Created:   doc.Created,
			Updated:   doc.Updated,
			VersionID: doc.VersionID,
		},
	}, nil
}

// didWebToURL converts a did:web DID to the URL where the DID Document can be fetched.
// did:web:example.com → https://example.com/.well-known/did.json
// did:web:example.com:path:to → https://example.com/path/to/did.json
func didWebToURL(did string) (string, error) {
	methodSpecific := did[len("did:web:"):]
	if methodSpecific == "" {
		return "", fmt.Errorf("did:web missing domain")
	}

	// Replace : path separators with / and decode percent-encoded characters.
	parts := strings.Split(methodSpecific, ":")
	domain := strings.ReplaceAll(parts[0], "%3A", ":")

	if len(parts) == 1 {
		return fmt.Sprintf("https://%s/.well-known/did.json", domain), nil
	}

	path := strings.Join(parts[1:], "/")
	return fmt.Sprintf("https://%s/%s/did.json", domain, path), nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// extractMethod parses the DID method from a DID string.
func extractMethod(did string) (string, error) {
	if !strings.HasPrefix(did, "did:") {
		return "", fmt.Errorf("invalid DID: must start with 'did:'")
	}
	parts := strings.SplitN(did, ":", 3)
	if len(parts) < 3 || parts[1] == "" {
		return "", fmt.Errorf("invalid DID: missing method")
	}
	return parts[1], nil
}
