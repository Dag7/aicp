// Package events implements the typed event system (Observer pattern) with
// Redis pub/sub bridging for cross-node event distribution.
package events

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// Handler is a function that processes an event.
type Handler func(event aicp.Event)

// Emitter is a typed event emitter that supports subscribe/publish with
// per-event-type handlers. It is safe for concurrent use.
type Emitter struct {
	mu       sync.RWMutex
	handlers map[aicp.EventType][]Handler
	logger   *slog.Logger
}

// NewEmitter creates a new event emitter.
func NewEmitter(logger *slog.Logger) *Emitter {
	return &Emitter{
		handlers: make(map[aicp.EventType][]Handler),
		logger:   logger,
	}
}

// Subscribe registers a handler for a specific event type.
// Returns an unsubscribe function.
func (e *Emitter) Subscribe(eventType aicp.EventType, handler Handler) func() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handlers[eventType] = append(e.handlers[eventType], handler)
	idx := len(e.handlers[eventType]) - 1

	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		handlers := e.handlers[eventType]
		if idx < len(handlers) {
			e.handlers[eventType] = append(handlers[:idx], handlers[idx+1:]...)
		}
	}
}

// SubscribeAll registers a handler for all event types.
func (e *Emitter) SubscribeAll(handler Handler) func() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Use empty event type as wildcard key.
	e.handlers["*"] = append(e.handlers["*"], handler)
	idx := len(e.handlers["*"]) - 1

	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		handlers := e.handlers["*"]
		if idx < len(handlers) {
			e.handlers["*"] = append(handlers[:idx], handlers[idx+1:]...)
		}
	}
}

// Emit publishes an event to all matching handlers.
func (e *Emitter) Emit(event aicp.Event) {
	e.mu.RLock()
	// Collect handlers while holding the lock.
	typeHandlers := make([]Handler, len(e.handlers[event.Type]))
	copy(typeHandlers, e.handlers[event.Type])

	wildcardHandlers := make([]Handler, len(e.handlers["*"]))
	copy(wildcardHandlers, e.handlers["*"])
	e.mu.RUnlock()

	for _, h := range typeHandlers {
		go e.safeCall(h, event)
	}

	for _, h := range wildcardHandlers {
		go e.safeCall(h, event)
	}
}

func (e *Emitter) safeCall(h Handler, event aicp.Event) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("event handler panicked",
				"event_type", event.Type,
				"event_id", event.EventID,
				"panic", r,
			)
		}
	}()
	h(event)
}

// MarshalEvent serializes an event to JSON.
func MarshalEvent(event aicp.Event) ([]byte, error) {
	return json.Marshal(event)
}

// UnmarshalEvent deserializes an event from JSON.
func UnmarshalEvent(data []byte) (*aicp.Event, error) {
	var event aicp.Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
