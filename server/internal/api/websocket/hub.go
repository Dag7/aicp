package websocket

import (
	"log/slog"
	"sync"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Hub manages WebSocket connections and broadcasts events to subscribed clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]*Client
	logger  *slog.Logger
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients: make(map[string]*Client),
		logger:  logger,
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	h.logger.Info("websocket client connected", "client_id", client.ID, "did", client.DID)
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client.ID)
	h.logger.Info("websocket client disconnected", "client_id", client.ID)
}

// Broadcast sends an event to connected clients. If the event has a Subject
// (a DID), it is only delivered to clients authenticated as that DID, preventing
// cross-DID information leakage. System-wide events (empty Subject) are broadcast
// to all subscribed clients.
func (h *Hub) Broadcast(event aicp.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if !client.IsSubscribed(event.Type) {
			continue
		}
		// If the event targets a specific DID, only deliver to that DID's clients.
		if event.Subject != "" && client.DID != event.Subject {
			continue
		}
		client.Send(event)
	}
}

// SendTo sends an event to a specific client by DID.
func (h *Hub) SendTo(did string, event aicp.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if client.DID == did {
			client.Send(event)
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
