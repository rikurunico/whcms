package domains_test

// Failure paths across the domains service and its background jobs:
// ownership checks, registrar and store errors, and registrar status
// mapping on sync.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/modules/domains"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateNameserversStoreFailure(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))
	d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error { return errBoom }
	_, err := d.svc().UpdateNameservers(context.Background(), 5, 10,
		[]string{"ns1.a.com", "ns2.a.com"})
	assert.Error(t, err)
}

func TestUpdateNameserversWrongOwner(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))
	_, err := d.svc().UpdateNameservers(context.Background(), 99, 10,
		[]string{"ns1.a.com", "ns2.a.com"})
	assertCode(t, err, apperr.CodeNotFound)
}

func TestDNSOwnershipAndErrors(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))

	_, err := d.svc().GetDNS(context.Background(), 99, 10)
	assertCode(t, err, apperr.CodeNotFound)

	_, err = d.svc().UpdateDNS(context.Background(), 99, 10, nil)
	assertCode(t, err, apperr.CodeNotFound)

	d.registrar.GetDNSRecordsFn = func(ctx context.Context, name string) ([]ports.DNSRecord, error) {
		return nil, apperr.External("rdash", errBoom)
	}
	_, err = d.svc().GetDNS(context.Background(), 5, 10)
	assertCode(t, err, apperr.CodeExternal)

	stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainPending }))
	_, err = d.svc().UpdateDNS(context.Background(), 5, 10, nil)
	assertCode(t, err, apperr.CodeConflict)
}

func TestGetEPPRegistrarError(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))
	d.registrar.GetEPPCodeFn = func(ctx context.Context, name string) (string, error) {
		return "", apperr.External("rdash", errBoom)
	}
	_, err := d.svc().GetEPP(context.Background(), 5, 10)
	assertCode(t, err, apperr.CodeExternal)

	_, err = d.svc().GetEPP(context.Background(), 99, 10)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestRenewNowSettingsError(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))
	d.settings.GetIntFn = func(ctx context.Context, key string, def int) (int, error) {
		return def, errBoom
	}
	_, err := d.svc().RenewNow(context.Background(), 5, 10)
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateDomainErrors(t *testing.T) {
	t.Run("wrong owner", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		on := true
		_, err := d.svc().UpdateDomain(context.Background(), 99, 10,
			domains.UpdateDomainRequest{AutoRenew: &on})
		assertCode(t, err, apperr.CodeNotFound)
	})

	t.Run("registrar contact failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.UpdateContactFn = func(ctx context.Context, name string, c ports.RegistrantContact) error {
			return apperr.External("rdash", errBoom)
		}
		_, err := d.svc().UpdateDomain(context.Background(), 5, 10, domains.UpdateDomainRequest{
			Contact: &domains.ContactInput{FirstName: "A", LastName: "B", Email: "a@b.co", Country: "ID"},
		})
		assertCode(t, err, apperr.CodeExternal)
	})

	t.Run("store failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error { return errBoom }
		on := true
		_, err := d.svc().UpdateDomain(context.Background(), 5, 10,
			domains.UpdateDomainRequest{AutoRenew: &on})
		assert.Error(t, err)
	})
}

func TestRenewDomainAfterPaymentYearsDefault(t *testing.T) {
	// Non-annual cycles (monthly, one_time) still renew 1 year at the registrar.
	for _, cycle := range []domain.BillingCycle{domain.CycleMonthly, domain.CycleOneTime} {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.BillingCycle = cycle }))
		require.NoError(t, d.svc().RenewDomainAfterPayment(context.Background(), 10))
		payload := d.enqueuer.Tasks[0].Payload.(jobs.DomainRenewPayload)
		assert.Equal(t, 1, payload.Years, string(cycle))
	}
}

func TestRegisterDomainJobErrorBranches(t *testing.T) {
	t.Run("domain not found", func(t *testing.T) {
		d := newFixture()
		stubGet(d, nil)
		assertCode(t, d.svc().RegisterDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("client lookup failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainPending }))
		d.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		assertCode(t, d.svc().RegisterDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("user lookup failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainPending }))
		d.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
			return &domain.Client{ID: id, UserID: 77}, nil
		}
		d.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
			return nil, apperr.NotFound("user")
		}
		assertCode(t, d.svc().RegisterDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("malformed stored nameservers", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainPending
			x.Nameservers = json.RawMessage(`{not json`)
		}))
		stubClientUser(d)
		assertCode(t, d.svc().RegisterDomainJob(context.Background(), 10), apperr.CodeInternal)
	})

	t.Run("registrar row lookup failure on ns fallback", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainPending
			x.Nameservers = nil
		}))
		stubClientUser(d)
		d.registrars.GetByIDFn = func(ctx context.Context, id int64) (*domain.Registrar, error) {
			return nil, apperr.NotFound("registrar")
		}
		assertCode(t, d.svc().RegisterDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("update failure propagates", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainPending }))
		stubClientUser(d)
		d.registrar.RegisterFn = func(ctx context.Context, r ports.RegisterDomainRequest) (*ports.DomainResult, error) {
			return &ports.DomainResult{ExpiryDate: fixedNow.AddDate(1, 0, 0)}, nil
		}
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error { return errBoom }
		assert.Error(t, d.svc().RegisterDomainJob(context.Background(), 10))
	})
}

