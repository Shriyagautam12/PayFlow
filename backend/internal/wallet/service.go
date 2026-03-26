package wallet

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// Service contains all wallet business logic.
type Service struct {
	repo *Repository
	log  *zap.Logger
}

func NewService(repo *Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// GetBalance returns the wallet for the given merchant, creating one if it doesn't exist.
func (s *Service) GetBalance(ctx context.Context, merchantID string) (*WalletResponse, error) {
	w, err := s.repo.GetOrCreate(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	return &WalletResponse{
		WalletID:   w.ID,
		MerchantID: w.MerchantID,
		Balance:    w.Balance,
		Currency:   w.Currency,
		UpdatedAt:  w.UpdatedAt,
	}, nil
}

// GetLedger returns a paginated list of ledger entries for the merchant's wallet.
func (s *Service) GetLedger(ctx context.Context, merchantID string, page, pageSize int) (*LedgerResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	w, err := s.repo.GetOrCreate(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("get ledger wallet: %w", err)
	}

	entries, total, err := s.repo.ListEntries(ctx, w.ID, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	return &LedgerResponse{
		Entries:    entries,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages(total, pageSize),
	}, nil
}

// Payout debits the merchant wallet and records the movement in the ledger.
// Returns ErrInsufficientFunds if the wallet balance is too low.
func (s *Service) Payout(ctx context.Context, merchantID string, req PayoutRequest) (*PayoutResponse, error) {
	w, err := s.repo.GetByMerchantID(ctx, merchantID)
	if errors.Is(err, ErrWalletNotFound) {
		// Merchant has never credited their wallet — definitely insufficient funds
		return nil, ErrInsufficientFunds
	}
	if err != nil {
		return nil, fmt.Errorf("payout wallet lookup: %w", err)
	}

	description := req.Description
	if description == "" {
		description = fmt.Sprintf("payout to %s", req.Destination)
	}

	debitEntry, newBalance, err := s.repo.Debit(ctx, w.ID, req.Amount, description, "")
	if err != nil {
		return nil, err // ErrInsufficientFunds passes through as-is
	}

	s.log.Info("payout initiated",
		zap.String("merchant_id", merchantID),
		zap.Int64("amount", req.Amount),
		zap.String("destination", req.Destination),
		zap.String("payout_id", debitEntry.ID),
	)

	return &PayoutResponse{
		PayoutID:    debitEntry.ID,
		Status:      "pending",
		Amount:      req.Amount,
		Currency:    req.Currency,
		Destination: req.Destination,
		NewBalance:  newBalance,
	}, nil
}
