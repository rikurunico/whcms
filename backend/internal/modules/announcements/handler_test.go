package announcements_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/announcements"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake middleware bundle

// fakeMW stubs announcements.Middlewares: injects a fixed identity and mirrors
// the production role/permission semantics. rateLimited counts RateLimit hits.
type fakeMW struct {
	identity    httpx.AuthIdentity
	authFail    bool
	permissions map[string]bool
	rateLimited int
}

func (m *fakeMW) RequireAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		if m.authFail {
			return apperr.Unauthorized("missing bearer token")
		}
		httpx.SetIdentity(c, m.identity)
		return c.Next()
	}
}

func (m *fakeMW) RequireRole(roles ...string) fiber.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c fiber.Ctx) error {
		id, _ := httpx.Identity(c)
		if !allowed[id.Role] {
			return apperr.Forbidden("insufficient role")
		}
		return c.Next()
	}
}

func (m *fakeMW) RequirePermission(module string) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, _ := httpx.Identity(c)
		if id.Role == "admin" {
			return c.Next()
		}
		if !m.permissions[module] {
			return apperr.Forbidden("missing permission: " + module)
		}
		return c.Next()
	}
}

func (m *fakeMW) RateLimit(prefix string, limit int, window time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		m.rateLimited++
		return c.Next()
	}
}

var _ announcements.Middlewares = (*fakeMW)(nil)

// Harness

type harness struct {
	app *fiber.App
	fx  *fixtures
	mw  *fakeMW
}

// newApp builds a Fiber app with the real *Service over the shared fakes
// (see service_test.go) and the fake middleware bundle.
func newApp(mut func(*fakeMW)) *harness {
	fx := newFixture()
	mw := &fakeMW{identity: httpx.AuthIdentity{UserID: 1, Role: "admin"}}
	if mut != nil {
		mut(mw)
	}
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error { return httpx.Fail(c, err) },
	})
	h := announcements.NewHandler(fx.svc, mw)
	h.RegisterRoutes(app.Group("/api/v1"))
	return &harness{app: app, fx: fx, mw: mw}
}

func (h *harness) do(t *testing.T, method, path string, body any) (*http.Response, httpx.Envelope) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.app.Test(req)
	require.NoError(t, err)
	var env httpx.Envelope
	if resp.StatusCode != fiber.StatusNoContent {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	}
	_ = resp.Body.Close()
	return resp, env
}

// Public routes

func TestHandlerPublicList(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.ListPublishedFn = func(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
		return []domain.Announcement{*announcement(nil)}, 1, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/announcements", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
	require.NotNil(t, env.Meta, "public list is paginated")
	assert.Equal(t, int64(1), env.Meta.Total)
	assert.Equal(t, 1, h.mw.rateLimited, "public route is rate limited")
}

func TestHandlerPublicGet(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetBySlugFn = func(ctx context.Context, slug string) (*domain.Announcement, error) {
		return announcement(func(a *domain.Announcement) { a.Slug = slug }), nil
	}
	resp, env := h.do(t, "GET", "/api/v1/announcements/maintenance", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
	raw, _ := json.Marshal(env.Data)
	var got announcements.AnnouncementResponse
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "maintenance", got.Slug)
}

func TestHandlerPublicGetNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetBySlugFn = func(ctx context.Context, slug string) (*domain.Announcement, error) {
		return nil, apperr.NotFound("announcement")
	}
	resp, env := h.do(t, "GET", "/api/v1/announcements/nope", nil)
	assert.Equal(t, 404, resp.StatusCode)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// Admin RBAC

func TestHandlerAdminRequiresAuth(t *testing.T) {
	h := newApp(func(m *fakeMW) { m.authFail = true })
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements", nil)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
}

func TestHandlerAdminRejectsClientRole(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "client", ClientID: 2}
	})
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements", nil)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Equal(t, "FORBIDDEN", env.Error.Code)
}

func TestHandlerAdminStaffNeedsPermission(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "staff"}
		m.permissions = map[string]bool{"billing": true}
	})
	resp, _ := h.do(t, "GET", "/api/v1/admin/announcements", nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestHandlerAdminStaffWithPermission(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "staff"}
		m.permissions = map[string]bool{"announcements": true}
	})
	resp, _ := h.do(t, "GET", "/api/v1/admin/announcements", nil)
	assert.Equal(t, 200, resp.StatusCode)
}

// Admin CRUD flows

func TestHandlerAdminList(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
		assert.Equal(t, "outage", p.Search)
		assert.Equal(t, "draft", p.Status)
		return []domain.Announcement{{ID: 1}}, 1, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements?search=outage&status=draft&page=2&per_page=10", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.NotNil(t, env.Meta)
	assert.Equal(t, 2, env.Meta.Page)
	assert.Equal(t, int64(1), env.Meta.Total)
}

