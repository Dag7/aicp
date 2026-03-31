package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ticketTTL    = 30 * time.Second
	ticketPrefix = "aicp:ws:ticket:"
)

// TicketAuth provides ticket-based WebSocket authentication.
// Clients first obtain a ticket via POST /aicp/v1/ws/ticket, then
// present it during the WebSocket upgrade.
type TicketAuth struct {
	redis *redis.Client
}

// NewTicketAuth creates a new ticket authenticator.
func NewTicketAuth(rdb *redis.Client) *TicketAuth {
	return &TicketAuth{redis: rdb}
}

// IssueTicket creates a time-limited ticket bound to a DID.
func (ta *TicketAuth) IssueTicket(ctx context.Context, did string) (string, error) {
	ticketBytes := make([]byte, 32)
	if _, err := rand.Read(ticketBytes); err != nil {
		return "", fmt.Errorf("generating ticket: %w", err)
	}
	ticket := hex.EncodeToString(ticketBytes)

	key := ticketPrefix + ticket
	if err := ta.redis.Set(ctx, key, did, ticketTTL).Err(); err != nil {
		return "", fmt.Errorf("storing ticket: %w", err)
	}

	return ticket, nil
}

// ValidateTicket checks a ticket and returns the associated DID.
// The ticket is consumed (one-time use).
func (ta *TicketAuth) ValidateTicket(ctx context.Context, ticket string) (string, error) {
	key := ticketPrefix + ticket

	did, err := ta.redis.GetDel(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("ticket not found or expired")
		}
		return "", fmt.Errorf("validating ticket: %w", err)
	}

	return did, nil
}
