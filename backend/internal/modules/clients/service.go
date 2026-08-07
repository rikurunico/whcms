// Package clients implements the M-CLIENTS module: admin client CRUD +
// search, the client detail aggregate, status changes, admin notes, contacts
// CRUD (sub-accounts without login), the credit ledger (AddCredit /
// DeductCredit consumed by billing and payments), credit deposits via
// ports.InvoiceCreator and a streamed CSV export.
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET    /account/profile                        client profile              [client]
//	PATCH  /account/profile                        update address fields       [client]
//	GET    /account/credit                         balance + ledger page       [client]
//	POST   /account/credit/deposit                 create deposit invoice      [client]
//	GET    /account/contacts                       list contacts               [client]
//	POST   /account/contacts                       create contact              [client]
//	PATCH  /account/contacts/:contactId            update contact              [client]
//	DELETE /account/contacts/:contactId            delete contact              [client]
//	GET    /admin/clients                          search/list                 [perm: clients]
//	POST   /admin/clients                          create user+client          [perm: clients]
//	GET    /admin/clients/export                   streamed CSV export         [perm: clients]
//	GET    /admin/clients/:id                      detail aggregate            [perm: clients]
//	PATCH  /admin/clients/:id                      update profile/notes        [perm: clients]
//	DELETE /admin/clients/:id                      soft delete                 [perm: clients]
//	POST   /admin/clients/:id/status               set status                  [perm: clients]
//	GET    /admin/clients/:id/credit               ledger page                 [perm: clients]
//	POST   /admin/clients/:id/credit               manual credit adjustment    [perm: clients]
//	GET    /admin/clients/:id/contacts             list contacts               [perm: clients]
//	POST   /admin/clients/:id/contacts             create contact              [perm: clients]
//	PATCH  /admin/clients/:id/contacts/:contactId  update contact              [perm: clients]
//	DELETE /admin/clients/:id/contacts/:contactId  delete contact              [perm: clients]
//
// Cross-module surface (MODULES §2): AddCredit and DeductCredit on *Service
// are consumed by billing (deposit paid, refund-to-credit) and payments
// (credit payment) through narrow consumer-side interfaces.
package clients

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// recentLimit is how many rows of each related entity the detail aggregate returns.
const recentLimit = 5

// ContactStore persists client contacts (implemented by *Repo).
type ContactStore interface {
	CreateContact(ctx context.Context, ct *Contact) error
	GetContact(ctx context.Context, clientID, id int64) (*Contact, error)
	ListContacts(ctx context.Context, clientID int64) ([]Contact, error)
	UpdateContact(ctx context.Context, ct *Contact) error
	DeleteContact(ctx context.Context, clientID, id int64) error
}

// KnownContactPermissions whitelists the client sub-account (contact)
// permissions JSONB keys - the client-area subset the account UI models
// (docs/RECONCILE.md: `permissions:{invoices,services,domains,tickets}`),
// distinct from the staff module permissions in
// adminops.KnownPermissionModules.
var KnownContactPermissions = map[string]bool{
	"invoices": true,
	"services": true,
	"domains":  true,
	"tickets":  true,
}

// validateContactPermissions checks every permission key against
// KnownContactPermissions.
func validateContactPermissions(perms map[string]bool) error {
	var details []apperr.FieldError
	for key := range perms {
		if !KnownContactPermissions[key] {
			details = append(details, apperr.FieldError{
				Field: "permissions." + key, Message: "unknown permission area",
			})
		}
	}
	if len(details) > 0 {
		return apperr.Validation("invalid permissions", details...)
	}
	return nil
}

// marshalContactPermissions renders perms as the JSONB payload persisted on
// client_contacts.permissions (nil -> `{}`, matching adminops' convention).
func marshalContactPermissions(perms map[string]bool) json.RawMessage {
	if perms == nil {
		perms = map[string]bool{}
	}
	raw, _ := json.Marshal(perms)
	return raw
}

// SearchStore runs the indexed admin client search (implemented by *Repo).
type SearchStore interface {
	SearchClients(ctx context.Context, f SearchInput) ([]ClientListRow, int64, error)
}

// AggregateStore runs the detail-aggregate queries (implemented by *Repo).
type AggregateStore interface {
	Counts(ctx context.Context, clientID int64) (*ClientCounts, error)
	Recent(ctx context.Context, clientID int64, limit int) (*ClientRecent, error)
}

// ExportStore streams export rows (implemented by *Repo).
type ExportStore interface {
	ForEachExportRow(ctx context.Context, fn func(ExportRow) error) error
}