func TestTransferDomainJobDecryptFailure(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(func(x *domain.Domain) {
		x.Status = domain.DomainPendingTransfer
		x.EPPCodeEnc = "enc:bad"
	}))
	d.encryptor.DecryptFn = func(ct string) (string, error) { return "", errBoom }
	assertCode(t, d.svc().TransferDomainJob(context.Background(), 10), apperr.CodeInternal)

	t.Run("not found", func(t *testing.T) {
		d := newFixture()
		stubGet(d, nil)
		assertCode(t, d.svc().TransferDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("contact lookup failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainPendingTransfer
			x.EPPCodeEnc = "enc:x"
		}))
		d.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		assertCode(t, d.svc().TransferDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("malformed nameservers", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainPendingTransfer
			x.EPPCodeEnc = "enc:x"
			x.Nameservers = json.RawMessage(`{bad`)
		}))
		stubClientUser(d)
		assertCode(t, d.svc().TransferDomainJob(context.Background(), 10), apperr.CodeInternal)
	})

	t.Run("update failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainPendingTransfer
			x.EPPCodeEnc = "enc:x"
		}))
		stubClientUser(d)
		d.registrar.TransferFn = func(ctx context.Context, r ports.TransferDomainRequest) (*ports.DomainResult, error) {
			return &ports.DomainResult{ExpiryDate: fixedNow.AddDate(1, 0, 0)}, nil
		}
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error { return errBoom }
		assert.Error(t, d.svc().TransferDomainJob(context.Background(), 10))
	})
}

func TestRenewDomainJobBranches(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		d := newFixture()
		stubGet(d, nil)
		assertCode(t, d.svc().RenewDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("update failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.RenewFn = func(ctx context.Context, name string, years int) (*ports.DomainResult, error) {
			return &ports.DomainResult{ExpiryDate: fixedNow.AddDate(1, 0, 0)}, nil
		}
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error { return errBoom }
		assert.Error(t, d.svc().RenewDomainJob(context.Background(), 10))
	})

	t.Run("notification skipped when client lookup fails", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.RenewFn = func(ctx context.Context, name string, years int) (*ports.DomainResult, error) {
			return &ports.DomainResult{ExpiryDate: fixedNow.AddDate(1, 0, 0)}, nil
		}
		d.clients.GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		sent := false
		d.notifier.SendTemplateFn = func(ctx context.Context, userID int64, key string, data map[string]any) error {
			sent = true
			return nil
		}
		require.NoError(t, d.svc().RenewDomainJob(context.Background(), 10))
		assert.False(t, sent)
	})

	t.Run("zero result expiry with past stored expiry uses now", func(t *testing.T) {
		d := newFixture()
		past := fixedNow.AddDate(-1, 0, 0)
		stubGet(d, testDomain(func(x *domain.Domain) {
			x.Status = domain.DomainExpired
			x.ExpiryDate = &past
		}))
		stubClientUser(d)
		d.registrar.RenewFn = func(ctx context.Context, name string, years int) (*ports.DomainResult, error) {
			return &ports.DomainResult{}, nil
		}
		var stored *domain.Domain
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error {
			cp := *dm
			stored = &cp
			return nil
		}
		require.NoError(t, d.svc().RenewDomainJob(context.Background(), 10))
		assert.Equal(t, fixedNow.AddDate(1, 0, 0), *stored.ExpiryDate)
		assert.Equal(t, domain.DomainActive, stored.Status)
	})
}

