package wallet

import (
	"context"
	"errors"
	"fmt"
	"math"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrWalletAlreadyExists = errors.New("wallet already exists")
)

// Repository handles all wallet DB operations.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// GetByMerchantID fetches the wallet for a merchant. Returns ErrWalletNotFound if absent.
func (r *Repository) GetByMerchantID(ctx context.Context, merchantID string) (*Wallet, error) {
	var w Wallet
	err := r.db.WithContext(ctx).Where("merchant_id = ?", merchantID).First(&w).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWalletNotFound
	}
	return &w, err
}

// CreateWallet inserts a new zero-balance wallet for a merchant.
func (r *Repository) CreateWallet(ctx context.Context, merchantID string) (*Wallet, error) {
	w := &Wallet{MerchantID: merchantID, Balance: 0, Currency: "USD"}
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return nil, fmt.Errorf("creating wallet: %w", err)
	}
	return w, nil
}

// GetOrCreate returns an existing wallet or creates one if it doesn't exist yet.
func (r *Repository) GetOrCreate(ctx context.Context, merchantID string) (*Wallet, error) {
	w, err := r.GetByMerchantID(ctx, merchantID)
	if errors.Is(err, ErrWalletNotFound) {
		return r.CreateWallet(ctx, merchantID)
	}
	return w, err
}

// Credit atomically adds amount to the wallet balance and inserts two ledger entries
func (r *Repository) Credit(ctx context.Context, walletID string, amount int64, description, refID string) (*LedgerEntry, error) {
	var entry *LedgerEntry

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the wallet row for this transaction
		var w Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&w, "id = ?", walletID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWalletNotFound
			}
			return fmt.Errorf("locking wallet: %w", err)
		}

		// Update balance
		w.Balance += amount
		if err := tx.Save(&w).Error; err != nil {
			return fmt.Errorf("updating balance: %w", err)
		}

		// Insert credit entry (money into merchant wallet)
		credit := &LedgerEntry{
			WalletID:    walletID,
			Type:        EntryCredit,
			Amount:      amount,
			Description: description,
			RefID:       refID,
		}
		if err := tx.Create(credit).Error; err != nil {
			return fmt.Errorf("inserting credit entry: %w", err)
		}

		// Insert corresponding debit entry (funding source side)
		debit := &LedgerEntry{
			WalletID:    walletID,
			Type:        EntryDebit,
			Amount:      amount,
			Description: "funding: " + description,
			RefID:       refID,
		}
		if err := tx.Create(debit).Error; err != nil {
			return fmt.Errorf("inserting debit entry: %w", err)
		}

		entry = credit
		return nil
	})

	return entry, err
}

// Debit atomically subtracts amount from wallet and records two ledger entries.
// Returns ErrInsufficientFunds if balance would go negative.
func (r *Repository) Debit(ctx context.Context, walletID string, amount int64, description, refID string) (*LedgerEntry, int64, error) {
	var entry *LedgerEntry
	var newBalance int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the wallet row
		var w Wallet
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&w, "id = ?", walletID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWalletNotFound
			}
			return fmt.Errorf("locking wallet: %w", err)
		}

		if w.Balance < amount {
			return ErrInsufficientFunds
		}

		w.Balance -= amount
		newBalance = w.Balance
		if err := tx.Save(&w).Error; err != nil {
			return fmt.Errorf("updating balance: %w", err)
		}

		// Insert debit entry (money leaving the merchant wallet)
		debit := &LedgerEntry{
			WalletID:    walletID,
			Type:        EntryDebit,
			Amount:      amount,
			Description: description,
			RefID:       refID,
		}
		if err := tx.Create(debit).Error; err != nil {
			return fmt.Errorf("inserting debit entry: %w", err)
		}

		// Insert corresponding credit entry (destination side)
		credit := &LedgerEntry{
			WalletID:    walletID,
			Type:        EntryCredit,
			Amount:      amount,
			Description: "payout: " + description,
			RefID:       refID,
		}
		if err := tx.Create(credit).Error; err != nil {
			return fmt.Errorf("inserting credit entry: %w", err)
		}

		entry = debit
		return nil
	})

	return entry, newBalance, err
}

// ListEntries returns paginated ledger entries for a wallet, newest first.
func (r *Repository) ListEntries(ctx context.Context, walletID string, page, pageSize int) ([]LedgerEntry, int64, error) {
	var entries []LedgerEntry
	var total int64

	offset := (page - 1) * pageSize

	if err := r.db.WithContext(ctx).
		Model(&LedgerEntry{}).
		Where("wallet_id = ?", walletID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting entries: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("wallet_id = ?", walletID).
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return nil, 0, fmt.Errorf("listing entries: %w", err)
	}

	return entries, total, nil
}

// totalPages calculates the number of pages given total rows and page size.
func totalPages(total int64, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(pageSize)))
}
