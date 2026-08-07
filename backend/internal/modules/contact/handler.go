package contact

import (
	"context"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
)

// Middlewares is the narrow middleware surface the handler needs; satisfied
// by *transporthttp.Middleware.
type Middlewares interface {
	RateLimit(prefix string, limit int, window time.Duration) fiber.Handler
}

var _ Middlewares = (*transporthttp.Middleware)(nil)

// ContactService is the use-case surface consumed by the handler (implemented
// by *Service).
type ContactService interface {
	Submit(ctx context.Context, in ContactInput, ip string) (*domain.Ticket, error)
}

var _ ContactService = (*Service)(nil)

// Handler exposes the public contact endpoint.
type Handler struct {
	svc ContactService
	mw  Middlewares
}

// NewHandler builds the contact Handler.
func NewHandler(svc ContactService, mw Middlewares) *Handler {
	return &Handler{svc: svc, mw: mw}
}

// RegisterRoutes mounts the contact route on r (the /api/v1 group). The public
// endpoint carries a tight rate limit to blunt spam.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/contact", h.mw.RateLimit("contact", 5, time.Minute), h.Submit)
}

// Submit opens a guest support ticket from the public contact form and returns
// only the ticket number to the anonymous caller.
func (h *Handler) Submit(c fiber.Ctx) error {
	var in ContactInput
	if err := c.Bind().Body(&in); err != nil {
		return apperr.Validation("invalid request body")
	}
	ticket, err := h.svc.Submit(c.Context(), in, c.IP())
	if err != nil {
		return err
	}
	return httpx.Created(c, ContactResult{TicketNumber: ticket.TicketNumber})
}
