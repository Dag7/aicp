package discovery

import (
	"context"
	"encoding/json"
	"io"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// SearchService provides capability-based search over ANS records
// using PostgreSQL full-text search.
type SearchService struct {
	backend ledger.LedgerBackend
}

// NewSearchService creates a new ANS search service.
func NewSearchService(backend ledger.LedgerBackend) *SearchService {
	return &SearchService{backend: backend}
}

// Search performs a full-text search over ANS records.
func (ss *SearchService) Search(ctx context.Context, query string, opts aicp.PaginationOpts) (*aicp.PaginatedResult[aicp.ANSRecord], error) {
	return ss.backend.SearchANSRecords(ctx, query, opts)
}

// decodeJSONBody decodes a JSON body into the given destination.
func decodeJSONBody(body io.Reader, dest any) error {
	return json.NewDecoder(body).Decode(dest)
}
