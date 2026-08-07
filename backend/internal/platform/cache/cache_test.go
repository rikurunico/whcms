package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/cache"
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

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestSetGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := cache.New(testRedis(t))
	key := "test:" + uuid.NewString()

	require.NoError(t, c.SetJSON(ctx, key, payload{Name: "hosting", Count: 3}, time.Minute))

	var got payload
	hit, err := c.GetJSON(ctx, key, &got)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, payload{Name: "hosting", Count: 3}, got)
}

func TestGetMiss(t *testing.T) {
	ctx := context.Background()
	c := cache.New(testRedis(t))

	var got payload
	hit, err := c.GetJSON(ctx, "test:missing:"+uuid.NewString(), &got)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	c := cache.New(testRedis(t))
	key := "test:" + uuid.NewString()

	require.NoError(t, c.SetJSON(ctx, key, payload{Name: "x"}, time.Minute))
	require.NoError(t, c.Delete(ctx, key))

	var got payload
	hit, err := c.GetJSON(ctx, key, &got)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestTTLExpiry(t *testing.T) {
	ctx := context.Background()
	c := cache.New(testRedis(t))
	key := "test:" + uuid.NewString()

	require.NoError(t, c.SetJSON(ctx, key, payload{Name: "x"}, 50*time.Millisecond))
	time.Sleep(80 * time.Millisecond)

	var got payload
	hit, err := c.GetJSON(ctx, key, &got)
	require.NoError(t, err)
	assert.False(t, hit)
}

func TestSetJSONMarshalError(t *testing.T) {
	ctx := context.Background()
	c := cache.New(testRedis(t))
	err := c.SetJSON(ctx, "test:bad", make(chan int), time.Minute)
	assert.Error(t, err)
}

func TestGetJSONUnmarshalError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	c := cache.New(rdb)
	k := "test:badjson:" + uuid.NewString()

	require.NoError(t, rdb.Set(ctx, redisx.Key("cache", k), "not-valid-json", time.Minute).Err())

	var got payload
	hit, err := c.GetJSON(ctx, k, &got)
	assert.Error(t, err)
	assert.False(t, hit)
}

func TestGetJSONRedisError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	c := cache.New(rdb)
	require.NoError(t, rdb.Close())

	var got payload
	hit, err := c.GetJSON(ctx, "test:closed:"+uuid.NewString(), &got)
	assert.Error(t, err)
	assert.False(t, hit)
}

func TestSetJSONRedisError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	c := cache.New(rdb)
	require.NoError(t, rdb.Close())

	err := c.SetJSON(ctx, "test:closed:"+uuid.NewString(), payload{Name: "x"}, time.Minute)
	assert.Error(t, err)
}

func TestDeleteRedisError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	c := cache.New(rdb)
	require.NoError(t, rdb.Close())

	err := c.Delete(ctx, "test:closed:"+uuid.NewString())
	assert.Error(t, err)
}
