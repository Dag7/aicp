package websocket

import (
	"github.com/aicp-protocol/aicp/server/internal/events"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// EventBridge connects the event emitter to the WebSocket hub,
// forwarding relevant events to connected clients.
type EventBridge struct {
	hub     *Hub
	emitter *events.Emitter
}

// NewEventBridge creates a new event-to-WebSocket bridge.
func NewEventBridge(hub *Hub, emitter *events.Emitter) *EventBridge {
	return &EventBridge{
		hub:     hub,
		emitter: emitter,
	}
}

// Start subscribes to all events and forwards them to WebSocket clients.
// Returns an unsubscribe function.
func (eb *EventBridge) Start() func() {
	return eb.emitter.SubscribeAll(func(event aicp.Event) {
		eb.hub.Broadcast(event)
	})
}
