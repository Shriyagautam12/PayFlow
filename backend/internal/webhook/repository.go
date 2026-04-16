package webhook

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Shriyagautam12/PayFlow/internal/events"
)

// Repository handles all DB operations for webhooks and deliveries.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ── Webhook CRUD ──────────────────────────────────────────────────────────────

func (r *Repository) Create(ctx context.Context, w *Webhook) error {
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("creating webhook: %w", err)
	}
	return nil
}

// ListByMerchant returns all webhooks (active and inactive) for a merchant.
func (r *Repository) ListByMerchant(ctx context.Context, merchantID string) ([]Webhook, error) {
	var hooks []Webhook
	if err := r.db.WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		Order("created_at DESC").
		Find(&hooks).Error; err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	return hooks, nil
}

// FindActiveByEventType returns all active webhooks registered for a given event type.
// Called by the service when an event arrives from Kafka — one query, fan-out to many merchants.
func (r *Repository) FindActiveByEventType(ctx context.Context, eventType events.EventType) ([]Webhook, error) {
	var hooks []Webhook
	if err := r.db.WithContext(ctx).
		Where("event_type = ? AND active = true", eventType).
		Find(&hooks).Error; err != nil {
		return nil, fmt.Errorf("finding webhooks for event type %s: %w", eventType, err)
	}
	return hooks, nil
}

// ── Delivery log ──────────────────────────────────────────────────────────────

func (r *Repository) CreateDelivery(ctx context.Context, d *WebhookDelivery) error {
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return fmt.Errorf("creating delivery record: %w", err)
	}
	return nil
}

// FindPendingRetries returns deliveries that are due for a retry attempt.
// The webhook service calls this on a ticker to process retries.
func (r *Repository) FindPendingRetries(ctx context.Context) ([]WebhookDelivery, error) {
	var deliveries []WebhookDelivery
	if err := r.db.WithContext(ctx).
		Where("status = ? AND next_retry_at <= ?", DeliveryFailed, time.Now()).
		Find(&deliveries).Error; err != nil {
		return nil, fmt.Errorf("finding pending retries: %w", err)
	}
	return deliveries, nil
}

// GetWebhookByID fetches a single webhook — used during retry to re-read URL and secret.
func (r *Repository) GetWebhookByID(ctx context.Context, webhookID string) (*Webhook, error) {
	var w Webhook
	if err := r.db.WithContext(ctx).First(&w, "id = ?", webhookID).Error; err != nil {
		return nil, fmt.Errorf("getting webhook %s: %w", webhookID, err)
	}
	return &w, nil
}
