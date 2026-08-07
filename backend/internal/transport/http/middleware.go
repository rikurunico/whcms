// Package http contains the Fiber v3 transport: shared middleware
// (foundation-owned) and per-module handlers (<mod>_handler.go, owned by the
// module agents). settings_handler.go is the reference handler.
package http

import (
	"log/slog"
	"strings"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/logger"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/service/authtoken"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// AccessParser validates access JWTs (implemented by authtoken.Manager).
type AccessParser interface {
	ParseAccess(token string) (*authtoken.AccessClaims, error)
}

// PermissionSource answers staff per-module permission checks (permissions
// JSONB on the users row). Implemented by the users module / wiring.
type PermissionSource interface {
	HasPermission(ctx fiber.Ctx, userID int64, module string) (bool, error)
}

// PermissionSourceFunc adapts a function to PermissionSource.
type PermissionSourceFunc func(ctx fiber.Ctx, userID int64, module string) (bool, error)

// HasPermission calls the function.
func (f PermissionSourceFunc) HasPermission(ctx fiber.Ctx, userID int64, module string) (bool, error) {
	return f(ctx, userID, module)
}

// Middleware bundles the shared HTTP middleware with its dependencies.
type Middleware struct {
	Tokens  AccessParser
	Perms   PermissionSource
	Limiter ports.RateLimiter
	Log     *slog.Logger
}

// RequestID assigns a request id (or propagates X-Request-ID), stores it in
// Locals + the request context (for slog) and echoes it in the response.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Locals(httpx.RequestIDKey, id)
		c.SetContext(logger.WithRequestID(c.Context(), id))
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

// ErrorHandler is the single Fiber error handler mapping apperr -> HTTP
// envelope (CONTRACTS.md §3). Wire it via fiber.Config{ErrorHandler: ...}.
func ErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		// Fiber router errors (404, 405, body limits, ...) keep their status.
		if fe, ok := err.(*fiber.Error); ok {
			code := apperr.CodeInternal
			switch fe.Code {
			case fiber.StatusNotFound:
				code = apperr.CodeNotFound
			case fiber.StatusMethodNotAllowed, fiber.StatusBadRequest,
				fiber.StatusRequestEntityTooLarge, fiber.StatusUnprocessableEntity:
				code = apperr.CodeValidation
			case fiber.StatusUnauthorized:
				code = apperr.CodeUnauthorized
			case fiber.StatusForbidden:
				code = apperr.CodeForbidden
			case fiber.StatusTooManyRequests:
				code = apperr.CodeRateLimited
			}
			return c.Status(fe.Code).JSON(httpx.Envelope{
				Error: &httpx.ErrorBody{Code: string(code), Message: fe.Message},
			})
		}

		e := apperr.From(err)
		if e.Code == apperr.CodeInternal {
			log.ErrorContext(c.Context(), "internal error",
				"path", c.Path(), "method", c.Method(), "error", err)
		}
		return httpx.Fail(c, e)
	}
}

// RequireAuth parses the Bearer access token and stores the identity in
// Locals. Missing/invalid tokens -> 401.
func (m *Middleware) RequireAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get(fiber.HeaderAuthorization)
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			return apperr.Unauthorized("missing bearer token")
		}
		claims, err := m.Tokens.ParseAccess(token)
		if err != nil {
			return err
		}
		httpx.SetIdentity(c, httpx.AuthIdentity{
			UserID:   claims.UserID,
			Role:     claims.Role,
			ClientID: claims.ClientID,
			JTI:      claims.JTI,
		})
		return c.Next()
	}
}

// RequireRole allows only the listed roles (use behind RequireAuth).
func (m *Middleware) RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c fiber.Ctx) error {
		id, ok := httpx.Identity(c)
		if !ok {
			return apperr.Unauthorized("authentication required")
		}
		if !allowed[id.Role] {
			return apperr.Forbidden("insufficient role")
		}
		return c.Next()
	}
}

// RequirePermission enforces staff per-module permissions. Admin bypasses;
// staff must have the module flag; everyone else is forbidden.
func (m *Middleware) RequirePermission(module string) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, ok := httpx.Identity(c)
		if !ok {
			return apperr.Unauthorized("authentication required")
		}
		if id.Role == "admin" {
			return c.Next()
		}
		if id.Role != "staff" {
			return apperr.Forbidden("insufficient role")
		}
		has, err := m.Perms.HasPermission(c, id.UserID, module)
		if err != nil {
			return apperr.Internal(err)
		}
		if !has {
			return apperr.Forbidden("missing permission: " + module)
		}
		return c.Next()
	}
}

// RequireClient allows only authenticated users with a client profile
// (cid != 0 in the token).
func (m *Middleware) RequireClient() fiber.Handler {
	return func(c fiber.Ctx) error {
		id, ok := httpx.Identity(c)
		if !ok {
			return apperr.Unauthorized("authentication required")
		}
		if id.ClientID == 0 {
			return apperr.Forbidden("client profile required")
		}
		return c.Next()
	}
}

// RateLimit applies a fixed-window limit per client IP under the given key
// prefix (e.g. public endpoints 60/min/IP).
func (m *Middleware) RateLimit(prefix string, limit int, window time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := prefix + ":" + c.IP()
		ok, err := m.Limiter.Allow(c.Context(), key, limit, window)
		if err != nil {
			// Fail open: a Redis hiccup must not take the API down.
			m.Log.WarnContext(c.Context(), "rate limiter unavailable", "error", err)
			return c.Next()
		}
		if !ok {
			return apperr.New(apperr.CodeRateLimited, "too many requests")
		}
		return c.Next()
	}
}
