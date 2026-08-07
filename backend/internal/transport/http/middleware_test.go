package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/clock"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/internal/service/authtoken"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func tokenManager() *authtoken.Manager {
	return authtoken.New("test-secret", nil, clock.New())
}

func readEnvelope(t *testing.T, body io.Reader) httpx.Envelope {
	t.Helper()
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(raw, &env), "body: %s", raw)
	return env
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	app := fiber.New()
	app.Use(transporthttp.RequestID())
	var seen string
	app.Get("/", func(c fiber.Ctx) error {
		seen = httpx.RequestID(c)
		return c.SendString("ok")
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	assert.NotEmpty(t, seen)
	assert.Equal(t, seen, resp.Header.Get("X-Request-ID"))
}

func TestRequestIDPropagated(t *testing.T) {
	app := fiber.New()
	app.Use(transporthttp.RequestID())
	app.Get("/", func(c fiber.Ctx) error { return c.SendString("ok") })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "client-supplied-id")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, "client-supplied-id", resp.Header.Get("X-Request-ID"))
}

func TestErrorHandlerMapsAppErrors(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	mw := &transporthttp.Middleware{Tokens: tokenManager(), Log: discard}
	app.Get("/protected", mw.RequireAuth(), func(c fiber.Ctx) error { return c.SendString("ok") })

	resp, err := app.Test(httptest.NewRequest("GET", "/protected", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Error)
	assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
}

func TestErrorHandlerMapsFiberNotFound(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	resp, err := app.Test(httptest.NewRequest("GET", "/no-such-route", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestErrorHandlerMapsFiberErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		wantCode string
	}{
		{"bad request", fiber.StatusBadRequest, "VALIDATION"},
		{"method not allowed", fiber.StatusMethodNotAllowed, "VALIDATION"},
		{"request entity too large", fiber.StatusRequestEntityTooLarge, "VALIDATION"},
		{"unprocessable entity", fiber.StatusUnprocessableEntity, "VALIDATION"},
		{"unauthorized", fiber.StatusUnauthorized, "UNAUTHORIZED"},
		{"forbidden", fiber.StatusForbidden, "FORBIDDEN"},
		{"too many requests", fiber.StatusTooManyRequests, "RATE_LIMITED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
			app.Get("/x", func(c fiber.Ctx) error {
				return fiber.NewError(tt.code, "boom")
			})
			resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
			require.NoError(t, err)
			assert.Equal(t, tt.code, resp.StatusCode)
			env := readEnvelope(t, resp.Body)
			require.NotNil(t, env.Error)
			assert.Equal(t, tt.wantCode, env.Error.Code)
		})
	}
}

func TestErrorHandlerHidesInternalDetails(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/boom", func(c fiber.Ctx) error { return errors.New("secret dsn leaked") })
	resp, err := app.Test(httptest.NewRequest("GET", "/boom", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "INTERNAL", env.Error.Code)
	assert.Equal(t, "internal error", env.Error.Message)
}

func TestRequireAuthAcceptsValidToken(t *testing.T) {
	tm := tokenManager()
	mw := &transporthttp.Middleware{Tokens: tm, Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	var got httpx.AuthIdentity
	app.Get("/me", mw.RequireAuth(), func(c fiber.Ctx) error {
		got = httpx.MustIdentity(c)
		return httpx.OK(c, nil)
	})

	token, jti, err := tm.IssueAccess(42, "client", 7)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int64(42), got.UserID)
	assert.Equal(t, "client", got.Role)
	assert.Equal(t, int64(7), got.ClientID)
	assert.Equal(t, jti, got.JTI)
}

func TestRequireAuthRejects(t *testing.T) {
	mw := &transporthttp.Middleware{Tokens: tokenManager(), Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/me", mw.RequireAuth(), func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"not bearer", "Basic dXNlcjpwYXNz"},
		{"empty bearer", "Bearer "},
		{"garbage token", "Bearer not.a.jwt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, 401, resp.StatusCode)
		})
	}
}

func roleApp(t *testing.T, handler fiber.Handler) (*fiber.App, *authtoken.Manager) {
	t.Helper()
	tm := tokenManager()
	mw := &transporthttp.Middleware{
		Tokens: tm,
		Log:    discard,
		Perms: transporthttp.PermissionSourceFunc(
			func(ctx fiber.Ctx, userID int64, module string) (bool, error) {
				return userID == 100, nil // staff user 100 has every permission
			}),
	}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	admin := app.Group("/admin", mw.RequireAuth(), mw.RequireRole("admin", "staff"))
	// Fiber v3 executes route handlers in argument order: middleware first,
	// final handler last.
	admin.Get("/settings", mw.RequirePermission("settings"), handler)
	client := app.Group("/client", mw.RequireAuth(), mw.RequireClient())
	client.Get("/services", handler)
	return app, tm
}

func TestRequireRoleAndPermission(t *testing.T) {
	ok := func(c fiber.Ctx) error { return httpx.OK(c, "ok") }
	app, tm := roleApp(t, ok)

	do := func(path string, userID int64, role string, cid int64) int {
		token, _, err := tm.IssueAccess(userID, role, cid)
		require.NoError(t, err)
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}

	assert.Equal(t, 200, do("/admin/settings", 1, "admin", 0), "admin bypasses permission")
	assert.Equal(t, 200, do("/admin/settings", 100, "staff", 0), "staff with permission")
	assert.Equal(t, 403, do("/admin/settings", 101, "staff", 0), "staff without permission")
	assert.Equal(t, 403, do("/admin/settings", 2, "client", 5), "client blocked by role")

	assert.Equal(t, 200, do("/client/services", 2, "client", 5), "client with profile")
	assert.Equal(t, 403, do("/client/services", 1, "admin", 0), "staff/admin has no client profile")
}

func TestRequireRoleRejectsMissingIdentity(t *testing.T) {
	mw := &transporthttp.Middleware{Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	// RequireRole mounted without a preceding RequireAuth: no identity in Locals.
	app.Get("/x", mw.RequireRole("admin"), func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestRequirePermissionRejectsMissingIdentity(t *testing.T) {
	mw := &transporthttp.Middleware{Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/x", mw.RequirePermission("billing"), func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestRequirePermissionRejectsNonStaffRole(t *testing.T) {
	tm := tokenManager()
	mw := &transporthttp.Middleware{Tokens: tm, Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	// RequirePermission mounted directly (no RequireRole gate), so a "client"
	// role reaches the "not admin, not staff" branch.
	app.Get("/x", mw.RequireAuth(), mw.RequirePermission("billing"),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	token, _, err := tm.IssueAccess(9, "client", 3)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestRequireClientRejectsMissingIdentity(t *testing.T) {
	mw := &transporthttp.Middleware{Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/x", mw.RequireClient(), func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestRequirePermissionSourceError(t *testing.T) {
	tm := tokenManager()
	mw := &transporthttp.Middleware{
		Tokens: tm, Log: discard,
		Perms: transporthttp.PermissionSourceFunc(
			func(ctx fiber.Ctx, userID int64, module string) (bool, error) {
				return false, errors.New("db down")
			}),
	}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/x", mw.RequireAuth(), mw.RequirePermission("billing"),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	token, _, _ := tm.IssueAccess(5, "staff", 0)
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestRateLimitMiddleware(t *testing.T) {
	calls := map[string]int{}
	limiter := &mocks.MockRateLimiter{
		AllowFn: func(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
			calls[key]++
			return calls[key] <= limit, nil
		},
	}
	mw := &transporthttp.Middleware{Limiter: limiter, Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/pub", mw.RateLimit("public", 2, time.Minute),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	for i, wantStatus := range []int{200, 200, 429} {
		resp, err := app.Test(httptest.NewRequest("GET", "/pub", nil))
		require.NoError(t, err)
		assert.Equal(t, wantStatus, resp.StatusCode, "request %d", i+1)
	}
}

// TestRateLimitTrustedProxyDifferentiatesClients mirrors cmd/api/main.go's
// fiber.Config (TrustProxy + ProxyHeader) against fiber's app.Test() harness,
// which always connects through a fixed peer address (0.0.0.0, added to
// Proxies here to stand in for the trusted internal frontend->api hop). It
// guards the fix for the critical bug where c.IP() - and therefore every
// RateLimit key and the login-lockout key built from it - resolved to that
// one constant hop address for every end user, letting a single client
// exhaust the shared public auth rate limit for the whole site. With the
// proxy trusted and the real client IP read from X-Forwarded-For, two
// distinct end users behind the same hop must land in separate buckets.
func TestRateLimitTrustedProxyDifferentiatesClients(t *testing.T) {
	calls := map[string]int{}
	limiter := &mocks.MockRateLimiter{
		AllowFn: func(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
			calls[key]++
			return calls[key] <= limit, nil
		},
	}
	mw := &transporthttp.Middleware{Limiter: limiter, Log: discard}
	app := fiber.New(fiber.Config{
		ErrorHandler: transporthttp.ErrorHandler(discard),
		TrustProxy:   true,
		ProxyHeader:  fiber.HeaderXForwardedFor,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies: []string{"0.0.0.0"}, // app.Test()'s fixed simulated peer
		},
	})
	app.Get("/pub", mw.RateLimit("auth", 1, time.Minute),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	forwardedReq := func(clientIP string) *http.Request {
		req := httptest.NewRequest("GET", "/pub", nil)
		req.Header.Set(fiber.HeaderXForwardedFor, clientIP)
		return req
	}

	resp, err := app.Test(forwardedReq("203.0.113.1"))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "first user's first request")

	resp, err = app.Test(forwardedReq("203.0.113.1"))
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode, "first user's second request hits its own limit")

	resp, err = app.Test(forwardedReq("203.0.113.9"))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode,
		"a second end user behind the same internal hop must not be locked out "+
			"by the first user's bucket (the bug: c.IP() collapsed to one constant IP)")
}

// TestRateLimitIgnoresForwardedHeaderWithoutTrustProxy documents the
// complementary safety property: an app that has NOT configured TrustProxy
// (e.g. the other tests in this file, and any future wiring mistake) must
// not let a client spoof its rate-limit bucket via X-Forwarded-For.
func TestRateLimitIgnoresForwardedHeaderWithoutTrustProxy(t *testing.T) {
	calls := map[string]int{}
	limiter := &mocks.MockRateLimiter{
		AllowFn: func(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
			calls[key]++
			return calls[key] <= limit, nil
		},
	}
	mw := &transporthttp.Middleware{Limiter: limiter, Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/pub", mw.RateLimit("auth", 1, time.Minute),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	req1 := httptest.NewRequest("GET", "/pub", nil)
	req1.Header.Set(fiber.HeaderXForwardedFor, "203.0.113.1")
	resp, err := app.Test(req1)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	req2 := httptest.NewRequest("GET", "/pub", nil)
	req2.Header.Set(fiber.HeaderXForwardedFor, "203.0.113.9") // different, should NOT matter
	resp, err = app.Test(req2)
	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode,
		"without TrustProxy configured, a spoofed X-Forwarded-For must not bypass the limit")
}

func TestRateLimitFailsOpen(t *testing.T) {
	limiter := &mocks.MockRateLimiter{
		AllowFn: func(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
			return false, errors.New("redis down")
		},
	}
	mw := &transporthttp.Middleware{Limiter: limiter, Log: discard}
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Get("/pub", mw.RateLimit("public", 1, time.Minute),
		func(c fiber.Ctx) error { return httpx.OK(c, nil) })

	resp, err := app.Test(httptest.NewRequest("GET", "/pub", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "limiter outage must not block requests")
}
