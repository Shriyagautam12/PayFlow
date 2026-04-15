package payment

import "time"

// PaymentStatus represents the current state of a payment
type PaymentStatus string

const (
	StatusInitiated  PaymentStatus = "initiated"
	StatusPending    PaymentStatus = "pending"
	StatusAuthorized PaymentStatus = "authorized"
	StatusCompleted  PaymentStatus = "completed"
	StatusFailed     PaymentStatus = "failed"
	StatusRefunded   PaymentStatus = "refunded"
)

// PaymentMethod represents how the customer is paying
type PaymentMethod string

const (
	MethodUPI        PaymentMethod = "upi"
	MethodCard       PaymentMethod = "card"
	MethodNetBanking PaymentMethod = "netbanking"
	MethodWallet     PaymentMethod = "wallet"
)

// Payment is the core domain model
type Payment struct {
	ID             string        `json:"id"              gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	MerchantID     string        `json:"merchant_id"     gorm:"not null;index"`
	IdempotencyKey string        `json:"-"               gorm:"uniqueIndex;not null"`
	Amount         int64         `json:"amount"`                                       // in paise — never float
	Currency       string        `json:"currency"        gorm:"default:INR"`
	Status         PaymentStatus `json:"status"          gorm:"not null"`
	Method         PaymentMethod `json:"method,omitempty"`
	FailureReason  string        `json:"failure_reason,omitempty"`
	Metadata       []byte        `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// IdempotencyRecord stores the cached result of a payment request in Redis
type IdempotencyRecord struct {
	PaymentID  string        `json:"payment_id"`
	Status     PaymentStatus `json:"status"`
	StatusCode int           `json:"status_code"`
}

// ── Request / Response DTOs ───────────────────────────────────────────────────

type InitiatePaymentRequest struct {
	Amount         int64         `json:"amount"   binding:"required,min=100"` // minimum ₹1 (100 paise)
	Currency       string        `json:"currency" binding:"omitempty,len=3"`
	Method         PaymentMethod `json:"method"   binding:"required"`
	IdempotencyKey string        `json:"-"` // set from header, not body
	Metadata       []byte        `json:"metadata,omitempty"`
}

type CapturePaymentRequest struct {
	PaymentID string `uri:"id" binding:"required"`
}

type RefundPaymentRequest struct {
	PaymentID string `uri:"id" binding:"required"`
	Reason    string `json:"reason"`
}

type ListPaymentsRequest struct {
	Status   PaymentStatus `form:"status"`
	Method   PaymentMethod `form:"method"`
	Page     int           `form:"page,default=1"`
	PageSize int           `form:"page_size,default=20"`
}

type ListPaymentsResponse struct {
	Payments   []Payment `json:"payments"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}
