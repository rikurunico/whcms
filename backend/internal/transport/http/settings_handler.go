package http

import (
	"context"
	"encoding/json"

	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
)

// SettingsService is the settings use-case surface consumed by the handler.
type SettingsService interface {
	Grouped(ctx context.Context) (map[string]map[string]any, error)
	Update(ctx context.Context, actorUserID int64, updates map[string]any) error
}

// SettingsHandler exposes admin settings endpoints. This is the reference
// handler implementation for module agents.
type SettingsHandler struct {
	svc SettingsService
}

// NewSettingsHandler builds a SettingsHandler.
func NewSettingsHandler(svc SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// Register mounts GET / and PUT / on the given router group
// (mounted at /api/v1/admin/settings by the composition root).
func (h *SettingsHandler) Register(r fiber.Router) {
	r.Get("/", h.Get)
	r.Put("/", h.Put)
}

// Get returns all settings grouped by prefix.
func (h *SettingsHandler) Get(c fiber.Ctx) error {
	grouped, err := h.svc.Grouped(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, grouped)
}

// Put updates settings. Body: {"company.name": "...", "billing.tax_rate": 11}.
func (h *SettingsHandler) Put(c fiber.Ctx) error {
	var updates map[string]any
	if err := json.Unmarshal(c.Body(), &updates); err != nil {
		return apperr.Validation("body must be a JSON object of settings keys")
	}
	id := httpx.MustIdentity(c)
	if err := h.svc.Update(c.Context(), id.UserID, updates); err != nil {
		return err
	}
	grouped, err := h.svc.Grouped(c.Context())
	if err != nil {
		return err
	}
	return httpx.OK(c, grouped)
}
