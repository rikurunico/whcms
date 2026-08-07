package knowledgebase_test

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
	"github.com/tsdlamongan/whcms/backend/internal/modules/knowledgebase"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake middleware bundle

// fakeMW stubs knowledgebase.Middlewares: injects a fixed identity and mirrors
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

var _ knowledgebase.Middlewares = (*fakeMW)(nil)

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
	h := knowledgebase.NewHandler(fx.svc, mw)
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

func TestHandlerPublicCategories(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
		assert.False(t, includeHidden)
		return []domain.KBCategory{{ID: 1, Name: "General"}}, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/categories", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
	assert.Equal(t, 1, h.mw.rateLimited, "public route is rate limited")
}

func TestHandlerPublicCategoryBySlug(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: 1, Slug: slug, Name: "General"}, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/categories/general", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
}

func TestHandlerPublicCategoryBySlugNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/categories/nope", nil)
	assert.Equal(t, 404, resp.StatusCode)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerPublicArticles(t *testing.T) {
	h := newApp(nil)
	var gotCat int64
	h.fx.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		gotCat = categoryID
		assert.Equal(t, "reset", p.Search)
		return []domain.KBArticle{{ID: 1, Published: true}}, 1, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/articles?category_id=5&search=reset&page=2&per_page=10", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.NotNil(t, env.Meta)
	assert.Equal(t, int64(5), gotCat)
	assert.Equal(t, 2, env.Meta.Page)
	assert.Equal(t, int64(1), env.Meta.Total)
}

func TestHandlerPublicArticleBySlug(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 1, Slug: slug, Published: true}, nil
	}
	bumped := false
	h.fx.repo.IncrementViewsFn = func(ctx context.Context, id int64) error {
		bumped = true
		return nil
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/articles/hello", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
	assert.True(t, bumped)
}

func TestHandlerPublicArticleBySlugNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 1, Slug: slug, Published: false}, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/kb/articles/draft", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// Admin RBAC

func TestHandlerAdminRequiresAuth(t *testing.T) {
	h := newApp(func(m *fakeMW) { m.authFail = true })
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/articles", nil)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Equal(t, "UNAUTHORIZED", env.Error.Code)
}

func TestHandlerAdminRejectsClientRole(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "client", ClientID: 2}
	})
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/articles", nil)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Equal(t, "FORBIDDEN", env.Error.Code)
}

func TestHandlerAdminStaffNeedsKnowledgebasePermission(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "staff"}
		m.permissions = map[string]bool{"billing": true}
	})
	resp, _ := h.do(t, "GET", "/api/v1/admin/kb/articles", nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestHandlerAdminStaffWithPermission(t *testing.T) {
	h := newApp(func(m *fakeMW) {
		m.identity = httpx.AuthIdentity{UserID: 5, Role: "staff"}
		m.permissions = map[string]bool{"knowledgebase": true}
	})
	resp, _ := h.do(t, "GET", "/api/v1/admin/kb/articles", nil)
	assert.Equal(t, 200, resp.StatusCode)
}

// Admin CRUD flows

func TestHandlerCategoryLifecycle(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.CreateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error {
		c.ID = 3
		return nil
	}
	resp, env := h.do(t, "POST", "/api/v1/admin/kb/categories", map[string]any{"name": "General"})
	assert.Equal(t, 201, resp.StatusCode)
	require.Nil(t, env.Error)

	h.fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id, Name: "General", Slug: "general"}, nil
	}
	resp, _ = h.do(t, "GET", "/api/v1/admin/kb/categories/3", nil)
	assert.Equal(t, 200, resp.StatusCode)

	resp, _ = h.do(t, "PATCH", "/api/v1/admin/kb/categories/3", map[string]any{"hidden": true})
	assert.Equal(t, 200, resp.StatusCode)

	resp, _ = h.do(t, "DELETE", "/api/v1/admin/kb/categories/3", nil)
	assert.Equal(t, 204, resp.StatusCode)

	resp, env = h.do(t, "GET", "/api/v1/admin/kb/categories", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)
}

func TestHandlerAdminListCategoriesIncludeHidden(t *testing.T) {
	h := newApp(nil)
	var gotInclude bool
	h.fx.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
		gotInclude = includeHidden
		return nil, nil
	}
	resp, _ := h.do(t, "GET", "/api/v1/admin/kb/categories", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, gotInclude, "admin listing includes hidden by default")

	resp, _ = h.do(t, "GET", "/api/v1/admin/kb/categories?include_hidden=false", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.False(t, gotInclude)
}

func TestHandlerCreateCategoryInvalidBody(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "POST", "/api/v1/admin/kb/categories", map[string]any{"name": "x"})
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

func TestHandlerDeleteCategoryConflict(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	h.fx.repo.CountArticlesInCategoryFn = func(ctx context.Context, categoryID int64) (int64, error) {
		return 2, nil
	}
	resp, env := h.do(t, "DELETE", "/api/v1/admin/kb/categories/4", nil)
	assert.Equal(t, 409, resp.StatusCode)
	assert.Equal(t, "CONFLICT", env.Error.Code)
}

