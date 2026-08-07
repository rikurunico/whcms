package notifications

import (
	"context"
	"strconv"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
)

// Middlewares is the narrow middleware surface the handler needs; satisfied
// by *transporthttp.Middleware.
type Middlewares interface {
	RequireAuth() fiber.Handler
	RequireRole(roles ...string) fiber.Handler
	RequirePermission(module string) fiber.Handler
	RequireClient() fiber.Handler
}

var _ Middlewares = (*transporthttp.Middleware)(nil)

// NotificationService is the use-case surface consumed by the handler
// (implemented by *Service).
type NotificationService interface {
	ListTemplates(ctx context.Context) ([]domain.EmailTemplate, error)
	GetTemplate(ctx context.Context, key, locale string) (*domain.EmailTemplate, error)
	SaveTemplate(ctx context.Context, actorUserID int64, in SaveTemplateInput) (*domain.EmailTemplate, error)
	DeleteTemplate(ctx context.Context, actorUserID int64, key, locale string) error
	Preview(ctx context.Context, in PreviewInput) (*PreviewResult, error)
	SendTestEmail(ctx context.Context, actorUserID int64, in SendTestEmailInput) (*TestEmailResult, error)
	EmailLogs(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error)
	RetryEmail(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error)
}

var _ NotificationService = (*Service)(nil)

// Handler exposes the notifications admin HTTP endpoints.
type Handler struct {
	svc NotificationService
	mw  Middlewares
}

// NewHandler builds the notifications Handler.
func NewHandler(svc NotificationService, mw Middlewares) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts the notifications routes on r (the /api/v1 group).
// See the package doc for the full route table.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	admin := r.Group("/admin", h.mw.RequireAuth(), h.mw.RequireRole("admin", "staff"))

	tpl := admin.Group("/email-templates", h.mw.RequirePermission("settings"))
	tpl.Get("/", h.ListTemplates)
	tpl.Put("/", h.SaveTemplate)
	tpl.Post("/preview", h.Preview)
	tpl.Get("/:key/:locale", h.GetTemplate)
	tpl.Delete("/:key/:locale", h.DeleteTemplate)

	admin.Post("/email/test", h.mw.RequirePermission("settings"), h.SendTestEmail)

	logs := admin.Group("/email-log", h.mw.RequirePermission("logs"))
	logs.Get("/", h.ListEmailLog)
	logs.Post("/:id/retry", h.RetryEmail)

	account := r.Group("/account", h.mw.RequireAuth(), h.mw.RequireClient())
	account.Get("/email-log", h.MyEmailLog)
}

// SendTestEmail sends a diagnostic email synchronously and reports the
// upstream failure verbatim when the relay rejects it.
func (h *Handler) SendTestEmail(c fiber.Ctx) error {
	var in SendTestEmailInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	id := httpx.MustIdentity(c)
	res, err := h.svc.SendTestEmail(c.Context(), id.UserID, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, res)
}

// ListTemplates returns all templates.
func (h *Handler) ListTemplates(c fiber.Ctx) error {
	templates, err := h.svc.ListTemplates(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, templates)
}

// GetTemplate returns one template by key+locale.
func (h *Handler) GetTemplate(c fiber.Ctx) error {
	t, err := h.svc.GetTemplate(c.Context(), c.Params("key"), c.Params("locale"))
	if err != nil {
		return err
	}
	return httpx.OK(c, t)
}

// SaveTemplate upserts one template.
func (h *Handler) SaveTemplate(c fiber.Ctx) error {
	var in SaveTemplateInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	id := httpx.MustIdentity(c)
	t, err := h.svc.SaveTemplate(c.Context(), id.UserID, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, t)
}

// DeleteTemplate removes one template locale variant.
func (h *Handler) DeleteTemplate(c fiber.Ctx) error {
	id := httpx.MustIdentity(c)
	if err := h.svc.DeleteTemplate(c.Context(), id.UserID, c.Params("key"), c.Params("locale")); err != nil {
		return err
	}
	return httpx.NoContent(c)
}

// Preview renders a stored template with sample data.
func (h *Handler) Preview(c fiber.Ctx) error {
	var in PreviewInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	res, err := h.svc.Preview(c.Context(), in)
	if err != nil {
		return err
	}
	return httpx.OK(c, res)
}

// ListEmailLog lists outbound emails (?search= recipient ILIKE, ?status=
// queued|sent|failed).
func (h *Handler) ListEmailLog(c fiber.Ctx) error {
	page := httpx.ParsePage(c)
	rows, total, err := h.svc.EmailLogs(c.Context(), ports.ListParams{
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  c.Query("search"),
		Status:  c.Query("status"),
	})
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// MyEmailLog lists the caller's own email delivery history (?search=
// recipient ILIKE, ?status= queued|sent|failed). Scoped to the authenticated
// user's own id from the token - never a caller-supplied one - so a client
// can only ever see their own mail.
func (h *Handler) MyEmailLog(c fiber.Ctx) error {
	page := httpx.ParsePage(c)
	id := httpx.MustIdentity(c)
	rows, total, err := h.svc.EmailLogs(c.Context(), ports.ListParams{
		UserID:  id.UserID,
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  c.Query("search"),
		Status:  c.Query("status"),
	})
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// RetryEmail re-enqueues a failed email.
func (h *Handler) RetryEmail(c fiber.Ctx) error {
	emailLogID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || emailLogID < 1 {
		return apperr.Validation("invalid email log id")
	}
	id := httpx.MustIdentity(c)
	entry, err := h.svc.RetryEmail(c.Context(), id.UserID, emailLogID)
	if err != nil {
		return err
	}
	return httpx.OK(c, entry)
}
