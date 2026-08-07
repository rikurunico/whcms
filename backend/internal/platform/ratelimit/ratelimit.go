// Package ratelimit implements ports.RateLimiter: a Redis fixed-window
// counter (CONTRACTS.md §3).
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/redis/go-redis/v9"
)

// Limiter is a Redis fixed-window ports.RateLimiter.
// Keys: whmcs:rl:<key>:<windowStart>.
type Limiter struct {
	rdb *redis.Client
}

// New builds a Limiter.
func New(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// Allow increments the counter for key in the current fixed window and
// reports whether the request is within limit.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	windowStart := time.Now().UnixNano() / int64(window)
	fullKey := redisx.Key("rl", key, fmt.Sprintf("%d", windowStart))

	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, window+time.Second)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("ratelimit: incr %s: %w", key, err)
	}
	return incr.Val() <= int64(limit), nil
}

// Reset clears all windows for key (e.g. unlock after successful login).
func (l *Limiter) Reset(ctx context.Context, key string) error {
	pattern := redisx.Key("rl", key, "*")
	iter := l.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := l.rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return fmt.Errorf("ratelimit: del: %w", err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("ratelimit: scan: %w", err)
	}
	return nil
}
