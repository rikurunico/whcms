package tokenstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/platform/tokenstore"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

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

func TestCreateAndConsume(t *testing.T) {
	ctx := context.Background()
	s := tokenstore.New(testRedis(t))

	token, err := s.Create(ctx, "verify_email", 42, time.Minute)
	require.NoError(t, err)
	assert.Len(t, token, 43, "opaque 43-char token")

	userID, err := s.Consume(ctx, "verify_email", token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), userID)
}

func TestConsumeIsSingleUse(t *testing.T) {
	ctx := context.Background()
	s := tokenstore.New(testRedis(t))

	token, err := s.Create(ctx, "reset_password", 7, time.Minute)
	require.NoError(t, err)

	_, err = s.Consume(ctx, "reset_password", token)
	require.NoError(t, err)

	_, err = s.Consume(ctx, "reset_password", token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeNotFound, ae.Code)
}

func TestConsumeWrongKind(t *testing.T) {
	ctx := context.Background()
	s := tokenstore.New(testRedis(t))

	token, err := s.Create(ctx, "verify_email", 7, time.Minute)
	require.NoError(t, err)

	_, err = s.Consume(ctx, "reset_password", token)
	assert.Error(t, err, "token kind is part of the key")

	// Still consumable under the right kind.
	userID, err := s.Consume(ctx, "verify_email", token)
	require.NoError(t, err)
	assert.Equal(t, int64(7), userID)
}

func TestConsumeExpired(t *testing.T) {
	ctx := context.Background()
	s := tokenstore.New(testRedis(t))

	token, err := s.Create(ctx, "verify_email", 7, 50*time.Millisecond)
	require.NoError(t, err)
	time.Sleep(80 * time.Millisecond)

	_, err = s.Consume(ctx, "verify_email", token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeNotFound, ae.Code)
}

func TestTokensAreUnique(t *testing.T) {
	ctx := context.Background()
	s := tokenstore.New(testRedis(t))
	a, _ := s.Create(ctx, "verify_email", 1, time.Minute)
	b, _ := s.Create(ctx, "verify_email", 1, time.Minute)
	assert.NotEqual(t, a, b)
}

// Note: crypto/rand.Read cannot be made to return an error in this Go
// version (it crashes the process irrecoverably instead, see
// https://pkg.go.dev/crypto/rand#Read), so that branch of Create is not
// exercised here.

func TestCreateSetError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	require.NoError(t, rdb.Close())
	s := tokenstore.New(rdb)

	_, err := s.Create(ctx, "verify_email", 1, time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenstore: set")
}

func TestConsumeGetDelError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	require.NoError(t, rdb.Close())
	s := tokenstore.New(rdb)

	_, err := s.Consume(ctx, "verify_email", "some-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenstore: getdel")
}

func TestConsumeParseIntError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	s := tokenstore.New(rdb)

	const kind, token = "verify_email", "not-a-real-token"
	require.NoError(t, rdb.Set(ctx, redisx.Key("token", kind, token), "not-an-int", time.Minute).Err())

	_, err := s.Consume(ctx, kind, token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokenstore: parse user id")
}
