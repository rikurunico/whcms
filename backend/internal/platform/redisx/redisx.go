// Package redisx builds the shared go-redis client. All WHCMS keys use
// the "whmcs:" prefix (CONTRACTS.md §2); helpers here centralize that.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// KeyPrefix is prepended to every Redis key.
const KeyPrefix = "whmcs:"

// Key builds a namespaced Redis key: Key("refresh", token) -> "whmcs:refresh:<token>".
func Key(parts ...string) string {
	k := KeyPrefix
	for i, p := range parts {
		if i > 0 {
			k += ":"
		}
		k += p
	}
	return k
}

// Connect opens a Redis client and verifies connectivity.
func Connect(ctx context.Context, addr, password string, dbNum int) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: dbNum})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis: ping %s: %w", addr, err)
	}
	return c, nil
}
