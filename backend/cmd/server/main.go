package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Shriyagautam12/PayFlow/internal/auth"
	"github.com/Shriyagautam12/PayFlow/pkg/config"
	"github.com/Shriyagautam12/PayFlow/pkg/middleware"
)

func main() {
	// ── Logger ──────────────────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Auto-migrate schema (in production, use proper migrations)
	db.AutoMigrate(&auth.Merchant{}, &auth.RefreshToken{})

	// ── Services ─────────────────────────────────────────────────────────────
	tokenSvc := auth.NewTokenService([]byte(cfg.JWTSecret))
	googleOAuth := auth.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	authSvc := auth.NewService(db, tokenSvc, log)
	authHandler := auth.NewHandler(authSvc, googleOAuth, log)

	// ── Router ───────────────────────────────────────────────────────────────
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.CORS(cfg.AppURL))

	// Health check — used by K8s liveness probe
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "payflow-auth"})
	})

	// All API routes under /v1
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1)

	// Protected route example — shows middleware usage
	protected := v1.Group("/")
	protected.Use(middleware.RequireAuth(tokenSvc))
	{
		protected.GET("/me", func(c *gin.Context) {
			merchantID := middleware.GetMerchantID(c)
			c.JSON(http.StatusOK, gin.H{"merchant_id": merchantID})
		})
	}

	// ── Server with graceful shutdown ─────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start in goroutine so we can listen for shutdown signal
	go func() {
		log.Info("auth service starting", zap.String("port", cfg.Port), zap.String("env", cfg.AppEnv))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// Wait for SIGTERM or SIGINT (K8s sends SIGTERM on pod shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("forced shutdown", zap.Error(err))
	}
	log.Info("server stopped")
}
