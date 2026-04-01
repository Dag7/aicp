package websocket

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ─── Hub Tests ──────────────────────────────────────────────────────────────

func TestHub_NewHub(t *testing.T) {
	hub := NewHub(testLogger())
	assert.NotNil(t, hub)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub(testLogger())
	client := &Client{
		ID:            "client-1",
		DID:           "did:aicp:test1",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}

	hub.Register(client)
	assert.Equal(t, 1, hub.ClientCount())

	hub.Unregister(client)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHub_RegisterMultiple(t *testing.T) {
	hub := NewHub(testLogger())

	for i := range 5 {
		client := &Client{
			ID:            "client-" + string(rune('0'+i)),
			DID:           "did:aicp:test",
			hub:           hub,
			send:          make(chan aicp.Event, sendBuffer),
			subscriptions: make(map[aicp.EventType]bool),
		}
		hub.Register(client)
	}

	assert.Equal(t, 5, hub.ClientCount())
}

func TestHub_Broadcast_SubscribedClients(t *testing.T) {
	hub := NewHub(testLogger())

	sub := &Client{
		ID:            "subscriber",
		DID:           "did:aicp:sub",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: map[aicp.EventType]bool{aicp.EventPaymentSettled: true},
	}
	unsub := &Client{
		ID:            "non-subscriber",
		DID:           "did:aicp:nosub",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}

	hub.Register(sub)
	hub.Register(unsub)

	event := aicp.Event{Type: aicp.EventPaymentSettled, Source: "test"}
	hub.Broadcast(event)

	// Subscriber should receive the event.
	assert.Len(t, sub.send, 1)
	received := <-sub.send
	assert.Equal(t, aicp.EventPaymentSettled, received.Type)

	// Non-subscriber should not.
	assert.Len(t, unsub.send, 0)
}

func TestHub_Broadcast_WildcardSubscription(t *testing.T) {
	hub := NewHub(testLogger())

	client := &Client{
		ID:            "wildcard",
		DID:           "did:aicp:wild",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: map[aicp.EventType]bool{"*": true},
	}

	hub.Register(client)

	hub.Broadcast(aicp.Event{Type: aicp.EventDIDCreated})
	hub.Broadcast(aicp.Event{Type: aicp.EventPaymentSettled})

	assert.Len(t, client.send, 2)
}

func TestHub_SendTo_ByDID(t *testing.T) {
	hub := NewHub(testLogger())

	target := &Client{
		ID:            "target",
		DID:           "did:aicp:target",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}
	other := &Client{
		ID:            "other",
		DID:           "did:aicp:other",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}

	hub.Register(target)
	hub.Register(other)

	event := aicp.Event{Type: aicp.EventPaymentAuthorized, Subject: "did:aicp:target"}
	hub.SendTo("did:aicp:target", event)

	assert.Len(t, target.send, 1)
	assert.Len(t, other.send, 0)
}

func TestHub_SendTo_MultipleSameDID(t *testing.T) {
	hub := NewHub(testLogger())

	c1 := &Client{
		ID:            "conn-1",
		DID:           "did:aicp:shared",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}
	c2 := &Client{
		ID:            "conn-2",
		DID:           "did:aicp:shared",
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		subscriptions: make(map[aicp.EventType]bool),
	}

	hub.Register(c1)
	hub.Register(c2)

	hub.SendTo("did:aicp:shared", aicp.Event{Type: aicp.EventDIDUpdated})

	assert.Len(t, c1.send, 1)
	assert.Len(t, c2.send, 1)
}

// ─── Client Tests ───────────────────────────────────────────────────────────

func TestClient_Subscribe(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: make(map[aicp.EventType]bool),
	}

	client.Subscribe(aicp.EventPaymentSettled)
	assert.True(t, client.IsSubscribed(aicp.EventPaymentSettled))
	assert.False(t, client.IsSubscribed(aicp.EventDIDCreated))
}

func TestClient_Unsubscribe(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: map[aicp.EventType]bool{aicp.EventPaymentSettled: true},
	}

	assert.True(t, client.IsSubscribed(aicp.EventPaymentSettled))

	client.Unsubscribe(aicp.EventPaymentSettled)
	assert.False(t, client.IsSubscribed(aicp.EventPaymentSettled))
}

func TestClient_IsSubscribed_Wildcard(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: map[aicp.EventType]bool{"*": true},
	}

	assert.True(t, client.IsSubscribed(aicp.EventPaymentSettled))
	assert.True(t, client.IsSubscribed(aicp.EventDIDCreated))
	assert.True(t, client.IsSubscribed(aicp.EventKeyRotated))
}

func TestClient_Send_BufferFull(t *testing.T) {
	client := &Client{
		ID:     "test",
		send:   make(chan aicp.Event, 1),
		logger: testLogger(),
	}

	// First send should succeed.
	client.Send(aicp.Event{Type: aicp.EventPaymentSettled})
	assert.Len(t, client.send, 1)

	// Second send should drop (buffer full) — should not block.
	client.Send(aicp.Event{Type: aicp.EventDIDCreated})
	assert.Len(t, client.send, 1) // Still 1.
}

func TestClient_HandleMessage_Subscribe(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: make(map[aicp.EventType]bool),
		logger:        testLogger(),
	}

	client.handleMessage([]byte(`{"action":"subscribe","event_type":"payment.settled"}`))
	assert.True(t, client.IsSubscribed(aicp.EventPaymentSettled))
}

func TestClient_HandleMessage_Unsubscribe(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: map[aicp.EventType]bool{aicp.EventPaymentSettled: true},
		logger:        testLogger(),
	}

	client.handleMessage([]byte(`{"action":"unsubscribe","event_type":"payment.settled"}`))
	assert.False(t, client.IsSubscribed(aicp.EventPaymentSettled))
}

func TestClient_HandleMessage_InvalidJSON(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: make(map[aicp.EventType]bool),
		logger:        testLogger(),
	}

	// Should not panic on invalid JSON.
	client.handleMessage([]byte(`not json`))
	assert.Equal(t, 0, len(client.subscriptions))
}

func TestClient_HandleMessage_UnknownAction(t *testing.T) {
	client := &Client{
		ID:            "test",
		subscriptions: make(map[aicp.EventType]bool),
		logger:        testLogger(),
	}

	// Unknown action should be silently ignored.
	client.handleMessage([]byte(`{"action":"unknown","event_type":"payment.settled"}`))
	assert.Equal(t, 0, len(client.subscriptions))
}
