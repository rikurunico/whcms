package lock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/lock"
	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

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

func TestWithLockRunsFn(t *testing.T) {
	l := lock.New(testRedis(t))
	ran := false
	err := l.WithLock(context.Background(), "test:run", time.Minute, func() error {
		ran = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, ran)
}

func TestWithLockPropagatesFnError(t *testing.T) {
	l := lock.New(testRedis(t))
	boom := errors.New("boom")
	err := l.WithLock(context.Background(), "test:err", time.Minute, func() error { return boom })
	assert.ErrorIs(t, err, boom)
}

func TestWithLockConflict(t *testing.T) {
	ctx := context.Background()
	l := lock.New(testRedis(t))

	inFirst := make(chan struct{})
	releaseFirst := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- l.WithLock(ctx, "test:conflict", time.Minute, func() error {
			close(inFirst)
			<-releaseFirst
			return nil
		})
	}()

	<-inFirst
	// Second acquisition while held must fail with CONFLICT, fn not run.
	ran := false
	err := l.WithLock(ctx, "test:conflict", time.Minute, func() error {
		ran = true
		return nil
	})
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeConflict, ae.Code)
	assert.False(t, ran)

	close(releaseFirst)
	require.NoError(t, <-done)

	// Released: can acquire again.
	err = l.WithLock(ctx, "test:conflict", time.Minute, func() error { return nil })
	assert.NoError(t, err)
}

func TestWithLockReleasedAfterFnError(t *testing.T) {
	ctx := context.Background()
	l := lock.New(testRedis(t))
	_ = l.WithLock(ctx, "test:release", time.Minute, func() error { return errors.New("x") })
	// Lock must be free again despite the error.
	err := l.WithLock(ctx, "test:release", time.Minute, func() error { return nil })
	assert.NoError(t, err)
}

func TestWithLockAcquireError(t *testing.T) {
	l := lock.New(testRedis(t))

	// A pre-canceled context makes the SETNX acquire call fail, exercising
	// the acquire-error path without touching production code.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ran := false
	err := l.WithLock(ctx, "test:acquire-err", time.Minute, func() error {
		ran = true
		return nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, ran)
}
