// Package tokenstore implements ports.TokenStore over Redis: single-use
// tokens for email verification and password reset.
package tokenstore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/redis/go-redis/v9"
)

// TokenStore is a Redis ports.TokenStore. Keys: whmcs:token:<kind>:<token>.
type TokenStore struct {
	rdb *redis.Client
}

// New builds a TokenStore.
func New(rdb *redis.Client) *TokenStore { return &TokenStore{rdb: rdb} }

func key(kind, token string) string { return redisx.Key("token", kind, token) }

// Create issues a random URL-safe token for userID with the given TTL.
func (s *TokenStore) Create(ctx context.Context, kind string, userID int64, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("tokenstore: rand: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw) // 43 chars
	if err := s.rdb.Set(ctx, key(kind, token), userID, ttl).Err(); err != nil {
		return "", fmt.Errorf("tokenstore: set: %w", err)
	}
	return token, nil
}

// Consume atomically deletes the token and returns the owning user id.
// Missing/expired/reused tokens return a NOT_FOUND apperr.
func (s *TokenStore) Consume(ctx context.Context, kind, token string) (int64, error) {
	val, err := s.rdb.GetDel(ctx, key(kind, token)).Result()
	if err == redis.Nil {
		return 0, apperr.New(apperr.CodeNotFound, "token not found or expired")
	}
	if err != nil {
		return 0, fmt.Errorf("tokenstore: getdel: %w", err)
	}
	userID, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("tokenstore: parse user id: %w", err)
	}
	return userID, nil
}
