package redisclient

import (
	"os"

	"github.com/redis/go-redis/v9"

	"hr-management/backend/internal/config"
)

// Connect builds a client either from a single REDIS_URL (how hosted Redis
// providers like Upstash hand you a connection string — scheme rediss://
// carries the password and TLS requirement that the local dev instance,
// with neither, doesn't need) or from separate host/port/DB for local dev.
func Connect(cfg *config.Config) (*redis.Client, error) {
	if url := os.Getenv("REDIS_URL"); url != "" {
		opts, err := redis.ParseURL(url)
		if err != nil {
			return nil, err
		}
		return redis.NewClient(opts), nil
	}
	return redis.NewClient(&redis.Options{
		Addr: cfg.RedisHost + ":" + cfg.RedisPort,
		DB:   cfg.RedisDB,
	}), nil
}
