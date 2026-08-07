package catalog_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func specBody() map[string]any {
	return map[string]any{
		"key": "disk", "label": "Disk", "provision_key": "disk", "unit": "gb",
		"included_qty": 5, "min_qty": 5, "max_qty": 100, "step_qty": 5, "default_qty": 10,
	}
}

func TestHandlerSpecCRUD(t *testing.T) {
	h := newApp(nil)
	h.fx.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: id, Module: domain.ModuleCpanel, Configurable: true}, nil
	}

	// Create
	resp, env := h.do(t, "POST", "/api/v1/admin/products/1/specs", specBody())
	assert.Equal(t, 201, resp.StatusCode)
	require.Nil(t, env.Error)

	// List
	h.fx.specs.ListSpecsFn = func(_ context.Context, productID int64) ([]domain.ProductSpec, error) {
		return []domain.ProductSpec{{ID: 1, Key: "disk"}}, nil
	}
	resp, env = h.do(t, "GET", "/api/v1/admin/products/1/specs", nil)
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)

	// Update
	resp, env = h.do(t, "PATCH", "/api/v1/admin/product-specs/9", specBody())
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)

	// Delete
	resp, _ = h.do(t, "DELETE", "/api/v1/admin/product-specs/9", nil)
	assert.Equal(t, 204, resp.StatusCode)

	// Upsert pricing
	resp, env = h.do(t, "PUT", "/api/v1/admin/product-specs/9/pricing", map[string]any{
		"cycle": "monthly", "unit_price": 5000, "unlimited_price": 200000,
	})
	assert.Equal(t, 200, resp.StatusCode)
	require.Nil(t, env.Error)

	// Delete pricing
	resp, _ = h.do(t, "DELETE", "/api/v1/admin/product-specs/9/pricing/monthly", nil)
	assert.Equal(t, 204, resp.StatusCode)
}

func TestHandlerSpecCreateValidation(t *testing.T) {
	h := newApp(nil)
	h.fx.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: id, Module: domain.ModuleCpanel, Configurable: true}, nil
	}
	body := specBody()
	body["provision_key"] = "cpu" // invalid
	resp, env := h.do(t, "POST", "/api/v1/admin/products/1/specs", body)
	assert.Equal(t, 422, resp.StatusCode)
	require.NotNil(t, env.Error)
	assert.Equal(t, "VALIDATION", env.Error.Code)
}

func TestHandlerSpecForbiddenForClient(t *testing.T) {
	h := newApp(func(mw *fakeMW) {
		mw.identity.Role = "client"
	})
	resp, _ := h.do(t, "POST", "/api/v1/admin/products/1/specs", specBody())
	assert.Equal(t, 403, resp.StatusCode)
}
