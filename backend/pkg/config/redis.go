package config

import (
	"crypto/tls"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient parses the REDIS_URL and returns a configured client.
// Handles both:
//
//	redis://...   (local, no TLS)
//	rediss://...  (Upstash / production, TLS enabled)
func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	// If URL starts with rediss:// — enable TLS (required for Upstash)
	if len(redisURL) > 8 && redisURL[:8] == "rediss://" {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return redis.NewClient(opts), nil
}
