package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/aicp-protocol/aicp/server/pkg/aicp"
)

const (
	redisChannel = "aicp:events"
)

// RedisBridge bridges the local event emitter to Redis pub/sub for cross-node
// event distribution.
type RedisBridge struct {
	redis   *redis.Client
	emitter *Emitter
	logger  *slog.Logger
}

// NewRedisBridge creates a new Redis pub/sub bridge.
func NewRedisBridge(rdb *redis.Client, emitter *Emitter, logger *slog.Logger) *RedisBridge {
	return &RedisBridge{
		redis:   rdb,
		emitter: emitter,
		logger:  logger,
	}
}

// Publish sends an event to Redis pub/sub.
func (rb *RedisBridge) Publish(ctx context.Context, event aicp.Event) error {
	data, err := MarshalEvent(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	return rb.redis.Publish(ctx, redisChannel, data).Err()
}

// Subscribe starts listening to Redis pub/sub and forwards events to the local emitter.
// This method blocks until the context is cancelled.
func (rb *RedisBridge) Subscribe(ctx context.Context) error {
	pubsub := rb.redis.Subscribe(ctx, redisChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			event, err := UnmarshalEvent([]byte(msg.Payload))
			if err != nil {
				rb.logger.Warn("failed to unmarshal event from Redis", "error", err)
				continue
			}

			rb.emitter.Emit(*event)
		}
	}
}

// EmitAndPublish emits an event locally and publishes it to Redis for other nodes.
func (rb *RedisBridge) EmitAndPublish(ctx context.Context, event aicp.Event) error {
	rb.emitter.Emit(event)
	return rb.Publish(ctx, event)
}
