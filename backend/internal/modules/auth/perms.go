package auth

import (
	"encoding/json"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	transporthttp "github.com/tsdlamongan/whcms/backend/internal/transport/http"

	"github.com/gofiber/fiber/v3"
)

// PermissionSource answers staff per-module permission checks from the users
// row (permissions JSONB {"billing": true, ...}). Wiring plugs it into
// transport/http Middleware.Perms.
type PermissionSource struct {
	users ports.UserRepo
}

// Compile-time check against the transport contract.
var _ transporthttp.PermissionSource = (*PermissionSource)(nil)

// NewPermissionSource builds a PermissionSource backed by the users repo.
func NewPermissionSource(users ports.UserRepo) *PermissionSource {
	return &PermissionSource{users: users}
}

// HasPermission reports whether the user has the module flag set. Unknown
// users and missing/invalid permission maps are simply "no permission".
func (p *PermissionSource) HasPermission(c fiber.Ctx, userID int64, module string) (bool, error) {
	u, err := p.users.GetByID(c.Context(), userID)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if u == nil || len(u.Permissions) == 0 {
		return false, nil
	}
	var perms map[string]bool
	if err := json.Unmarshal(u.Permissions, &perms); err != nil {
		return false, nil
	}
	return perms[module], nil
}
