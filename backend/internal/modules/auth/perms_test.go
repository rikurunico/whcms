package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/auth"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callHasPermission exercises PermissionSource with a real fiber.Ctx.
func callHasPermission(t *testing.T, src *auth.PermissionSource, userID int64, module string) (bool, error) {
	t.Helper()
	var has bool
	var err error
	app := fiber.New()
	app.Get("/t", func(c fiber.Ctx) error {
		has, err = src.HasPermission(c, userID, module)
		return c.SendStatus(200)
	})
	resp, terr := app.Test(httptest.NewRequest("GET", "/t", nil))
	require.NoError(t, terr)
	require.Equal(t, 200, resp.StatusCode)
	return has, err
}

func TestPermissionSource(t *testing.T) {
	users := &mocks.MockUserRepo{
		GetByIDFn: func(_ context.Context, id int64) (*domain.User, error) {
			switch id {
			case 3:
				return &domain.User{
					ID: 3, Role: domain.RoleStaff,
					Permissions: json.RawMessage(`{"billing":true,"clients":false}`),
				}, nil
			case 4:
				return &domain.User{ID: 4, Role: domain.RoleStaff}, nil // no permissions map
			case 5:
				return &domain.User{ID: 5, Permissions: json.RawMessage(`not-json`)}, nil
			}
			return nil, apperr.NotFound("user")
		},
	}
	src := auth.NewPermissionSource(users)

	has, err := callHasPermission(t, src, 3, "billing")
	require.NoError(t, err)
	assert.True(t, has)

	has, err = callHasPermission(t, src, 3, "clients")
	require.NoError(t, err)
	assert.False(t, has, "explicit false")

	has, err = callHasPermission(t, src, 3, "products")
	require.NoError(t, err)
	assert.False(t, has, "missing key")

	has, err = callHasPermission(t, src, 4, "billing")
	require.NoError(t, err)
	assert.False(t, has, "empty permissions map")

	has, err = callHasPermission(t, src, 5, "billing")
	require.NoError(t, err)
	assert.False(t, has, "invalid JSON treated as no permission")

	has, err = callHasPermission(t, src, 999, "billing")
	require.NoError(t, err)
	assert.False(t, has, "unknown user is not an error")
}

func TestPermissionSourceRepoError(t *testing.T) {
	users := &mocks.MockUserRepo{
		GetByIDFn: func(context.Context, int64) (*domain.User, error) {
			return nil, errors.New("db down")
		},
	}
	src := auth.NewPermissionSource(users)
	has, err := callHasPermission(t, src, 3, "billing")
	require.Error(t, err)
	assert.False(t, has)
}
