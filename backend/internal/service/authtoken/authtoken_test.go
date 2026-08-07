package authtoken_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/clock"
	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/service/authtoken"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "test-jwt-secret"

func fixedClock() clock.Fixed {
	return clock.Fixed{T: time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)}
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb, err := redisx.Connect(context.Background(), "localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// badRedis returns a client pointed at a port nothing listens on, so any
// command fails fast with a dial/connection error (no Ping on construction).
func badRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// failOnCommand is a go-redis hook that forces a specific command name to
// fail while letting every other command pass through untouched. Used to
// exercise error branches that only trigger on a later call within the same
// Manager method (e.g. RotateRefresh's internal CreateRefresh).
type failOnCommand struct{ name string }

func (f failOnCommand) DialHook(next redis.DialHook) redis.DialHook { return next }

func (f failOnCommand) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == f.name {
			return errors.New("forced failure for test")
		}
		return next(ctx, cmd)
	}
}

func (f failOnCommand) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			if cmd.Name() == f.name {
				return errors.New("forced failure for test")
			}
		}
		return next(ctx, cmds)
	}
}

func TestIssueAndParseAccess(t *testing.T) {
	m := authtoken.New(secret, nil, fixedClock())

	token, jti, err := m.IssueAccess(42, "client", 7)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.NotEmpty(t, jti)

	claims, err := m.ParseAccess(token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UserID)
	assert.Equal(t, "client", claims.Role)
	assert.Equal(t, int64(7), claims.ClientID)
	assert.Equal(t, jti, claims.JTI)
}

func TestParseAccessRejectsWrongSecret(t *testing.T) {
	m := authtoken.New(secret, nil, fixedClock())
	other := authtoken.New("different-secret", nil, fixedClock())

	token, _, err := m.IssueAccess(1, "admin", 0)
	require.NoError(t, err)

	_, err = other.ParseAccess(token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeUnauthorized, ae.Code)
}

func TestParseAccessRejectsExpired(t *testing.T) {
	issuer := authtoken.New(secret, nil, fixedClock())
	token, _, err := issuer.IssueAccess(1, "client", 2)
	require.NoError(t, err)

	// Verifier's clock is 16 minutes later (past the 15m TTL).
	later := clock.Fixed{T: fixedClock().T.Add(16 * time.Minute)}
	verifier := authtoken.New(secret, nil, later)
	_, err = verifier.ParseAccess(token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeUnauthorized, ae.Code)
}

func TestParseAccessRejectsGarbage(t *testing.T) {
	m := authtoken.New(secret, nil, fixedClock())
	for _, tok := range []string{"", "not-a-jwt", "aaa.bbb.ccc"} {
		_, err := m.ParseAccess(tok)
		assert.Error(t, err, "token=%q", tok)
	}
}

func TestParseAccessRejectsNoneAlgorithm(t *testing.T) {
	m := authtoken.New(secret, nil, fixedClock())
	// Unsigned token (alg=none): header {"alg":"none","typ":"JWT"}.
	unsigned := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiIxIiwicm9sZSI6ImFkbWluIn0."
	_, err := m.ParseAccess(unsigned)
	assert.Error(t, err)
}

func TestRefreshLifecycle(t *testing.T) {
	ctx := context.Background()
	m := authtoken.New(secret, testRedis(t), fixedClock())

	token, err := m.CreateRefresh(ctx, 42, "jti-1")
	require.NoError(t, err)
	assert.Len(t, token, 43, "opaque 43-char token")

	userID, jti, err := m.ConsumeRefresh(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, int64(42), userID)
	assert.Equal(t, "jti-1", jti)

	// Single use: second consume fails.
	_, _, err = m.ConsumeRefresh(ctx, token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeUnauthorized, ae.Code)
}

func TestRotateRefresh(t *testing.T) {
	ctx := context.Background()
	m := authtoken.New(secret, testRedis(t), fixedClock())

	oldToken, err := m.CreateRefresh(ctx, 7, "jti-old")
	require.NoError(t, err)

	userID, newToken, err := m.RotateRefresh(ctx, oldToken, "jti-new")
	require.NoError(t, err)
	assert.Equal(t, int64(7), userID)
	assert.NotEqual(t, oldToken, newToken)

	// Old token unusable, new token carries the new jti.
	_, _, err = m.ConsumeRefresh(ctx, oldToken)
	assert.Error(t, err)

	uid, jti, err := m.ConsumeRefresh(ctx, newToken)
	require.NoError(t, err)
	assert.Equal(t, int64(7), uid)
	assert.Equal(t, "jti-new", jti)
}

