package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	// defaultTimeout is the HTTP client timeout for validator requests.
	defaultTimeout = 10 * time.Second

	// maxResponseBytes caps response body reads to prevent resource exhaustion.
	maxResponseBytes = 10 * 1024 * 1024 // 10 MiB
)

// ── HTTPValidatorSigner ──────────────────────────────────────────────────────

// HTTPValidatorSigner implements ledger.ValidatorSigner by calling a remote
// validator's HTTP transport endpoint. Each instance targets a single peer.
type HTTPValidatorSigner struct {
	baseURL string
	client  *http.Client
}

// Compile-time check: ensure HTTPValidatorSigner satisfies the interface.
// The interface lives in package ledger which we cannot import here without
// creating a cycle. The caller wires this up via consensus.RegisterSigner.

// NewHTTPValidatorSigner creates a signer that reaches the validator at baseURL.
// baseURL should be the root of the validator transport API, e.g.
// "http://validator-2.aicp:8080/internal/validator".
func NewHTTPValidatorSigner(baseURL string) *HTTPValidatorSigner {
	return &HTTPValidatorSigner{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// NewHTTPValidatorSignerWithClient creates a signer with a custom HTTP client
// (useful for TLS configuration or testing).
func NewHTTPValidatorSignerWithClient(baseURL string, client *http.Client) *HTTPValidatorSigner {
	return &HTTPValidatorSigner{
		baseURL: baseURL,
		client:  client,
	}
}

// RequestSignature sends the block hash and proposed block to a remote
// validator and returns the resulting signature.
func (s *HTTPValidatorSigner) RequestSignature(ctx context.Context, blockHash []byte, proposed *aicp.ProposedBlock) (*aicp.ValidatorSig, error) {
	req := SignatureRequest{
		BlockHash:     string(blockHash),
		ProposedBlock: ProposedBlockToWire(proposed),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling signature request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/sign", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating signature HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending signature request to %s: %w", s.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signature request to %s returned status %d", s.baseURL, resp.StatusCode)
	}

	var sigResp SignatureResponse
	if err := decodeResponse(resp.Body, &sigResp); err != nil {
		return nil, fmt.Errorf("decoding signature response: %w", err)
	}

	return &aicp.ValidatorSig{
		ValidatorDID: sigResp.ValidatorDID,
		Signature:    sigResp.Signature,
	}, nil
}

// ── HTTPBlockFetcher ─────────────────────────────────────────────────────────

// HTTPBlockFetcher implements ledger.BlockFetcher by calling a remote
// validator's HTTP transport endpoint.
type HTTPBlockFetcher struct {
	baseURL string
	client  *http.Client
}

// NewHTTPBlockFetcher creates a fetcher that reaches the validator at baseURL.
func NewHTTPBlockFetcher(baseURL string) *HTTPBlockFetcher {
	return &HTTPBlockFetcher{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second, // blocks response can be large
		},
	}
}

// NewHTTPBlockFetcherWithClient creates a fetcher with a custom HTTP client.
func NewHTTPBlockFetcherWithClient(baseURL string, client *http.Client) *HTTPBlockFetcher {
	return &HTTPBlockFetcher{
		baseURL: baseURL,
		client:  client,
	}
}

// FetchBlocks retrieves finalized blocks from a remote peer starting at fromBlock.
func (f *HTTPBlockFetcher) FetchBlocks(ctx context.Context, fromBlock uint64) ([]aicp.FinalizedBlock, error) {
	req := FetchBlocksRequest{FromBlock: fromBlock}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling fetch blocks request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/blocks", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating fetch blocks HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fetching blocks from %s: %w", f.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch blocks from %s returned status %d", f.baseURL, resp.StatusCode)
	}

	var blocksResp FetchBlocksResponse
	if err := decodeResponse(resp.Body, &blocksResp); err != nil {
		return nil, fmt.Errorf("decoding fetch blocks response: %w", err)
	}

	result := make([]aicp.FinalizedBlock, len(blocksResp.Blocks))
	for i, wb := range blocksResp.Blocks {
		result[i] = FinalizedBlockFromWire(wb)
	}

	return result, nil
}

// LatestBlockNumber retrieves the latest block number from the remote peer.
func (f *HTTPBlockFetcher) LatestBlockNumber(ctx context.Context) (uint64, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/latest", nil)
	if err != nil {
		return 0, fmt.Errorf("creating latest block HTTP request: %w", err)
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("getting latest block from %s: %w", f.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("latest block from %s returned status %d", f.baseURL, resp.StatusCode)
	}

	var latestResp LatestBlockResponse
	if err := decodeResponse(resp.Body, &latestResp); err != nil {
		return 0, fmt.Errorf("decoding latest block response: %w", err)
	}

	return latestResp.BlockNumber, nil
}

// Healthy checks if the remote peer's validator transport is responsive.
func (f *HTTPBlockFetcher) Healthy(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("creating health HTTP request: %w", err)
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check to %s: %w", f.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check to %s returned status %d", f.baseURL, resp.StatusCode)
	}

	var healthResp HealthResponse
	if err := decodeResponse(resp.Body, &healthResp); err != nil {
		return fmt.Errorf("decoding health response: %w", err)
	}

	if !healthResp.Healthy {
		return fmt.Errorf("remote validator at %s reports unhealthy", f.baseURL)
	}

	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// decodeResponse reads a capped number of bytes from the body and decodes JSON.
func decodeResponse(body io.Reader, dest any) error {
	limited := io.LimitReader(body, maxResponseBytes)
	return json.NewDecoder(limited).Decode(dest)
}
