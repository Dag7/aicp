// Copyright 2024 AICP Protocol Contributors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// ─── Emitter Tests ───────────────────────────────────────────────────────────

func TestEmitter_SubscribeAndEmit(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var received atomic.Value

	done := make(chan struct{})
	emitter.Subscribe(aicp.EventDIDCreated, func(event aicp.Event) {
		received.Store(event)
		close(done)
	})

	event := aicp.Event{
		EventID:   "evt-1",
		Type:      aicp.EventDIDCreated,
		Timestamp: time.Now().UTC(),
		Source:    "test",
		Subject:   "did:aicp:test",
	}

	emitter.Emit(event)

	select {
	case <-done:
		got := received.Load().(aicp.Event)
		assert.Equal(t, "evt-1", got.EventID)
		assert.Equal(t, aicp.EventDIDCreated, got.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive event within timeout")
	}
}

func TestEmitter_MultipleSubscribers(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var count atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		emitter.Subscribe(aicp.EventPaymentSettled, func(_ aicp.Event) {
			count.Add(1)
			wg.Done()
		})
	}

	emitter.Emit(aicp.Event{
		EventID: "evt-multi",
		Type:    aicp.EventPaymentSettled,
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, int32(3), count.Load())
	case <-time.After(2 * time.Second):
		t.Fatalf("only %d/3 handlers received the event", count.Load())
	}
}

func TestEmitter_DifferentEventTypes(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var didCount atomic.Int32
	var payCount atomic.Int32

	didDone := make(chan struct{})
	emitter.Subscribe(aicp.EventDIDCreated, func(_ aicp.Event) {
		didCount.Add(1)
		didDone <- struct{}{}
	})

	payDone := make(chan struct{}, 1)
	emitter.Subscribe(aicp.EventPaymentSettled, func(_ aicp.Event) {
		payCount.Add(1)
		payDone <- struct{}{}
	})

	// Emit a DID event -- only DID handler should fire.
	emitter.Emit(aicp.Event{Type: aicp.EventDIDCreated})

	select {
	case <-didDone:
	case <-time.After(2 * time.Second):
		t.Fatal("DID handler did not fire")
	}

	// Brief wait to confirm payment handler was not called.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), payCount.Load(), "payment handler should not have been called")
}

func TestEmitter_SubscribeAll(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var count atomic.Int32

	done := make(chan struct{}, 2)
	emitter.SubscribeAll(func(_ aicp.Event) {
		count.Add(1)
		done <- struct{}{}
	})

	emitter.Emit(aicp.Event{Type: aicp.EventDIDCreated})
	emitter.Emit(aicp.Event{Type: aicp.EventPaymentSettled})

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("wildcard handler did not receive event")
		}
	}

	assert.Equal(t, int32(2), count.Load())
}

func TestEmitter_Unsubscribe(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var count atomic.Int32

	done := make(chan struct{}, 1)
	unsub := emitter.Subscribe(aicp.EventDIDCreated, func(_ aicp.Event) {
		count.Add(1)
		done <- struct{}{}
	})

	// First emit should be received.
	emitter.Emit(aicp.Event{Type: aicp.EventDIDCreated})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not fire before unsubscribe")
	}

	// Unsubscribe and emit again.
	unsub()
	emitter.Emit(aicp.Event{Type: aicp.EventDIDCreated})

	// Wait briefly, count should not have increased.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), count.Load(), "handler should not fire after unsubscribe")
}

func TestEmitter_PanicRecovery(t *testing.T) {
	emitter := NewEmitter(testLogger())
	var recovered atomic.Bool

	done := make(chan struct{})
	emitter.Subscribe(aicp.EventDIDCreated, func(_ aicp.Event) {
		panic("test panic")
	})
	emitter.Subscribe(aicp.EventDIDCreated, func(_ aicp.Event) {
		recovered.Store(true)
		close(done)
	})

	// Emit should not crash -- panicking handler is recovered.
	emitter.Emit(aicp.Event{Type: aicp.EventDIDCreated})

	select {
	case <-done:
		assert.True(t, recovered.Load(), "second handler should still have been called")
	case <-time.After(2 * time.Second):
		t.Fatal("second handler did not fire after first panicked")
	}
}

// ─── Event Marshaling Tests ──────────────────────────────────────────────────

