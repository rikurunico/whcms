package clients_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/clients"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Local store fakes (function-field pattern, nil-safe)

type fakeContactStore struct {
	CreateContactFn func(ctx context.Context, ct *clients.Contact) error
	GetContactFn    func(ctx context.Context, clientID, id int64) (*clients.Contact, error)
	ListContactsFn  func(ctx context.Context, clientID int64) ([]clients.Contact, error)
	UpdateContactFn func(ctx context.Context, ct *clients.Contact) error
	DeleteContactFn func(ctx context.Context, clientID, id int64) error
}

func (f *fakeContactStore) CreateContact(ctx context.Context, ct *clients.Contact) error {
	if f.CreateContactFn != nil {
		return f.CreateContactFn(ctx, ct)
	}
	return nil
}

func (f *fakeContactStore) GetContact(ctx context.Context, clientID, id int64) (*clients.Contact, error) {
	if f.GetContactFn != nil {
		return f.GetContactFn(ctx, clientID, id)
	}
	return &clients.Contact{ClientContact: domain.ClientContact{ID: id, ClientID: clientID}}, nil
}

func (f *fakeContactStore) ListContacts(ctx context.Context, clientID int64) ([]clients.Contact, error) {
	if f.ListContactsFn != nil {
		return f.ListContactsFn(ctx, clientID)
	}
	return nil, nil
}

func (f *fakeContactStore) UpdateContact(ctx context.Context, ct *clients.Contact) error {
	if f.UpdateContactFn != nil {
		return f.UpdateContactFn(ctx, ct)
	}
	return nil
}

func (f *fakeContactStore) DeleteContact(ctx context.Context, clientID, id int64) error {
	if f.DeleteContactFn != nil {
		return f.DeleteContactFn(ctx, clientID, id)
	}
	return nil
}

type fakeSearchStore struct {
	SearchClientsFn func(ctx context.Context, f clients.SearchInput) ([]clients.ClientListRow, int64, error)
}

func (f *fakeSearchStore) SearchClients(ctx context.Context, in clients.SearchInput) ([]clients.ClientListRow, int64, error) {
	if f.SearchClientsFn != nil {
		return f.SearchClientsFn(ctx, in)
	}
	return nil, 0, nil
}

type fakeAggregateStore struct {
	CountsFn func(ctx context.Context, clientID int64) (*clients.ClientCounts, error)
	RecentFn func(ctx context.Context, clientID int64, limit int) (*clients.ClientRecent, error)
}

func (f *fakeAggregateStore) Counts(ctx context.Context, clientID int64) (*clients.ClientCounts, error) {
	if f.CountsFn != nil {
		return f.CountsFn(ctx, clientID)
	}
	return &clients.ClientCounts{}, nil
}

func (f *fakeAggregateStore) Recent(ctx context.Context, clientID int64, limit int) (*clients.ClientRecent, error) {
	if f.RecentFn != nil {
		return f.RecentFn(ctx, clientID, limit)
	}
	return &clients.ClientRecent{}, nil
}

type fakeExportStore struct {
	rows []clients.ExportRow
	err  error
}

func (f *fakeExportStore) ForEachExportRow(ctx context.Context, fn func(clients.ExportRow) error) error {
	if f.err != nil {
		return f.err
	}
	for _, r := range f.rows {
		if err := fn(r); err != nil {
			return err
		}
	}
	return nil
}

// Test harness

type env struct {
	clientsRepo *mocks.MockClientRepo
	credits     *mocks.MockCreditRepo
	users       *mocks.MockUserRepo
	contacts    *fakeContactStore
	search      *fakeSearchStore
	aggregate   *fakeAggregateStore
	export      *fakeExportStore
	invoices    *mocks.MockInvoiceCreator
	settings    *mocks.MockSettingsRepo
	hasher      *mocks.MockPasswordHasher
	audit       *mocks.MockAuditLogger
	clock       *mocks.MockClock
	svc         *clients.Service
}

var fixedNow = time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

