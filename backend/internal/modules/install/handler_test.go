package install_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/install"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeSvc implements install.ServiceAPI with function fields.
type fakeSvc struct {
	StatusFn       func(ctx context.Context) (*install.StatusResponse, error)
	CreateAdminFn  func(ctx context.Context, in install.CreateAdminRequest) (*domain.User, error)
	SaveSettingsFn func(ctx context.Context, actorUserID int64, in install.SiteSettingsRequest) error
}

func (f *fakeSvc) Status(ctx context.Context) (*install.StatusResponse, error) {
	if f.StatusFn != nil {
		return f.StatusFn(ctx)
	}
	return &install.StatusResponse{ConfigReady: true}, nil
}

func (f *fakeSvc) CreateAdmin(ctx context.Context, in install.CreateAdminRequest) (*domain.User, error) {
	if f.CreateAdminFn != nil {
		return f.CreateAdminFn(ctx, in)
	}
	return &domain.User{ID: 1, Email: in.Email}, nil
}

func (f *fakeSvc) SaveSettings(ctx context.Context, actorUserID int64, in install.SiteSettingsRequest) error {
	if f.SaveSettingsFn != nil {
		return f.SaveSettingsFn(ctx, actorUserID, in)
	}
	return nil
}

var _ install.ServiceAPI = (*fakeSvc)(nil)

func newApp(svc install.ServiceAPI) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discardLog)})
	install.NewHandler(svc).RegisterRoutes(app.Group("/api/v1"))
	return app
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doJSON(t *testing.T, app *fiber.App, method, path, body string) (int, envelope) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var env envelope
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	return resp.StatusCode, env
}

func TestHandlerStatus(t *testing.T) {
	svc := &fakeSvc{StatusFn: func(ctx context.Context) (*install.StatusResponse, error) {
		return &install.StatusResponse{ConfigReady: true, Installed: true}, nil
	}}
	status, env := doJSON(t, newApp(svc), "GET", "/api/v1/install/status", "")
	require.Equal(t, fiber.StatusOK, status)
	var got install.StatusResponse
	require.NoError(t, json.Unmarshal(env.Data, &got))
	assert.True(t, got.ConfigReady)
	assert.True(t, got.Installed)
}

func TestHandlerStatusPropagatesError(t *testing.T) {
	svc := &fakeSvc{StatusFn: func(ctx context.Context) (*install.StatusResponse, error) {
		return nil, apperr.Internal(assert.AnError)
	}}
	status, env := doJSON(t, newApp(svc), "GET", "/api/v1/install/status", "")
	assert.Equal(t, fiber.StatusInternalServerError, status)
	require.NotNil(t, env.Error)
}

func TestHandlerCreateAdmin(t *testing.T) {
	var got install.CreateAdminRequest
	svc := &fakeSvc{CreateAdminFn: func(ctx context.Context, in install.CreateAdminRequest) (*domain.User, error) {
		got = in
		return &domain.User{ID: 99, Email: in.Email}, nil
	}}
	status, env := doJSON(t, newApp(svc), "POST", "/api/v1/install/admin",
		`{"email":"admin@example.test","password":"correcthorsebattery"}`)
	require.Equal(t, fiber.StatusCreated, status)
	assert.Equal(t, "admin@example.test", got.Email)
	assert.Contains(t, string(env.Data), `"id":99`)
}

func TestHandlerCreateAdminRejectsBadBody(t *testing.T) {
	status, env := doJSON(t, newApp(&fakeSvc{}), "POST", "/api/v1/install/admin", `not-json`)
	assert.Equal(t, fiber.StatusUnprocessableEntity, status)
	require.NotNil(t, env.Error)
}

func TestHandlerCreateAdminConflictOnceInstalled(t *testing.T) {
	svc := &fakeSvc{CreateAdminFn: func(ctx context.Context, in install.CreateAdminRequest) (*domain.User, error) {
		return nil, apperr.Conflict("already installed")
	}}
	status, env := doJSON(t, newApp(svc), "POST", "/api/v1/install/admin",
		`{"email":"admin@example.test","password":"correcthorsebattery"}`)
	assert.Equal(t, fiber.StatusConflict, status)
	require.NotNil(t, env.Error)
	assert.Equal(t, "CONFLICT", env.Error.Code)
}

func TestHandlerSaveSettings(t *testing.T) {
	var gotActor int64 = -1
	svc := &fakeSvc{SaveSettingsFn: func(ctx context.Context, actorUserID int64, in install.SiteSettingsRequest) error {
		gotActor = actorUserID
		return nil
	}}
	status, env := doJSON(t, newApp(svc), "POST", "/api/v1/install/settings", `{"company_name":"Acme"}`)
	require.Equal(t, fiber.StatusOK, status)
	assert.Nil(t, env.Error)
	assert.Equal(t, int64(0), gotActor)
}

func TestHandlerSaveSettingsPropagatesError(t *testing.T) {
	svc := &fakeSvc{SaveSettingsFn: func(ctx context.Context, actorUserID int64, in install.SiteSettingsRequest) error {
		return apperr.Internal(assert.AnError)
	}}
	status, env := doJSON(t, newApp(svc), "POST", "/api/v1/install/settings", `{}`)
	assert.Equal(t, fiber.StatusInternalServerError, status)
	require.NotNil(t, env.Error)
}
