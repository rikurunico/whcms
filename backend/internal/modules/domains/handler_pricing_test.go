package domains_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/domains"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doNoContent issues a request expecting a 204 (no JSON body to parse).
func doNoContent(t *testing.T, app *fiber.App, method, path string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestHandlerPublicTLDsAndAddons(t *testing.T) {
	svc := &fakeService{
		ListActiveTLDsFn: func(ctx context.Context) ([]domain.TLDPricing, error) {
			return []domain.TLDPricing{{ID: 1, TLD: "com", Active: true, MinYears: 1, MaxYears: 10}}, nil
		},
		ListActiveDomainAddonsFn: func(ctx context.Context) ([]domain.DomainAddon, error) {
			return []domain.DomainAddon{{ID: 1, Key: "id_protection", Price: 20000, Active: true}}, nil
		},
	}
	app := newApp(svc, &fakeMW{})

	status, env := doJSON(t, app, "GET", "/api/v1/domains/tlds", "")
	assert.Equal(t, 200, status)
	list := env["data"].([]any)
	require.Len(t, list, 1)
	assert.Equal(t, "com", list[0].(map[string]any)["tld"])

	status, env = doJSON(t, app, "GET", "/api/v1/domains/addons", "")
	assert.Equal(t, 200, status)
	list = env["data"].([]any)
	require.Len(t, list, 1)
	assert.Equal(t, "id_protection", list[0].(map[string]any)["key"])
}

func TestHandlerUpdateDomainAddons(t *testing.T) {
	svc := &fakeService{
		UpdateDomainAddonsFn: func(ctx context.Context, clientID, domainID int64, keys []string) (*domain.Domain, error) {
			assert.EqualValues(t, 5, clientID)
			assert.EqualValues(t, 10, domainID)
			assert.Equal(t, []string{"dns_management"}, keys)
			return &domain.Domain{ID: domainID, DNSManagementEnabled: true, RecurringAmount: 175000}, nil
		},
	}
	app := newApp(svc, &fakeMW{identity: clientIdentity()})
	status, env := doJSON(t, app, "POST", "/api/v1/domains/10/addons", `{"addons":["dns_management"]}`)
	assert.Equal(t, 200, status)
	assert.Equal(t, true, env["data"].(map[string]any)["dns_management_enabled"])

	t.Run("rejects an unknown addon key", func(t *testing.T) {
		status, _ := doJSON(t, app, "POST", "/api/v1/domains/10/addons", `{"addons":["not_a_real_addon"]}`)
		assert.Equal(t, 422, status)
	})
}

func TestHandlerTLDPricingAdmin(t *testing.T) {
	body := `{"tld":"com","registrar_id":1,"active":true,"min_years":1,"max_years":10,"register_prices":{"1":150000},"renew_prices":{"1":160000},"transfer_price":150000}`

	svc := &fakeService{
		ListTLDPricingFn: func(ctx context.Context) ([]domain.TLDPricing, error) {
			return []domain.TLDPricing{{ID: 1, TLD: "com"}}, nil
		},
		GetTLDPricingFn: func(ctx context.Context, id int64) (*domain.TLDPricing, error) {
			return &domain.TLDPricing{ID: id, TLD: "com"}, nil
		},
		CreateTLDPricingFn: func(ctx context.Context, actorUserID int64, in domains.TLDPricingRequest) (*domain.TLDPricing, error) {
			assert.EqualValues(t, 1, actorUserID)
			return &domain.TLDPricing{ID: 2, TLD: in.TLD}, nil
		},
		UpdateTLDPricingFn: func(ctx context.Context, actorUserID, id int64, in domains.TLDPricingRequest) (*domain.TLDPricing, error) {
			return &domain.TLDPricing{ID: id, TLD: in.TLD, Active: in.Active}, nil
		},
		DeleteTLDPricingFn: func(ctx context.Context, actorUserID, id int64) error {
			return nil
		},
	}
	app := newApp(svc, &fakeMW{identity: adminIdentity()})

	status, env := doJSON(t, app, "GET", "/api/v1/admin/tld-pricing/", "")
	assert.Equal(t, 200, status)
	assert.Len(t, env["data"].([]any), 1)

	status, _ = doJSON(t, app, "GET", "/api/v1/admin/tld-pricing/1", "")
	assert.Equal(t, 200, status)

	status, env = doJSON(t, app, "POST", "/api/v1/admin/tld-pricing/", body)
	assert.Equal(t, 201, status)
	assert.Equal(t, "com", env["data"].(map[string]any)["tld"])

	status, env = doJSON(t, app, "PUT", "/api/v1/admin/tld-pricing/1", body)
	assert.Equal(t, 200, status)
	assert.Equal(t, true, env["data"].(map[string]any)["active"])

	status = doNoContent(t, app, "DELETE", "/api/v1/admin/tld-pricing/1")
	assert.Equal(t, 204, status)

	t.Run("create rejects an invalid body", func(t *testing.T) {
		status, _ := doJSON(t, app, "POST", "/api/v1/admin/tld-pricing/", `{"tld":""}`)
		assert.Equal(t, 422, status)
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		app := newApp(svc, &fakeMW{identity: clientIdentity()})
		status, _ := doJSON(t, app, "GET", "/api/v1/admin/tld-pricing/", "")
		assert.Equal(t, 403, status)
	})
}

func TestHandlerRegistrarCatalogAndImport(t *testing.T) {
	svc := &fakeService{
		ListRegistrarCatalogFn: func(ctx context.Context, registrarID int64) ([]domains.RegistrarCatalogResponse, error) {
			if registrarID == 99 {
				return nil, apperr.NotFound("registrar")
			}
			return []domains.RegistrarCatalogResponse{
				{TLD: "co.id", RegisterPrices: map[string]int64{"1": 100000}, AlreadyConfigured: false},
			}, nil
		},
		ImportTLDPricingFn: func(ctx context.Context, actorUserID int64, in domains.ImportTLDPricingRequest) (*domains.ImportTLDPricingResponse, error) {
			assert.EqualValues(t, 1, actorUserID)
			assert.Equal(t, []string{"co.id"}, in.TLDs)
			assert.InDelta(t, 20, in.MarkupPercent, 0.001)
			return &domains.ImportTLDPricingResponse{Imported: []string{"co.id"}, Skipped: []string{}}, nil
		},
	}
	app := newApp(svc, &fakeMW{identity: adminIdentity()})

	status, env := doJSON(t, app, "GET", "/api/v1/admin/registrars/1/catalog", "")
	assert.Equal(t, 200, status)
	list := env["data"].([]any)
	require.Len(t, list, 1)
	assert.Equal(t, "co.id", list[0].(map[string]any)["tld"])

	status, env = doJSON(t, app, "POST", "/api/v1/admin/tld-pricing/import",
		`{"registrar_id":1,"tlds":["co.id"],"markup_percent":20}`)
	assert.Equal(t, 200, status)
	imported := env["data"].(map[string]any)["imported"].([]any)
	assert.Equal(t, []any{"co.id"}, imported)

	t.Run("catalog propagates registrar not found", func(t *testing.T) {
		status, _ := doJSON(t, app, "GET", "/api/v1/admin/registrars/99/catalog", "")
		assert.Equal(t, 404, status)
	})

	t.Run("import rejects an empty tld list", func(t *testing.T) {
		status, _ := doJSON(t, app, "POST", "/api/v1/admin/tld-pricing/import", `{"registrar_id":1,"tlds":[]}`)
		assert.Equal(t, 422, status)
	})

	t.Run("import rejects a missing registrar id", func(t *testing.T) {
		status, _ := doJSON(t, app, "POST", "/api/v1/admin/tld-pricing/import", `{"tlds":["co.id"]}`)
		assert.Equal(t, 422, status)
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		app := newApp(svc, &fakeMW{identity: clientIdentity()})
		status, _ := doJSON(t, app, "GET", "/api/v1/admin/registrars/1/catalog", "")
		assert.Equal(t, 403, status)
	})
}

func TestHandlerPremiumPricingAdmin(t *testing.T) {
	body := `{"domain_name":"shop.id","register_price":5000000,"renew_price":5000000,"transfer_price":3000000}`

	svc := &fakeService{
		ListPremiumPricingFn: func(ctx context.Context) ([]domain.PremiumDomainPricing, error) {
			return []domain.PremiumDomainPricing{{ID: 1, DomainName: "shop.id"}}, nil
		},
		CreatePremiumPricingFn: func(ctx context.Context, actorUserID int64, in domains.PremiumDomainPricingRequest) (*domain.PremiumDomainPricing, error) {
			return &domain.PremiumDomainPricing{ID: 2, DomainName: in.DomainName}, nil
		},
		UpdatePremiumPricingFn: func(ctx context.Context, actorUserID, id int64, in domains.PremiumDomainPricingRequest) (*domain.PremiumDomainPricing, error) {
			return &domain.PremiumDomainPricing{ID: id, DomainName: in.DomainName, RegisterPrice: in.RegisterPrice}, nil
		},
		DeletePremiumPricingFn: func(ctx context.Context, actorUserID, id int64) error {
			return nil
		},
	}
	app := newApp(svc, &fakeMW{identity: adminIdentity()})

	status, env := doJSON(t, app, "GET", "/api/v1/admin/premium-domain-pricing/", "")
	assert.Equal(t, 200, status)
	assert.Len(t, env["data"].([]any), 1)

	status, env = doJSON(t, app, "POST", "/api/v1/admin/premium-domain-pricing/", body)
	assert.Equal(t, 201, status)
	assert.Equal(t, "shop.id", env["data"].(map[string]any)["domain_name"])

	status, env = doJSON(t, app, "PUT", "/api/v1/admin/premium-domain-pricing/1", body)
	assert.Equal(t, 200, status)
	assert.EqualValues(t, 5000000, env["data"].(map[string]any)["register_price"])

	status = doNoContent(t, app, "DELETE", "/api/v1/admin/premium-domain-pricing/1")
	assert.Equal(t, 204, status)
}

func TestHandlerPremiumLengthPricingAdmin(t *testing.T) {
	body := `{"tld":"id","char_length":2,"price":485000000}`

	svc := &fakeService{
		ListPremiumLengthPricingFn: func(ctx context.Context) ([]domain.PremiumLengthPricing, error) {
			return []domain.PremiumLengthPricing{{ID: 1, TLD: "id", CharLength: 2}}, nil
		},
		CreatePremiumLengthPricingFn: func(ctx context.Context, actorUserID int64, in domains.PremiumLengthPricingRequest) (*domain.PremiumLengthPricing, error) {
			return &domain.PremiumLengthPricing{ID: 2, TLD: in.TLD, CharLength: in.CharLength, Price: in.Price}, nil
		},
		UpdatePremiumLengthPricingFn: func(ctx context.Context, actorUserID, id int64, in domains.PremiumLengthPricingRequest) (*domain.PremiumLengthPricing, error) {
			return &domain.PremiumLengthPricing{ID: id, TLD: in.TLD, CharLength: in.CharLength, Price: in.Price}, nil
		},
		DeletePremiumLengthPricingFn: func(ctx context.Context, actorUserID, id int64) error {
			return nil
		},
	}
	app := newApp(svc, &fakeMW{identity: adminIdentity()})

	status, env := doJSON(t, app, "GET", "/api/v1/admin/premium-length-pricing/", "")
	assert.Equal(t, 200, status)
	assert.Len(t, env["data"].([]any), 1)

	status, env = doJSON(t, app, "POST", "/api/v1/admin/premium-length-pricing/", body)
	assert.Equal(t, 201, status)
	assert.Equal(t, "id", env["data"].(map[string]any)["tld"])
	assert.EqualValues(t, 2, env["data"].(map[string]any)["char_length"])

	status, env = doJSON(t, app, "PUT", "/api/v1/admin/premium-length-pricing/1", body)
	assert.Equal(t, 200, status)
	assert.EqualValues(t, 485000000, env["data"].(map[string]any)["price"])

	status = doNoContent(t, app, "DELETE", "/api/v1/admin/premium-length-pricing/1")
	assert.Equal(t, 204, status)
}

func TestHandlerDomainAddonsAdmin(t *testing.T) {
	svc := &fakeService{
		ListDomainAddonsFn: func(ctx context.Context) ([]domain.DomainAddon, error) {
			return []domain.DomainAddon{{ID: 1, Key: "id_protection", Name: "ID Protection"}}, nil
		},
		UpdateDomainAddonFn: func(ctx context.Context, actorUserID, id int64, in domains.UpdateDomainAddonRequest) (*domain.DomainAddon, error) {
			return &domain.DomainAddon{ID: id, Key: "id_protection", Price: in.Price, Active: in.Active}, nil
		},
	}
	app := newApp(svc, &fakeMW{identity: adminIdentity()})

	status, env := doJSON(t, app, "GET", "/api/v1/admin/domain-addons/", "")
	assert.Equal(t, 200, status)
	assert.Len(t, env["data"].([]any), 1)

	status, env = doJSON(t, app, "PUT", "/api/v1/admin/domain-addons/1", `{"price":25000,"active":true}`)
	assert.Equal(t, 200, status)
	assert.EqualValues(t, 25000, env["data"].(map[string]any)["price"])
	assert.Equal(t, true, env["data"].(map[string]any)["active"])
}