func TestSyncDomainJobBranches(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		d := newFixture()
		stubGet(d, nil)
		assertCode(t, d.svc().SyncDomainJob(context.Background(), 10), apperr.CodeNotFound)
	})

	t.Run("unknown registrar status ignored", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.SyncDomainFn = func(ctx context.Context, name string) (*ports.DomainSyncInfo, error) {
			return &ports.DomainSyncInfo{Status: "weird-status"}, nil
		}
		var stored *domain.Domain
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error {
			cp := *dm
			stored = &cp
			return nil
		}
		require.NoError(t, d.svc().SyncDomainJob(context.Background(), 10))
		assert.Equal(t, domain.DomainActive, stored.Status)
	})

	t.Run("status alias mapping", func(t *testing.T) {
		// "Transfer In Progress" (spaces, mixed case) -> pending_transfer;
		// applied only when the transition is legal.
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainPendingTransfer }))
		d.registrar.SyncDomainFn = func(ctx context.Context, name string) (*ports.DomainSyncInfo, error) {
			return &ports.DomainSyncInfo{Status: "OK"}, nil // alias for active
		}
		var stored *domain.Domain
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error {
			cp := *dm
			stored = &cp
			return nil
		}
		require.NoError(t, d.svc().SyncDomainJob(context.Background(), 10))
		assert.Equal(t, domain.DomainActive, stored.Status)
	})

	t.Run("expired to active on renewal seen at registrar", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(func(x *domain.Domain) { x.Status = domain.DomainExpired }))
		d.registrar.SyncDomainFn = func(ctx context.Context, name string) (*ports.DomainSyncInfo, error) {
			return &ports.DomainSyncInfo{Status: "Registered"}, nil
		}
		var stored *domain.Domain
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error {
			cp := *dm
			stored = &cp
			return nil
		}
		require.NoError(t, d.svc().SyncDomainJob(context.Background(), 10))
		assert.Equal(t, domain.DomainActive, stored.Status)
	})

	t.Run("cancelled alias never applied from active", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.SyncDomainFn = func(ctx context.Context, name string) (*ports.DomainSyncInfo, error) {
			return &ports.DomainSyncInfo{Status: "Deleted"}, nil
		}
		var stored *domain.Domain
		d.domains.UpdateFn = func(ctx context.Context, dm *domain.Domain) error {
			cp := *dm
			stored = &cp
			return nil
		}
		require.NoError(t, d.svc().SyncDomainJob(context.Background(), 10))
		assert.Equal(t, domain.DomainActive, stored.Status)
	})
}

func TestAdminSyncErrors(t *testing.T) {
	t.Run("sync failure", func(t *testing.T) {
		d := newFixture()
		stubGet(d, testDomain(nil))
		d.registrar.SyncDomainFn = func(ctx context.Context, name string) (*ports.DomainSyncInfo, error) {
			return nil, apperr.External("rdash", errBoom)
		}
		_, err := d.svc().AdminSync(context.Background(), 1, 10)
		assertCode(t, err, apperr.CodeExternal)
		assert.Empty(t, d.audit.Entries)
	})

	t.Run("not found", func(t *testing.T) {
		d := newFixture()
		stubGet(d, nil)
		_, err := d.svc().AdminSync(context.Background(), 1, 10)
		assertCode(t, err, apperr.CodeNotFound)
	})
}

func TestAdminForceRenewEnqueueFailure(t *testing.T) {
	d := newFixture()
	stubGet(d, testDomain(nil))
	d.enqueuer.EnqueueFn = func(ctx context.Context, taskType string, payload any, opts ...ports.JobOption) error {
		return errBoom
	}
	assert.Error(t, d.svc().AdminForceRenew(context.Background(), 1, 10))
	assert.Empty(t, d.audit.Entries)
}

func TestUpdateRegistrarErrors(t *testing.T) {
	on := true

	t.Run("not found", func(t *testing.T) {
		d := newFixture()
		d.registrars.GetByIDFn = func(ctx context.Context, id int64) (*domain.Registrar, error) {
			return nil, apperr.NotFound("registrar")
		}
		_, err := d.svc().UpdateRegistrar(context.Background(), 1, 9,
			domains.UpdateRegistrarRequest{Active: &on})
		assertCode(t, err, apperr.CodeNotFound)
	})

	t.Run("update failure", func(t *testing.T) {
		d := newFixture()
		d.registrars.GetByIDFn = func(ctx context.Context, id int64) (*domain.Registrar, error) {
			return &domain.Registrar{ID: id, Name: "rdash"}, nil
		}
		d.registrars.UpdateFn = func(ctx context.Context, r *domain.Registrar) error { return errBoom }
		_, err := d.svc().UpdateRegistrar(context.Background(), 1, 1,
			domains.UpdateRegistrarRequest{Active: &on})
		assert.Error(t, err)
		assert.Empty(t, d.audit.Entries)
	})
}
