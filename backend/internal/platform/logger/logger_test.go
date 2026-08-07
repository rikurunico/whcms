package logger_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/logger"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", logger.RequestID(ctx))

	ctx = logger.WithRequestID(ctx, "req-123")
	assert.Equal(t, "req-123", logger.RequestID(ctx))
}

func TestNewDoesNotPanicAndLogs(t *testing.T) {
	log := logger.New("development")
	assert.NotNil(t, log)
	// Log with a request-id context; must not panic.
	ctx := logger.WithRequestID(context.Background(), "req-1")
	log.InfoContext(ctx, "hello", "k", "v")
	log.With("mod", "test").InfoContext(ctx, "with attrs")
	log.WithGroup("grp").InfoContext(ctx, "with group")

	prod := logger.New("production")
	prod.DebugContext(ctx, "should be filtered")
}
