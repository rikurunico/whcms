// Package totpguard implements ports.TOTPReplayGuard: a Redis-backed record
// of the last-accepted TOTP time-step per user, used to reject replayed 2FA
// codes on login.
package totpguard

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/redis/go-redis/v9"
)

// Guard is a Redis ports.TOTPReplayGuard. Keys: whmcs:totp:step:<userID>.
type Guard struct {
	rdb *redis.Client
}

// New builds a Guard.
func New(rdb *redis.Client) *Guard { return &Guard{rdb: rdb} }

// acceptScript atomically rejects a step that is not strictly newer than the
// stored last-accepted step, otherwise records the new step with a TTL (in
// milliseconds, so sub-second TTLs are honored precisely). Returns 1 when
// accepted, 0 when rejected as a replay.
var acceptScript = redis.NewScript(`
local last = redis.call("GET", KEYS[1])
if last and tonumber(last) >= tonumber(ARGV[1]) then
    return 0
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
return 1
`)

// Accept reports whether step is newer than the last-accepted step recorded
// for userID; if so it atomically records step (with ttl) and returns true.
func (g *Guard) Accept(ctx context.Context, userID int64, step int64, ttl time.Duration) (bool, error) {
	key := redisx.Key("totp", "step", strconv.FormatInt(userID, 10))
	ttlMillis := ttl.Milliseconds()
	if ttlMillis < 1 {
		ttlMillis = 1
	}
	res, err := acceptScript.Run(ctx, g.rdb, []string{key}, step, ttlMillis).Int()
	if err != nil {
		return false, fmt.Errorf("totpguard: accept: %w", err)
	}
	return res == 1, nil
}
