package orders

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// ActivateOrder activates every item of a paid (or admin-accepted) order:
// product items become service rows (+ provision:create unless manual/none
// module), domain items become domains rows (+ domain:register|transfer).
// It is idempotent: items whose service_id/domain_id back-refs are already
// set are skipped, and an already-active order returns nil immediately.
// Implements ports.ServiceActivator (consumed by billing.ProcessPaid).
func (s *Service) ActivateOrder(ctx context.Context, orderID int64) error {
	order, err := s.getOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if order.Status == domain.OrderActive {
		return nil // already fully activated
	}
	if order.Status != domain.OrderPending {
		return apperr.Newf(apperr.CodeConflict, "order %s cannot be activated", order.Status)
	}

	client, err := s.d.Clients.GetByID(ctx, order.ClientID)
	if err != nil {
		return err
	}
	if client == nil {
		return apperr.NotFound("client")
	}
	items, err := s.d.Orders.GetItems(ctx, orderID)
	if err != nil {
		return err
	}

	var coupon *domain.Coupon
	if order.CouponID != nil {
		coupon, err = s.d.Coupons.GetByID(ctx, *order.CouponID)
		if err != nil && apperr.From(err).Code != apperr.CodeNotFound {
			return err
		}
	}

	now := s.d.Clock.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var manualAlerts []string
	err = s.d.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		for i := range items {
			item := &items[i]
			if item.ServiceID != nil || item.DomainID != nil {
				continue // already activated (idempotent re-run)
			}
			switch item.ItemType {
			case domain.ItemProduct:
				alert, err := s.activateProductItem(txCtx, order, item, coupon, today)
				if err != nil {
					return err
				}
				if alert != "" {
					manualAlerts = append(manualAlerts, alert)
				}
			case domain.ItemDomainRegister, domain.ItemDomainTransfer:
				if err := s.activateDomainItem(txCtx, order, item); err != nil {
					return err
				}
			}
		}
		newStatus, err := domain.TransitionOrder(order.Status, domain.OrderActive)
		if err != nil {
			return err
		}
		return s.d.Orders.UpdateStatus(txCtx, orderID, newStatus)
	})
	if err != nil {
		return err
	}

	// Best-effort post-commit notifications; failures never undo activation.
	for _, msg := range manualAlerts {
		_ = s.d.Notifier.AlertAdmin(ctx, "Manual provisioning required", msg)
	}
	_ = s.d.Notifier.SendTemplate(ctx, client.UserID, templateOrderActivated, map[string]any{
		"OrderNumber": order.OrderNumber,
		"Name":        client.FullName(),
	})
	return nil
}

// activateProductItem creates the services row for a product order item and
// enqueues provisioning. Returns a non-empty admin alert message when the
// product requires manual setup.
func (s *Service) activateProductItem(ctx context.Context, order *domain.Order, item *domain.OrderItem, coupon *domain.Coupon, today time.Time) (string, error) {
	if item.ProductID == nil {
		return "", apperr.Newf(apperr.CodeInternal, "order item %d has no product", item.ID)
	}
	product, err := s.d.Products.GetByID(ctx, *item.ProductID)
	if err != nil {
		return "", err
	}
	if product == nil {
		return "", apperr.NotFound("product")
	}

	recurring := item.UnitPrice
	var couponID *int64
	if coupon != nil && coupon.Recurring && couponAppliesToProduct(coupon, product.ID) {
		recurring = recurringWithCoupon(coupon, item.UnitPrice)
		id := coupon.ID
		couponID = &id
	}

	password, err := domain.GeneratePassword(16)
	if err != nil {
		return "", apperr.Internal(err)
	}
	passwordEnc, err := s.d.Encryptor.Encrypt(password)
	if err != nil {
		return "", apperr.Internal(err)
	}

	svc := &domain.Service{
		ClientID:        order.ClientID,
		OrderItemID:     &item.ID,
		ProductID:       product.ID,
		Domain:          item.Domain,
		Username:        domain.UsernameFromDomain(item.Domain),
		PasswordEnc:     passwordEnc,
		Status:          domain.ServicePending,
		BillingCycle:    item.Cycle,
		RecurringAmount: recurring,
		SetupFee:        item.SetupFee,
		CouponID:        couponID,
	}
	if item.Cycle != domain.CycleOneTime {
		due := domain.AddCycle(today, item.Cycle)
		svc.NextDueDate = &due
	}
	// Products without a provisioning module are active immediately.
	if product.Module == domain.ModuleNone {
		svc.Status = domain.ServiceActive
		reg := today
		svc.RegistrationDate = &reg
	}
	// Carry the chosen dynamic specs into panel_meta so provisioning can build
	// the per-service package without reaching back into the order.
	if product.Configurable {
		var opts itemOptions
		if len(item.Options) > 0 {
			_ = json.Unmarshal(item.Options, &opts)
		}
		if len(opts.Specs) > 0 {
			meta, err := json.Marshal(map[string]any{domain.PanelMetaChosenSpecs: opts.Specs})
			if err != nil {
				return "", apperr.Internal(err)
			}
			svc.PanelMeta = meta
		}
	}

	if err := s.d.Services.Create(ctx, svc); err != nil {
		return "", err
	}
	if err := s.d.Orders.UpdateItemBackRefs(ctx, item.ID, &svc.ID, nil); err != nil {
		return "", err
	}
	item.ServiceID = &svc.ID

	if product.Module == domain.ModuleNone {
		return "", nil
	}
	if product.AutoSetup == domain.SetupManual {
		return "Order " + order.OrderNumber + ": service for product '" + product.Name +
			"' (" + item.Domain + ") requires manual setup", nil
	}
	err = s.d.Enqueuer.Enqueue(ctx, jobs.TypeProvisionCreate,
		jobs.ProvisionCreatePayload{ServiceID: svc.ID})
	if err != nil {
		return "", err
	}
	return "", nil
}