// Deps are the service dependencies (small interfaces, wired by the
// composition root).
type Deps struct {
	Tx        ports.TxManager
	Clients   ports.ClientRepo
	Credits   ports.CreditRepo
	Users     ports.UserRepo // impl provided by the auth module
	Contacts  ContactStore
	Search    SearchStore
	Aggregate AggregateStore
	Export    ExportStore
	Invoices  ports.InvoiceCreator // impl provided by the billing module
	Settings  ports.SettingsRepo
	Hasher    ports.PasswordHasher
	Audit     ports.AuditLogger
	Clock     ports.Clock
}

// Service implements the clients use-cases.
type Service struct {
	d   Deps
	val *validate.Validator
}

// New builds the clients Service.
func New(d Deps) *Service {
	return &Service{d: d, val: validate.New()}
}

// Admin: CRUD + search

// SearchClients lists clients for the admin area (ILIKE across
// name/company/email, exact status filter, optional has-product join).
func (s *Service) SearchClients(ctx context.Context, f SearchInput) ([]ClientListRow, int64, error) {
	if f.Status != "" {
		switch domain.ClientStatus(f.Status) {
		case domain.ClientActive, domain.ClientInactive, domain.ClientClosed:
		default:
			return nil, 0, apperr.Validation("invalid status filter",
				apperr.FieldError{Field: "status", Message: "must be one of: active, inactive, closed"})
		}
	}
	rows, total, err := s.d.Search.SearchClients(ctx, f)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return rows, total, nil
}

// CreateClient creates a user (role client, active, verified not set) plus
// its client profile in one transaction.
func (s *Service) CreateClient(ctx context.Context, actorUserID int64, in CreateClientInput) (*ClientWithEmail, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}

	existing, err := s.d.Users.GetByEmail(ctx, in.Email)
	if err != nil && apperr.From(err).Code != apperr.CodeNotFound {
		return nil, apperr.Internal(err)
	}
	if existing != nil {
		return nil, apperr.Conflict("email already in use")
	}

	hash, err := s.d.Hasher.Hash(in.Password)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	client := &domain.Client{
		FirstName: in.FirstName,
		LastName:  in.LastName,
		Company:   in.Company,
		Address1:  in.Address1,
		Address2:  in.Address2,
		City:      in.City,
		State:     in.State,
		Postcode:  in.Postcode,
		Country:   strings.ToUpper(in.Country),
		Phone:     in.Phone,
		Status:    domain.ClientActive,
	}
	err = s.d.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		user := &domain.User{
			Email:        in.Email,
			PasswordHash: hash,
			Role:         domain.RoleClient,
			Status:       domain.UserActive,
			Locale:       "id",
		}
		if err := s.d.Users.Create(txCtx, user); err != nil {
			return err
		}
		client.UserID = user.ID
		return s.d.Clients.Create(txCtx, client)
	})
	if err != nil {
		if e := apperr.From(err); e.Code != apperr.CodeInternal {
			return nil, e
		}
		return nil, apperr.Internal(err)
	}

	s.d.Audit.Log(ctx, actorUserID, "client.create", "client", client.ID, nil, map[string]any{
		"email": in.Email, "name": client.FullName(),
	})
	return &ClientWithEmail{Client: *client, Email: in.Email}, nil
}

// GetClientDetail returns the client detail aggregate: profile + user email,
// entity counters and the recent rows of each related entity.
func (s *Service) GetClientDetail(ctx context.Context, id int64) (*ClientDetail, error) {
	client, err := s.d.Clients.GetByID(ctx, id)
	if err != nil {
		return nil, normalize(err)
	}
	user, err := s.d.Users.GetByID(ctx, client.UserID)
	if err != nil {
		return nil, normalize(err)
	}
	counts, err := s.d.Aggregate.Counts(ctx, id)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if counts == nil {
		counts = &ClientCounts{}
	}
	recent, err := s.d.Aggregate.Recent(ctx, id, recentLimit)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if recent == nil {
		recent = &ClientRecent{}
	}
	detail := &ClientDetail{Client: *client, Counts: *counts, Recent: *recent}
	if user != nil {
		detail.Email = user.Email
		detail.LastLoginAt = user.LastLoginAt
	}
	return detail, nil
}

