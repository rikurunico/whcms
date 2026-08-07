// Package cache implements ports.Cache: JSON values in Redis with TTL.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/redis/go-redis/v9"
)

// Cache is a Redis JSON ports.Cache. Keys: whmcs:cache:<key>.
type Cache struct {
	rdb *redis.Client
}

// New builds a Cache.
func New(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

func key(k string) string { return redisx.Key("cache", k) }

// GetJSON unmarshals the cached value into out; returns (false, nil) on miss.
func (c *Cache) GetJSON(ctx context.Context, k string, out any) (bool, error) {
	raw, err := c.rdb.Get(ctx, key(k)).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache: get %s: %w", k, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("cache: unmarshal %s: %w", k, err)
	}
	return true, nil
}

// SetJSON marshals val and stores it with ttl.
func (c *Cache) SetJSON(ctx context.Context, k string, val any, ttl time.Duration) error {
	raw, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("cache: marshal %s: %w", k, err)
	}
	if err := c.rdb.Set(ctx, key(k), raw, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set %s: %w", k, err)
	}
	return nil
}

// Delete removes the cached value.
func (c *Cache) Delete(ctx context.Context, k string) error {
	if err := c.rdb.Del(ctx, key(k)).Err(); err != nil {
		return fmt.Errorf("cache: del %s: %w", k, err)
	}
	return nil
}
