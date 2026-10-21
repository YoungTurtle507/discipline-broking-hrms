package redisclient

import (
	"github.com/redis/go-redis/v9"

	"hr-management/backend/internal/config"
)

func Connect(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: cfg.RedisHost + ":" + cfg.RedisPort,
		DB:   cfg.RedisDB,
	})
}
