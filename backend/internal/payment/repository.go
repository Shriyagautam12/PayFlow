package payment

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrPaymentNotFound = errors.New("payment not found")

// Repository handles all DB operations for payments.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new payment record.
func (r *Repository) Create(ctx context.Context, p *Payment) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("creating payment: %w", err)
	}
	return nil
}

// GetByID fetches a single payment scoped to merchantID.
// A merchant can never fetch another merchant's payment.
func (r *Repository) GetByID(ctx context.Context, id, merchantID string) (*Payment, error) {
	var p Payment
	err := r.db.WithContext(ctx).
		Where("id = ? AND merchant_id = ?", id, merchantID).
		First(&p).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching payment: %w", err)
	}
	return &p, nil
}

// GetByIdempotencyKey looks up a payment by its idempotency key.
func (r *Repository) GetByIdempotencyKey(ctx context.Context, key string) (*Payment, error) {
	var p Payment
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&p).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPaymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetching by idempotency key: %w", err)
	}
	return &p, nil
}

// UpdateStatus transitions a payment to a new status.
// Uses WHERE status = from as an optimistic lock — if two requests race,
// only one will find RowsAffected > 0.
func (r *Repository) UpdateStatus(ctx context.Context, id string, from, to PaymentStatus, failureReason string) error {
	if err := ValidateTransition(from, to); err != nil {
		return err
	}

	result := r.db.WithContext(ctx).
		Model(&Payment{}).
		Where("id = ? AND status = ?", id, from).
		Updates(map[string]interface{}{
			"status":         to,
			"failure_reason": failureReason,
		})

	if result.Error != nil {
		return fmt.Errorf("updating payment status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("payment status already changed, possible concurrent update")
	}
	return nil
}

// List returns paginated payments for a merchant with optional filters.
func (r *Repository) List(ctx context.Context, merchantID string, req ListPaymentsRequest) ([]Payment, int64, error) {
	query := r.db.WithContext(ctx).
		Where("merchant_id = ?", merchantID).
		Order("created_at DESC")

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Method != "" {
		query = query.Where("method = ?", req.Method)
	}

	var total int64
	if err := query.Model(&Payment{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting payments: %w", err)
	}

	offset := (req.Page - 1) * req.PageSize
	var payments []Payment
	if err := query.Limit(req.PageSize).Offset(offset).Find(&payments).Error; err != nil {
		return nil, 0, fmt.Errorf("listing payments: %w", err)
	}

	return payments, total, nil
}
