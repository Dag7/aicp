package discovery

import (
	"context"
	"fmt"
	"sort"

	"github.com/aicp-protocol/aicp/server/internal/ledger"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// RankingService applies trust-based and relevance ranking to ANS search results.
type RankingService struct {
	backend ledger.LedgerBackend
}

// NewRankingService creates a new ranking service.
func NewRankingService(backend ledger.LedgerBackend) *RankingService {
	return &RankingService{backend: backend}
}

// RankedResult wraps an ANS record with its ranking score.
type RankedResult struct {
	Record     aicp.ANSRecord `json:"record"`
	TrustScore int            `json:"trust_score"`
	Rank       float64        `json:"rank"`
}

// RankResults takes raw search results and re-ranks them by combining
// text relevance (position-based) with trust scores.
func (rs *RankingService) RankResults(ctx context.Context, records []aicp.ANSRecord) ([]RankedResult, error) {
	results := make([]RankedResult, 0, len(records))

	for i, rec := range records {
		trustScore := 0
		ts, err := rs.backend.GetTrustScore(ctx, rec.Service.DID)
		if err == nil {
			trustScore = ts.Score
		}

		// Combine text relevance (inverse position) with trust score.
		// Text relevance weight: 60%, Trust score weight: 40%.
		textRelevance := 1.0 - (float64(i) / float64(len(records)+1))
		normalizedTrust := float64(trustScore) / 1000.0
		rank := 0.6*textRelevance + 0.4*normalizedTrust

		results = append(results, RankedResult{
			Record:     rec,
			TrustScore: trustScore,
			Rank:       rank,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Rank > results[j].Rank
	})

	return results, nil
}

// FilterByMinTrustScore removes results below a minimum trust score.
func FilterByMinTrustScore(results []RankedResult, minScore int) []RankedResult {
	filtered := make([]RankedResult, 0, len(results))
	for _, r := range results {
		if r.TrustScore >= minScore {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// FilterByCategory removes results that don't match any of the specified categories.
func FilterByCategory(results []RankedResult, categories []string) []RankedResult {
	if len(categories) == 0 {
		return results
	}

	catSet := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		catSet[c] = struct{}{}
	}

	filtered := make([]RankedResult, 0, len(results))
	for _, r := range results {
		for _, cat := range r.Record.Service.Category {
			if _, ok := catSet[cat]; ok {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// FormatSearchResponse converts ranked results to a paginated response.
func FormatSearchResponse(results []RankedResult, limit int, offset int) *aicp.PaginatedResult[aicp.ANSRecord] {
	if limit <= 0 {
		limit = aicp.DefaultPageLimit
	}

	total := len(results)
	end := offset + limit
	if end > total {
		end = total
	}

	page := results[offset:end]
	items := make([]aicp.ANSRecord, len(page))
	for i, r := range page {
		items[i] = r.Record
	}

	result := &aicp.PaginatedResult[aicp.ANSRecord]{
		Items:   items,
		HasMore: end < total,
	}
	if result.HasMore {
		result.NextCursor = fmt.Sprintf("%d", end)
	}
	return result
}
