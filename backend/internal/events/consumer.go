package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Handler is the function signature the webhook service implements to receive events.
type Handler func(ctx context.Context, event PaymentEvent) error

// Consumer reads PaymentEvents from Kafka and dispatches them to a Handler.
type Consumer struct {
	reader  *kafka.Reader
	handler Handler
	log     *zap.Logger
}

// NewConsumer creates a consumer in the given consumer group.
// groupID example: "webhook-service"
func NewConsumer(brokers []string, groupID string, handler Handler, log *zap.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    Topic,
		GroupID:  groupID, // consumer group ensures each event is processed once
		MinBytes: 1,
		MaxBytes: 10e6, // 10 MB
	})
	return &Consumer{reader: r, handler: handler, log: log}
}

// Start blocks and processes messages until ctx is cancelled.
// Run this in a goroutine: go consumer.Start(ctx)
func (c *Consumer) Start(ctx context.Context) {
	c.log.Info("kafka consumer started", zap.String("topic", Topic))

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled — clean shutdown
				c.log.Info("kafka consumer stopping")
				return
			}
			c.log.Error("kafka fetch error", zap.Error(err))
			continue
		}

		var event PaymentEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.log.Error("failed to unmarshal event",
				zap.Error(err),
				zap.ByteString("raw", msg.Value),
			)
			// Commit bad message so we don't get stuck on it forever
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if err := c.handler(ctx, event); err != nil {
			// Log but still commit — dead-letter queue would go here in production
			c.log.Error("handler failed for event",
				zap.String("event_id", event.EventID),
				zap.String("event_type", string(event.EventType)),
				zap.Error(err),
			)
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("failed to commit kafka message", zap.Error(fmt.Errorf("commit: %w", err)))
		}
	}
}

// Close closes the underlying reader. Call in main defer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
