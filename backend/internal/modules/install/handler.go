package install

import (
	"context"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
)

// ServiceAPI is the use-case surface the handler consumes (implemented by
// *Service; mocked in handler tests).
type ServiceAPI interface {
	Status(ctx context.Context) (*StatusResponse, error)
	CreateAdmin(ctx context.Context, in CreateAdminRequest) (*domain.User, error)
	SaveSettings(ctx context.Context, actorUserID int64, in SiteSettingsRequest) error
}

// Compile-time check.
var _ ServiceAPI = (*Service)(nil)

// Handler exposes the install HTTP endpoints. Deliberately unauthenticated -
// there is no admin to authenticate as yet - the service layer's own
// ExistsAnyAdmin re-check on every mutating call is what keeps this safe
// after a real install (see Service.CreateAdmin).
type Handler struct {
	svc ServiceAPI
}

// NewHandler builds a Handler.
func NewHandler(svc ServiceAPI) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the app-level install routes on r (the /api/v1
// group):
//
//	GET  /install/status     public
//	POST /install/admin      public (rejects with CONFLICT once installed)
//	POST /install/settings   public (rejects with CONFLICT once installed)
func (h *Handler) RegisterRoutes(r fiber.Router) {
	grp := r.Group("/install")
	grp.Get("/status", h.status)
	grp.Post("/admin", h.createAdmin)
	grp.Post("/settings", h.saveSettings)
}

func (h *Handler) status(c fiber.Ctx) error {
	resp, err := h.svc.Status(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, resp)
}

func (h *Handler) createAdmin(c fiber.Ctx) error {
	var in CreateAdminRequest
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	user, err := h.svc.CreateAdmin(c.Context(), in)
	if err != nil {
		return err
	}
	return httpx.Created(c, fiber.Map{"id": user.ID, "email": user.Email})
}

func (h *Handler) saveSettings(c fiber.Ctx) error {
	// Only reachable pre-install (the frontend gates /install once
	// installed), but there is no authenticated actor yet either way - audit
	// as a system action (actor 0) rather than guessing a user id.
	var in SiteSettingsRequest
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	if err := h.svc.SaveSettings(c.Context(), 0, in); err != nil {
		return err
	}
	return httpx.OK(c, fiber.Map{"ok": true})
}
