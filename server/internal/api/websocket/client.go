package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	ws "github.com/coder/websocket"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	maxMsgSize = 4096
	sendBuffer = 256

	actionSubscribe   = "subscribe"
	actionUnsubscribe = "unsubscribe"
)

// Client represents a single WebSocket connection.
type Client struct {
	ID     string
	DID    string
	conn   *ws.Conn
	hub    *Hub
	send   chan aicp.Event
	logger *slog.Logger

	mu            sync.RWMutex
	subscriptions map[aicp.EventType]bool
}

// NewClient creates a new WebSocket client.
func NewClient(id, did string, conn *ws.Conn, hub *Hub, logger *slog.Logger) *Client {
	return &Client{
		ID:            id,
		DID:           did,
		conn:          conn,
		hub:           hub,
		send:          make(chan aicp.Event, sendBuffer),
		logger:        logger,
		subscriptions: make(map[aicp.EventType]bool),
	}
}

// Send queues an event for sending to the client.
func (c *Client) Send(event aicp.Event) {
	select {
	case c.send <- event:
	default:
		c.logger.Warn("client send buffer full, dropping event", "client_id", c.ID)
	}
}

// Subscribe adds an event type subscription.
func (c *Client) Subscribe(eventType aicp.EventType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions[eventType] = true
}

// Unsubscribe removes an event type subscription.
func (c *Client) Unsubscribe(eventType aicp.EventType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, eventType)
}

// IsSubscribed checks if the client is subscribed to an event type.
func (c *Client) IsSubscribed(eventType aicp.EventType) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Wildcard subscription.
	if c.subscriptions["*"] {
		return true
	}
	return c.subscriptions[eventType]
}

// Run starts the client's read and write loops.
func (c *Client) Run(ctx context.Context) {
	go c.writePump(ctx)
	c.readPump(ctx)
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close(ws.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMsgSize)

	for {
		_, msg, err := c.conn.Read(ctx)
		if err != nil {
			if ws.CloseStatus(err) != ws.StatusNormalClosure {
				c.logger.Debug("websocket read error", "client_id", c.ID, "error", err)
			}
			return
		}

		c.handleMessage(msg)
	}
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-c.send:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				c.logger.Warn("failed to marshal event", "error", err)
				continue
			}

			writeCtx, cancel := context.WithTimeout(ctx, writeWait)
			err = c.conn.Write(writeCtx, ws.MessageText, data)
			cancel()
			if err != nil {
				c.logger.Debug("websocket write error", "client_id", c.ID, "error", err)
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeWait)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// clientMessage represents an incoming WebSocket message from the client.
type clientMessage struct {
	Action    string          `json:"action"`
	EventType aicp.EventType `json:"event_type,omitempty"`
}

func (c *Client) handleMessage(msg []byte) {
	var cm clientMessage
	if err := json.Unmarshal(msg, &cm); err != nil {
		c.logger.Debug("invalid client message", "error", err)
		return
	}

	switch cm.Action {
	case actionSubscribe:
		c.Subscribe(cm.EventType)
	case actionUnsubscribe:
		c.Unsubscribe(cm.EventType)
	}
}
