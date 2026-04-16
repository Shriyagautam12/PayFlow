package events

import "time"

// EventType identifies what happened to a payment.
type EventType string

const (
	EventPaymentInitiated  EventType = "payment.initiated"
	EventPaymentAuthorized EventType = "payment.authorized"
	EventPaymentCompleted  EventType = "payment.completed"
	EventPaymentFailed     EventType = "payment.failed"
	EventPaymentRefunded   EventType = "payment.refunded"
)

// PaymentEvent is the message published to Kafka on every payment state transition.
// It is also the payload forwarded to merchant webhook endpoints.
type PaymentEvent struct {
	EventID    string    `json:"event_id"`    // uuid, unique per event — not per payment
	EventType  EventType `json:"event_type"`  // e.g. "payment.completed"
	OccurredAt time.Time `json:"occurred_at"` // when the transition happened

	// Payment fields — duplicated here so consumers don't need to call the DB
	PaymentID  string `json:"payment_id"`
	MerchantID string `json:"merchant_id"`
	Amount     int64  `json:"amount"`     // paise
	Currency   string `json:"currency"`
	Method     string `json:"method"`
	Status     string `json:"status"`     // the new status after the transition
}

// Topic is the Kafka topic all payment events are published to.
const Topic = "payment-events"
