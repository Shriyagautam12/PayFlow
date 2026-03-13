package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrEmailTaken        = errors.New("email already registered")
	ErrInvalidCreds      = errors.New("invalid email or password")
	ErrTokenExpired      = errors.New("refresh token expired or invalid")
	ErrMerchantSuspended = errors.New("merchant account is suspended")
)

// Service contains all auth business logic.
// It is intentionally decoupled from HTTP — easy to unit test.
type Service struct {
	db     *gorm.DB
	tokens *TokenService
	log    *zap.Logger
}

func NewService(db *gorm.DB, tokens *TokenService, log *zap.Logger) *Service {
	return &Service{db: db, tokens: tokens, log: log}
}

// ── Email / Password ─────────────────────────────────────────────────────────

// Register creates a new merchant account with a hashed password and generated API key.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, string, error) {
	// 1. Check email uniqueness
	var existing Merchant
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return nil, "", ErrEmailTaken
	}

	// 2. Hash password with bcrypt (cost=12 — good balance of security vs speed)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, "", fmt.Errorf("hashing password: %w", err)
	}

	// 3. Generate a unique API key for server-to-server calls
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generating api key: %w", err)
	}

	merchant := &Merchant{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hash),
		APIKey:   apiKey,
		Status:   "active",
	}

	if err := s.db.WithContext(ctx).Create(merchant).Error; err != nil {
		return nil, "", fmt.Errorf("creating merchant: %w", err)
	}

	s.log.Info("merchant registered", zap.String("merchant_id", merchant.ID), zap.String("email", merchant.Email))

	return s.issueTokens(ctx, merchant)
}

// Login verifies credentials and issues tokens.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, string, error) {
	var merchant Merchant
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&merchant).Error; err != nil {
		// Return a generic error — don't leak whether the email exists
		return nil, "", ErrInvalidCreds
	}

	if merchant.Status == "suspended" {
		return nil, "", ErrMerchantSuspended
	}

	// Constant-time comparison — prevents timing attacks
	if err := bcrypt.CompareHashAndPassword([]byte(merchant.Password), []byte(req.Password)); err != nil {
		return nil, "", ErrInvalidCreds
	}

	s.log.Info("merchant logged in", zap.String("merchant_id", merchant.ID))
	return s.issueTokens(ctx, &merchant)
}

// ── Token Lifecycle ──────────────────────────────────────────────────────────

// Refresh validates a raw refresh token and issues a new access + refresh token pair.
// Old refresh token is deleted (rotation) — mitigates token theft.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*AuthResponse, string, error) {
	hashed := hashToken(rawToken)

	var stored RefreshToken
	err := s.db.WithContext(ctx).Where("token_hash = ?", hashed).First(&stored).Error
	if err != nil {
		return nil, "", ErrTokenExpired
	}

	if time.Now().After(stored.ExpiresAt) {
		// Clean up expired token
		s.db.Delete(&stored)
		return nil, "", ErrTokenExpired
	}

	// Load merchant
	var merchant Merchant
	if err := s.db.WithContext(ctx).First(&merchant, "id = ?", stored.MerchantID).Error; err != nil {
		return nil, "", fmt.Errorf("merchant not found: %w", err)
	}

	// Delete old refresh token (rotation — each refresh token is single-use)
	s.db.WithContext(ctx).Delete(&stored)

	s.log.Info("token refreshed", zap.String("merchant_id", merchant.ID))
	return s.issueTokens(ctx, &merchant)
}

// Logout revokes the refresh token — access token expires naturally (15min)
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	hashed := hashToken(rawToken)
	return s.db.WithContext(ctx).Where("token_hash = ?", hashed).Delete(&RefreshToken{}).Error
}

// ── Google OAuth ─────────────────────────────────────────────────────────────

// HandleGoogleCallback processes the verified Google user info.
// If the merchant exists → login. If not → auto-register them.
// Either way, we issue PayFlow's own JWT — backend stays provider-agnostic.
func (s *Service) HandleGoogleCallback(ctx context.Context, googleUser *GoogleUserInfo) (*AuthResponse, string, error) {
	var merchant Merchant

	// Try to find by Google ID first (fastest path for returning users)
	err := s.db.WithContext(ctx).Where("google_id = ?", googleUser.ID).First(&merchant).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Try by email — merchant may have registered with email/password before
		emailErr := s.db.WithContext(ctx).Where("email = ?", googleUser.Email).First(&merchant).Error

		if errors.Is(emailErr, gorm.ErrRecordNotFound) {
			// Brand new merchant — auto-register
			apiKey, err := generateAPIKey()
			if err != nil {
				return nil, "", err
			}

			merchant = Merchant{
				Name:      googleUser.Name,
				Email:     googleUser.Email,
				GoogleID:  googleUser.ID,
				AvatarURL: googleUser.Picture,
				Password:  "", // No password for OAuth merchants
				APIKey:    apiKey,
				Status:    "active",
			}
			if err := s.db.WithContext(ctx).Create(&merchant).Error; err != nil {
				return nil, "", fmt.Errorf("creating google merchant: %w", err)
			}
			s.log.Info("merchant registered via google", zap.String("merchant_id", merchant.ID))
		} else if emailErr != nil {
			return nil, "", fmt.Errorf("looking up merchant: %w", emailErr)
		} else {
			// Existing email-password merchant — link their Google ID
			s.db.WithContext(ctx).Model(&merchant).Updates(map[string]interface{}{
				"google_id":  googleUser.ID,
				"avatar_url": googleUser.Picture,
			})
			s.log.Info("linked google to existing account", zap.String("merchant_id", merchant.ID))
		}
	} else if err != nil {
		return nil, "", fmt.Errorf("google lookup: %w", err)
	}

	if merchant.Status == "suspended" {
		return nil, "", ErrMerchantSuspended
	}

	return s.issueTokens(ctx, &merchant)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// issueTokens creates an access token + refresh token and persists the refresh token.
// Returns: AuthResponse (JSON body), raw refresh token (for cookie), error
func (s *Service) issueTokens(ctx context.Context, merchant *Merchant) (*AuthResponse, string, error) {
	accessToken, err := s.tokens.GenerateAccessToken(merchant.ID, merchant.Email)
	if err != nil {
		return nil, "", fmt.Errorf("generating access token: %w", err)
	}

	rawRefresh, hashedRefresh, err := s.tokens.GenerateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("generating refresh token: %w", err)
	}

	// Persist hashed refresh token
	rt := RefreshToken{
		MerchantID: merchant.ID,
		TokenHash:  hashedRefresh,
		ExpiresAt:  s.tokens.RefreshTokenExpiry(),
	}
	if err := s.db.WithContext(ctx).Create(&rt).Error; err != nil {
		return nil, "", fmt.Errorf("persisting refresh token: %w", err)
	}

	return &AuthResponse{
		Merchant:    merchant,
		AccessToken: accessToken,
		ExpiresIn:   int(accessTokenTTL.Seconds()), // 900
	}, rawRefresh, nil
}

// generateAPIKey creates a random hex string for merchant API key
func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "pf_live_" + hex.EncodeToString(b), nil
}