func newFixture() *env {
	e := &env{
		clientsRepo: &mocks.MockClientRepo{},
		credits:     &mocks.MockCreditRepo{},
		users:       &mocks.MockUserRepo{},
		contacts:    &fakeContactStore{},
		search:      &fakeSearchStore{},
		aggregate:   &fakeAggregateStore{},
		export:      &fakeExportStore{},
		invoices:    &mocks.MockInvoiceCreator{},
		settings:    &mocks.MockSettingsRepo{},
		hasher:      &mocks.MockPasswordHasher{},
		audit:       &mocks.MockAuditLogger{},
		clock:       &mocks.MockClock{FixedTime: fixedNow},
	}
	e.svc = clients.New(clients.Deps{
		Tx:        &mocks.MockTxManager{},
		Clients:   e.clientsRepo,
		Credits:   e.credits,
		Users:     e.users,
		Contacts:  e.contacts,
		Search:    e.search,
		Aggregate: e.aggregate,
		Export:    e.export,
		Invoices:  e.invoices,
		Settings:  e.settings,
		Hasher:    e.hasher,
		Audit:     e.audit,
		Clock:     e.clock,
	})
	return e
}

func testClient(id int64) *domain.Client {
	return &domain.Client{
		ID: id, UserID: id + 100, FirstName: "Budi", LastName: "Santoso",
		Country: "ID", Currency: "IDR", CreditBalance: 50000,
		Status: domain.ClientActive,
	}
}

func codeOf(t *testing.T, err error) apperr.Code {
	t.Helper()
	require.Error(t, err)
	return apperr.From(err).Code
}

// Search

func TestSearchClients(t *testing.T) {
	t.Run("invalid status filter", func(t *testing.T) {
		e := newFixture()
		_, _, err := e.svc.SearchClients(context.Background(), clients.SearchInput{Status: "bogus"})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("passes filters through and returns rows", func(t *testing.T) {
		e := newFixture()
		var got clients.SearchInput
		e.search.SearchClientsFn = func(_ context.Context, f clients.SearchInput) ([]clients.ClientListRow, int64, error) {
			got = f
			return []clients.ClientListRow{{ID: 1, Email: "a@b.c"}}, 1, nil
		}
		rows, total, err := e.svc.SearchClients(context.Background(), clients.SearchInput{
			Search: "budi", Status: "active", HasProduct: true, Page: 2, PerPage: 10,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, rows, 1)
		assert.Equal(t, "budi", got.Search)
		assert.True(t, got.HasProduct)
	})

	t.Run("repo error is internal", func(t *testing.T) {
		e := newFixture()
		e.search.SearchClientsFn = func(context.Context, clients.SearchInput) ([]clients.ClientListRow, int64, error) {
			return nil, 0, errors.New("boom")
		}
		_, _, err := e.svc.SearchClients(context.Background(), clients.SearchInput{})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})
}

// Create client

func validCreateInput() clients.CreateClientInput {
	return clients.CreateClientInput{
		Email: "Budi@Example.com", Password: "supersecret",
		FirstName: "Budi", LastName: "Santoso", Country: "id",
	}
}

func TestCreateClient(t *testing.T) {
	t.Run("validation failure", func(t *testing.T) {
		e := newFixture()
		in := validCreateInput()
		in.Email = "not-an-email"
		_, err := e.svc.CreateClient(context.Background(), 1, in)
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("duplicate email conflict", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(_ context.Context, email string) (*domain.User, error) {
			assert.Equal(t, "budi@example.com", email) // normalized
			return &domain.User{ID: 9, Email: email}, nil
		}
		_, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		assert.Equal(t, apperr.CodeConflict, codeOf(t, err))
	})

	t.Run("email lookup internal error", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
			return nil, errors.New("db down")
		}
		_, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("hash error is internal", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
			return nil, apperr.NotFound("user")
		}
		e.hasher.HashFn = func(string) (string, error) { return "", errors.New("argon2 fail") }
		_, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("creates user role client plus profile in tx and audits", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
			return nil, apperr.NotFound("user")
		}
		e.hasher.HashFn = func(pw string) (string, error) {
			assert.Equal(t, "supersecret", pw)
			return "phc-hash", nil
		}
		var createdUser *domain.User
		e.users.CreateFn = func(_ context.Context, u *domain.User) error {
			u.ID = 42
			createdUser = u
			return nil
		}
		var createdClient *domain.Client
		e.clientsRepo.CreateFn = func(_ context.Context, c *domain.Client) error {
			c.ID = 7
			createdClient = c
			return nil
		}

		out, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		require.NoError(t, err)
		require.NotNil(t, createdUser)
		assert.Equal(t, domain.RoleClient, createdUser.Role)
		assert.Equal(t, "phc-hash", createdUser.PasswordHash)
		assert.Equal(t, "budi@example.com", createdUser.Email)
		require.NotNil(t, createdClient)
		assert.Equal(t, int64(42), createdClient.UserID)
		assert.Equal(t, "ID", createdClient.Country) // uppercased
		assert.Equal(t, int64(7), out.ID)
		assert.Equal(t, "budi@example.com", out.Email)
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.create", e.audit.Entries[0].Action)
		assert.Equal(t, int64(7), e.audit.Entries[0].EntityID)
	})

	t.Run("user create error propagates", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
			return nil, apperr.NotFound("user")
		}
		e.users.CreateFn = func(context.Context, *domain.User) error {
			return apperr.Conflict("email already in use")
		}
		_, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		assert.Equal(t, apperr.CodeConflict, codeOf(t, err))
	})

	t.Run("plain tx error wraps as internal", func(t *testing.T) {
		e := newFixture()
		e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
			return nil, apperr.NotFound("user")
		}
		e.clientsRepo.CreateFn = func(context.Context, *domain.Client) error {
			return errors.New("connection reset") // not an *apperr.Error
		}
		_, err := e.svc.CreateClient(context.Background(), 1, validCreateInput())
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})
}

