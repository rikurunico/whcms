package notifications_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/notifications"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeMW satisfies notifications.Middlewares: it injects a fixed identity and
// records which role/permission guards were mounted.
type fakeMW struct {
	identity httpx.AuthIdentity
	perms    []string
}

func (s *fakeMW) RequireAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		httpx.SetIdentity(c, s.identity)
		return c.Next()
	}
}

func (s *fakeMW) RequireRole(roles ...string) fiber.Handler {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c fiber.Ctx) error {
		id := httpx.MustIdentity(c)
		if !allowed[id.Role] {
			return apperr.Forbidden("insufficient role")
		}
		return c.Next()
	}
}

func (s *fakeMW) RequirePermission(module string) fiber.Handler {
	s.perms = append(s.perms, module)
	return func(c fiber.Ctx) error { return c.Next() }
}

func (s *fakeMW) RequireClient() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := httpx.MustIdentity(c)
		if id.ClientID == 0 {
			return apperr.Forbidden("client profile required")
		}
		return c.Next()
	}
}

// mockService mocks the NotificationService surface.
type mockService struct {
	ListTemplatesFn  func(ctx context.Context) ([]domain.EmailTemplate, error)
	GetTemplateFn    func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error)
	SaveTemplateFn   func(ctx context.Context, actorUserID int64, in notifications.SaveTemplateInput) (*domain.EmailTemplate, error)
	DeleteTemplateFn func(ctx context.Context, actorUserID int64, key, locale string) error
	PreviewFn        func(ctx context.Context, in notifications.PreviewInput) (*notifications.PreviewResult, error)
	SendTestEmailFn  func(ctx context.Context, actorUserID int64, in notifications.SendTestEmailInput) (*notifications.TestEmailResult, error)
	EmailLogsFn      func(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error)
	RetryEmailFn     func(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error)
}

func (m *mockService) ListTemplates(ctx context.Context) ([]domain.EmailTemplate, error) {
	if m.ListTemplatesFn != nil {
		return m.ListTemplatesFn(ctx)
	}
	return nil, nil
}

func (m *mockService) GetTemplate(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
	if m.GetTemplateFn != nil {
		return m.GetTemplateFn(ctx, key, locale)
	}
	return &domain.EmailTemplate{Key: key, Locale: locale}, nil
}

func (m *mockService) SaveTemplate(ctx context.Context, actorUserID int64, in notifications.SaveTemplateInput) (*domain.EmailTemplate, error) {
	if m.SaveTemplateFn != nil {
		return m.SaveTemplateFn(ctx, actorUserID, in)
	}
	return &domain.EmailTemplate{Key: in.Key, Locale: in.Locale}, nil
}

func (m *mockService) DeleteTemplate(ctx context.Context, actorUserID int64, key, locale string) error {
	if m.DeleteTemplateFn != nil {
		return m.DeleteTemplateFn(ctx, actorUserID, key, locale)
	}
	return nil
}

func (m *mockService) Preview(ctx context.Context, in notifications.PreviewInput) (*notifications.PreviewResult, error) {
	if m.PreviewFn != nil {
		return m.PreviewFn(ctx, in)
	}
	return &notifications.PreviewResult{Key: in.Key}, nil
}

func (m *mockService) SendTestEmail(ctx context.Context, actorUserID int64, in notifications.SendTestEmailInput) (*notifications.TestEmailResult, error) {
	if m.SendTestEmailFn != nil {
		return m.SendTestEmailFn(ctx, actorUserID, in)
	}
	return &notifications.TestEmailResult{EmailLogID: 7, To: in.To, Subject: "[WHCMS] Test email"}, nil
}

func (m *mockService) EmailLogs(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
	if m.EmailLogsFn != nil {
		return m.EmailLogsFn(ctx, p)
	}
	return nil, 0, nil
}

func (m *mockService) RetryEmail(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error) {
	if m.RetryEmailFn != nil {
		return m.RetryEmailFn(ctx, actorUserID, emailLogID)
	}
	return &domain.EmailLogEntry{ID: emailLogID}, nil
}

