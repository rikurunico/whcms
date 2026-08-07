package orders_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/modules/orders"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// activationEnv sets up a pending order with the given items and a product
// catalog lookup.
func activationEnv(e *testEnv, items []domain.OrderItem, products map[int64]*domain.Product) {
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, OrderNumber: "ORD-202607-000042", ClientID: 7, Status: domain.OrderPending}, nil
	}
	e.orderRepo.GetItemsFn = func(_ context.Context, _ int64) ([]domain.OrderItem, error) {
		return items, nil
	}
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		if p, ok := products[id]; ok {
			return p, nil
		}
		return nil, apperr.NotFound("product")
	}
}

func hostingItem(id int64, productID int64) domain.OrderItem {
	pid := productID
	return domain.OrderItem{
		ID: id, OrderID: 100, ItemType: domain.ItemProduct, ProductID: &pid,
		Domain: "budi.example.com", Cycle: domain.CycleMonthly,
		UnitPrice: 50000, SetupFee: 10000,
	}
}

func cpanelProduct() *domain.Product {
	return &domain.Product{
		ID: 10, Name: "Basic Hosting", Type: domain.ProductSharedHosting,
		Module: domain.ModuleCpanel, AutoSetup: domain.SetupOnPayment,
	}
}

func TestActivateOrderHostingProduct(t *testing.T) {
	e := newFixture()
	activationEnv(e, []domain.OrderItem{hostingItem(200, 10)},
		map[int64]*domain.Product{10: cpanelProduct()})
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))

	// Service row.
	require.Len(t, e.created, 1)
	s := e.created[0]
	assert.Equal(t, int64(7), s.ClientID)
	assert.Equal(t, int64(10), s.ProductID)
	assert.Equal(t, domain.ServicePending, s.Status)
	assert.Equal(t, "budi.example.com", s.Domain)
	assert.Equal(t, "budi", s.Username, "username derived from domain")
	assert.Contains(t, s.PasswordEnc, "enc:", "password stored encrypted")
	assert.Equal(t, int64(50000), s.RecurringAmount)
	assert.Equal(t, int64(10000), s.SetupFee)
	require.NotNil(t, s.NextDueDate)
	assert.Equal(t, time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), *s.NextDueDate, "today + monthly cycle")
	require.NotNil(t, s.OrderItemID)
	assert.Equal(t, int64(200), *s.OrderItemID)

	// Back-ref.
	ref, ok := e.backrefs[200]
	require.True(t, ok)
	require.NotNil(t, ref[0])
	assert.Equal(t, s.ID, *ref[0])
	assert.Nil(t, ref[1])

	// provision:create enqueued.
	require.Len(t, e.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeProvisionCreate, e.enqueuer.Tasks[0].TaskType)
	assert.Equal(t, jobs.ProvisionCreatePayload{ServiceID: s.ID}, e.enqueuer.Tasks[0].Payload)

	// Order active + notification.
	assert.Contains(t, e.statusSet, domain.OrderActive)
}

func TestActivateOrderNotifiesClient(t *testing.T) {
	e := newFixture()
	activationEnv(e, []domain.OrderItem{hostingItem(200, 10)},
		map[int64]*domain.Product{10: cpanelProduct()})
	var sentUser int64
	var sentKey string
	e.notifier.SendTemplateFn = func(_ context.Context, userID int64, key string, _ map[string]any) error {
		sentUser, sentKey = userID, key
		return nil
	}
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	assert.Equal(t, int64(3), sentUser, "client's user notified")
	assert.Equal(t, "order_activated", sentKey)
}

func TestActivateOrderManualSetup(t *testing.T) {
	e := newFixture()
	p := cpanelProduct()
	p.AutoSetup = domain.SetupManual
	activationEnv(e, []domain.OrderItem{hostingItem(200, 10)}, map[int64]*domain.Product{10: p})
	var alerts []string
	e.notifier.AlertAdminFn = func(_ context.Context, subject, msg string) error {
		alerts = append(alerts, subject+": "+msg)
		return nil
	}
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Equal(t, domain.ServicePending, e.created[0].Status, "service stays pending")
	assert.Empty(t, e.enqueuer.Tasks, "no provision job for manual setup")
	require.Len(t, alerts, 1)
	assert.Contains(t, alerts[0], "Manual provisioning required")
}

