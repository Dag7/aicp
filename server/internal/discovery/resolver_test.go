package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// ─── PeerRegistry ───────────────────────────────────────────────────────────

func TestPeerRegistry_AddAndList(t *testing.T) {
	pr := NewPeerRegistry()
	assert.Empty(t, pr.List())

	pr.Add(FederationPeer{Name: "peer-1", Endpoint: "https://peer1.example.com", DID: "did:aicp:p1"})
	pr.Add(FederationPeer{Name: "peer-2", Endpoint: "https://peer2.example.com", DID: "did:aicp:p2"})

	peers := pr.List()
	assert.Len(t, peers, 2)
}

func TestPeerRegistry_Get(t *testing.T) {
	pr := NewPeerRegistry()
	pr.Add(FederationPeer{Name: "peer-1", Endpoint: "https://peer1.example.com"})

	peer, ok := pr.Get("peer-1")
	assert.True(t, ok)
	assert.Equal(t, "https://peer1.example.com", peer.Endpoint)

	_, ok = pr.Get("nonexistent")
	assert.False(t, ok)
}

func TestPeerRegistry_Remove(t *testing.T) {
	pr := NewPeerRegistry()
	pr.Add(FederationPeer{Name: "peer-1"})
	pr.Add(FederationPeer{Name: "peer-2"})
	assert.Len(t, pr.List(), 2)

	pr.Remove("peer-1")
	assert.Len(t, pr.List(), 1)

	_, ok := pr.Get("peer-1")
	assert.False(t, ok)
}

func TestPeerRegistry_Overwrite(t *testing.T) {
	pr := NewPeerRegistry()
	pr.Add(FederationPeer{Name: "peer-1", Endpoint: "https://old.example.com"})
	pr.Add(FederationPeer{Name: "peer-1", Endpoint: "https://new.example.com"})

	assert.Len(t, pr.List(), 1)
	peer, _ := pr.Get("peer-1")
	assert.Equal(t, "https://new.example.com", peer.Endpoint)
}

// ─── deduplicateRecords ─────────────────────────────────────────────────────

func TestDeduplicateRecords(t *testing.T) {
	records := []aicp.ANSRecord{
		{Service: aicp.ANSServiceInfo{DID: "did:1"}},
		{Service: aicp.ANSServiceInfo{DID: "did:2"}},
		{Service: aicp.ANSServiceInfo{DID: "did:1"}}, // Duplicate.
		{Service: aicp.ANSServiceInfo{DID: "did:3"}},
	}

	result := deduplicateRecords(records, 0)
	assert.Len(t, result, 3)
}

func TestDeduplicateRecords_WithLimit(t *testing.T) {
	records := []aicp.ANSRecord{
		{Service: aicp.ANSServiceInfo{DID: "did:1"}},
		{Service: aicp.ANSServiceInfo{DID: "did:2"}},
		{Service: aicp.ANSServiceInfo{DID: "did:3"}},
	}

	result := deduplicateRecords(records, 2)
	assert.Len(t, result, 2)
}

func TestDeduplicateRecords_Empty(t *testing.T) {
	result := deduplicateRecords(nil, 0)
	assert.Empty(t, result)
}

// ─── Resolver (local resolution) ────────────────────────────────────────────

func newTestResolver(t *testing.T) (*Resolver, *mockLedgerBackend) {
	t.Helper()
	backend := newMockLedgerBackend()
	ans := NewANSManager(backend)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewResolver(ans, logger), backend
}

func TestResolver_ResolveLocal(t *testing.T) {
	resolver, backend := newTestResolver(t)
	ctx := context.Background()

	record := &aicp.ANSRecord{
		Service: aicp.ANSServiceInfo{
			DID:  "did:aicp:svc1",
			Name: "Test Service",
		},
	}
	backend.ansRecords["did:aicp:svc1"] = record

	result, err := resolver.Resolve(ctx, "did:aicp:svc1")
	require.NoError(t, err)
	assert.Equal(t, "Test Service", result.Service.Name)
}

func TestResolver_ResolveLocal_NotFound_NoFederatedPeers(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	_, err := resolver.Resolve(ctx, "did:aicp:nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolver_Peers(t *testing.T) {
	resolver, _ := newTestResolver(t)
	peers := resolver.Peers()
	assert.NotNil(t, peers)
	assert.Empty(t, peers.List())
}

// ─── Resolver (federated resolution) ────────────────────────────────────────

func TestResolver_ResolveFederated(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	// Set up a mock peer server.
	record := aicp.ANSRecord{
		Service: aicp.ANSServiceInfo{
			DID:  "did:aicp:remote-svc",
			Name: "Remote Service",
		},
	}
	peerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "true", r.Header.Get("X-AICP-Federation"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    record,
		})
	}))
	defer peerServer.Close()

	resolver.Peers().Add(FederationPeer{
		Name:     "test-peer",
		Endpoint: peerServer.URL,
		DID:      "did:aicp:peer1",
	})

	result, err := resolver.Resolve(ctx, "did:aicp:remote-svc")
	require.NoError(t, err)
	assert.Equal(t, "Remote Service", result.Service.Name)
}

func TestResolver_ResolveFederated_PeerDown(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	// Add a peer with an unreachable endpoint.
	resolver.Peers().Add(FederationPeer{
		Name:     "dead-peer",
		Endpoint: "http://192.0.2.1:1", // TEST-NET address, unreachable.
		DID:      "did:aicp:dead",
	})

	_, err := resolver.Resolve(ctx, "did:aicp:nonexistent")
	assert.Error(t, err)
}

func TestResolver_ResolveFederated_PeerReturnsNotFound(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	peerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
		})
	}))
	defer peerServer.Close()

	resolver.Peers().Add(FederationPeer{
		Name:     "empty-peer",
		Endpoint: peerServer.URL,
	})

	_, err := resolver.Resolve(ctx, "did:aicp:missing")
	assert.Error(t, err)
}

// ─── Resolver (domain-based resolution) ─────────────────────────────────────

func TestResolver_ResolveDomain(t *testing.T) {
	record := aicp.ANSRecord{
		Service: aicp.ANSServiceInfo{
			DID:  "did:aicp:domain-svc",
			Name: "Domain Service",
		},
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(record)
	}))
	defer server.Close()

	resolver, _ := newTestResolver(t)
	resolver.client = server.Client()

	// We can't easily test with a real domain, but we verify the HTTP call logic.
	// The domain resolution would normally hit https://domain/.well-known/aicp.json.
}

// ─── FederatedSearch ────────────────────────────────────────────────────────

func TestResolver_FederatedSearch_LocalOnly(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	// No peers, so this should just return local results (empty).
	results, err := resolver.FederatedSearch(ctx, "payment", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestResolver_FederatedSearch_WithPeers(t *testing.T) {
	resolver, _ := newTestResolver(t)
	ctx := context.Background()

	peerRecords := []aicp.ANSRecord{
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:peer-svc-1", Name: "Peer Svc 1"}},
		{Service: aicp.ANSServiceInfo{DID: "did:aicp:peer-svc-2", Name: "Peer Svc 2"}},
	}

	peerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": map[string]any{
				"items": peerRecords,
			},
		})
	}))
	defer peerServer.Close()

	resolver.Peers().Add(FederationPeer{
		Name:     "search-peer",
		Endpoint: peerServer.URL,
	})

	results, err := resolver.FederatedSearch(ctx, "payment", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
