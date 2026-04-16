package payment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Shriyagautam12/PayFlow/internal/events"
)

const (
	idempotencyTTL    = 24 * time.Hour
	idempotencyPrefix = "idempotency:"
)

var (
	ErrDuplicateRequest   = errors.New("duplicate request")
	ErrInvalidTransition  = errors.New("invalid payment state transition")
	ErrPaymentNotComplete = errors.New("payment is not in completed state")
)

// WalletService is the interface the payment service uses to move money.
// Using an interface keeps payment and wallet packages decoupled.
type WalletService interface {
	Credit(ctx context.Context, merchantID string, amount int64, referenceID string) error
	Debit(ctx context.Context, merchantID string, amount int64, referenceID string) error
}

// EventPublisher is the interface the payment service uses to publish events.
// Using an interface keeps payment and events packages decoupled and testable.
type EventPublisher interface {
	Publish(ctx context.Context, event events.PaymentEvent) error
}

// Service contains all payment business logic.
type Service struct {
	repo      *Repository
	redis     *redis.Client
	wallet    WalletService
	publisher EventPublisher // nil = no-op (Kafka not configured)
	log       *zap.Logger
}

func NewService(repo *Repository, redis *redis.Client, wallet WalletService, publisher EventPublisher, log *zap.Logger) *Service {
	return &Service{repo: repo, redis: redis, wallet: wallet, publisher: publisher, log: log}
}

// publish is a safe wrapper — skips if no publisher is wired.
func (s *Service) publish(ctx context.Context, event events.PaymentEvent) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		// Non-fatal: log and continue. A real system would write to an outbox table.
		s.log.Error("failed to publish event",
			zap.String("event_type", string(event.EventType)),
			zap.String("payment_id", event.PaymentID),
			zap.Error(err),
		)
	}
}

// newEventID generates a random hex ID for each event (distinct from payment ID).
func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Core Operations ───────────────────────────────────────────────────────────

// Initiate creates a new payment with idempotency protection.
// If the same idempotency key is used twice, returns the cached result — no double charge.
func (s *Service) Initiate(ctx context.Context, merchantID string, req InitiatePaymentRequest) (*Payment, error) {
	// Step 1: Redis cache check (fast path)
	cached, err := s.getIdempotencyCache(ctx, req.IdempotencyKey)
	if err == nil && cached != nil {
		s.log.Info("idempotency cache hit", zap.String("key", req.IdempotencyKey))
		return s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	}

	// Step 2: DB check (slow path — Redis may have expired)
	existing, err := s.repo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err == nil {
		s.cacheIdempotencyResult(ctx, req.IdempotencyKey, existing)
		return existing, nil
	}

	// Step 3: New request — create payment
	if req.Currency == "" {
		req.Currency = "INR"
	}

	p := &Payment{
		MerchantID:     merchantID,
		IdempotencyKey: req.IdempotencyKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         StatusInitiated,
		Method:         req.Method,
		Metadata:       req.Metadata,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("initiating payment: %w", err)
	}

	s.cacheIdempotencyResult(ctx, req.IdempotencyKey, p)

	s.log.Info("payment initiated",
		zap.String("payment_id", p.ID),
		zap.String("merchant_id", merchantID),
		zap.Int64("amount", p.Amount),
	)

	// Move to PENDING immediately (fraud check would be async via Kafka in full impl)
	_ = s.repo.UpdateStatus(ctx, p.ID, StatusInitiated, StatusPending, "")
	p.Status = StatusPending

	return p, nil
}

// Authorize marks a payment as authorized (called by payment gateway callback).
func (s *Service) Authorize(ctx context.Context, paymentID, merchantID string) (*Payment, error) {
	p, err := s.repo.GetByID(ctx, paymentID, merchantID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, p.ID, p.Status, StatusAuthorized, ""); err != nil {
		return nil, fmt.Errorf("authorizing payment: %w", err)
	}

	p.Status = StatusAuthorized
	s.log.Info("payment authorized", zap.String("payment_id", p.ID))
	return p, nil
}

