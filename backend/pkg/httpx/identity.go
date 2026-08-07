package httpx

import "github.com/gofiber/fiber/v3"

// identityKey is the Locals key under which auth middleware stores the identity.
type identityCtxKey struct{}

// IdentityKey is the c.Locals key for the authenticated identity.
var IdentityKey = identityCtxKey{}

// AuthIdentity is the authenticated caller extracted from the access JWT.
type AuthIdentity struct {
	UserID   int64  // JWT "sub"
	Role     string // admin|staff|client
	ClientID int64  // JWT "cid"; 0 for staff/admin
	JTI      string // JWT id, used for refresh binding
}

// IsAdmin reports whether the identity has the admin role.
func (i AuthIdentity) IsAdmin() bool { return i.Role == "admin" }

// IsStaff reports whether the identity is admin or staff.
func (i AuthIdentity) IsStaff() bool { return i.Role == "admin" || i.Role == "staff" }

// SetIdentity stores the identity in the request Locals (called by RequireAuth).
func SetIdentity(c fiber.Ctx, id AuthIdentity) {
	c.Locals(IdentityKey, id)
}

// Identity returns the identity stored by auth middleware and whether it exists.
func Identity(c fiber.Ctx) (AuthIdentity, bool) {
	id, ok := c.Locals(IdentityKey).(AuthIdentity)
	return id, ok
}

// MustIdentity returns the identity or a zero value if unauthenticated.
// Use only behind RequireAuth.
func MustIdentity(c fiber.Ctx) AuthIdentity {
	id, _ := Identity(c)
	return id
}

// RequestIDKey is the Locals key for the per-request ID.
type requestIDCtxKey struct{}

// RequestIDKey is the c.Locals key for the request id string.
var RequestIDKey = requestIDCtxKey{}

// RequestID returns the request id set by the request-id middleware.
func RequestID(c fiber.Ctx) string {
	s, _ := c.Locals(RequestIDKey).(string)
	return s
}
