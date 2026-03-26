package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/Shriyagautam12/PayFlow/internal/auth"
	"github.com/Shriyagautam12/PayFlow/internal/wallet"
	"github.com/Shriyagautam12/PayFlow/pkg/middleware"
)

const (
	// Auth
	routeAuthRegister       = "/auth/register"
	routeAuthLogin          = "/auth/login"
	routeAuthRefresh        = "/auth/refresh"
	routeAuthLogout         = "/auth/logout"
	routeAuthGoogle         = "/auth/google"
	routeAuthGoogleCallback = "/auth/google/callback"

	// Merchant
	routeMerchantMe = "/me"

	// Wallet
	routeWalletGet    = "/wallet"
	routeWalletLedger = "/wallet/ledger"
	routeWalletPayout = "/wallet/payout"
)

type Deps struct {
	Auth   *auth.Handler
	Wallet *wallet.Handler
	Log    *zap.Logger
}

type RouterConfig struct {
	Env           string
	AllowedOrigin string
	TokenSvc      *auth.TokenService
}

func New(cfg RouterConfig, deps Deps) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(deps.Log))
	r.Use(middleware.CORS(cfg.AllowedOrigin))

	r.GET("/health", healthCheck)

	v1 := r.Group("/v1")
	mountAuth(v1, deps.Auth)
	mountProtected(v1, cfg.TokenSvc, deps)

	return r
}

func mountAuth(v1 *gin.RouterGroup, h *auth.Handler) {
	v1.POST(routeAuthRegister, h.Register)
	v1.POST(routeAuthLogin, h.Login)
	v1.POST(routeAuthRefresh, h.Refresh)
	v1.POST(routeAuthLogout, h.Logout)
	v1.GET(routeAuthGoogle, h.GoogleLogin)
	v1.GET(routeAuthGoogleCallback, h.GoogleCallback)
}

func mountProtected(v1 *gin.RouterGroup, tokenSvc *auth.TokenService, deps Deps) {
	g := v1.Group("/")
	g.Use(middleware.RequireAuth(tokenSvc))

	mountMerchant(g)
	mountWallet(g, deps.Wallet)
}

func mountMerchant(g *gin.RouterGroup) {
	g.GET(routeMerchantMe, getMe)
}

func mountWallet(g *gin.RouterGroup, h *wallet.Handler) {
	g.GET(routeWalletGet, h.GetWallet)
	g.GET(routeWalletLedger, h.GetLedger)
	g.POST(routeWalletPayout, h.Payout)
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "payflow"})
}

func getMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"merchant_id":    middleware.GetMerchantID(c),
		"merchant_email": middleware.GetMerchantEmail(c),
	})
}
