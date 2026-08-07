// Package presence tracks which admin/staff users have been active recently,
// backing the HostPanel "Staff Online" widget (sidebar + dashboard). Touch is
// called on every authenticated GET /auth/me (which the frontend already
// issues on every SSR page load), so an idle-but-not-logged-out user falls
// off the list once the freshness window elapses. Untouch additionally
// removes a user immediately on explicit logout, so "Staff Online" reflects
// a deliberate sign-out right away instead of waiting out the window.
//
// Storage: a Redis sorted set (whmcs:presence:staff) scored by last-seen unix
// time answers "who's active within the window" without a key-scan; a
// companion hash (whmcs:presence:meta) carries the display fields (email,
// role) for whoever is currently in that set. Stale hash entries for users
// who fall out of the window are harmless (bounded by total staff headcount)
// and are overwritten on their next Touch.
package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/redis/go-redis/v9"
)

const (
	zsetKey = "presence:staff"
	metaKey = "presence:meta"
)

// Service is a Redis-backed ports.PresenceTracker.
type Service struct {
	rdb    *redis.Client
	window time.Duration
}

// New builds a Service. window is how long a user is considered "online"
// after their last Touch (e.g. 5 minutes).
func New(rdb *redis.Client, window time.Duration) *Service {
	return &Service{rdb: rdb, window: window}
}

var _ ports.PresenceTracker = (*Service)(nil)

type meta struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Touch records userID as active now and opportunistically trims entries
// older than the window from the sorted set.
func (s *Service) Touch(ctx context.Context, userID int64, email, role string) error {
	b, err := json.Marshal(meta{Email: email, Role: role})
	if err != nil {
		return fmt.Errorf("presence: marshal meta: %w", err)
	}
	member := strconv.FormatInt(userID, 10)
	now := time.Now().UTC()
	cutoff := now.Add(-s.window).Unix()

	pipe := s.rdb.TxPipeline()
	pipe.ZAdd(ctx, redisx.Key(zsetKey), redis.Z{Score: float64(now.Unix()), Member: member})
	pipe.HSet(ctx, redisx.Key(metaKey), member, b)
	pipe.ZRemRangeByScore(ctx, redisx.Key(zsetKey), "-inf", strconv.FormatInt(cutoff, 10))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("presence: touch: %w", err)
	}
	return nil
}

// Untouch removes userID from the online set immediately (called on logout).
// A no-op if the user wasn't tracked (e.g. a client, or already expired).
func (s *Service) Untouch(ctx context.Context, userID int64) error {
	member := strconv.FormatInt(userID, 10)
	pipe := s.rdb.TxPipeline()
	pipe.ZRem(ctx, redisx.Key(zsetKey), member)
	pipe.HDel(ctx, redisx.Key(metaKey), member)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("presence: untouch: %w", err)
	}
	return nil
}

// List returns everyone active within the window, most-recently-active first.
func (s *Service) List(ctx context.Context) ([]ports.PresenceEntry, error) {
	cutoff := time.Now().UTC().Add(-s.window).Unix()
	members, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     redisx.Key(zsetKey),
		Start:   "+inf",
		Stop:    strconv.FormatInt(cutoff, 10),
		ByScore: true,
		Rev:     true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("presence: list: %w", err)
	}
	if len(members) == 0 {
		return []ports.PresenceEntry{}, nil
	}

	vals, err := s.rdb.HMGet(ctx, redisx.Key(metaKey), members...).Result()
	if err != nil {
		return nil, fmt.Errorf("presence: fetch meta: %w", err)
	}

	out := make([]ports.PresenceEntry, 0, len(members))
	for i, m := range members {
		userID, perr := strconv.ParseInt(m, 10, 64)
		if perr != nil {
			continue
		}
		raw, ok := vals[i].(string)
		if !ok || raw == "" {
			continue // metadata missing (e.g. flushed independently) - skip rather than show a blank row
		}
		var md meta
		if err := json.Unmarshal([]byte(raw), &md); err != nil {
			continue
		}
		out = append(out, ports.PresenceEntry{UserID: userID, Email: md.Email, Role: md.Role})
	}
	return out, nil
}
