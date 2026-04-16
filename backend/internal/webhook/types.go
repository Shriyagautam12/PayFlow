package webhook

import (
	"time"

	"github.com/Shriyagautam12/PayFlow/internal/events"
)

// DeliveryStatus tracks the outcome of a single webhook delivery attempt.
type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliverySucceeded DeliveryStatus = "succeeded"
	DeliveryFailed    DeliveryStatus = "failed"
)

// Webhook is a merchant-registered endpoint for a specific event type.
// A merchant can register multiple webhooks for different event types.
type Webhook struct {
	ID         string          `json:"id"          gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	MerchantID string          `json:"merchant_id" gorm:"not null;index"`
	URL        string          `json:"url"         gorm:"not null"`
	EventType  events.EventType `json:"event_type"  gorm:"not null;index"` // e.g. "payment.completed"
	Secret     string          `json:"-"           gorm:"not null"`       // HMAC signing key — never returned in API
	Active     bool            `json:"active"      gorm:"not null;default:true"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// WebhookDelivery is an immutable log of one delivery attempt for one event.
// Each retry creates a new row — never mutates existing rows.
type WebhookDelivery struct {
	ID             string         `json:"id"               gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WebhookID      string         `json:"webhook_id"       gorm:"not null;index"`
	EventID        string         `json:"event_id"         gorm:"not null;index"` // links back to PaymentEvent.EventID
	AttemptNumber  int            `json:"attempt_number"   gorm:"not null;default:1"`
	Status         DeliveryStatus `json:"status"           gorm:"not null"`
	ResponseCode   int            `json:"response_code"`   // HTTP status the merchant endpoint returned
	ResponseBody   string         `json:"response_body"    gorm:"type:text"`
	NextRetryAt    *time.Time     `json:"next_retry_at"`   // nil when no more retries
	CreatedAt      time.Time      `json:"created_at"`
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

type RegisterWebhookRequest struct {
	URL       string           `json:"url"        binding:"required,url"`
	EventType events.EventType `json:"event_type" binding:"required"`
}

type RegisterWebhookResponse struct {
	ID        string           `json:"id"`
	URL       string           `json:"url"`
	EventType events.EventType `json:"event_type"`
	Active    bool             `json:"active"`
	CreatedAt time.Time        `json:"created_at"`
}

type ListWebhooksResponse struct {
	Webhooks []RegisterWebhookResponse `json:"webhooks"`
}
