package totpguard_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/platform/totpguard"

	"github.com/google/uuid"
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
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func uniqueUserID() int64 {
	// Derive a large pseudo-unique int64 from a uuid so parallel test runs
	// against the same shared test DB don't collide on the same key.
	return int64(uuid.New().ID())
}

func TestAcceptFirstUseSucceeds(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userID := uniqueUserID()

	ok, err := g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "first use of a step must be accepted")
}

func TestAcceptSameStepRejected(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userID := uniqueUserID()

	ok, err := g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "replaying the same step must be rejected")
}

func TestAcceptOlderStepRejected(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userID := uniqueUserID()

	ok, err := g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = g.Accept(ctx, userID, 999, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "a step older than the last accepted one must be rejected (clock skew replay)")
}

func TestAcceptNewerStepSucceeds(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userID := uniqueUserID()

	ok, err := g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = g.Accept(ctx, userID, 1001, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "a genuinely newer step must be accepted")
}

func TestAcceptIsolatedPerUser(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userA, userB := uniqueUserID(), uniqueUserID()

	ok, err := g.Accept(ctx, userA, 1000, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = g.Accept(ctx, userB, 1000, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "another user's guard must be independent")
}

func TestAcceptExpiresAfterTTL(t *testing.T) {
	ctx := context.Background()
	g := totpguard.New(testRedis(t))
	userID := uniqueUserID()

	ok, err := g.Accept(ctx, userID, 1000, 100*time.Millisecond)
	require.NoError(t, err)
	require.True(t, ok)

	time.Sleep(150 * time.Millisecond)

	// After TTL expiry even the SAME step is accepted again (record forgotten).
	// This is fine: TTL is chosen well beyond any code's real validity window,
	// so an expired record can never coincide with a still-valid code.
	ok, err = g.Accept(ctx, userID, 1000, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "record must expire after ttl")
}

func TestAcceptRedisError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	g := totpguard.New(rdb)
	require.NoError(t, rdb.Close())

	_, err := g.Accept(ctx, uniqueUserID(), 1, time.Minute)
	assert.Error(t, err, "script eval against a closed client must surface an error")
}

func TestKeyIsNamespacedPerUser(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	g := totpguard.New(rdb)
	userID := uniqueUserID()
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), redisx.Key("totp", "step", fmt.Sprintf("%d", userID))).Err()
	})

	ok, err := g.Accept(ctx, userID, 42, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	val, err := rdb.Get(ctx, redisx.Key("totp", "step", fmt.Sprintf("%d", userID))).Result()
	require.NoError(t, err)
	assert.Equal(t, "42", val)
}
