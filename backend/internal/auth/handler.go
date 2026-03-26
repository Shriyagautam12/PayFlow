package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Shriyagautam12/PayFlow/pkg/response"
	"go.uber.org/zap"
)

const (
	refreshTokenCookie = "pf_refresh_token"
	csrfStateCookie    = "pf_oauth_state"
	cookieMaxAge       = int(7 * 24 * time.Hour / time.Second) // 7 days in seconds
)

// Handler wires HTTP routes to the auth Service
type Handler struct {
	svc    *Service
	google *GoogleOAuth
	log    *zap.Logger
}

func NewHandler(svc *Service, google *GoogleOAuth, log *zap.Logger) *Handler {
	return &Handler{svc: svc, google: google, log: log}
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// Register godoc
// @Summary      Register a new merchant
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Merchant details"
// @Success      201  {object} AuthResponse
// @Failure      409  {object} ErrorResponse
// @Router       /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, rawRefresh, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.Conflict(c, "email already registered")
			return
		}
		h.log.Error("register failed", zap.Error(err))
		response.Internal(c, "registration failed")
		return
	}

	h.setRefreshCookie(c, rawRefresh)
	c.JSON(http.StatusCreated, resp)
}

// Login godoc
// @Summary      Login with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Credentials"
// @Success      200  {object} AuthResponse
// @Failure      401  {object} ErrorResponse
// @Router       /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, rawRefresh, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCreds) {
			response.Unauthorized(c, "invalid email or password")
			return
		}
		if errors.Is(err, ErrMerchantSuspended) {
			response.Forbidden(c, "account suspended")
			return
		}
		h.log.Error("login failed", zap.Error(err))
		response.Internal(c, "login failed")
		return
	}

	h.setRefreshCookie(c, rawRefresh)
	c.JSON(http.StatusOK, resp)
}

// Refresh godoc
// @Summary      Refresh access token using httpOnly cookie
// @Tags         auth
// @Produce      json
// @Success      200  {object} AuthResponse
// @Failure      401  {object} ErrorResponse
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	rawToken, err := c.Cookie(refreshTokenCookie)
	if err != nil {
		response.Unauthorized(c, "missing refresh token")
		return
	}

	resp, newRawRefresh, err := h.svc.Refresh(c.Request.Context(), rawToken)
	if err != nil {
		h.clearRefreshCookie(c)
		response.Unauthorized(c, "session expired, please log in again")
		return
	}

	// Rotate the cookie with the new refresh token
	h.setRefreshCookie(c, newRawRefresh)
	c.JSON(http.StatusOK, resp)
}

// Logout godoc
// @Summary      Logout — revokes refresh token
// @Tags         auth
// @Success      204
// @Router       /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	rawToken, err := c.Cookie(refreshTokenCookie)
	if err == nil {
		// Best-effort revocation — don't fail if token not found
		_ = h.svc.Logout(c.Request.Context(), rawToken)
	}
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

// GoogleLogin redirects the merchant to Google's OAuth consent screen.
// We set a CSRF state token in a short-lived cookie to verify on callback.
func (h *Handler) GoogleLogin(c *gin.Context) {
	state, err := generateState()
	if err != nil {
		response.Internal(c, "failed to initiate oauth")
		return
	}

	// Store state in cookie — verified in callback to prevent CSRF
	c.SetCookie(csrfStateCookie, state, 300, "/", "", true, true) // 5 min TTL
	c.Redirect(http.StatusTemporaryRedirect, h.google.AuthURL(state))
}

// GoogleCallback handles the redirect from Google after the user consents.
func (h *Handler) GoogleCallback(c *gin.Context) {
	// 1. Verify CSRF state
	storedState, err := c.Cookie(csrfStateCookie)
	if err != nil || storedState != c.Query("state") {
		response.BadRequest(c, "invalid oauth state")
		return
	}
	h.clearStateCookie(c)

	// 2. Exchange code for Google user info
	googleUser, err := h.google.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		h.log.Error("google exchange failed", zap.Error(err))
		response.BadRequest(c, "google authentication failed")
		return
	}

	// 3. Find or create merchant, issue PayFlow JWT
	resp, rawRefresh, err := h.svc.HandleGoogleCallback(c.Request.Context(), googleUser)
	if err != nil {
		if errors.Is(err, ErrMerchantSuspended) {
			response.Forbidden(c, "account suspended")
			return
		}
		h.log.Error("google callback failed", zap.Error(err))
		response.Internal(c, "authentication failed")
		return
	}

	h.setRefreshCookie(c, rawRefresh)

	// Redirect to dashboard — frontend picks up access_token from JSON or query param
	// In production, use a short-lived one-time code here instead of embedding in URL
	c.Redirect(http.StatusTemporaryRedirect, "/dashboard?token="+resp.AccessToken)
}

// ── Cookie Helpers ────────────────────────────────────────────────────────────

// setRefreshCookie writes the refresh token as a Secure, HttpOnly cookie.
// HttpOnly = JS cannot read it (XSS protection)
// Secure   = only sent over HTTPS
// SameSite = Strict prevents CSRF
func (h *Handler) setRefreshCookie(c *gin.Context, rawToken string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshTokenCookie, rawToken, cookieMaxAge, "/", "", true, true)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetCookie(refreshTokenCookie, "", -1, "/", "", true, true)
}

func (h *Handler) clearStateCookie(c *gin.Context) {
	c.SetCookie(csrfStateCookie, "", -1, "/", "", true, true)
}

// ── Utilities ─────────────────────────────────────────────────────────────────

// generateState creates a random hex string for OAuth CSRF protection
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