func newApp(svc notifications.NotificationService, role string) (*fiber.App, *fakeMW) {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	mw := &fakeMW{identity: httpx.AuthIdentity{UserID: 42, Role: role}}
	notifications.NewHandler(svc, mw).RegisterRoutes(app.Group("/api/v1"))
	return app, mw
}

// clientApp is newApp with a client identity (ClientID set), needed for
// the RequireClient-gated /account/email-log route.
func clientApp(svc notifications.NotificationService) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: transporthttp.ErrorHandler(discard)})
	mw := &fakeMW{identity: httpx.AuthIdentity{UserID: 42, Role: "client", ClientID: 7}}
	notifications.NewHandler(svc, mw).RegisterRoutes(app.Group("/api/v1"))
	return app
}

func readEnvelope(t *testing.T, body io.Reader) httpx.Envelope {
	t.Helper()
	var env httpx.Envelope
	require.NoError(t, json.NewDecoder(body).Decode(&env))
	return env
}

func TestRouteGuards(t *testing.T) {
	_, mw := newApp(&mockService{}, "admin")
	assert.ElementsMatch(t, []string{"settings", "settings", "logs"}, mw.perms,
		"templates and the test-send behind settings perm, email log behind logs perm")
}

func TestClientRoleForbidden(t *testing.T) {
	app, _ := newApp(&mockService{}, "client")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-templates/", nil))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestListTemplates(t *testing.T) {
	svc := &mockService{
		ListTemplatesFn: func(ctx context.Context) ([]domain.EmailTemplate, error) {
			return []domain.EmailTemplate{
				{Key: "invoice_created", Locale: "id", Subject: "Tagihan"},
				{Key: "invoice_created", Locale: "en", Subject: "Invoice"},
			}, nil
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-templates/", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	require.Nil(t, env.Error)
	assert.Len(t, env.Data.([]any), 2)
}

func TestListTemplatesServiceError(t *testing.T) {
	svc := &mockService{
		ListTemplatesFn: func(ctx context.Context) ([]domain.EmailTemplate, error) {
			return nil, apperr.Internal(errors.New("db down"))
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-templates/", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestDeleteTemplateNotFoundPassthrough(t *testing.T) {
	svc := &mockService{
		DeleteTemplateFn: func(ctx context.Context, actorUserID int64, key, locale string) error {
			return apperr.NotFound("email template")
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("DELETE", "/api/v1/admin/email-templates/x/id", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestPreviewServiceError(t *testing.T) {
	svc := &mockService{
		PreviewFn: func(ctx context.Context, in notifications.PreviewInput) (*notifications.PreviewResult, error) {
			return nil, apperr.NotFound("email template nope")
		},
	}
	app, _ := newApp(svc, "admin")
	req := httptest.NewRequest("POST", "/api/v1/admin/email-templates/preview", strings.NewReader(`{"key":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

func TestGetTemplate(t *testing.T) {
	var gotKey, gotLocale string
	svc := &mockService{
		GetTemplateFn: func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
			gotKey, gotLocale = key, locale
			return &domain.EmailTemplate{Key: key, Locale: locale, Subject: "S"}, nil
		},
	}
	app, _ := newApp(svc, "staff")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-templates/invoice_created/en", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "invoice_created", gotKey)
	assert.Equal(t, "en", gotLocale)
}

func TestGetTemplateNotFound(t *testing.T) {
	svc := &mockService{
		GetTemplateFn: func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
			return nil, apperr.NotFound("email template")
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-templates/x/id", nil))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestSaveTemplate(t *testing.T) {
	var gotActor int64
	var gotIn notifications.SaveTemplateInput
	svc := &mockService{
		SaveTemplateFn: func(ctx context.Context, actorUserID int64, in notifications.SaveTemplateInput) (*domain.EmailTemplate, error) {
			gotActor, gotIn = actorUserID, in
			return &domain.EmailTemplate{ID: 9, Key: in.Key, Locale: in.Locale}, nil
		},
	}
	app, _ := newApp(svc, "admin")
	body := `{"key":"welcome","locale":"id","subject":"Halo {{.Name}}","body_html":"<p>Hi</p>","body_text":"Hi"}`
	req := httptest.NewRequest("PUT", "/api/v1/admin/email-templates/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int64(42), gotActor)
	assert.Equal(t, "welcome", gotIn.Key)
	assert.Equal(t, "Halo {{.Name}}", gotIn.Subject)
}

func TestSaveTemplateInvalidBody(t *testing.T) {
	app, _ := newApp(&mockService{}, "admin")
	req := httptest.NewRequest("PUT", "/api/v1/admin/email-templates/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestSaveTemplateValidationPassthrough(t *testing.T) {
	svc := &mockService{
		SaveTemplateFn: func(ctx context.Context, actorUserID int64, in notifications.SaveTemplateInput) (*domain.EmailTemplate, error) {
			return nil, apperr.Validation("invalid template syntax",
				apperr.FieldError{Field: "body_html", Message: "unclosed action"})
		},
	}
	app, _ := newApp(svc, "admin")
	req := httptest.NewRequest("PUT", "/api/v1/admin/email-templates/", strings.NewReader(`{"key":"k"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	require.Len(t, env.Error.Details, 1)
	assert.Equal(t, "body_html", env.Error.Details[0].Field)
}

func TestDeleteTemplate(t *testing.T) {
	var gotKey, gotLocale string
	svc := &mockService{
		DeleteTemplateFn: func(ctx context.Context, actorUserID int64, key, locale string) error {
			gotKey, gotLocale = key, locale
			return nil
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("DELETE", "/api/v1/admin/email-templates/welcome/en", nil))
	require.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)
	assert.Equal(t, "welcome", gotKey)
	assert.Equal(t, "en", gotLocale)
}

func TestPreview(t *testing.T) {
	var gotIn notifications.PreviewInput
	svc := &mockService{
		PreviewFn: func(ctx context.Context, in notifications.PreviewInput) (*notifications.PreviewResult, error) {
			gotIn = in
			return &notifications.PreviewResult{Key: in.Key, Locale: "id", Subject: "Rendered"}, nil
		},
	}
	app, _ := newApp(svc, "admin")
	body := `{"key":"invoice_created","locale":"id","sample_data":{"InvoiceNumber":"INV-1"}}`
	req := httptest.NewRequest("POST", "/api/v1/admin/email-templates/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "invoice_created", gotIn.Key)
	assert.Equal(t, "INV-1", gotIn.SampleData["InvoiceNumber"])
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "Rendered", env.Data.(map[string]any)["subject"])
}

func TestPreviewInvalidBody(t *testing.T) {
	app, _ := newApp(&mockService{}, "admin")
	req := httptest.NewRequest("POST", "/api/v1/admin/email-templates/preview", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

func TestListEmailLog(t *testing.T) {
	var gotParams ports.ListParams
	svc := &mockService{
		EmailLogsFn: func(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
			gotParams = p
			return []domain.EmailLogEntry{{ID: 1, ToEmail: "a@b.c"}}, 37, nil
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("GET",
		"/api/v1/admin/email-log/?page=2&per_page=10&search=budi&status=failed", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, gotParams.Page)
	assert.Equal(t, 10, gotParams.PerPage)
	assert.Equal(t, "budi", gotParams.Search)
	assert.Equal(t, "failed", gotParams.Status)

	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Meta)
	assert.Equal(t, int64(37), env.Meta.Total)
	assert.Equal(t, 2, env.Meta.Page)
}

func TestListEmailLogServiceError(t *testing.T) {
	svc := &mockService{
		EmailLogsFn: func(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
			return nil, 0, apperr.Internal(errors.New("db down"))
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/email-log/", nil))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
}

func TestMyEmailLogScopesToCallerIdentity(t *testing.T) {
	var gotParams ports.ListParams
	svc := &mockService{
		EmailLogsFn: func(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
			gotParams = p
			return []domain.EmailLogEntry{{ID: 1, ToEmail: "me@example.com"}}, 1, nil
		},
	}
	app := clientApp(svc)
	resp, err := app.Test(httptest.NewRequest("GET",
		"/api/v1/account/email-log?page=1&per_page=10&status=sent", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int64(42), gotParams.UserID, "scoped to the caller's own user id from the token")
	assert.Equal(t, "sent", gotParams.Status)

	env := readEnvelope(t, resp.Body)
	require.NotNil(t, env.Meta)
	assert.Equal(t, int64(1), env.Meta.Total)
}

func TestMyEmailLogRequiresClientRole(t *testing.T) {
	app, _ := newApp(&mockService{}, "staff")
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/account/email-log", nil))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestRetryEmail(t *testing.T) {
	var gotActor, gotID int64
	svc := &mockService{
		RetryEmailFn: func(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error) {
			gotActor, gotID = actorUserID, emailLogID
			return &domain.EmailLogEntry{ID: emailLogID, Status: domain.EmailFailed}, nil
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/admin/email-log/123/retry", nil))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int64(42), gotActor)
	assert.Equal(t, int64(123), gotID)
}

func TestRetryEmailInvalidID(t *testing.T) {
	app, _ := newApp(&mockService{}, "admin")
	for _, id := range []string{"abc", "0", "-5"} {
		resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/admin/email-log/"+id+"/retry", nil))
		require.NoError(t, err)
		assert.Equal(t, 422, resp.StatusCode, "id=%s", id)
	}
}

func TestRetryEmailConflictPassthrough(t *testing.T) {
	svc := &mockService{
		RetryEmailFn: func(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error) {
			return nil, apperr.Conflict("only failed emails can be retried")
		},
	}
	app, _ := newApp(svc, "admin")
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/admin/email-log/5/retry", nil))
	require.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "CONFLICT", env.Error.Code)
}

// POST /admin/email/test

func TestSendTestEmailHandler(t *testing.T) {
	var gotActor int64
	var gotIn notifications.SendTestEmailInput
	svc := &mockService{
		SendTestEmailFn: func(ctx context.Context, actorUserID int64, in notifications.SendTestEmailInput) (*notifications.TestEmailResult, error) {
			gotActor, gotIn = actorUserID, in
			return &notifications.TestEmailResult{EmailLogID: 5, To: in.To, Subject: "[WHCMS] Test email"}, nil
		},
	}
	app, _ := newApp(svc, "admin")
	req := httptest.NewRequest("POST", "/api/v1/admin/email/test", strings.NewReader(`{"to":"ops@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, int64(42), gotActor)
	assert.Equal(t, "ops@example.com", gotIn.To)
}

func TestSendTestEmailHandlerInvalidBody(t *testing.T) {
	app, _ := newApp(&mockService{}, "admin")
	req := httptest.NewRequest("POST", "/api/v1/admin/email/test", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 422, resp.StatusCode)
}

// A failing relay must reach the admin as EXTERNAL with the upstream text.
func TestSendTestEmailHandlerRelayFailure(t *testing.T) {
	svc := &mockService{
		SendTestEmailFn: func(ctx context.Context, actorUserID int64, in notifications.SendTestEmailInput) (*notifications.TestEmailResult, error) {
			return nil, apperr.New(apperr.CodeExternal, "test email delivery failed: dial tcp: connection refused")
		},
	}
	app, _ := newApp(svc, "admin")
	req := httptest.NewRequest("POST", "/api/v1/admin/email/test", strings.NewReader(`{"to":"ops@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	env := readEnvelope(t, resp.Body)
	assert.Equal(t, "EXTERNAL", env.Error.Code)
	assert.Contains(t, env.Error.Message, "connection refused")
}