// activateDomainItem creates the domains row for a domain order item and
// enqueues the registrar job.
func (s *Service) activateDomainItem(ctx context.Context, order *domain.Order, item *domain.OrderItem) error {
	registrar, err := s.d.Registrars.GetByName(ctx, registrarName)
	if err != nil {
		return err
	}
	if registrar == nil {
		return apperr.NotFound("registrar")
	}

	var opts itemOptions
	if len(item.Options) > 0 {
		if err := json.Unmarshal(item.Options, &opts); err != nil {
			return apperr.Internal(err)
		}
	}
	years := opts.DomainYears
	if years == 0 {
		years = defaultDomainYears
	}

	var ns []string
	if err := s.d.Settings.GetJSON(ctx, settingDefaultNameservers, &ns); err != nil {
		return apperr.Internal(err)
	}
	nsJSON, err := json.Marshal(ns)
	if err != nil {
		return apperr.Internal(err)
	}

	status := domain.DomainPending
	if item.ItemType == domain.ItemDomainTransfer {
		status = domain.DomainPendingTransfer
	}
	// The renewal rate is frozen at checkout time (planDomain) from the
	// TLD/premium renew price plus addon costs; fall back to the old
	// total/years approximation only when no admin pricing was configured.
	perYear := opts.DomainRenewPerYear
	if perYear == 0 {
		perYear = item.UnitPrice
		if years > 0 {
			perYear = item.UnitPrice / int64(years)
		}
	}
	d := &domain.Domain{
		ClientID:               order.ClientID,
		RegistrarID:            registrar.ID,
		Name:                   item.Domain,
		Status:                 status,
		RecurringAmount:        perYear,
		BillingCycle:           domain.CycleAnnually,
		AutoRenew:              true,
		Nameservers:            nsJSON,
		EPPCodeEnc:             opts.EPPCodeEnc,
		IDProtection:           containsAddon(opts.DomainAddons, string(domain.DomainAddonIDProtection)),
		DNSManagementEnabled:   containsAddon(opts.DomainAddons, string(domain.DomainAddonDNSManagement)),
		EmailForwardingEnabled: containsAddon(opts.DomainAddons, string(domain.DomainAddonEmailForwarding)),
	}
	if err := s.d.Domains.Create(ctx, d); err != nil {
		return err
	}
	if err := s.d.Orders.UpdateItemBackRefs(ctx, item.ID, nil, &d.ID); err != nil {
		return err
	}
	item.DomainID = &d.ID

	if item.ItemType == domain.ItemDomainTransfer {
		// The transfer job reads the EPP code from domains.epp_code_enc.
		return s.d.Enqueuer.Enqueue(ctx, jobs.TypeDomainTransfer,
			jobs.DomainTransferPayload{DomainID: d.ID, Years: years})
	}
	return s.d.Enqueuer.Enqueue(ctx, jobs.TypeDomainRegister,
		jobs.DomainRegisterPayload{DomainID: d.ID, Years: years})
}

// containsAddon reports whether key is present in keys.
func containsAddon(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
