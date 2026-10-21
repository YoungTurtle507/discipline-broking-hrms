package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	loginAttemptPrefix = "login_attempts:"
	maxLoginAttempts   = 5
	loginLockoutWindow = 15 * time.Minute
)

// LoginLimiter blocks brute-forcing a username's password after too many
// failed attempts in a rolling window, per the spec's rate-limiting requirement.
type LoginLimiter struct {
	rdb *redis.Client
}

func NewLoginLimiter(rdb *redis.Client) *LoginLimiter {
	return &LoginLimiter{rdb: rdb}
}

func (l *LoginLimiter) TooManyAttempts(ctx context.Context, username string) bool {
	count, err := l.rdb.Get(ctx, loginAttemptPrefix+username).Int()
	if err != nil {
		return false
	}
	return count >= maxLoginAttempts
}

func (l *LoginLimiter) RecordFailure(ctx context.Context, username string) {
	key := loginAttemptPrefix + username
	count, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return
	}
	if count == 1 {
		l.rdb.Expire(ctx, key, loginLockoutWindow)
	}
}

func (l *LoginLimiter) Reset(ctx context.Context, username string) {
	l.rdb.Del(ctx, loginAttemptPrefix+username)
}