// applyProfilePatch copies non-nil fields onto the client and returns the
// before/after change maps for auditing.
func applyProfilePatch(c *domain.Client, fields map[string]*string) (before, after map[string]any) {
	before, after = map[string]any{}, map[string]any{}
	targets := map[string]*string{
		"first_name": &c.FirstName, "last_name": &c.LastName, "company": &c.Company,
		"address1": &c.Address1, "address2": &c.Address2, "city": &c.City,
		"state": &c.State, "postcode": &c.Postcode, "country": &c.Country,
		"phone": &c.Phone,
	}
	for name, val := range fields {
		if val == nil {
			continue
		}
		dst, ok := targets[name]
		if !ok {
			continue
		}
		v := *val
		if name == "country" {
			v = strings.ToUpper(v)
		}
		if *dst == v {
			continue
		}
		before[name], after[name] = *dst, v
		*dst = v
	}
	return before, after
}

// requiredProfileFields lists the address fields domain registration itself
// requires on a client profile (domains.Service, resolveRegistrantContact) -
// kept here, not just re-derived at registration time, so a client can never
// blank one of them back out via a later profile edit.
var requiredProfileFields = []string{"address1", "city", "state", "postcode"}

// blankRequiredProfileFields reports every field in requiredProfileFields
// whose pointer is non-nil but whose dereferenced value is blank
// (whitespace-only counts as blank) - "the caller chose to touch this
// field" without "left it empty on purpose". A nil pointer (field not
// included in the request at all) is never flagged; validator's own
// `omitempty` can't express this distinction on a pointer, since it treats a
// pointer to the zero value the same as a nil pointer.
func blankRequiredProfileFields(fields map[string]*string) []apperr.FieldError {
	var details []apperr.FieldError
	for _, name := range requiredProfileFields {
		if v := fields[name]; v != nil && strings.TrimSpace(*v) == "" {
			details = append(details, apperr.FieldError{Field: name, Message: "is required"})
		}
	}
	return details
}

