package wallet

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/Shriyagautam12/PayFlow/pkg/middleware"
	"github.com/Shriyagautam12/PayFlow/pkg/response"
	"go.uber.org/zap"
)

// Handler wires HTTP routes to the wallet Service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// @Router       /wallet [get]
func (h *Handler) GetWallet(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	resp, err := h.svc.GetBalance(c.Request.Context(), merchantID)
	if err != nil {
		h.log.Error("get wallet failed", zap.String("merchant_id", merchantID), zap.Error(err))
		response.Internal(c, "could not retrieve wallet")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Router       /wallet/ledger [get]
func (h *Handler) GetLedger(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	page := middleware.QueryInt(c, "page", 1)
	pageSize := middleware.QueryInt(c, "page_size", 20)

	resp, err := h.svc.GetLedger(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		h.log.Error("get ledger failed", zap.String("merchant_id", merchantID), zap.Error(err))
		response.Internal(c, "could not retrieve ledger")
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Router       /wallet/payout [post]
func (h *Handler) Payout(c *gin.Context) {
	merchantID := middleware.GetMerchantID(c)

	var req PayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.Payout(c.Request.Context(), merchantID, req)
	if err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			response.PaymentRequired(c, "insufficient funds")
			return
		}
		h.log.Error("payout failed", zap.String("merchant_id", merchantID), zap.Error(err))
		response.Internal(c, "payout failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

