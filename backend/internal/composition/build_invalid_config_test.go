package composition

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both error paths below return before Build ever dereferences database or
// rdb (the repository/redis constructors only store the handle - see
// build.go steps 1-3), so a nil *db.DB / *redis.Client is safe here and no
// live Postgres/Redis connection is required.

func TestBuild_InvalidEncryptionKey(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		JWTSecret:     "secret",
		EncryptionKey: []byte("too-short"), // must be exactly 32 bytes
		MailDriver:    "log",
	}

	app, err := Build(context.Background(), cfg, (*db.DB)(nil), (*redis.Client)(nil), nil, log)
	require.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestBuild_MailerError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		JWTSecret:     "secret",
		EncryptionKey: make([]byte, 32),
		MailDriver:    "http",
		MailHTTPURL:   "", // http driver requires a non-empty URL
	}

	app, err := Build(context.Background(), cfg, (*db.DB)(nil), (*redis.Client)(nil), nil, log)
	require.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "MAIL_HTTP_URL")
}
