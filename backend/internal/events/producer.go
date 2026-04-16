package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// Producer publishes PaymentEvents to Kafka.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a producer that writes to the given broker addresses.
// brokers example: []string{"localhost:9092"}
func NewProducer(brokers []string) *Producer {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  Topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true, // fine for dev; disable in production
	}
	return &Producer{writer: w}
}

// Publish serialises the event and writes it to Kafka.
// The PaymentID is used as the message key so all events for the same payment
// land on the same partition (preserving order).
func (p *Producer) Publish(ctx context.Context, event PaymentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshalling event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(event.PaymentID),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("writing kafka message: %w", err)
	}
	return nil
}

// Close flushes and closes the underlying writer. Call in main defer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