// Capture moves an authorized payment to completed and credits the merchant wallet.
// Wallet is credited BEFORE status update — never update status before money moves.
func (s *Service) Capture(ctx context.Context, paymentID, merchantID string) (*Payment, error) {
	p, err := s.repo.GetByID(ctx, paymentID, merchantID)
	if err != nil {
		return nil, err
	}

	if err := ValidateTransition(p.Status, StatusCompleted); err != nil {
		return nil, ErrInvalidTransition
	}

	if err := s.wallet.Credit(ctx, merchantID, p.Amount, p.ID); err != nil {
		s.log.Error("wallet credit failed during capture",
			zap.String("payment_id", p.ID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("crediting wallet: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, p.ID, p.Status, StatusCompleted, ""); err != nil {
		// Critical: wallet credited but payment status not updated — needs reconciliation
		s.log.Error("CRITICAL: wallet credited but payment status update failed",
			zap.String("payment_id", p.ID),
		)
		return nil, fmt.Errorf("updating payment status after capture: %w", err)
	}

	p.Status = StatusCompleted
	s.log.Info("payment captured",
		zap.String("payment_id", p.ID),
		zap.Int64("amount", p.Amount),
	)

	s.publish(ctx, events.PaymentEvent{
		EventID:    newEventID(),
		EventType:  events.EventPaymentCompleted,
		OccurredAt: time.Now(),
		PaymentID:  p.ID,
		MerchantID: p.MerchantID,
		Amount:     p.Amount,
		Currency:   p.Currency,
		Method:     string(p.Method),
		Status:     string(p.Status),
	})

	return p, nil
}

// Refund reverses a completed payment and debits the merchant wallet.
func (s *Service) Refund(ctx context.Context, paymentID, merchantID, reason string) (*Payment, error) {
	p, err := s.repo.GetByID(ctx, paymentID, merchantID)
	if err != nil {
		return nil, err
	}

	if p.Status != StatusCompleted {
		return nil, ErrPaymentNotComplete
	}

	if err := s.wallet.Debit(ctx, merchantID, p.Amount, p.ID); err != nil {
		return nil, fmt.Errorf("debiting wallet for refund: %w", err)
	}

	if err := s.repo.UpdateStatus(ctx, p.ID, p.Status, StatusRefunded, reason); err != nil {
		s.log.Error("CRITICAL: wallet debited but refund status update failed",
			zap.String("payment_id", p.ID),
		)
		return nil, fmt.Errorf("updating payment status after refund: %w", err)
	}

	p.Status = StatusRefunded
	s.log.Info("payment refunded", zap.String("payment_id", p.ID))

	s.publish(ctx, events.PaymentEvent{
		EventID:    newEventID(),
		EventType:  events.EventPaymentRefunded,
		OccurredAt: time.Now(),
		PaymentID:  p.ID,
		MerchantID: p.MerchantID,
		Amount:     p.Amount,
		Currency:   p.Currency,
		Method:     string(p.Method),
		Status:     string(p.Status),
	})

	return p, nil
}

// Fail marks a payment as failed with a reason.
func (s *Service) Fail(ctx context.Context, paymentID, merchantID, reason string) (*Payment, error) {
	p, err := s.repo.GetByID(ctx, paymentID, merchantID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateStatus(ctx, p.ID, p.Status, StatusFailed, reason); err != nil {
		return nil, fmt.Errorf("failing payment: %w", err)
	}

	p.Status = StatusFailed
	s.log.Info("payment failed",
		zap.String("payment_id", p.ID),
		zap.String("reason", reason),
	)
	return p, nil
}

// GetByID fetches a single payment scoped to the merchant.
func (s *Service) GetByID(ctx context.Context, paymentID, merchantID string) (*Payment, error) {
	return s.repo.GetByID(ctx, paymentID, merchantID)
}

// List returns paginated payments for a merchant.
func (s *Service) List(ctx context.Context, merchantID string, req ListPaymentsRequest) (*ListPaymentsResponse, error) {
	payments, total, err := s.repo.List(ctx, merchantID, req)
	if err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	return &ListPaymentsResponse{
		Payments:   payments,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ── Idempotency Helpers ───────────────────────────────────────────────────────

func (s *Service) getIdempotencyCache(ctx context.Context, key string) (*IdempotencyRecord, error) {
	val, err := s.redis.Get(ctx, idempotencyPrefix+key).Result()
	if err != nil {
		return nil, err
	}
	var record IdempotencyRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Service) cacheIdempotencyResult(ctx context.Context, key string, p *Payment) {
	record := IdempotencyRecord{
		PaymentID: p.ID,
		Status:    p.Status,
	}
	data, err := json.Marshal(record)
	if err != nil {
		s.log.Error("failed to marshal idempotency record", zap.Error(err))
		return
	}
	_ = s.redis.Set(ctx, idempotencyPrefix+key, data, idempotencyTTL).Err()
}
