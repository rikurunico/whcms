// Package lock implements ports.Locker: a Redis SET NX distributed lock used
// to keep cron jobs single-flight.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/redis/go-redis/v9"
)

// Locker is a Redis ports.Locker. Keys: whmcs:lock:<key>.
type Locker struct {
	rdb *redis.Client
}

// New builds a Locker.
func New(rdb *redis.Client) *Locker { return &Locker{rdb: rdb} }

// releaseScript deletes the lock only if we still own it.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`)

// WithLock acquires whmcs:lock:<key> for ttl and runs fn. If the lock is held
// elsewhere it returns a CONFLICT apperr without running fn. The lock is
// released after fn (only if still owned; expiry makes crashes safe).
func (l *Locker) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return fmt.Errorf("lock: rand: %w", err)
	}
	owner := hex.EncodeToString(token)
	fullKey := redisx.Key("lock", key)

	ok, err := l.rdb.SetNX(ctx, fullKey, owner, ttl).Result()
	if err != nil {
		return fmt.Errorf("lock: acquire %s: %w", key, err)
	}
	if !ok {
		return apperr.Newf(apperr.CodeConflict, "lock %s already held", key)
	}
	defer func() {
		// Best-effort release with fresh context in case ctx is done.
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = releaseScript.Run(releaseCtx, l.rdb, []string{fullKey}, owner).Err()
	}()
	return fn()
}