// Detail aggregate

func TestGetClientDetail(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.GetClientDetail(context.Background(), 999)
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("aggregates profile, email, counts and recents", func(t *testing.T) {
		e := newFixture()
		last := fixedNow.Add(-time.Hour)
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.users.GetByIDFn = func(_ context.Context, id int64) (*domain.User, error) {
			assert.Equal(t, int64(105), id)
			return &domain.User{ID: id, Email: "budi@example.com", LastLoginAt: &last}, nil
		}
		e.aggregate.CountsFn = func(_ context.Context, clientID int64) (*clients.ClientCounts, error) {
			return &clients.ClientCounts{Services: 3, InvoicesUnpaid: 2, UnpaidTotal: 150000}, nil
		}
		e.aggregate.RecentFn = func(_ context.Context, clientID int64, limit int) (*clients.ClientRecent, error) {
			assert.Equal(t, 5, limit)
			return &clients.ClientRecent{Invoices: []clients.InvoiceSummary{{ID: 11}}}, nil
		}

		detail, err := e.svc.GetClientDetail(context.Background(), 5)
		require.NoError(t, err)
		assert.Equal(t, "budi@example.com", detail.Email)
		assert.Equal(t, &last, detail.LastLoginAt)
		assert.Equal(t, int64(3), detail.Counts.Services)
		require.Len(t, detail.Recent.Invoices, 1)
	})

	t.Run("counts error is internal", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.users.GetByIDFn = func(_ context.Context, id int64) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		}
		e.aggregate.CountsFn = func(context.Context, int64) (*clients.ClientCounts, error) {
			return nil, errors.New("boom")
		}
		_, err := e.svc.GetClientDetail(context.Background(), 5)
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})
}

// Update / status / delete

func strPtr(s string) *string { return &s }

func TestUpdateClient(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.UpdateClient(context.Background(), 1, 9, clients.UpdateClientInput{})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("patches fields, notes and audits", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		var updated *domain.Client
		e.clientsRepo.UpdateFn = func(_ context.Context, c *domain.Client) error {
			updated = c
			return nil
		}
		out, err := e.svc.UpdateClient(context.Background(), 1, 5, clients.UpdateClientInput{
			City:       strPtr("Lamongan"),
			Country:    strPtr("sg"),
			NotesAdmin: strPtr("vip"),
		})
		require.NoError(t, err)
		require.NotNil(t, updated)
		assert.Equal(t, "Lamongan", out.City)
		assert.Equal(t, "SG", out.Country) // uppercased
		assert.Equal(t, "vip", out.NotesAdmin)
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.update", e.audit.Entries[0].Action)
	})

	t.Run("no-op patch skips update and audit", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		called := false
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error {
			called = true
			return nil
		}
		_, err := e.svc.UpdateClient(context.Background(), 1, 5, clients.UpdateClientInput{
			FirstName: strPtr("Budi"), // unchanged value
		})
		require.NoError(t, err)
		assert.False(t, called)
		assert.Empty(t, e.audit.Entries)
	})

	t.Run("validation failure", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.UpdateClient(context.Background(), 1, 5, clients.UpdateClientInput{
			Country: strPtr("IDN"), // must be 2 chars
		})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("persist error propagates", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error {
			return errors.New("boom")
		}
		_, err := e.svc.UpdateClient(context.Background(), 1, 5, clients.UpdateClientInput{
			City: strPtr("Lamongan"),
		})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	// TestUpdateClient/blank_state_rejected mirrors TestProfile's coverage
	// (same blankRequiredProfileFields guard) for the admin-side edit path -
	// an admin must not be able to blank a client's province either, same as
	// the client themselves.
	t.Run("blank state rejected", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.UpdateClient(context.Background(), 1, 5, clients.UpdateClientInput{
			State: strPtr(""),
		})
		require.Error(t, err)
		ae := apperr.From(err)
		assert.Equal(t, apperr.CodeValidation, ae.Code)
		require.Len(t, ae.Details, 1)
		assert.Equal(t, "state", ae.Details[0].Field)
	})
}

