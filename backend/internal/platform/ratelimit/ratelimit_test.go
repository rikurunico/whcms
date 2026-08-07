package ratelimit_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/ratelimit"
	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb, err := redisx.Connect(context.Background(), "localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s:%s", prefix, uuid.NewString())
}

// errOnCmdHook forces the named Redis command to fail, leaving every other
// command untouched. It lets us exercise a single failing call (e.g. DEL)
// against a real server without breaking the SCAN that precedes it.
type errOnCmdHook struct {
	name string
	err  error
}

func (h *errOnCmdHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *errOnCmdHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == h.name {
			cmd.SetErr(h.err)
			return h.err
		}
		return next(ctx, cmd)
	}
}

func (h *errOnCmdHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func TestAllowWithinLimit(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testRedis(t))
	key := uniqueKey("login")

	for i := 1; i <= 5; i++ {
		ok, err := l.Allow(ctx, key, 5, time.Minute)
		require.NoError(t, err)
		assert.True(t, ok, "call %d within limit", i)
	}
	ok, err := l.Allow(ctx, key, 5, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "6th call exceeds limit 5")
}

func TestAllowSeparateKeys(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testRedis(t))
	a, b := uniqueKey("a"), uniqueKey("b")

	ok, err := l.Allow(ctx, a, 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = l.Allow(ctx, a, 1, time.Minute)
	assert.False(t, ok)

	ok, err = l.Allow(ctx, b, 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "other key unaffected")
}

func TestReset(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testRedis(t))
	key := uniqueKey("reset")

	ok, _ := l.Allow(ctx, key, 1, time.Minute)
	require.True(t, ok)
	ok, _ = l.Allow(ctx, key, 1, time.Minute)
	require.False(t, ok)

	require.NoError(t, l.Reset(ctx, key))

	ok, err := l.Allow(ctx, key, 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "counter cleared after Reset")
}

func TestWindowExpiry(t *testing.T) {
	ctx := context.Background()
	l := ratelimit.New(testRedis(t))
	key := uniqueKey("window")

	ok, _ := l.Allow(ctx, key, 1, 100*time.Millisecond)
	require.True(t, ok)
	ok, _ = l.Allow(ctx, key, 1, 100*time.Millisecond)
	require.False(t, ok)

	time.Sleep(150 * time.Millisecond)
	ok, err := l.Allow(ctx, key, 1, 100*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, ok, "new fixed window after expiry")
}

func TestAllowRedisError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	l := ratelimit.New(rdb)
	require.NoError(t, rdb.Close())

	ok, err := l.Allow(ctx, uniqueKey("closed"), 5, time.Minute)
	assert.Error(t, err, "pipeline exec against a closed client must surface an error")
	assert.False(t, ok)
}

func TestResetScanError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	l := ratelimit.New(rdb)
	require.NoError(t, rdb.Close())

	err := l.Reset(ctx, uniqueKey("closed"))
	assert.Error(t, err, "scan against a closed client must surface an error")
}

func TestResetDelError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	key := uniqueKey("resetdel")
	windowKey := redisx.Key("rl", key, "0")
	require.NoError(t, rdb.Set(ctx, windowKey, "1", time.Minute).Err())
	t.Cleanup(func() { _ = rdb.Del(context.Background(), windowKey).Err() })

	hooked := testRedis(t)
	hooked.AddHook(&errOnCmdHook{name: "del", err: errors.New("del boom")})
	l := ratelimit.New(hooked)

	err := l.Reset(ctx, key)
	assert.Error(t, err, "del failing mid-scan must surface an error")
}
