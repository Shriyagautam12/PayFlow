package auth

import "time"

// Merchant is the core domain model
type Merchant struct {
	ID        string    `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string    `json:"name" gorm:"not null"`
	Email     string    `json:"email" gorm:"uniqueIndex;not null"`
	Password  string    `json:"-" gorm:"not null"`           // bcrypt hash — never serialized
	GoogleID  string    `json:"-" gorm:"index"`              // set if signed up via Google
	AvatarURL string    `json:"avatar_url,omitempty"`
	APIKey    string    `json:"-" gorm:"uniqueIndex;not null"`
	Status    string    `json:"status" gorm:"default:active"` // active | suspended
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RefreshToken stored in DB — allows revocation
type RefreshToken struct {
	ID         string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	MerchantID string    `gorm:"not null;index"`
	TokenHash  string    `gorm:"not null;uniqueIndex"` // SHA-256 hash of the actual token
	ExpiresAt  time.Time `gorm:"not null"`
	CreatedAt  time.Time
}

// ── Request / Response DTOs ──────────────────────────────────────────────────

type RegisterRequest struct {
	Name     string `json:"name"     binding:"required,min=2,max=100"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Merchant    *Merchant `json:"merchant"`
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"` // seconds
}

// GoogleUserInfo is what we get from Google's userinfo endpoint
type GoogleUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	Verified  bool   `json:"verified_email"`
}

// Claims are the JWT payload fields
type Claims struct {
	MerchantID string `json:"merchant_id"`
	Email      string `json:"email"`
	// Standard fields (exp, iat) handled by the jwt library
}