// UpdateClient patches profile fields and admin notes (admin side).
func (s *Service) UpdateClient(ctx context.Context, actorUserID, id int64, in UpdateClientInput) (*domain.Client, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	fields := map[string]*string{
		"first_name": in.FirstName, "last_name": in.LastName, "company": in.Company,
		"address1": in.Address1, "address2": in.Address2, "city": in.City,
		"state": in.State, "postcode": in.Postcode, "country": in.Country,
		"phone": in.Phone,
	}
	if details := blankRequiredProfileFields(fields); len(details) > 0 {
		return nil, apperr.Validation("required field cannot be blank", details...)
	}
	client, err := s.d.Clients.GetByID(ctx, id)
	if err != nil {
		return nil, normalize(err)
	}

	before, after := applyProfilePatch(client, fields)
	if in.NotesAdmin != nil && *in.NotesAdmin != client.NotesAdmin {
		before["notes_admin"], after["notes_admin"] = client.NotesAdmin, *in.NotesAdmin
		client.NotesAdmin = *in.NotesAdmin
	}
	if len(after) == 0 {
		return client, nil
	}

	if err := s.d.Clients.Update(ctx, client); err != nil {
		return nil, normalize(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.update", "client", client.ID, before, after)
	return client, nil
}

// SetStatus changes the client status (active|inactive|closed); idempotent
// when the status is unchanged.
func (s *Service) SetStatus(ctx context.Context, actorUserID, id int64, in SetStatusInput) (*domain.Client, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	client, err := s.d.Clients.GetByID(ctx, id)
	if err != nil {
		return nil, normalize(err)
	}
	newStatus := domain.ClientStatus(in.Status)
	if client.Status == newStatus {
		return client, nil
	}
	before := map[string]any{"status": client.Status}
	client.Status = newStatus
	if err := s.d.Clients.Update(ctx, client); err != nil {
		return nil, normalize(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.status", "client", client.ID,
		before, map[string]any{"status": newStatus})
	return client, nil
}

// DeleteClient soft-deletes the client profile.
func (s *Service) DeleteClient(ctx context.Context, actorUserID, id int64) error {
	client, err := s.d.Clients.GetByID(ctx, id)
	if err != nil {
		return normalize(err)
	}
	if err := s.d.Clients.SoftDelete(ctx, id); err != nil {
		return normalize(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.delete", "client", id,
		map[string]any{"name": client.FullName(), "status": client.Status}, nil)
	return nil
}

// Client self-service profile

// GetProfile returns the caller's own client profile.
func (s *Service) GetProfile(ctx context.Context, clientID int64) (*domain.Client, error) {
	client, err := s.d.Clients.GetByID(ctx, clientID)
	if err != nil {
		return nil, normalize(err)
	}
	return client, nil
}

// UpdateProfile patches the caller's own address/contact fields
// (PATCH /account/profile; identity fields change via /auth/me).
func (s *Service) UpdateProfile(ctx context.Context, clientID int64, in UpdateProfileInput) (*domain.Client, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	fields := map[string]*string{
		"first_name": in.FirstName, "last_name": in.LastName, "company": in.Company,
		"address1": in.Address1, "address2": in.Address2, "city": in.City,
		"state": in.State, "postcode": in.Postcode, "country": in.Country,
		"phone": in.Phone,
	}
	if details := blankRequiredProfileFields(fields); len(details) > 0 {
		return nil, apperr.Validation("required field cannot be blank", details...)
	}
	client, err := s.d.Clients.GetByID(ctx, clientID)
	if err != nil {
		return nil, normalize(err)
	}
	before, after := applyProfilePatch(client, fields)
	if len(after) == 0 {
		return client, nil
	}
	if err := s.d.Clients.Update(ctx, client); err != nil {
		return nil, normalize(err)
	}
	s.d.Audit.Log(ctx, 0, "client.profile_update", "client", client.ID, before, after)
	return client, nil
}

// Contacts (sub-accounts without login - documented limitation)

// ListContacts returns all contacts of the client.
func (s *Service) ListContacts(ctx context.Context, clientID int64) ([]Contact, error) {
	if _, err := s.d.Clients.GetByID(ctx, clientID); err != nil {
		return nil, normalize(err)
	}
	contacts, err := s.d.Contacts.ListContacts(ctx, clientID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return contacts, nil
}

// CreateContact adds a contact to the client.
func (s *Service) CreateContact(ctx context.Context, actorUserID, clientID int64, in ContactInput) (*Contact, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if err := validateContactPermissions(in.Permissions); err != nil {
		return nil, err
	}
	if _, err := s.d.Clients.GetByID(ctx, clientID); err != nil {
		return nil, normalize(err)
	}
	ct := &Contact{
		ClientContact: domain.ClientContact{
			ClientID:  clientID,
			FirstName: in.FirstName,
			LastName:  in.LastName,
			Email:     in.Email,
			Phone:     in.Phone,
		},
		Permissions: marshalContactPermissions(in.Permissions),
	}
	if err := s.d.Contacts.CreateContact(ctx, ct); err != nil {
		return nil, apperr.Internal(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.contact_create", "client_contact", ct.ID,
		nil, map[string]any{"client_id": clientID, "email": ct.Email, "permissions": ct.Permissions})
	return ct, nil
}

// UpdateContact updates a contact; contacts of other clients yield NOT_FOUND.
func (s *Service) UpdateContact(ctx context.Context, actorUserID, clientID, contactID int64, in ContactInput) (*Contact, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if err := validateContactPermissions(in.Permissions); err != nil {
		return nil, err
	}
	ct, err := s.d.Contacts.GetContact(ctx, clientID, contactID)
	if err != nil {
		return nil, normalize(err)
	}
	before := map[string]any{
		"first_name": ct.FirstName, "last_name": ct.LastName,
		"email": ct.Email, "phone": ct.Phone, "permissions": ct.Permissions,
	}
	ct.FirstName, ct.LastName, ct.Email, ct.Phone = in.FirstName, in.LastName, in.Email, in.Phone
	ct.Permissions = marshalContactPermissions(in.Permissions)
	if err := s.d.Contacts.UpdateContact(ctx, ct); err != nil {
		return nil, normalize(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.contact_update", "client_contact", ct.ID,
		before, map[string]any{
			"first_name": ct.FirstName, "last_name": ct.LastName,
			"email": ct.Email, "phone": ct.Phone, "permissions": ct.Permissions,
		})
	return ct, nil
}

// DeleteContact removes a contact; contacts of other clients yield NOT_FOUND.
func (s *Service) DeleteContact(ctx context.Context, actorUserID, clientID, contactID int64) error {
	if err := s.d.Contacts.DeleteContact(ctx, clientID, contactID); err != nil {
		return normalize(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "client.contact_delete", "client_contact", contactID,
		map[string]any{"client_id": clientID}, nil)
	return nil
}

// Credit (cross-module surface - MODULES §2 exact signatures)

// AddCredit atomically adds delta to the client's balance and writes a ledger
// row. Consumed by billing (deposit paid, refund-to-credit). delta must be > 0.
func (s *Service) AddCredit(ctx context.Context, clientID int64, delta int64, reason string, relatedInvoiceID int64) error {
	if delta <= 0 {
		return apperr.Validation("credit delta must be positive")
	}
	_, err := s.applyCredit(ctx, clientID, delta, reason, relatedInvoiceID)
	return err
}

// DeductCredit atomically subtracts amount from the client's balance and
// writes a ledger row. Consumed by payments (credit pay). amount must be > 0;
// an insufficient balance yields CONFLICT (negative-balance guard).
func (s *Service) DeductCredit(ctx context.Context, clientID int64, amount int64, reason string, relatedInvoiceID int64) error {
	if amount <= 0 {
		return apperr.Validation("credit amount must be positive")
	}
	_, err := s.applyCredit(ctx, clientID, -amount, reason, relatedInvoiceID)
	return err
}

// applyCredit runs the atomic balance update + ledger insert inside a
// transaction (joins the caller's tx when one is active).
func (s *Service) applyCredit(ctx context.Context, clientID, delta int64, reason string, relatedInvoiceID int64) (int64, error) {
	var invoiceRef *int64
	if relatedInvoiceID > 0 {
		invoiceRef = &relatedInvoiceID
	}
	var balance int64
	err := s.d.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		balance, err = s.d.Clients.AdjustCredit(txCtx, clientID, delta, reason, invoiceRef)
		return err
	})
	if err != nil {
		return 0, normalize(err)
	}
	return balance, nil
}

// AdminAdjustCredit is the manual admin credit adjustment (positive delta
// adds, negative deducts) with an audit entry; returns the new balance.
func (s *Service) AdminAdjustCredit(ctx context.Context, actorUserID, clientID int64, in AdjustCreditInput) (int64, error) {
	if err := s.val.Struct(in); err != nil {
		return 0, err
	}
	balance, err := s.applyCredit(ctx, clientID, in.Delta, in.Reason, 0)
	if err != nil {
		return 0, err
	}
	s.d.Audit.Log(ctx, actorUserID, "client.credit_adjust", "client", clientID,
		nil, map[string]any{"delta": in.Delta, "reason": in.Reason, "balance_after": balance})
	return balance, nil
}

// Credit returns the client's balance plus one page of the ledger.
func (s *Service) Credit(ctx context.Context, clientID int64, p ports.ListParams) (*CreditOverview, int64, error) {
	client, err := s.d.Clients.GetByID(ctx, clientID)
	if err != nil {
		return nil, 0, normalize(err)
	}
	entries, total, err := s.d.Credits.ListByClient(ctx, clientID, p)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	if entries == nil {
		entries = []domain.CreditLedgerEntry{}
	}
	return &CreditOverview{Balance: client.CreditBalance, Entries: entries}, total, nil
}

// Deposit creates a credit deposit invoice (item related_type=deposit,
// Taxed=false - deposit invoices are never taxed and cannot be paid with
// credit, enforced by payments). Minimum amount Rp10.000.
func (s *Service) Deposit(ctx context.Context, clientID int64, in DepositInput) (*domain.Invoice, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if _, err := s.d.Clients.GetByID(ctx, clientID); err != nil {
		return nil, normalize(err)
	}
	dueDays, err := s.d.Settings.GetInt(ctx, "billing.invoice_due_days", 3)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	inv, err := s.d.Invoices.CreateInvoice(ctx, ports.CreateInvoiceInput{
		ClientID: clientID,
		Items: []ports.CreateInvoiceItem{{
			Description: "Credit deposit",
			Amount:      in.Amount,
			Taxed:       false,
			RelatedType: domain.RelatedDeposit,
		}},
		DueDate: s.d.Clock.Now().UTC().AddDate(0, 0, dueDays),
		Notes:   "Credit deposit",
	})
	if err != nil {
		return nil, normalize(err)
	}
	s.d.Audit.Log(ctx, 0, "client.deposit", "client", clientID, nil,
		map[string]any{"invoice_id": inv.ID, "amount": in.Amount})
	return inv, nil
}

// CSV export

// ExportCSV streams every non-deleted client as CSV to w.
func (s *Service) ExportCSV(ctx context.Context, w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"id", "email", "first_name", "last_name", "company", "city", "country",
		"phone", "status", "credit_balance", "created_at",
	}); err != nil {
		return apperr.Internal(err)
	}
	err := s.d.Export.ForEachExportRow(ctx, func(row ExportRow) error {
		return cw.Write([]string{
			strconv.FormatInt(row.ID, 10),
			row.Email,
			row.FirstName,
			row.LastName,
			row.Company,
			row.City,
			row.Country,
			row.Phone,
			row.Status,
			strconv.FormatInt(row.CreditBalance, 10),
			row.CreatedAt.UTC().Format("2006-01-02 15:04:05"),
		})
	})
	if err != nil {
		return normalize(err)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// normalize passes through *apperr.Error values and wraps anything else as
// INTERNAL.
func normalize(err error) error {
	if err == nil {
		return nil
	}
	if e := apperr.From(err); e.Code != apperr.CodeInternal {
		return e
	}
	return apperr.Internal(err)
}
