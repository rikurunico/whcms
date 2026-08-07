package announcements

import (
	"context"
	"strconv"
	"time"

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
	RateLimit(prefix string, limit int, window time.Duration) fiber.Handler
}

var _ Middlewares = (*transporthttp.Middleware)(nil)

// AnnouncementService is the use-case surface consumed by the handler
// (implemented by *Service).
type AnnouncementService interface {
	// Public
	ListPublished(ctx context.Context, p ports.ListParams) ([]AnnouncementResponse, int64, error)
	GetPublishedBySlug(ctx context.Context, slug string) (*AnnouncementResponse, error)
	// Admin
	List(ctx context.Context, p ports.ListParams) ([]AnnouncementResponse, int64, error)
	Get(ctx context.Context, id int64) (*AnnouncementResponse, error)
	Create(ctx context.Context, actorUserID int64, in AnnouncementInput) (*AnnouncementResponse, error)
	Update(ctx context.Context, actorUserID, id int64, in AnnouncementUpdateInput) (*AnnouncementResponse, error)
	Delete(ctx context.Context, actorUserID, id int64) error
}

var _ AnnouncementService = (*Service)(nil)

// Handler exposes the announcements HTTP endpoints.
type Handler struct {
	svc AnnouncementService
	mw  Middlewares
}

// NewHandler builds the announcements Handler.
func NewHandler(svc AnnouncementService, mw Middlewares) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts the announcements routes on r (the /api/v1 group). See
// the package doc for the full route table.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	publicLimit := h.mw.RateLimit("announcements", 60, time.Minute)
	r.Get("/announcements", publicLimit, h.PublicList)
	r.Get("/announcements/:slug", publicLimit, h.PublicGet)

	admin := r.Group("/admin/announcements",
		h.mw.RequireAuth(),
		h.mw.RequireRole("admin", "staff"),
		h.mw.RequirePermission("announcements"))
	admin.Get("/", h.AdminList)
	admin.Post("/", h.AdminCreate)
	admin.Get("/:id", h.AdminGet)
	admin.Patch("/:id", h.AdminUpdate)
	admin.Delete("/:id", h.AdminDelete)
}

func parseID(c fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, apperr.Validation("invalid id")
	}
	return id, nil
}

func listParams(c fiber.Ctx) (httpx.Page, ports.ListParams) {
	page := httpx.ParsePage(c)
	return page, ports.ListParams{
		Page:    page.Page,
		PerPage: page.PerPage,
		Search:  c.Query("search"),
		Status:  c.Query("status"),
		Sort:    c.Query("sort"),
	}
}

// Public

// PublicList returns published announcements (newest first, paginated).
func (h *Handler) PublicList(c fiber.Ctx) error {
	page, params := listParams(c)
	rows, total, err := h.svc.ListPublished(c.Context(), params)
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// PublicGet returns one published announcement by slug.
func (h *Handler) PublicGet(c fiber.Ctx) error {
	data, err := h.svc.GetPublishedBySlug(c.Context(), c.Params("slug"))
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// Admin

// AdminList lists announcements (?search&status=published|draft).
func (h *Handler) AdminList(c fiber.Ctx) error {
	page, params := listParams(c)
	rows, total, err := h.svc.List(c.Context(), params)
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// AdminCreate creates an announcement.
func (h *Handler) AdminCreate(c fiber.Ctx) error {
	var in AnnouncementInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	a, err := h.svc.Create(c.Context(), actor.UserID, in)
	if err != nil {
		return err
	}
	return httpx.Created(c, a)
}

// AdminGet returns one announcement.
func (h *Handler) AdminGet(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	a, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, a)
}

// AdminUpdate patches an announcement.
func (h *Handler) AdminUpdate(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var in AnnouncementUpdateInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	a, err := h.svc.Update(c.Context(), actor.UserID, id, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, a)
}

// AdminDelete soft-deletes an announcement.
func (h *Handler) AdminDelete(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	actor := httpx.MustIdentity(c)
	if err := h.svc.Delete(c.Context(), actor.UserID, id); err != nil {
		return err
	}
	return httpx.NoContent(c)
}
