package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aicp-protocol/aicp/server/internal/crypto"
	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

// NewEvent creates a new event with generated ID and timestamp.
func NewEvent(eventType aicp.EventType, source, subject string, data any) (aicp.Event, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return aicp.Event{}, fmt.Errorf("marshaling event data: %w", err)
	}

	now := time.Now().UTC()
	eventID := fmt.Sprintf("evt_%s_%d", crypto.SHA256Hex([]byte(string(eventType)+source+now.String()))[:12], now.UnixNano())

	return aicp.Event{
		EventID:   eventID,
		Type:      eventType,
		Timestamp: now,
		Source:    source,
		Subject:   subject,
		Data:      dataJSON,
	}, nil
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
	return NewEvent(aicp.EventDIDCreated, source, did, map[string]string{"did": did})
}

func NewDIDUpdatedEvent(source, did string) (aicp.Event, error) {
	return NewEvent(aicp.EventDIDUpdated, source, did, map[string]string{"did": did})
}

func NewDIDDeactivatedEvent(source, did string) (aicp.Event, error) {
	return NewEvent(aicp.EventDIDDeactivated, source, did, map[string]string{"did": did})
}

func NewCredentialRevokedEvent(source, credentialID, did string) (aicp.Event, error) {
	return NewEvent(aicp.EventCredentialRevoked, source, did, map[string]string{
		"credential_id": credentialID,
		"did":           did,
	})
}

func NewPaymentSettledEvent(source, payerDID, payeeDID, receiptID string, amount aicp.MoneyAmount) (aicp.Event, error) {
	return NewEvent(aicp.EventPaymentSettled, source, payerDID, map[string]any{
		"payer_did":  payerDID,
		"payee_did":  payeeDID,
		"receipt_id": receiptID,
		"amount":     amount,
	})
}

func NewTrustScoreUpdatedEvent(source, did string, score int) (aicp.Event, error) {
	return NewEvent(aicp.EventTrustScoreUpdated, source, did, map[string]any{
		"did":   did,
		"score": score,
	})
}