func TestSetStatus(t *testing.T) {
	t.Run("invalid status", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.SetStatus(context.Background(), 1, 5, clients.SetStatusInput{Status: "banned"})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.SetStatus(context.Background(), 1, 9, clients.SetStatusInput{Status: "closed"})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("idempotent when unchanged", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		called := false
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error {
			called = true
			return nil
		}
		out, err := e.svc.SetStatus(context.Background(), 1, 5, clients.SetStatusInput{Status: "active"})
		require.NoError(t, err)
		assert.Equal(t, domain.ClientActive, out.Status)
		assert.False(t, called)
	})

	t.Run("transition persists and audits", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		out, err := e.svc.SetStatus(context.Background(), 1, 5, clients.SetStatusInput{Status: "closed"})
		require.NoError(t, err)
		assert.Equal(t, domain.ClientClosed, out.Status)
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.status", e.audit.Entries[0].Action)
	})
}

func TestDeleteClient(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		err := e.svc.DeleteClient(context.Background(), 1, 9)
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("soft deletes and audits", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		deleted := int64(0)
		e.clientsRepo.SoftDeleteFn = func(_ context.Context, id int64) error {
			deleted = id
			return nil
		}
		require.NoError(t, e.svc.DeleteClient(context.Background(), 1, 5))
		assert.Equal(t, int64(5), deleted)
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.delete", e.audit.Entries[0].Action)
	})
}

// Self-service profile

func TestProfile(t *testing.T) {
	t.Run("get own profile", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			assert.Equal(t, int64(5), id)
			return testClient(id), nil
		}
		c, err := e.svc.GetProfile(context.Background(), 5)
		require.NoError(t, err)
		assert.Equal(t, int64(5), c.ID)
	})

	t.Run("update patches address fields", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		out, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{
			Address1: strPtr("Jl. Merdeka 1"), Phone: strPtr("0812000111"),
		})
		require.NoError(t, err)
		assert.Equal(t, "Jl. Merdeka 1", out.Address1)
		assert.Equal(t, "0812000111", out.Phone)
	})

	t.Run("update validation failure", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{
			Country: strPtr("XYZ"),
		})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	// TestUpdateProfileBlankRequiredField covers a real bug found 2026-07-27:
	// address1/city/state/postcode were fully optional on the client
	// profile, matching the frontend account page's own (Optional) label on
	// State - but domain registration requires all four
	// (domains.Service.resolveRegistrantContact) and rejected a client whose
	// profile had them blank, with no way for the client to know why until
	// they tried to register a domain. Blank (not just omitted) is now
	// rejected up front.
	for _, field := range []string{"address1", "city", "state", "postcode"} {
		t.Run("blank "+field+" rejected", func(t *testing.T) {
			e := newFixture()
			e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
				return testClient(id), nil
			}
			in := clients.UpdateProfileInput{}
			blank := "  "
			switch field {
			case "address1":
				in.Address1 = &blank
			case "city":
				in.City = &blank
			case "state":
				in.State = &blank
			case "postcode":
				in.Postcode = &blank
			}
			_, err := e.svc.UpdateProfile(context.Background(), 5, in)
			require.Error(t, err)
			ae := apperr.From(err)
			assert.Equal(t, apperr.CodeValidation, ae.Code)
			require.Len(t, ae.Details, 1)
			assert.Equal(t, field, ae.Details[0].Field)
		})
	}

	t.Run("omitting address fields entirely does not require them", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		out, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{
			Phone: strPtr("0812999888"),
		})
		require.NoError(t, err)
		assert.Equal(t, "0812999888", out.Phone)
	})
}

