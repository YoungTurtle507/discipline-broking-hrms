package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// SessionStore keeps sessions in Redis: a random ID mapping to a user ID,
// with a sliding TTL that gets pushed out on every authenticated request.
type SessionStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewSessionStore(rdb *redis.Client, ttlHours int) *SessionStore {
	return &SessionStore{rdb: rdb, ttl: time.Duration(ttlHours) * time.Hour}
}

func (s *SessionStore) TTL() time.Duration {
	return s.ttl
}

type sessionData struct {
	UserID int64 `json:"user_id"`
	// IssuedAt lets RequireAuth reject sessions that predate the user's last
	// password change, which is how an admin password reset logs that user out
	// everywhere without needing a user->sessions index in Redis.
	IssuedAt time.Time `json:"issued_at"`
}

func (s *SessionStore) Create(ctx context.Context, userID int64) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(sessionData{UserID: userID, IssuedAt: time.Now()})
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, sessionKeyPrefix+id, data, s.ttl).Err(); err != nil {
		return "", err
	}
	return id, nil
}

func (s *SessionStore) UserID(ctx context.Context, sessionID string) (int64, bool) {
	userID, _, ok := s.Lookup(ctx, sessionID)
	return userID, ok
}

// Lookup returns the session's user and when it was issued.
func (s *SessionStore) Lookup(ctx context.Context, sessionID string) (int64, time.Time, bool) {
	val, err := s.rdb.Get(ctx, sessionKeyPrefix+sessionID).Result()
	if err != nil {
		return 0, time.Time{}, false
	}
	var data sessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return 0, time.Time{}, false
	}
	return data.UserID, data.IssuedAt, true
}

func (s *SessionStore) Touch(ctx context.Context, sessionID string) {
	s.rdb.Expire(ctx, sessionKeyPrefix+sessionID, s.ttl)
}

func (s *SessionStore) Delete(ctx context.Context, sessionID string) {
	s.rdb.Del(ctx, sessionKeyPrefix+sessionID)
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