func TestHandlerArticleLifecycle(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	h.fx.repo.CreateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		a.ID = 11
		return nil
	}
	resp, env := h.do(t, "POST", "/api/v1/admin/kb/articles", map[string]any{
		"category_id": 1, "title": "How to reset password",
	})
	assert.Equal(t, 201, resp.StatusCode)
	require.Nil(t, env.Error)

	h.fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id, CategoryID: 1, Title: "How to reset password", Slug: "how-to-reset-password"}, nil
	}
	resp, _ = h.do(t, "GET", "/api/v1/admin/kb/articles/11", nil)
	assert.Equal(t, 200, resp.StatusCode)

	resp, _ = h.do(t, "PATCH", "/api/v1/admin/kb/articles/11", map[string]any{"published": true})
	assert.Equal(t, 200, resp.StatusCode)

	resp, _ = h.do(t, "DELETE", "/api/v1/admin/kb/articles/11", nil)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestHandlerAdminListArticlesMeta(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.ListArticlesAdminFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		assert.Equal(t, int64(3), categoryID)
		assert.Equal(t, "faq", p.Search)
		assert.Equal(t, "draft", p.Status)
		return []domain.KBArticle{{ID: 1}}, 1, nil
	}
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/articles?category_id=3&search=faq&status=draft&page=2&per_page=10", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.NotNil(t, env.Meta)
	assert.Equal(t, 2, env.Meta.Page)
	assert.Equal(t, int64(1), env.Meta.Total)
}

func TestHandlerInvalidID(t *testing.T) {
	h := newApp(nil)
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/articles/abc", nil)
	assert.Equal(t, 422, resp.StatusCode)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

// Admin lookups and mutations of missing categories/articles yield 404
// NOT_FOUND.

func TestHandlerAdminGetCategoryNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/categories/4", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerAdminUpdateCategoryNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	resp, env := h.do(t, "PATCH", "/api/v1/admin/kb/categories/4", map[string]any{"name": "New Name"})
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerAdminGetArticleNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return nil, apperr.NotFound("article")
	}
	resp, env := h.do(t, "GET", "/api/v1/admin/kb/articles/10", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestHandlerAdminDeleteArticleNotFound(t *testing.T) {
	h := newApp(nil)
	h.fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return nil, apperr.NotFound("article")
	}
	resp, env := h.do(t, "DELETE", "/api/v1/admin/kb/articles/10", nil)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

// Malformed JSON body: every Bind().Body endpoint must reject non-JSON with
// 422 VALIDATION, independent of whatever the service layer would say.

func TestHandlerAdminInvalidJSONBodySweep(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"create category", "POST", "/api/v1/admin/kb/categories"},
		{"update category", "PATCH", "/api/v1/admin/kb/categories/1"},
		{"create article", "POST", "/api/v1/admin/kb/articles"},
		{"update article", "PATCH", "/api/v1/admin/kb/articles/1"},
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
		{"get category", "GET", "/api/v1/admin/kb/categories/abc"},
		{"update category", "PATCH", "/api/v1/admin/kb/categories/abc"},
		{"delete category", "DELETE", "/api/v1/admin/kb/categories/abc"},
		{"get article", "GET", "/api/v1/admin/kb/articles/abc"},
		{"update article", "PATCH", "/api/v1/admin/kb/articles/abc"},
		{"delete article", "DELETE", "/api/v1/admin/kb/articles/abc"},
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
		{"public categories", "GET", "/api/v1/kb/categories", nil, func(fx *fixtures) {
			fx.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
				return nil, errBoom
			}
		}},
		{"public category by slug", "GET", "/api/v1/kb/categories/general", nil, func(fx *fixtures) {
			fx.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
				return nil, errBoom
			}
		}},
		{"public articles", "GET", "/api/v1/kb/articles", nil, func(fx *fixtures) {
			fx.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
				return nil, 0, errBoom
			}
		}},
		{"public article by slug", "GET", "/api/v1/kb/articles/hello", nil, func(fx *fixtures) {
			fx.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
				return nil, errBoom
			}
		}},
		{"admin list categories", "GET", "/api/v1/admin/kb/categories", nil, func(fx *fixtures) {
			fx.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
				return nil, errBoom
			}
		}},
		{"admin create category", "POST", "/api/v1/admin/kb/categories",
			map[string]any{"name": "General"}, func(fx *fixtures) {
				fx.repo.CreateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error { return errBoom }
			}},
		{"admin get category", "GET", "/api/v1/admin/kb/categories/4", nil, func(fx *fixtures) {
			fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) { return nil, errBoom }
		}},
		{"admin update category", "PATCH", "/api/v1/admin/kb/categories/4",
			map[string]any{"name": "New Name"}, func(fx *fixtures) {
				fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) { return nil, errBoom }
			}},
		{"admin delete category", "DELETE", "/api/v1/admin/kb/categories/4", nil, func(fx *fixtures) {
			fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) { return nil, errBoom }
		}},
		{"admin list articles", "GET", "/api/v1/admin/kb/articles", nil, func(fx *fixtures) {
			fx.repo.ListArticlesAdminFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
				return nil, 0, errBoom
			}
		}},
		{"admin create article", "POST", "/api/v1/admin/kb/articles",
			map[string]any{"category_id": 1, "title": "How to reset password"}, func(fx *fixtures) {
				fx.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
					return &domain.KBCategory{ID: id}, nil
				}
				fx.repo.CreateArticleFn = func(ctx context.Context, a *domain.KBArticle) error { return errBoom }
			}},
		{"admin get article", "GET", "/api/v1/admin/kb/articles/10", nil, func(fx *fixtures) {
			fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) { return nil, errBoom }
		}},
		{"admin update article", "PATCH", "/api/v1/admin/kb/articles/10",
			map[string]any{"published": true}, func(fx *fixtures) {
				fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) { return nil, errBoom }
			}},
		{"admin delete article", "DELETE", "/api/v1/admin/kb/articles/10", nil, func(fx *fixtures) {
			fx.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) { return nil, errBoom }
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
