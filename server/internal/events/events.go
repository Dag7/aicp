package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// EventBuilder provides a fluent API for constructing events.
type EventBuilder struct {
	eventType aicp.EventType
	source    string
	subject   string
	data      any
}

// NewEventBuilder creates a new event builder with required fields.
func NewEventBuilder(eventType aicp.EventType, source, subject string) *EventBuilder {
	return &EventBuilder{eventType: eventType, source: source, subject: subject}
}

// WithData sets the event data payload.
func (b *EventBuilder) WithData(data any) *EventBuilder {
	b.data = data
	return b
}

// Build constructs the final event, marshalling data to JSON.
func (b *EventBuilder) Build() (aicp.Event, error) {
	var dataJSON json.RawMessage
	if b.data != nil {
		d, err := json.Marshal(b.data)
		if err != nil {
			return aicp.Event{}, fmt.Errorf("marshalling event data: %w", err)
		}
		dataJSON = d
	}
	now := time.Now().UTC()
	eventID := fmt.Sprintf("evt_%s_%d", crypto.SHA256Hex([]byte(string(b.eventType) + b.source + now.String()))[:12], now.UnixNano())
	return aicp.Event{
		EventID:   eventID,
		Type:      b.eventType,
		Timestamp: now,
		Source:    b.source,
		Subject:   b.subject,
		Data:      dataJSON,
	}, nil
}

// NewEvent creates a new event with generated ID and timestamp.
func NewEvent(eventType aicp.EventType, source, subject string, data any) (aicp.Event, error) {
	return NewEventBuilder(eventType, source, subject).WithData(data).Build()
}

// NewSignedEvent creates an event and signs it with the given key pair.
func NewSignedEvent(eventType aicp.EventType, source, subject string, data any, kp *crypto.KeyPair) (aicp.Event, error) {
	event, err := NewEvent(eventType, source, subject, data)
	if err != nil {
		return aicp.Event{}, err
	}

	// Sign the event payload.
	payload := fmt.Sprintf("%s:%s:%s:%s", event.EventID, event.Type, event.Source, event.Timestamp)
	sig, err := crypto.SignJSON(kp.PrivateKey, []byte(payload))
	if err != nil {
		return aicp.Event{}, fmt.Errorf("signing event: %w", err)
	}
	event.Signature = sig

	return event, nil
}

// ─── Convenience constructors for common events ─────────────────────────────

func NewDIDCreatedEvent(source, did string) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventDIDCreated, source, did).Build()
}

func NewDIDUpdatedEvent(source, did string) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventDIDUpdated, source, did).Build()
}

func NewDIDDeactivatedEvent(source, did string) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventDIDDeactivated, source, did).Build()
}

func NewCredentialRevokedEvent(source, credentialID, did string) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventCredentialRevoked, source, did).
		WithData(map[string]string{
			"credential_id": credentialID,
			"did":           did,
		}).Build()
}

func NewPaymentSettledEvent(source, payerDID, payeeDID, receiptID string, amount aicp.MoneyAmount) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventPaymentSettled, source, payerDID).
		WithData(map[string]any{
			"payer_did":  payerDID,
			"payee_did":  payeeDID,
			"receipt_id": receiptID,
			"amount":     amount,
		}).Build()
}

func NewTrustScoreUpdatedEvent(source, did string, score int) (aicp.Event, error) {
	return NewEventBuilder(aicp.EventTrustScoreUpdated, source, did).
		WithData(map[string]any{
			"did":   did,
			"score": score,
		}).Build()
}
