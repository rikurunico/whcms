package http_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingsService mocks the handler's service dependency.
type mockSettingsService struct {
	GroupedFn func(ctx context.Context) (map[string]map[string]any, error)
	UpdateFn  func(ctx context.Context, actorUserID int64, updates map[string]any) error
}

func (m *mockSettingsService) Grouped(ctx context.Context) (map[string]map[string]any, error) {
	if m.GroupedFn != nil {
		return m.GroupedFn(ctx)
	}
	return map[string]map[string]any{"company": {"name": "WHCMS"}}, nil
}

func (m *mockSettingsService) Update(ctx context.Context, actorUserID int64, updates map[string]any) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, actorUserID, updates)
	}
	return nil
}

// settingsApp mounts the handler behind a stub identity middleware, mirroring
// the production stack (RequireAuth -> RequireRole -> handler).
func settingsApp(svc transporthttp.SettingsService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	app.Use(func(c fiber.Ctx) error {
		httpx.SetIdentity(c, httpx.AuthIdentity{UserID: 42, Role: "admin"})
		return c.Next()
	})
	transporthttp.NewSettingsHandler(svc).Register(app.Group("/api/v1/admin/settings"))
	return app
}

func TestSettingsGet(t *testing.T) {
	svc := &mockSettingsService{
		GroupedFn: func(ctx context.Context) (map[string]map[string]any, error) {
			return map[string]map[string]any{
				"company": {"name": "WHCMS"},
				"billing": {"tax_rate": float64(11)},
			}, nil
		},
	}
	app := settingsApp(svc)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/settings/", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	env := readEnvelope(t, resp.Body)
	require.Nil(t, env.Error)
	data := env.Data.(map[string]any)
	company := data["company"].(map[string]any)
	assert.Equal(t, "WHCMS", company["name"])
}

func TestSettingsGetServiceError(t *testing.T) {
	svc := &mockSettingsService{
		GroupedFn: func(ctx context.Context) (map[string]map[string]any, error) {
			return nil, apperr.Internal(errors.New("db down"))
		},
	}
	app := settingsApp(svc)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/settings/", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestSettingsPut(t *testing.T) {
	var gotActor int64
	var gotUpdates map[string]any
	svc := &mockSettingsService{
		UpdateFn: func(ctx context.Context, actorUserID int64, updates map[string]any) error {
			gotActor = actorUserID
			gotUpdates = updates
			return nil
		},
	}
	app := settingsApp(svc)

	body := `{"company.name":"Hosting Kita","billing.tax_rate":12}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/settings/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	assert.Equal(t, int64(42), gotActor, "actor from identity")
	assert.Equal(t, "Hosting Kita", gotUpdates["company.name"])
	assert.Equal(t, float64(12), gotUpdates["billing.tax_rate"])

	env := readEnvelope(t, resp.Body)
	assert.Nil(t, env.Error)
	assert.NotNil(t, env.Data, "returns refreshed settings")
}

func TestSettingsPutRefreshErrorAfterUpdate(t *testing.T) {
	svc := &mockSettingsService{
		UpdateFn: func(ctx context.Context, actorUserID int64, updates map[string]any) error {
			return nil
		},
		GroupedFn: func(ctx context.Context) (map[string]map[string]any, error) {
			return nil, apperr.Internal(errors.New("db down"))
		},
	}
	app := settingsApp(svc)
	req := httptest.NewRequest("PUT", "/api/v1/admin/settings/", strings.NewReader(`{"company.name":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestSettingsPutInvalidBody(t *testing.T) {
	app := settingsApp(&mockSettingsService{})
	req := httptest.NewRequest("PUT", "/api/v1/admin/settings/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

func TestSettingsPutValidationErrorPassthrough(t *testing.T) {
	svc := &mockSettingsService{
		UpdateFn: func(ctx context.Context, actorUserID int64, updates map[string]any) error {
			return apperr.Validation("invalid settings",
				apperr.FieldError{Field: "bogus.key", Message: "unknown settings key"})
		},
	}
	app := settingsApp(svc)
	req := httptest.NewRequest("PUT", "/api/v1/admin/settings/", strings.NewReader(`{"bogus.key":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)

	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Error)
	require.Len(t, env.Error.Details, 1)
	assert.Equal(t, "bogus.key", env.Error.Details[0].Field)
}