func TestMarshalEvent_RoundTrip(t *testing.T) {
	original := aicp.Event{
		EventID:   "evt-rt",
		Type:      aicp.EventDIDCreated,
		Timestamp: time.Now().UTC().Truncate(time.Millisecond),
		Source:    "server",
		Subject:   "did:aicp:test",
		Data:      json.RawMessage(`{"did":"did:aicp:test"}`),
	}

	data, err := MarshalEvent(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	unmarshaled, err := UnmarshalEvent(data)
	require.NoError(t, err)
	assert.Equal(t, original.EventID, unmarshaled.EventID)
	assert.Equal(t, original.Type, unmarshaled.Type)
	assert.Equal(t, original.Source, unmarshaled.Source)
	assert.Equal(t, original.Subject, unmarshaled.Subject)
}

func TestUnmarshalEvent_InvalidJSON(t *testing.T) {
	_, err := UnmarshalEvent([]byte("not json"))
	require.Error(t, err)
}

// ─── NewEvent Tests ──────────────────────────────────────────────────────────

func TestNewEvent(t *testing.T) {
	event, err := NewEvent(aicp.EventDIDCreated, "server", "did:aicp:test", map[string]string{"did": "did:aicp:test"})
	require.NoError(t, err)

	assert.NotEmpty(t, event.EventID)
	assert.True(t, len(event.EventID) > 4, "event ID should have meaningful length")
	assert.Equal(t, aicp.EventDIDCreated, event.Type)
	assert.Equal(t, "server", event.Source)
	assert.Equal(t, "did:aicp:test", event.Subject)
	assert.NotEmpty(t, event.Data)
	assert.False(t, event.Timestamp.IsZero())
	assert.Empty(t, event.Signature)
}

func TestNewEvent_UniqueIDs(t *testing.T) {
	e1, err := NewEvent(aicp.EventDIDCreated, "s", "d", nil)
	require.NoError(t, err)
	e2, err := NewEvent(aicp.EventDIDCreated, "s", "d", nil)
	require.NoError(t, err)
	assert.NotEqual(t, e1.EventID, e2.EventID)
}

// ─── Convenience Constructor Tests ───────────────────────────────────────────

func TestNewDIDCreatedEvent(t *testing.T) {
	event, err := NewDIDCreatedEvent("server", "did:aicp:new")
	require.NoError(t, err)
	assert.Equal(t, aicp.EventDIDCreated, event.Type)
	assert.Equal(t, "did:aicp:new", event.Subject)
}

func TestNewDIDUpdatedEvent(t *testing.T) {
	event, err := NewDIDUpdatedEvent("server", "did:aicp:updated")
	require.NoError(t, err)
	assert.Equal(t, aicp.EventDIDUpdated, event.Type)
}

func TestNewDIDDeactivatedEvent(t *testing.T) {
	event, err := NewDIDDeactivatedEvent("server", "did:aicp:deactivated")
	require.NoError(t, err)
	assert.Equal(t, aicp.EventDIDDeactivated, event.Type)
}

func TestNewCredentialRevokedEvent(t *testing.T) {
	event, err := NewCredentialRevokedEvent("server", "cred-1", "did:aicp:issuer")
	require.NoError(t, err)
	assert.Equal(t, aicp.EventCredentialRevoked, event.Type)
}

func TestNewPaymentSettledEvent(t *testing.T) {
	event, err := NewPaymentSettledEvent("server", "did:aicp:payer", "did:aicp:payee", "rcpt-1", aicp.MoneyAmount{Value: "10.00", Currency: "USD"})
	require.NoError(t, err)
	assert.Equal(t, aicp.EventPaymentSettled, event.Type)
	assert.Equal(t, "did:aicp:payer", event.Subject)
}

func TestNewTrustScoreUpdatedEvent(t *testing.T) {
	event, err := NewTrustScoreUpdatedEvent("server", "did:aicp:scored", 750)
	require.NoError(t, err)
	assert.Equal(t, aicp.EventTrustScoreUpdated, event.Type)
	assert.Equal(t, "did:aicp:scored", event.Subject)
}

// ─── Event Type Constants Test ───────────────────────────────────────────────

func TestEventTypeConstants(t *testing.T) {
	// Verify that all event type constants have the expected dot-separated format.
	types := []aicp.EventType{
		aicp.EventCredentialRevoked,
		aicp.EventRevocationListUpdated,
		aicp.EventPaymentAuthorized,
		aicp.EventPaymentSettled,
		aicp.EventPaymentFailed,
		aicp.EventPaymentRefunded,
		aicp.EventDIDCreated,
		aicp.EventDIDUpdated,
		aicp.EventDIDDeactivated,
		aicp.EventKeyRotated,
		aicp.EventServiceStatusChanged,
		aicp.EventServiceANSUpdated,
		aicp.EventTrustScoreUpdated,
		aicp.EventHeartbeat,
		aicp.EventError,
	}

	for _, et := range types {
		assert.NotEmpty(t, string(et))
		assert.Contains(t, string(et), ".", "event type %q should be dot-separated", et)
	}
}