func TestRotateRefreshUnknownToken(t *testing.T) {
	m := authtoken.New(secret, testRedis(t), fixedClock())
	_, _, err := m.RotateRefresh(context.Background(), "unknown-token", "jti")
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeUnauthorized, ae.Code)
}

func TestRevokeRefresh(t *testing.T) {
	ctx := context.Background()
	m := authtoken.New(secret, testRedis(t), fixedClock())

	token, err := m.CreateRefresh(ctx, 9, "jti-r")
	require.NoError(t, err)
	require.NoError(t, m.RevokeRefresh(ctx, token))

	_, _, err = m.ConsumeRefresh(ctx, token)
	assert.Error(t, err, "revoked token unusable")

	// Revoking unknown token is a no-op.
	assert.NoError(t, m.RevokeRefresh(ctx, "does-not-exist"))
}

func TestParseAccessRejectsNonNumericSubject(t *testing.T) {
	now := fixedClock().T
	// Same wire shape as the package's internal jwtClaims, built and signed
	// directly so we can put a non-numeric "sub" past a valid signature.
	claims := struct {
		Role string `json:"role"`
		CID  int64  `json:"cid"`
		jwt.RegisteredClaims
	}{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "not-a-number",
			ID:        "jti-x",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)

	m := authtoken.New(secret, nil, fixedClock())
	_, err = m.ParseAccess(token)
	var ae *apperr.Error
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, apperr.CodeUnauthorized, ae.Code)
}

func TestCreateRefreshRedisError(t *testing.T) {
	m := authtoken.New(secret, badRedis(t), fixedClock())
	_, err := m.CreateRefresh(context.Background(), 1, "jti")
	assert.Error(t, err)
}

func TestConsumeRefreshRedisError(t *testing.T) {
	m := authtoken.New(secret, badRedis(t), fixedClock())
	_, _, err := m.ConsumeRefresh(context.Background(), "whatever-token")
	assert.Error(t, err)
}

func TestConsumeRefreshDecodeError(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	m := authtoken.New(secret, rdb, fixedClock())

	token := "garbage-token-payload"
	require.NoError(t, rdb.Set(ctx, redisx.Key("refresh", token), "not-json", time.Minute).Err())

	_, _, err := m.ConsumeRefresh(ctx, token)
	assert.Error(t, err)
}

func TestRevokeRefreshRedisError(t *testing.T) {
	m := authtoken.New(secret, badRedis(t), fixedClock())
	err := m.RevokeRefresh(context.Background(), "token")
	assert.Error(t, err)
}

func TestRotateRefreshCreateFails(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	m := authtoken.New(secret, rdb, fixedClock())

	oldToken, err := m.CreateRefresh(ctx, 7, "jti-old")
	require.NoError(t, err)

	// Force the replacement CreateRefresh's SET to fail while the initial
	// ConsumeRefresh's GETDEL still succeeds, to reach RotateRefresh's
	// second error return.
	rdb.AddHook(failOnCommand{name: "set"})

	_, _, err = m.RotateRefresh(ctx, oldToken, "jti-new")
	assert.Error(t, err)
}

func TestRevokeAllForUser(t *testing.T) {
	ctx := context.Background()
	m := authtoken.New(secret, testRedis(t), fixedClock())

	t1, err := m.CreateRefresh(ctx, 900001, "jti-1")
	require.NoError(t, err)
	t2, err := m.CreateRefresh(ctx, 900001, "jti-2")
	require.NoError(t, err)
	other, err := m.CreateRefresh(ctx, 900002, "jti-other")
	require.NoError(t, err)

	require.NoError(t, m.RevokeAllForUser(ctx, 900001))

	_, _, err = m.ConsumeRefresh(ctx, t1)
	assert.Error(t, err, "revoked token must be unusable")
	_, _, err = m.ConsumeRefresh(ctx, t2)
	assert.Error(t, err, "revoked token must be unusable")
	_, _, err = m.ConsumeRefresh(ctx, other)
	assert.NoError(t, err, "another user's session must survive")

	// Idempotent on a user with no outstanding sessions.
	require.NoError(t, m.RevokeAllForUser(ctx, 900001))
}

func TestRevokeAllForUserRedisDown(t *testing.T) {
	m := authtoken.New(secret, badRedis(t), fixedClock())
	assert.Error(t, m.RevokeAllForUser(context.Background(), 1))
}
