package events

import (
	"log/slog"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// WebSocketHub is the interface the bridge needs from the WebSocket package.
// Defining it here avoids a circular import between events and api/websocket.
type WebSocketHub interface {
	Broadcast(event aicp.Event)
}

// WebSocketBridge forwards emitted events to a WebSocket hub so that
// connected clients receive real-time notifications for protocol events.
type WebSocketBridge struct {
	emitter *Emitter
	hub     WebSocketHub
	logger  *slog.Logger
}

// NewWebSocketBridge creates a bridge between the event emitter and the
// WebSocket hub.
func NewWebSocketBridge(emitter *Emitter, hub WebSocketHub, logger *slog.Logger) *WebSocketBridge {
	return &WebSocketBridge{
		emitter: emitter,
		hub:     hub,
		logger:  logger,
	}
}

// Start subscribes to all events on the emitter and forwards each one
// to the WebSocket hub's Broadcast method. It should be called once
// during server initialization, after both the emitter and hub are ready.
func (b *WebSocketBridge) Start() {
	b.emitter.SubscribeAll(func(event aicp.Event) {
		b.logger.Debug("forwarding event to websocket hub",
			"event_id", event.EventID,
			"event_type", event.Type,
		)
		b.hub.Broadcast(event)
	})

	b.logger.Info("websocket bridge started, forwarding all events to hub")
}
