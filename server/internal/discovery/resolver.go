package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Resolver implements tiered ANS resolution: local ledger, domain-based
// (/.well-known/aicp.json), and federated global lookup.
type Resolver struct {
	local  *ANSManager
	peers  *PeerRegistry
	logger *slog.Logger
	client *http.Client
}

// NewResolver creates a new tiered ANS resolver.
func NewResolver(local *ANSManager, logger *slog.Logger) *Resolver {
	return &Resolver{
		local:  local,
		peers:  NewPeerRegistry(),
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Resolve attempts to find the ANS record for a given service DID using
// the tiered resolution strategy: local -> domain -> federated.
func (r *Resolver) Resolve(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	// Tier 1: Local ledger lookup.
	record, err := r.local.Get(ctx, serviceDID)
	if err == nil {
		return record, nil
	}
	if err != aicp.ErrNotFound {
		r.logger.Warn("local ANS lookup failed", "did", serviceDID, "error", err)
	}

	// Tier 2: Domain-based resolution (/.well-known/aicp.json).
	// Attempted when DID contains domain hints in service endpoints.

	// Tier 3: Federated resolution across known peers.
	record, err = r.resolveFederated(ctx, serviceDID)
	if err == nil {
		return record, nil
	}

	return nil, fmt.Errorf("ANS record not found for %s", serviceDID)
}

// resolveFederated queries all registered federation peers for the ANS record.
func (r *Resolver) resolveFederated(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	peers := r.peers.List()
	if len(peers) == 0 {
		return nil, aicp.ErrNotFound
	}

	type result struct {
		record *aicp.ANSRecord
		err    error
	}

	results := make(chan result, len(peers))

	for _, peer := range peers {
		go func(p FederationPeer) {
			rec, err := r.queryPeer(ctx, p, serviceDID)
			results <- result{record: rec, err: err}
		}(peer)
	}

	for range peers {
		res := <-results
		if res.err == nil && res.record != nil {
			return res.record, nil
		}
	}

	return nil, aicp.ErrNotFound
}

// maxPeerResponseSize is the maximum response body size accepted from federation peers.
const maxPeerResponseSize = 1 << 20 // 1 MiB

// queryPeer queries a single federation peer for an ANS record.
func (r *Resolver) queryPeer(ctx context.Context, peer FederationPeer, serviceDID string) (*aicp.ANSRecord, error) {
	peerURL := fmt.Sprintf("%s/aicp/v1/discover/%s", peer.Endpoint, url.PathEscape(serviceDID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, peerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating peer request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-AICP-Federation", "true")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Debug("federation peer unreachable", "peer", peer.Name, "error", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s returned %d", peer.Name, resp.StatusCode)
	}

	var envelope struct {
		Success bool           `json:"success"`
		Data    aicp.ANSRecord `json:"data"`
	}
	if err := decodeLimitedJSONBody(resp.Body, &envelope); err != nil {
		return nil, fmt.Errorf("decoding peer response: %w", err)
	}
	if !envelope.Success {
		return nil, aicp.ErrNotFound
	}

	return &envelope.Data, nil
}

// FederatedSearch queries local and all federation peers for ANS records matching a capability.
func (r *Resolver) FederatedSearch(ctx context.Context, capability string, limit int) ([]aicp.ANSRecord, error) {
	localResults, err := r.local.backend.SearchANSRecords(ctx, capability, aicp.PaginationOpts{Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("local search: %w", err)
	}

	records := localResults.Items

	peers := r.peers.List()
	if len(peers) == 0 {
		return records, nil
	}

	type peerResult struct {
		records []aicp.ANSRecord
	}

	results := make(chan peerResult, len(peers))

	for _, peer := range peers {
		go func(p FederationPeer) {
			peerRecords, err := r.searchPeer(ctx, p, capability, limit)
			if err != nil {
				r.logger.Debug("federated search failed", "peer", p.Name, "error", err)
				results <- peerResult{}
				return
			}
			results <- peerResult{records: peerRecords}
		}(peer)
	}

	for range peers {
		res := <-results
		records = append(records, res.records...)
	}

	return deduplicateRecords(records, limit), nil
}

// searchPeer queries a single peer for ANS search results.
func (r *Resolver) searchPeer(ctx context.Context, peer FederationPeer, capability string, limit int) ([]aicp.ANSRecord, error) {
	searchURL := fmt.Sprintf("%s/aicp/v1/discover/search?capability=%s&limit=%d", peer.Endpoint, url.QueryEscape(capability), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-AICP-Federation", "true")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer %s returned %d", peer.Name, resp.StatusCode)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Items []aicp.ANSRecord `json:"items"`
		} `json:"data"`
	}
	if err := decodeLimitedJSONBody(resp.Body, &envelope); err != nil {
		return nil, fmt.Errorf("decoding peer search response: %w", err)
	}

	return envelope.Data.Items, nil
}

// Peers returns the federation peer registry for external configuration.
func (r *Resolver) Peers() *PeerRegistry {
	return r.peers
}

// ResolveDomain fetches the ANS record from /.well-known/aicp.json at the given domain.
// The domain must be a valid hostname without path, query, or fragment components.
func (r *Resolver) ResolveDomain(ctx context.Context, domain string) (*aicp.ANSRecord, error) {
	if err := validateDomain(domain); err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	wellKnownURL := fmt.Sprintf("https://%s/.well-known/aicp.json", domain)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching .well-known/aicp.json from %s: %w", domain, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, domain)
	}

	var record aicp.ANSRecord
	if err := decodeLimitedJSONBody(resp.Body, &record); err != nil {
		return nil, fmt.Errorf("decoding ANS record from %s: %w", domain, err)
	}

	return &record, nil
}

