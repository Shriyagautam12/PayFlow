package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string // Server port
	DatabaseURL        string // Database
	RedisURL           string // Redis URL
	JWTSecret          string // JWT secret
	GoogleClientID     string // Google Client ID
	GoogleClientSecret string // Google Client Secret
	GoogleRedirectURL  string // Google Redirect URL
	AppEnv             string // development | production
	AppURL             string // App URL
}

// Load reads config from environment variables.
func LoadConfig() (*Config, error) {
	// Load .env in dev — no-op in production (env vars set by infra)
	_ = godotenv.Load()

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        mustGetEnv("DATABASE_URL"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:          mustGetEnv("JWT_SECRET"),
		GoogleClientID:     mustGetEnv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: mustGetEnv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/v1/auth/google/callback"),
		AppEnv:             getEnv("APP_ENV", "development"),
		AppURL:             getEnv("APP_URL", "http://localhost:3000"),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}
