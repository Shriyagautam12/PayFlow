package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/Shriyagautam12/PayFlow/internal/auth"
	"github.com/Shriyagautam12/PayFlow/internal/events"
	"github.com/Shriyagautam12/PayFlow/internal/payment"
	"github.com/Shriyagautam12/PayFlow/internal/wallet"
	"github.com/Shriyagautam12/PayFlow/internal/webhook"
	"github.com/Shriyagautam12/PayFlow/pkg/config"
	"github.com/Shriyagautam12/PayFlow/router"
)

func main() {
	// ── Logger ───────────────────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ───────────────────────────────────────────────────────────────
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := config.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Auto-migrate schema (in production, use proper migrations)
	db.AutoMigrate(
		&auth.Merchant{}, &auth.RefreshToken{},
		&wallet.Wallet{}, &wallet.LedgerEntry{},
		&webhook.Webhook{}, &webhook.WebhookDelivery{},
	)

	// ── Kafka ─────────────────────────────────────────────────────────────────
	kafkaBrokers := strings.Split(cfg.KafkaBrokers, ",")
	eventProducer := events.NewProducer(kafkaBrokers)
	defer eventProducer.Close()

	// ── Services ─────────────────────────────────────────────────────────────
	tokenSvc := auth.NewTokenService([]byte(cfg.JWTSecret))
	googleOAuth := auth.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	authSvc := auth.NewService(db, tokenSvc, log)
	authHandler := auth.NewHandler(authSvc, googleOAuth, log)

	walletRepo := wallet.NewRepository(db)
	walletSvc := wallet.NewService(walletRepo, log)
	walletHandler := wallet.NewHandler(walletSvc, log)

	webhookRepo := webhook.NewRepository(db)
	webhookSvc := webhook.NewService(webhookRepo, log)
	webhookHandler := webhook.NewHandler(webhookSvc, log)

	paymentRepo := payment.NewRepository(db)
	paymentSvc := payment.NewService(paymentRepo, redisClient, walletSvc, eventProducer, log)
	paymentHandler := payment.NewHandler(paymentSvc, log)

	// ── Kafka consumer (webhook delivery) ────────────────────────────────────
	consumer := events.NewConsumer(kafkaBrokers, "webhook-service", webhookSvc.HandleEvent, log)
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()
	go consumer.Start(consumerCtx)
	defer consumer.Close()

	// ── Retry ticker — re-attempt failed deliveries every minute ─────────────
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				webhookSvc.ProcessRetries(context.Background())
			case <-consumerCtx.Done():
				return
			}
		}
	}()

	// ── Router ───────────────────────────────────────────────────────────────
	r := router.New(
		router.RouterConfig{
			Env:           cfg.AppEnv,
			AllowedOrigin: cfg.AppURL,
			TokenSvc:      tokenSvc,
		},
		router.Deps{
			Auth:    authHandler,
			Wallet:  walletHandler,
			Payment: paymentHandler,
			Webhook: webhookHandler,
			Log:     log,
		},
	)

	// ── Server with graceful shutdown ─────────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
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