// validateDomain checks that a domain string is a valid hostname
// without path traversal, query, or fragment components.
func validateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain is empty")
	}
	if strings.ContainsAny(domain, "/:@?#") {
		return fmt.Errorf("domain contains invalid characters")
	}
	if strings.Contains(domain, "..") {
		return fmt.Errorf("domain contains path traversal")
	}
	return nil
}

// ─── Federation Peer Registry ───────────────────────────────────────────────

// FederationPeer represents a known AICP deployment for federated discovery.
type FederationPeer struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"` // Base URL, e.g., "https://aicp.example.com"
	DID      string `json:"did"`      // Peer's service DID
}

// PeerRegistry manages known federation peers (thread-safe).
type PeerRegistry struct {
	mu    sync.RWMutex
	peers map[string]FederationPeer
}

// NewPeerRegistry creates an empty peer registry.
func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		peers: make(map[string]FederationPeer),
	}
}

// Add registers a federation peer.
func (pr *PeerRegistry) Add(peer FederationPeer) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.peers[peer.Name] = peer
}

// Remove unregisters a federation peer by name.
func (pr *PeerRegistry) Remove(name string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	delete(pr.peers, name)
}

// List returns all registered federation peers.
func (pr *PeerRegistry) List() []FederationPeer {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	peers := make([]FederationPeer, 0, len(pr.peers))
	for _, p := range pr.peers {
		peers = append(peers, p)
	}
	return peers
}

// Get returns a specific federation peer by name.
func (pr *PeerRegistry) Get(name string) (FederationPeer, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	p, ok := pr.peers[name]
	return p, ok
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// deduplicateRecords removes duplicate ANS records by service DID, keeping first seen.
func deduplicateRecords(records []aicp.ANSRecord, limit int) []aicp.ANSRecord {
	seen := make(map[string]bool, len(records))
	result := make([]aicp.ANSRecord, 0, len(records))
	for _, r := range records {
		if !seen[r.Service.DID] {
			seen[r.Service.DID] = true
			result = append(result, r)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}
