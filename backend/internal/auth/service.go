package auth

import (
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCreds      = errors.New("invalid email or password")
	ErrTokenExpired      = errors.New("refresh token expired or invalid")
	ErrMerchantSuspended = errors.New("merchant account is suspended")
)

type Service struct {
	db     *gorm.DB
	tokens *TokenService
	log    *zap.Logger
}

func NewService(db *gorm.DB, tokens *TokenService, log *zap.Logger) *Service {
	return &Service{db: db, tokens: tokens, log: log}
}
