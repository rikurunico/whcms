package networkstatus

import (
	"context"
	"strconv"
	"time"

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
	RateLimit(prefix string, limit int, window time.Duration) fiber.Handler
}

var _ Middlewares = (*transporthttp.Middleware)(nil)

// NetworkStatusService is the use-case surface consumed by the handler
// (implemented by *Service).
type NetworkStatusService interface {
	PublicList(ctx context.Context) ([]domain.NetworkIssue, error)
	List(ctx context.Context, p ports.ListParams) ([]domain.NetworkIssue, int64, error)
	Get(ctx context.Context, id int64) (*domain.NetworkIssue, error)
	Create(ctx context.Context, actorUserID int64, in IssueInput) (*domain.NetworkIssue, error)
	Update(ctx context.Context, actorUserID, id int64, in IssueUpdateInput) (*domain.NetworkIssue, error)
	Delete(ctx context.Context, actorUserID, id int64) error
}

var _ NetworkStatusService = (*Service)(nil)

// Handler exposes the network status HTTP endpoints.
type Handler struct {
	svc NetworkStatusService
	mw  Middlewares
}

// NewHandler builds the network status Handler.
func NewHandler(svc NetworkStatusService, mw Middlewares) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts the network status routes on r (the /api/v1 group).
// See the package doc for the full route table.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	publicLimit := h.mw.RateLimit("network", 60, time.Minute)
	r.Get("/network-status", publicLimit, h.PublicList)
	r.Get("/network-status/:id", publicLimit, h.PublicGet)

	admin := r.Group("/admin",
		h.mw.RequireAuth(),
		h.mw.RequireRole("admin", "staff"),
		h.mw.RequirePermission("network"))

	issues := admin.Group("/network-issues")
	issues.Get("/", h.AdminList)
	issues.Post("/", h.AdminCreate)
	issues.Get("/:id", h.AdminGet)
	issues.Patch("/:id", h.AdminUpdate)
	issues.Delete("/:id", h.AdminDelete)
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

// PublicList returns the active/recent network status entries.
func (h *Handler) PublicList(c fiber.Ctx) error {
	data, err := h.svc.PublicList(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// PublicGet returns one network status entry by id.
func (h *Handler) PublicGet(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	data, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, data)
}

// Admin

// AdminList lists status entries (?search&status).
func (h *Handler) AdminList(c fiber.Ctx) error {
	page, params := listParams(c)
	rows, total, err := h.svc.List(c.Context(), params)
	if err != nil {
		return err
	}
	return httpx.OK(c, rows, page.Meta(total))
}

// AdminCreate creates a status entry.
func (h *Handler) AdminCreate(c fiber.Ctx) error {
	var in IssueInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	n, err := h.svc.Create(c.Context(), actor.UserID, in)
	if err != nil {
		return err
	}
	return httpx.Created(c, n)
}

// AdminGet returns one status entry.
func (h *Handler) AdminGet(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	n, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return httpx.OK(c, n)
}

// AdminUpdate patches a status entry.
func (h *Handler) AdminUpdate(c fiber.Ctx) error {
	id, err := parseID(c)
	if err != nil {
		return err
	}
	var in IssueUpdateInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	actor := httpx.MustIdentity(c)
	n, err := h.svc.Update(c.Context(), actor.UserID, id, in)
	if err != nil {
		return err
	}
	return httpx.OK(c, n)
}

// AdminDelete soft-deletes a status entry.
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
