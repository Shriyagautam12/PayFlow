package wallet

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

// RegisterRoutes mounts all wallet endpoints under /v1/wallet.
// All routes require the caller to be authenticated (JWT injected by RequireAuth middleware).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	w := rg.Group("/wallet")
	{
		w.GET("", h.GetWallet)
		w.GET("/ledger", h.GetLedger)
		w.POST("/payout", h.Payout)
	}
}

// @Router       /wallet [get]
func (h *Handler) GetWallet(c *gin.Context) {
	merchantID := getMerchantID(c)

	resp, err := h.svc.GetBalance(c.Request.Context(), merchantID)
	if err != nil {
		h.log.Error("get wallet failed", zap.String("merchant_id", merchantID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errResp("could not retrieve wallet"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Router       /wallet/ledger [get]
func (h *Handler) GetLedger(c *gin.Context) {
	merchantID := getMerchantID(c)

	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "page_size", 20)

	resp, err := h.svc.GetLedger(c.Request.Context(), merchantID, page, pageSize)
	if err != nil {
		h.log.Error("get ledger failed", zap.String("merchant_id", merchantID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errResp("could not retrieve ledger"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// @Router       /wallet/payout [post]
func (h *Handler) Payout(c *gin.Context) {
	merchantID := getMerchantID(c)

	var req PayoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errResp(err.Error()))
		return
	}

	resp, err := h.svc.Payout(c.Request.Context(), merchantID, req)
	if err != nil {
		if errors.Is(err, ErrInsufficientFunds) {
			c.JSON(http.StatusPaymentRequired, errResp("insufficient funds"))
			return
		}
		h.log.Error("payout failed", zap.String("merchant_id", merchantID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, errResp("payout failed"))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ── Helpers

type errorResponse struct {
	Error string `json:"error"`
}

func errResp(msg string) errorResponse {
	return errorResponse{Error: msg}
}

// getMerchantID reads the merchant_id injected by the RequireAuth middleware.
func getMerchantID(c *gin.Context) string {
	id, _ := c.Get("merchant_id")
	return id.(string)
}

// queryInt reads an integer query param, falling back to defaultVal on missing or invalid input.
func queryInt(c *gin.Context, key string, defaultVal int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
