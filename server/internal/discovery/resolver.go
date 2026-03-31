package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Resolver implements tiered ANS resolution: local ledger, domain-based
// (/.well-known/aicp.json), and federated global lookup.
type Resolver struct {
	local  *ANSManager
	logger *slog.Logger
	client *http.Client
}

// NewResolver creates a new tiered ANS resolver.
func NewResolver(local *ANSManager, logger *slog.Logger) *Resolver {
	return &Resolver{
		local:  local,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Resolve attempts to find the ANS record for a given service DID using
// the tiered resolution strategy: local -> domain -> global.
func (r *Resolver) Resolve(ctx context.Context, serviceDID string) (*aicp.ANSRecord, error) {
	// Tier 1: Local ledger lookup.
	record, err := r.local.Get(ctx, serviceDID)
	if err == nil {
		return record, nil
	}
	if err != aicp.ErrNotFound {
		r.logger.Warn("local ANS lookup failed", "did", serviceDID, "error", err)
	}

	// Tier 2 and 3 would involve domain-based and federated resolution.
	// For now, only local resolution is implemented. Remote resolution
	// will be added when federation is enabled.

	return nil, fmt.Errorf("ANS record not found for %s", serviceDID)
}

// ResolveDomain fetches the ANS record from /.well-known/aicp.json at the given domain.
func (r *Resolver) ResolveDomain(ctx context.Context, domain string) (*aicp.ANSRecord, error) {
	url := fmt.Sprintf("https://%s/.well-known/aicp.json", domain)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching .well-known/aicp.json from %s: %w", domain, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, domain)
	}

	var record aicp.ANSRecord
	if err := decodeJSONBody(resp.Body, &record); err != nil {
		return nil, fmt.Errorf("decoding ANS record from %s: %w", domain, err)
	}

	return &record, nil
}
