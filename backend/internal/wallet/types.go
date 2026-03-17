package wallet

import "time"

// Wallet holds the current balance for a merchant.
// Balance is stored in the smallest currency unit (e.g. cents for USD).
type Wallet struct {
	ID         string    `json:"id"          gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	MerchantID string    `json:"merchant_id" gorm:"uniqueIndex;not null"`
	Balance    int64     `json:"balance"     gorm:"not null;default:0"` // cents
	Currency   string    `json:"currency"    gorm:"not null;default:'USD'"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// EntryType distinguishes the two sides of every double-entry movement.
type EntryType string

const (
	EntryCredit EntryType = "credit" // money in  (balance ↑)
	EntryDebit  EntryType = "debit"  // money out (balance ↓)
)

// LedgerEntry is an immutable record of a single side of a money movement.
// Every transaction produces exactly two entries (one debit + one credit).
type LedgerEntry struct {
	ID          string    `json:"id"           gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	WalletID    string    `json:"wallet_id"    gorm:"not null;index"`
	Type        EntryType `json:"type"         gorm:"not null"`                 // credit | debit
	Amount      int64     `json:"amount"       gorm:"not null"`                 // always positive, cents
	Description string    `json:"description"  gorm:"not null"`
	RefID       string    `json:"ref_id"       gorm:"index"`                    // idempotency / external ref
	CreatedAt   time.Time `json:"created_at"`
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

// WalletResponse is the JSON body for GET /v1/wallet
type WalletResponse struct {
	WalletID   string    `json:"wallet_id"`
	MerchantID string    `json:"merchant_id"`
	Balance    int64     `json:"balance"`  // cents
	Currency   string    `json:"currency"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LedgerResponse is the paginated JSON body for GET /v1/wallet/ledger
type LedgerResponse struct {
	Entries    []LedgerEntry `json:"entries"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalPages int           `json:"total_pages"`
}

// PayoutRequest is the JSON body for POST /v1/wallet/payout
type PayoutRequest struct {
	Amount      int64  `json:"amount"      binding:"required,min=1"`         // cents
	Currency    string `json:"currency"    binding:"required,len=3"`
	Destination string `json:"destination" binding:"required,min=5,max=100"` // bank / address ref
	Description string `json:"description" binding:"max=255"`
}

// PayoutResponse is returned after a successful payout initiation
type PayoutResponse struct {
	PayoutID    string `json:"payout_id"`    // the debit ledger entry ID
	Status      string `json:"status"`       // "pending"
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	Destination string `json:"destination"`
	NewBalance  int64  `json:"new_balance"`
}