func TestActivateOrderModuleNoneActiveImmediately(t *testing.T) {
	e := newFixture()
	p := cpanelProduct()
	p.Module = domain.ModuleNone
	activationEnv(e, []domain.OrderItem{hostingItem(200, 10)}, map[int64]*domain.Product{10: p})
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Equal(t, domain.ServiceActive, e.created[0].Status)
	require.NotNil(t, e.created[0].RegistrationDate)
	assert.Empty(t, e.enqueuer.Tasks, "no provisioning for module=none")
}

func TestActivateOrderOneTimeCycleHasNoDueDate(t *testing.T) {
	e := newFixture()
	item := hostingItem(200, 10)
	item.Cycle = domain.CycleOneTime
	activationEnv(e, []domain.OrderItem{item}, map[int64]*domain.Product{10: cpanelProduct()})
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Nil(t, e.created[0].NextDueDate)
}

func TestActivateOrderRecurringCoupon(t *testing.T) {
	e := newFixture()
	cid := int64(5)
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderPending, CouponID: &cid}, nil
	}
	e.orderRepo.GetItemsFn = func(_ context.Context, _ int64) ([]domain.OrderItem, error) {
		return []domain.OrderItem{hostingItem(200, 10)}, nil
	}
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return cpanelProduct(), nil
	}
	withCoupon(e, couponFixture(func(c *domain.Coupon) { c.Recurring = true }))
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	s := e.created[0]
	assert.Equal(t, int64(45000), s.RecurringAmount, "10% recurring coupon applied")
	require.NotNil(t, s.CouponID)
	assert.Equal(t, int64(5), *s.CouponID)
}

func TestActivateOrderNonRecurringCouponIgnored(t *testing.T) {
	e := newFixture()
	cid := int64(5)
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderPending, CouponID: &cid}, nil
	}
	e.orderRepo.GetItemsFn = func(_ context.Context, _ int64) ([]domain.OrderItem, error) {
		return []domain.OrderItem{hostingItem(200, 10)}, nil
	}
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return cpanelProduct(), nil
	}
	withCoupon(e, couponFixture(nil)) // recurring=false
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Equal(t, int64(50000), e.created[0].RecurringAmount)
	assert.Nil(t, e.created[0].CouponID)
}

func TestActivateOrderDomainRegister(t *testing.T) {
	e := newFixture()
	opts, _ := json.Marshal(map[string]any{"domain_years": 2})
	item := domain.OrderItem{
		ID: 201, OrderID: 100, ItemType: domain.ItemDomainRegister,
		Domain: "example.com", Cycle: domain.CycleAnnually, UnitPrice: 300000,
		Options: opts,
	}
	activationEnv(e, []domain.OrderItem{item}, nil)
	e.settings.GetJSONFn = func(_ context.Context, key string, out any) error {
		if key == "domains.default_nameservers" {
			*(out.(*[]string)) = []string{"ns1.example.net", "ns2.example.net"}
		}
		return nil
	}
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.domCreated, 1)
	d := e.domCreated[0]
	assert.Equal(t, "example.com", d.Name)
	assert.Equal(t, domain.DomainPending, d.Status)
	assert.Equal(t, int64(150000), d.RecurringAmount, "per-year renewal price (unit / years)")
	assert.Equal(t, domain.CycleAnnually, d.BillingCycle)
	assert.True(t, d.AutoRenew)
	assert.Equal(t, int64(1), d.RegistrarID)
	var ns []string
	require.NoError(t, json.Unmarshal(d.Nameservers, &ns))
	assert.Equal(t, []string{"ns1.example.net", "ns2.example.net"}, ns)

	ref := e.backrefs[201]
	assert.Nil(t, ref[0])
	require.NotNil(t, ref[1])
	assert.Equal(t, d.ID, *ref[1])

	require.Len(t, e.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeDomainRegister, e.enqueuer.Tasks[0].TaskType)
	assert.Equal(t, jobs.DomainRegisterPayload{DomainID: d.ID, Years: 2}, e.enqueuer.Tasks[0].Payload)
}

