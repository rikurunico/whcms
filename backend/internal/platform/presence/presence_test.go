package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/presence"
	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb, err := redisx.Connect(context.Background(), "localhost:6379", "", 15) // isolated test DB
	if err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		_ = rdb.Close()
	})
	return rdb
}

func TestTouchAndList(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)
	s := presence.New(rdb, 5*time.Minute)

	require.NoError(t, s.Touch(ctx, 1, "admin@example.com", "admin"))
	require.NoError(t, s.Touch(ctx, 2, "staff@example.com", "staff"))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2, "both recently-touched users are online")

	byID := map[int64]string{}
	for _, e := range list {
		byID[e.UserID] = e.Email
	}
	assert.Equal(t, "admin@example.com", byID[1])
	assert.Equal(t, "staff@example.com", byID[2])
}

func TestUntouchRemovesImmediately(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)
	s := presence.New(rdb, 5*time.Minute)

	require.NoError(t, s.Touch(ctx, 1, "admin@example.com", "admin"))
	require.NoError(t, s.Touch(ctx, 2, "staff@example.com", "staff"))

	require.NoError(t, s.Untouch(ctx, 2))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1, "logged-out user is gone right away, not after the window elapses")
	assert.Equal(t, "admin@example.com", list[0].Email)
}

func TestUntouchUnknownUserIsNoop(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)
	s := presence.New(rdb, 5*time.Minute)

	require.NoError(t, s.Untouch(ctx, 999))
}

func TestTouchUpdatesMetaOnRepeat(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)
	s := presence.New(rdb, 5*time.Minute)

	require.NoError(t, s.Touch(ctx, 1, "old@example.com", "staff"))
	require.NoError(t, s.Touch(ctx, 1, "new@example.com", "admin"))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1, "same user touched twice is one entry, not two")
	assert.Equal(t, "new@example.com", list[0].Email)
	assert.Equal(t, "admin", list[0].Role)
}

func TestListExpiresStaleEntries(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)

	// A window measured in whole seconds: ZSet scores are unix-second
	// granularity, so anything sub-second isn't reliably distinguishable
	// against real Redis round-trip latency.
	s := presence.New(rdb, 1*time.Second)
	require.NoError(t, s.Touch(ctx, 1, "gone@example.com", "staff"))

	// Force user 1's score into the past directly (rather than sleeping past
	// a 1s window, which would slow the test) so the next Touch's trim and
	// List's own cutoff both exclude it.
	require.NoError(t, rdb.ZAdd(ctx, redisx.Key("presence:staff"), redis.Z{
		Score: float64(time.Now().Add(-time.Hour).Unix()), Member: "1",
	}).Err())

	require.NoError(t, s.Touch(ctx, 2, "fresh@example.com", "admin"))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1, "the stale entry fell out of the window")
	assert.Equal(t, "fresh@example.com", list[0].Email)
}

func TestListEmpty(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	rdb.FlushDB(ctx)
	s := presence.New(rdb, 5*time.Minute)

	list, err := s.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}
