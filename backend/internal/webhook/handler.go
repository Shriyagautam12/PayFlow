package webhook

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/Shriyagautam12/PayFlow/pkg/middleware"
	"github.com/Shriyagautam12/PayFlow/pkg/response"
)

// Handler wires HTTP routes to the webhook Service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register godoc
// @Router POST /v1/webhook
// Body: { "url": "https://...", "event_type": "payment.completed" }
// Response: webhook ID + the signing secret (shown only once — merchant must save it)
func (h *Handler) Register(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	var req RegisterWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Generate a random 32-byte signing secret. Shown to the merchant once.
	secret, err := generateSecret()
	if err != nil {
		h.log.Error("generating webhook secret", zap.Error(err))
		response.Internal(c, "could not generate secret")
		return
	}

	w, err := h.svc.Register(c.Request.Context(), merchantID, req, secret)
	if err != nil {
		h.log.Error("registering webhook", zap.Error(err))
		response.Internal(c, "could not register webhook")
		return
	}

	// Return the secret here — it is never stored in plaintext after this response.
	// The merchant uses it to verify X-PayFlow-Signature on their end.
	c.JSON(http.StatusCreated, gin.H{
		"id":         w.ID,
		"url":        w.URL,
		"event_type": w.EventType,
		"active":     w.Active,
		"secret":     secret, // shown once
		"created_at": w.CreatedAt,
	})
}

// List godoc
// @Router GET /v1/webhooks
func (h *Handler) List(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	hooks, err := h.svc.List(c.Request.Context(), merchantID)
	if err != nil {
		h.log.Error("listing webhooks", zap.Error(err))
		response.Internal(c, "could not list webhooks")
		return
	}

	out := make([]RegisterWebhookResponse, len(hooks))
	for i, hook := range hooks {
		out[i] = RegisterWebhookResponse{
			ID:        hook.ID,
			URL:       hook.URL,
			EventType: hook.EventType,
			Active:    hook.Active,
			CreatedAt: hook.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, ListWebhooksResponse{Webhooks: out})
}

// generateSecret creates a random 32-byte hex string used as the HMAC signing key.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
