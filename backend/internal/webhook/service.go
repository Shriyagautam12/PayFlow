package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/Shriyagautam12/PayFlow/internal/events"
)

const (
	maxAttempts    = 5
	deliveryTimeout = 10 * time.Second

	// Exponential backoff: attempt 1→30s, 2→5m, 3→30m, 4→2h, 5→8h
	// Stored in nextRetryAt so a separate retry worker picks them up.
)

var retryDelays = []time.Duration{
	30 * time.Second,
	5 * time.Minute,
	30 * time.Minute,
	2 * time.Hour,
	8 * time.Hour,
}

// Service handles webhook registration and event delivery.
type Service struct {
	repo   *Repository
	client *http.Client
	log    *zap.Logger
}

func NewService(repo *Repository, log *zap.Logger) *Service {
	return &Service{
		repo: repo,
		client: &http.Client{Timeout: deliveryTimeout},
		log:  log,
	}
}

// ── Registration ──────────────────────────────────────────────────────────────

// Register creates a new webhook for a merchant.
func (s *Service) Register(ctx context.Context, merchantID string, req RegisterWebhookRequest, secret string) (*Webhook, error) {
	w := &Webhook{
		MerchantID: merchantID,
		URL:        req.URL,
		EventType:  req.EventType,
		Secret:     secret,
		Active:     true,
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	s.log.Info("webhook registered",
		zap.String("merchant_id", merchantID),
		zap.String("event_type", string(req.EventType)),
		zap.String("url", req.URL),
	)
	return w, nil
}

// List returns all webhooks for a merchant.
func (s *Service) List(ctx context.Context, merchantID string) ([]Webhook, error) {
	return s.repo.ListByMerchant(ctx, merchantID)
}

// ── Delivery ──────────────────────────────────────────────────────────────────

// HandleEvent is the events.Handler the Kafka consumer calls on every message.
// It fans out the event to every active webhook registered for that event type.
func (s *Service) HandleEvent(ctx context.Context, event events.PaymentEvent) error {
	hooks, err := s.repo.FindActiveByEventType(ctx, event.EventType)
	if err != nil {
		return fmt.Errorf("finding webhooks: %w", err)
	}

	for _, hook := range hooks {
		s.deliver(ctx, hook, event, 1)
	}
	return nil
}

// ProcessRetries finds failed deliveries that are due and re-attempts them.
// Call this on a ticker (e.g. every minute) from main.
func (s *Service) ProcessRetries(ctx context.Context) {
	deliveries, err := s.repo.FindPendingRetries(ctx)
	if err != nil {
		s.log.Error("fetching pending retries", zap.Error(err))
		return
	}

	for _, d := range deliveries {
		hook, err := s.repo.GetWebhookByID(ctx, d.WebhookID)
		if err != nil {
			s.log.Error("fetching webhook for retry", zap.Error(err))
			continue
		}

		// Reconstruct the event from the delivery's stored payload is the
		// production approach (store raw JSON). Here we re-fetch by event_id
		// via a minimal struct — sufficient for the interview scope.
		var event events.PaymentEvent
		// In a real system you'd store the raw event JSON in WebhookDelivery.
		// For now we just re-deliver with the event ID so the attempt is logged.
		event.EventID = d.EventID

		s.deliver(ctx, *hook, event, d.AttemptNumber+1)
	}
}

// deliver POSTs the event to the webhook URL, signs the payload with HMAC,
// and writes a WebhookDelivery row regardless of outcome.
func (s *Service) deliver(ctx context.Context, hook Webhook, event events.PaymentEvent, attempt int) {
	payload, err := json.Marshal(event)
	if err != nil {
		s.log.Error("marshalling event for delivery", zap.Error(err))
		return
	}

	signature := sign(payload, hook.Secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(payload))
	if err != nil {
		s.log.Error("building webhook request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// X-PayFlow-Signature: sha256=<hex>
	// Merchant verifies this — same pattern as GitHub webhooks.
	req.Header.Set("X-PayFlow-Signature", "sha256="+signature)
	req.Header.Set("X-PayFlow-Event", string(event.EventType))
	req.Header.Set("X-PayFlow-Delivery", event.EventID)

	resp, err := s.client.Do(req)

	delivery := &WebhookDelivery{
		WebhookID:     hook.ID,
		EventID:       event.EventID,
		AttemptNumber: attempt,
	}

	if err != nil || resp.StatusCode >= 300 {
		// Failure path
		delivery.Status = DeliveryFailed

		if err == nil {
			delivery.ResponseCode = resp.StatusCode
			body := make([]byte, 512)
			n, _ := resp.Body.Read(body)
			resp.Body.Close()
			delivery.ResponseBody = string(body[:n])
		}

		// Schedule next retry if we haven't hit the limit
		if attempt < maxAttempts {
			next := time.Now().Add(retryDelays[attempt-1])
			delivery.NextRetryAt = &next
		}

		s.log.Warn("webhook delivery failed",
			zap.String("webhook_id", hook.ID),
			zap.String("event_id", event.EventID),
			zap.Int("attempt", attempt),
		)
	} else {
		// Success path
		delivery.Status = DeliverySucceeded
		delivery.ResponseCode = resp.StatusCode
		resp.Body.Close()

		s.log.Info("webhook delivered",
			zap.String("webhook_id", hook.ID),
			zap.String("event_id", event.EventID),
			zap.Int("attempt", attempt),
		)
	}

	if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
		s.log.Error("saving delivery record", zap.Error(err))
	}
}

// sign returns the HMAC-SHA256 hex digest of payload using secret.
// The merchant recomputes this on their end to verify the request is from us.
func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
