package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

type jwtClaims struct {
	MerchantID string `json:"merchant_id"`
	Email      string `json:"email"`
	jwt.RegisteredClaims
}

// TokenService handles all JWT and refresh token operations
type TokenService struct {
	jwtSecret []byte // JWT secret 
}

func NewTokenService(jwtSecret []byte) *TokenService {
	return &TokenService{jwtSecret: jwtSecret}
}

// GenerateAccessToken creates a signed JWT for a merchant.
// TTL: 15 minutes. Stored in memory (Zustand) on the frontend — never in localStorage.
func (t *TokenService) GenerateAccessToken(merchantID, email string) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		MerchantID: merchantID,
		Email:      email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   merchantID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			Issuer:    "payflow",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.jwtSecret)
}

// ValidateAccessToken parses and validates a JWT, returning the embedded claims.
func (t *TokenService) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return t.jwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &Claims{
		MerchantID: claims.MerchantID,
		Email:      claims.Email,
	}, nil
}

// GenerateRefreshToken creates a cryptographically secure random token.
// We store only its SHA-256 hash in the DB — like password hashing for tokens.
// The raw token goes into an httpOnly cookie on the client.
func (t *TokenService) GenerateRefreshToken() (raw string, hashed string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	raw = hex.EncodeToString(b)
	hashed = hashToken(raw)
	return raw, hashed, nil
}

// RefreshTokenExpiry returns the absolute expiry time for a new refresh token
func (t *TokenService) RefreshTokenExpiry() time.Time {
	return time.Now().Add(refreshTokenTTL)
}

// hashToken produces a SHA-256 hex digest — used to store refresh tokens safely
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