func TestHandlerAdminCreate(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.CreateFn = func(ctx context.Context, a *domain.Announcement) error {
		a.ID = 3
		return nil
	}
	resp, env := h.do(t, "POST", "/api/v1/admin/announcements", map[string]any{
		"title": "New Feature", "body": "details", "published": true,
	})
	assert.Equal(t, 201, resp.StatusCode)
	require.Nil(t, env.Error)
}

func TestHandlerAdminCreateInvalidBody(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "POST", "/api/v1/admin/announcements", map[string]any{"title": "x"})
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

func TestHandlerAdminGet(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Hello"}, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements/7", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
}

func TestHandlerAdminGetNotFound(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements/7", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerAdminUpdate(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id, Title: "Old", Slug: "old"}, nil
	}
	resp, env := h.do(t, "PATCH", "/api/v1/admin/announcements/7", map[string]any{"title": "New Title"})
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
}

func TestHandlerAdminDelete(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetByIDFn = func(ctx context.Context, id int64) (*domain.Announcement, error) {
		return &domain.Announcement{ID: id}, nil
	}
	resp, _ := h.do(t, "DELETE", "/api/v1/admin/announcements/7", nil)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestHandlerAdminDeleteNotFound(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "DELETE", "/api/v1/admin/announcements/7", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerInvalidID(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "GET", "/api/v1/admin/announcements/abc", nil)
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

// Malformed JSON body: every Bind().Body endpoint must reject non-JSON with
// 422 VALIDATION, independent of whatever the service layer would say.

func TestHandlerAdminInvalidJSONBodySweep(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"create", "POST", "/api/v1/admin/announcements"},
		{"update", "PATCH", "/api/v1/admin/announcements/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newApp(nil)
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(`{"broken`)))
			req.Header.Set("Content-Type", "application/json")
			resp, err := h.app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, 422, resp.StatusCode)
			_ = resp.Body.Close()
		})
	}
}

// Invalid :id params: every parseID-guarded endpoint must reject a non-numeric
// id with 422 VALIDATION before ever reaching the service.

func TestHandlerAdminInvalidIDSweep(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"get", "GET", "/api/v1/admin/announcements/abc"},
		{"update", "PATCH", "/api/v1/admin/announcements/abc"},
		{"delete", "DELETE", "/api/v1/admin/announcements/abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newApp(nil)
			resp, env := h.do(t, tt.method, tt.path, nil)
			assert.Equal(t, 422, resp.StatusCode)
			require.NotNil(t, env.Error)
			assert.Equal(t, "VALIDATION", env.Error.Code)
		})
	}
}

// Service-layer error propagation: once the body binds and the id parses,
// a failing repo must surface through the handler as an error envelope,
// never as a 2xx.

func TestHandlerServiceErrorSweep(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
		prep   func(fx *fixtures)
	}{
		{"public list", "GET", "/api/v1/announcements", nil, func(fx *fixtures) {
			fx.repo.ListPublishedFn = func(context.Context, ports.ListParams) ([]domain.Announcement, int64, error) {
				return nil, 0, errBoom
			}
		}},
		{"public get", "GET", "/api/v1/announcements/x", nil, func(fx *fixtures) {
			fx.repo.GetBySlugFn = func(context.Context, string) (*domain.Announcement, error) { return nil, errBoom }
		}},
		{"admin list", "GET", "/api/v1/admin/announcements", nil, func(fx *fixtures) {
			fx.repo.ListFn = func(context.Context, ports.ListParams) ([]domain.Announcement, int64, error) {
				return nil, 0, errBoom
			}
		}},
		{"admin create", "POST", "/api/v1/admin/announcements",
			map[string]any{"title": "A valid title"}, func(fx *fixtures) {
				fx.repo.CreateFn = func(context.Context, *domain.Announcement) error { return errBoom }
			}},
		{"admin get", "GET", "/api/v1/admin/announcements/7", nil, func(fx *fixtures) {
			fx.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
		}},
		{"admin update", "PATCH", "/api/v1/admin/announcements/7",
			map[string]any{"title": "A valid title"}, func(fx *fixtures) {
				fx.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
			}},
		{"admin delete", "DELETE", "/api/v1/admin/announcements/7", nil, func(fx *fixtures) {
			fx.repo.GetByIDFn = func(context.Context, int64) (*domain.Announcement, error) { return nil, errBoom }
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newApp(nil)
			tt.prep(h.fx)
			resp, env := h.do(t, tt.method, tt.path, tt.body)
			assert.NotEqual(t, fiber.StatusOK, resp.StatusCode)
			assert.NotEqual(t, fiber.StatusCreated, resp.StatusCode)
			assert.NotEqual(t, fiber.StatusNoContent, resp.StatusCode)
			require.NotNil(t, env.Error)
		})
	}
}
