package payment

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/Shriyagautam12/PayFlow/pkg/middleware"
	"github.com/Shriyagautam12/PayFlow/pkg/response"
	"go.uber.org/zap"
)

// Handler wires HTTP routes to the payment Service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// Initiate godoc
// @Router  /payment [post]
func (h *Handler) Initiate(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	idempotencyKey := c.GetHeader("Idempotency-Key")
	if idempotencyKey == "" {
		response.BadRequest(c, "Idempotency-Key header is required")
		return
	}

	var req InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.IdempotencyKey = idempotencyKey

	p, err := h.svc.Initiate(c.Request.Context(), merchantID, req)
	if err != nil {
		h.log.Error("initiate payment failed", zap.String("merchant_id", merchantID), zap.Error(err))
		response.Internal(c, "failed to initiate payment")
		return
	}

	c.JSON(http.StatusCreated, p)
}

// Get godoc
// @Router  /payment/:id [get]
func (h *Handler) Get(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)
	paymentID := c.Param("id")

	p, err := h.svc.GetByID(c.Request.Context(), paymentID, merchantID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			response.NotFound(c, "payment not found")
			return
		}
		h.log.Error("get payment failed", zap.String("payment_id", paymentID), zap.Error(err))
		response.Internal(c, "failed to retrieve payment")
		return
	}

	c.JSON(http.StatusOK, p)
}

// List godoc
// @Router  /payment [get]
func (h *Handler) List(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	var req ListPaymentsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	resp, err := h.svc.List(c.Request.Context(), merchantID, req)
	if err != nil {
		h.log.Error("list payments failed", zap.String("merchant_id", merchantID), zap.Error(err))
		response.Internal(c, "failed to list payments")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Capture godoc
// @Router  /payment/:id/capture [post]
func (h *Handler) Capture(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)
	paymentID := c.Param("id")

	p, err := h.svc.Capture(c.Request.Context(), paymentID, merchantID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			response.NotFound(c, "payment not found")
			return
		}
		if errors.Is(err, ErrInvalidTransition) {
			response.BadRequest(c, "payment cannot be captured in its current state")
			return
		}
		h.log.Error("capture failed", zap.String("payment_id", paymentID), zap.Error(err))
		response.Internal(c, "failed to capture payment")
		return
	}

	c.JSON(http.StatusOK, p)
}

// Refund godoc
// @Router  /payment/:id/refund [post]
func (h *Handler) Refund(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)
	paymentID := c.Param("id")

	var req RefundPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	p, err := h.svc.Refund(c.Request.Context(), paymentID, merchantID, req.Reason)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			response.NotFound(c, "payment not found")
			return
		}
		if errors.Is(err, ErrPaymentNotComplete) {
			response.BadRequest(c, "only completed payments can be refunded")
			return
		}
		h.log.Error("refund failed", zap.String("payment_id", paymentID), zap.Error(err))
		response.Internal(c, "failed to refund payment")
		return
	}

	c.JSON(http.StatusOK, p)
}