// Contacts

func TestContacts(t *testing.T) {
	t.Run("list requires existing client", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.ListContacts(context.Background(), 9)
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("create validates and normalizes email", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		var created *clients.Contact
		e.contacts.CreateContactFn = func(_ context.Context, ct *clients.Contact) error {
			ct.ID = 3
			created = ct
			return nil
		}
		ct, err := e.svc.CreateContact(context.Background(), 1, 5, clients.ContactInput{
			FirstName: "Siti", Email: "SITI@Example.com",
		})
		require.NoError(t, err)
		assert.Equal(t, "siti@example.com", created.Email)
		assert.Equal(t, int64(5), ct.ClientID)
		assert.JSONEq(t, `{}`, string(ct.Permissions))
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.contact_create", e.audit.Entries[0].Action)
	})

	t.Run("create persists permissions", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		var created *clients.Contact
		e.contacts.CreateContactFn = func(_ context.Context, ct *clients.Contact) error {
			ct.ID = 3
			created = ct
			return nil
		}
		ct, err := e.svc.CreateContact(context.Background(), 1, 5, clients.ContactInput{
			FirstName: "Siti", Email: "siti@example.com",
			Permissions: map[string]bool{"invoices": true, "domains": true, "services": false},
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"invoices":true,"domains":true,"services":false}`, string(created.Permissions))
		assert.JSONEq(t, `{"invoices":true,"domains":true,"services":false}`, string(ct.Permissions))
	})

	t.Run("create rejects unknown permission key", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.CreateContact(context.Background(), 1, 5, clients.ContactInput{
			FirstName: "Siti", Permissions: map[string]bool{"billing": true},
		})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("create validation failure", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.CreateContact(context.Background(), 1, 5, clients.ContactInput{Email: "x@y.z"})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err)) // first_name required
	})

	t.Run("update other client's contact is not found", func(t *testing.T) {
		e := newFixture()
		e.contacts.GetContactFn = func(context.Context, int64, int64) (*clients.Contact, error) {
			return nil, apperr.NotFound("contact")
		}
		_, err := e.svc.UpdateContact(context.Background(), 1, 5, 77, clients.ContactInput{FirstName: "Siti"})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("update persists and audits", func(t *testing.T) {
		e := newFixture()
		e.contacts.GetContactFn = func(_ context.Context, clientID, id int64) (*clients.Contact, error) {
			return &clients.Contact{
				ClientContact: domain.ClientContact{ID: id, ClientID: clientID, FirstName: "Old"},
				Permissions:   json.RawMessage(`{"invoices":true}`),
			}, nil
		}
		ct, err := e.svc.UpdateContact(context.Background(), 1, 5, 77, clients.ContactInput{
			FirstName: "Siti", Phone: "0812",
			Permissions: map[string]bool{"tickets": true},
		})
		require.NoError(t, err)
		assert.Equal(t, "Siti", ct.FirstName)
		assert.JSONEq(t, `{"tickets":true}`, string(ct.Permissions))
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.contact_update", e.audit.Entries[0].Action)
	})

	t.Run("update rejects unknown permission key", func(t *testing.T) {
		e := newFixture()
		e.contacts.GetContactFn = func(_ context.Context, clientID, id int64) (*clients.Contact, error) {
			return &clients.Contact{ClientContact: domain.ClientContact{ID: id, ClientID: clientID}}, nil
		}
		_, err := e.svc.UpdateContact(context.Background(), 1, 5, 77, clients.ContactInput{
			FirstName: "Siti", Permissions: map[string]bool{"staff": true},
		})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("delete not found passthrough", func(t *testing.T) {
		e := newFixture()
		e.contacts.DeleteContactFn = func(context.Context, int64, int64) error {
			return apperr.NotFound("contact")
		}
		err := e.svc.DeleteContact(context.Background(), 1, 5, 77)
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("delete audits", func(t *testing.T) {
		e := newFixture()
		require.NoError(t, e.svc.DeleteContact(context.Background(), 1, 5, 77))
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.contact_delete", e.audit.Entries[0].Action)
	})
}

// Credit

func TestAddCredit(t *testing.T) {
	t.Run("rejects non-positive delta", func(t *testing.T) {
		e := newFixture()
		assert.Equal(t, apperr.CodeValidation,
			codeOf(t, e.svc.AddCredit(context.Background(), 5, 0, "x", 0)))
		assert.Equal(t, apperr.CodeValidation,
			codeOf(t, e.svc.AddCredit(context.Background(), 5, -100, "x", 0)))
	})

	t.Run("adjusts balance with ledger reference", func(t *testing.T) {
		e := newFixture()
		var gotDelta int64
		var gotRef *int64
		e.clientsRepo.AdjustCreditFn = func(_ context.Context, clientID, delta int64, reason string, ref *int64) (int64, error) {
			assert.Equal(t, int64(5), clientID)
			assert.Equal(t, "deposit paid", reason)
			gotDelta, gotRef = delta, ref
			return 150000, nil
		}
		require.NoError(t, e.svc.AddCredit(context.Background(), 5, 100000, "deposit paid", 33))
		assert.Equal(t, int64(100000), gotDelta)
		require.NotNil(t, gotRef)
		assert.Equal(t, int64(33), *gotRef)
	})

	t.Run("zero invoice id stores nil reference", func(t *testing.T) {
		e := newFixture()
		gotRef := new(int64)
		e.clientsRepo.AdjustCreditFn = func(_ context.Context, _, _ int64, _ string, ref *int64) (int64, error) {
			gotRef = ref
			return 1, nil
		}
		require.NoError(t, e.svc.AddCredit(context.Background(), 5, 1000, "manual", 0))
		assert.Nil(t, gotRef)
	})

	t.Run("client not found passthrough", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.AdjustCreditFn = func(context.Context, int64, int64, string, *int64) (int64, error) {
			return 0, apperr.NotFound("client")
		}
		assert.Equal(t, apperr.CodeNotFound,
			codeOf(t, e.svc.AddCredit(context.Background(), 9, 1000, "x", 0)))
	})
}

func TestDeductCredit(t *testing.T) {
	t.Run("rejects non-positive amount", func(t *testing.T) {
		e := newFixture()
		assert.Equal(t, apperr.CodeValidation,
			codeOf(t, e.svc.DeductCredit(context.Background(), 5, 0, "x", 0)))
	})

	t.Run("negates the amount", func(t *testing.T) {
		e := newFixture()
		var gotDelta int64
		e.clientsRepo.AdjustCreditFn = func(_ context.Context, _, delta int64, _ string, _ *int64) (int64, error) {
			gotDelta = delta
			return 0, nil
		}
		require.NoError(t, e.svc.DeductCredit(context.Background(), 5, 25000, "invoice paid with credit", 12))
		assert.Equal(t, int64(-25000), gotDelta)
	})

	t.Run("insufficient balance is conflict", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.AdjustCreditFn = func(context.Context, int64, int64, string, *int64) (int64, error) {
			return 0, apperr.Conflict("insufficient credit balance")
		}
		assert.Equal(t, apperr.CodeConflict,
			codeOf(t, e.svc.DeductCredit(context.Background(), 5, 999999, "x", 0)))
	})
}

func TestAdminAdjustCredit(t *testing.T) {
	t.Run("requires delta and reason", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.AdminAdjustCredit(context.Background(), 1, 5, clients.AdjustCreditInput{})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("negative delta deducts and audits", func(t *testing.T) {
		e := newFixture()
		var gotDelta int64
		e.clientsRepo.AdjustCreditFn = func(_ context.Context, _, delta int64, _ string, _ *int64) (int64, error) {
			gotDelta = delta
			return 20000, nil
		}
		balance, err := e.svc.AdminAdjustCredit(context.Background(), 1, 5, clients.AdjustCreditInput{
			Delta: -30000, Reason: "correction",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(-30000), gotDelta)
		assert.Equal(t, int64(20000), balance)
		require.Len(t, e.audit.Entries, 1)
		assert.Equal(t, "client.credit_adjust", e.audit.Entries[0].Action)
	})
}

func TestCredit(t *testing.T) {
	t.Run("returns balance plus ledger page", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.credits.ListByClientFn = func(_ context.Context, clientID int64, p ports.ListParams) ([]domain.CreditLedgerEntry, int64, error) {
			return []domain.CreditLedgerEntry{{ID: 1, ClientID: clientID, Delta: 50000}}, 1, nil
		}
		overview, total, err := e.svc.Credit(context.Background(), 5, ports.ListParams{Page: 1, PerPage: 25})
		require.NoError(t, err)
		assert.Equal(t, int64(50000), overview.Balance)
		assert.Equal(t, int64(1), total)
		require.Len(t, overview.Entries, 1)
	})

	t.Run("empty ledger yields empty slice", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		overview, _, err := e.svc.Credit(context.Background(), 5, ports.ListParams{})
		require.NoError(t, err)
		assert.NotNil(t, overview.Entries)
		assert.Empty(t, overview.Entries)
	})

	t.Run("missing client is not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, _, err := e.svc.Credit(context.Background(), 9, ports.ListParams{})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})
}

// Deposit

func TestDeposit(t *testing.T) {
	t.Run("rejects amount below minimum", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.Deposit(context.Background(), 5, clients.DepositInput{Amount: 9999})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("missing client is not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.Deposit(context.Background(), 9, clients.DepositInput{Amount: 50000})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("creates deposit invoice: untaxed item, due from settings", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.settings.GetIntFn = func(_ context.Context, key string, def int) (int, error) {
			assert.Equal(t, "billing.invoice_due_days", key)
			return 7, nil
		}
		var gotIn ports.CreateInvoiceInput
		e.invoices.CreateInvoiceFn = func(_ context.Context, in ports.CreateInvoiceInput) (*domain.Invoice, error) {
			gotIn = in
			return &domain.Invoice{ID: 88, ClientID: in.ClientID, Status: domain.InvoiceUnpaid, Total: 50000}, nil
		}

		inv, err := e.svc.Deposit(context.Background(), 5, clients.DepositInput{Amount: 50000})
		require.NoError(t, err)
		assert.Equal(t, int64(88), inv.ID)
		assert.Equal(t, int64(5), gotIn.ClientID)
		require.Len(t, gotIn.Items, 1)
		item := gotIn.Items[0]
		assert.Equal(t, int64(50000), item.Amount)
		assert.False(t, item.Taxed)
		assert.Equal(t, domain.RelatedDeposit, item.RelatedType)
		assert.Equal(t, fixedNow.AddDate(0, 0, 7), gotIn.DueDate)
	})

	t.Run("invoice creator error passthrough", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.invoices.CreateInvoiceFn = func(context.Context, ports.CreateInvoiceInput) (*domain.Invoice, error) {
			return nil, apperr.Conflict("counter busy")
		}
		_, err := e.svc.Deposit(context.Background(), 5, clients.DepositInput{Amount: 50000})
		assert.Equal(t, apperr.CodeConflict, codeOf(t, err))
	})
}

// CSV export

func TestExportCSV(t *testing.T) {
	t.Run("writes header and rows", func(t *testing.T) {
		e := newFixture()
		e.export.rows = []clients.ExportRow{{
			ID: 1, Email: "budi@example.com", FirstName: "Budi", LastName: "Santoso",
			Company: "PT Maju", City: "Lamongan", Country: "ID", Phone: "0812",
			Status: "active", CreditBalance: 50000,
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}}
		var buf bytes.Buffer
		require.NoError(t, e.svc.ExportCSV(context.Background(), &buf))
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		require.Len(t, lines, 2)
		assert.Equal(t, "id,email,first_name,last_name,company,city,country,phone,status,credit_balance,created_at", lines[0])
		assert.Equal(t, "1,budi@example.com,Budi,Santoso,PT Maju,Lamongan,ID,0812,active,50000,2026-01-02 03:04:05", lines[1])
	})

	t.Run("repo error is internal", func(t *testing.T) {
		e := newFixture()
		e.export.err = errors.New("boom")
		err := e.svc.ExportCSV(context.Background(), &bytes.Buffer{})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})
}

// Remaining error paths

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("pipe closed") }

func TestErrorPaths(t *testing.T) {
	t.Run("detail user lookup error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.users.GetByIDFn = func(context.Context, int64) (*domain.User, error) {
			return nil, errors.New("db down")
		}
		_, err := e.svc.GetClientDetail(context.Background(), 5)
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("detail recent error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.aggregate.RecentFn = func(context.Context, int64, int) (*clients.ClientRecent, error) {
			return nil, errors.New("boom")
		}
		_, err := e.svc.GetClientDetail(context.Background(), 5)
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("detail tolerates nil aggregates and nil user", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.users.GetByIDFn = func(context.Context, int64) (*domain.User, error) { return nil, nil }
		e.aggregate.CountsFn = func(context.Context, int64) (*clients.ClientCounts, error) { return nil, nil }
		e.aggregate.RecentFn = func(context.Context, int64, int) (*clients.ClientRecent, error) { return nil, nil }
		detail, err := e.svc.GetClientDetail(context.Background(), 5)
		require.NoError(t, err)
		assert.Empty(t, detail.Email)
		assert.Zero(t, detail.Counts.Services)
	})

	t.Run("set status update error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error { return errors.New("boom") }
		_, err := e.svc.SetStatus(context.Background(), 1, 5, clients.SetStatusInput{Status: "closed"})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("delete soft-delete error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.clientsRepo.SoftDeleteFn = func(context.Context, int64) error { return errors.New("boom") }
		assert.Equal(t, apperr.CodeInternal, codeOf(t, e.svc.DeleteClient(context.Background(), 1, 5)))
	})

	t.Run("get profile not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.GetProfile(context.Background(), 5)
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("update profile not found", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{City: strPtr("X")})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("update profile no-op skips persist", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		called := false
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error { called = true; return nil }
		_, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{})
		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("update profile persist error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.clientsRepo.UpdateFn = func(context.Context, *domain.Client) error { return errors.New("boom") }
		_, err := e.svc.UpdateProfile(context.Background(), 5, clients.UpdateProfileInput{City: strPtr("X")})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("list contacts success", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.contacts.ListContactsFn = func(_ context.Context, clientID int64) ([]clients.Contact, error) {
			return []clients.Contact{{ClientContact: domain.ClientContact{ID: 1, ClientID: clientID}}}, nil
		}
		list, err := e.svc.ListContacts(context.Background(), 5)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("list contacts store error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.contacts.ListContactsFn = func(context.Context, int64) ([]clients.Contact, error) {
			return nil, errors.New("boom")
		}
		_, err := e.svc.ListContacts(context.Background(), 5)
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("create contact missing client", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(context.Context, int64) (*domain.Client, error) {
			return nil, apperr.NotFound("client")
		}
		_, err := e.svc.CreateContact(context.Background(), 1, 9, clients.ContactInput{FirstName: "Siti"})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("create contact store error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.contacts.CreateContactFn = func(context.Context, *clients.Contact) error {
			return errors.New("boom")
		}
		_, err := e.svc.CreateContact(context.Background(), 1, 5, clients.ContactInput{FirstName: "Siti"})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("update contact validation", func(t *testing.T) {
		e := newFixture()
		_, err := e.svc.UpdateContact(context.Background(), 1, 5, 7, clients.ContactInput{})
		assert.Equal(t, apperr.CodeValidation, codeOf(t, err))
	})

	t.Run("update contact store error", func(t *testing.T) {
		e := newFixture()
		e.contacts.UpdateContactFn = func(context.Context, *clients.Contact) error {
			return errors.New("boom")
		}
		_, err := e.svc.UpdateContact(context.Background(), 1, 5, 7, clients.ContactInput{FirstName: "Siti"})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("admin adjust credit passthrough error", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.AdjustCreditFn = func(context.Context, int64, int64, string, *int64) (int64, error) {
			return 0, apperr.NotFound("client")
		}
		_, err := e.svc.AdminAdjustCredit(context.Background(), 1, 9, clients.AdjustCreditInput{
			Delta: 1000, Reason: "x",
		})
		assert.Equal(t, apperr.CodeNotFound, codeOf(t, err))
	})

	t.Run("credit ledger error internal", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.credits.ListByClientFn = func(context.Context, int64, ports.ListParams) ([]domain.CreditLedgerEntry, int64, error) {
			return nil, 0, errors.New("boom")
		}
		_, _, err := e.svc.Credit(context.Background(), 5, ports.ListParams{})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("deposit settings error internal", func(t *testing.T) {
		e := newFixture()
		e.clientsRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Client, error) {
			return testClient(id), nil
		}
		e.settings.GetIntFn = func(context.Context, string, int) (int, error) {
			return 0, errors.New("boom")
		}
		_, err := e.svc.Deposit(context.Background(), 5, clients.DepositInput{Amount: 50000})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})

	t.Run("export writer failure", func(t *testing.T) {
		e := newFixture()
		err := e.svc.ExportCSV(context.Background(), failWriter{})
		assert.Equal(t, apperr.CodeInternal, codeOf(t, err))
	})
}