func TestActivateOrderDomainTransfer(t *testing.T) {
	e := newFixture()
	opts, _ := json.Marshal(map[string]any{"domain_years": 1, "epp_code_enc": "enc:secret"})
	item := domain.OrderItem{
		ID: 202, OrderID: 100, ItemType: domain.ItemDomainTransfer,
		Domain: "moving.com", Cycle: domain.CycleAnnually, UnitPrice: 120000,
		Options: opts,
	}
	activationEnv(e, []domain.OrderItem{item}, nil)
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.domCreated, 1)
	d := e.domCreated[0]
	assert.Equal(t, domain.DomainPendingTransfer, d.Status)
	assert.Equal(t, "enc:secret", d.EPPCodeEnc, "epp carried over encrypted")

	require.Len(t, e.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeDomainTransfer, e.enqueuer.Tasks[0].TaskType)
	assert.Equal(t, jobs.DomainTransferPayload{DomainID: d.ID, Years: 1}, e.enqueuer.Tasks[0].Payload)
}

func TestActivateOrderDomainRegisterWithAddonsAndFrozenRenewRate(t *testing.T) {
	e := newFixture()
	opts, _ := json.Marshal(map[string]any{
		"domain_years":          1,
		"domain_addons":         []string{"id_protection", "dns_management"},
		"domain_renew_per_year": 195000,
	})
	item := domain.OrderItem{
		ID: 203, OrderID: 100, ItemType: domain.ItemDomainRegister,
		Domain: "example.com", Cycle: domain.CycleAnnually, UnitPrice: 185000,
		Options: opts,
	}
	activationEnv(e, []domain.OrderItem{item}, nil)
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.domCreated, 1)
	d := e.domCreated[0]
	assert.Equal(t, int64(195000), d.RecurringAmount, "frozen renew rate used directly, not unit/years")
	assert.True(t, d.IDProtection)
	assert.True(t, d.DNSManagementEnabled)
	assert.False(t, d.EmailForwardingEnabled)
}

func TestActivateOrderIdempotent(t *testing.T) {
	e := newFixture()
	sid := int64(700)
	done := hostingItem(200, 10)
	done.ServiceID = &sid // already activated
	fresh := hostingItem(201, 10)
	activationEnv(e, []domain.OrderItem{done, fresh}, map[int64]*domain.Product{10: cpanelProduct()})
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	assert.Len(t, e.created, 1, "only the un-activated item creates a service")
	_, refDone := e.backrefs[200]
	assert.False(t, refDone, "activated item untouched")
	_, refFresh := e.backrefs[201]
	assert.True(t, refFresh)
}

func TestActivateOrderAlreadyActiveIsNoop(t *testing.T) {
	e := newFixture()
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderActive}, nil
	}
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	assert.Empty(t, e.created)
	assert.Empty(t, e.enqueuer.Tasks)
	assert.Empty(t, e.statusSet)
}

func TestActivateOrderTerminalStatusConflicts(t *testing.T) {
	for _, st := range []domain.OrderStatus{domain.OrderCancelled, domain.OrderFraud} {
		e := newFixture()
		e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
			return &domain.Order{ID: id, ClientID: 7, Status: st}, nil
		}
		svc := orders.New(e.deps)
		err := svc.ActivateOrder(context.Background(), 100)
		require.Error(t, err, "status %s", st)
		assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	}
}

func TestActivateOrderNotificationFailureDoesNotFail(t *testing.T) {
	e := newFixture()
	activationEnv(e, []domain.OrderItem{hostingItem(200, 10)},
		map[int64]*domain.Product{10: cpanelProduct()})
	e.notifier.SendTemplateFn = func(_ context.Context, _ int64, _ string, _ map[string]any) error {
		return apperr.Internal(assert.AnError)
	}
	svc := orders.New(e.deps)

	assert.NoError(t, svc.ActivateOrder(context.Background(), 100), "notification failure never undoes activation")
}
