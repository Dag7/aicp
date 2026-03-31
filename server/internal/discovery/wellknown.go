package discovery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// WellKnownService generates the /.well-known/aicp.json response
// for a given service.
type WellKnownService struct {
	ansMgr *ANSManager
}

// NewWellKnownService creates a new well-known service.
func NewWellKnownService(ansMgr *ANSManager) *WellKnownService {
	return &WellKnownService{ansMgr: ansMgr}
}

// Generate returns the JSON bytes for /.well-known/aicp.json for the given service.
func (wk *WellKnownService) Generate(ctx context.Context, serviceDID string) ([]byte, error) {
	record, err := wk.ansMgr.Get(ctx, serviceDID)
	if err != nil {
		return nil, fmt.Errorf("getting ANS record: %w", err)
	}

	return json.Marshal(record)
}

// GenerateFromRecord serializes an ANS record to JSON.
func GenerateFromRecord(record aicp.ANSRecord) ([]byte, error) {
	return json.Marshal(record)
}
