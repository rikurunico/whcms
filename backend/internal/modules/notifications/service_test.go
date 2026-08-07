package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedNow = time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

// memStorage is an in-memory ports.Storage for round-tripping rendered bodies.
type memStorage struct {
	mocks.MockStorage
	objects map[string][]byte
	putErr  error
}

func newMemStorage() *memStorage {
	s := &memStorage{objects: map[string][]byte{}}
	s.PutFn = func(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
		if s.putErr != nil {
			return s.putErr
		}
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		s.objects[key] = b
		return nil
	}
	s.GetFn = func(ctx context.Context, key string) (io.ReadCloser, error) {
		b, ok := s.objects[key]
		if !ok {
			return nil, errors.New("object not found")
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	return s
}

// memLogs is an in-memory ports.EmailLogRepo.
type memLogs struct {
	mocks.MockEmailLogRepo
	rows      map[int64]*domain.EmailLogEntry
	nextID    int64
	createErr error
}

func newMemLogs() *memLogs {
	l := &memLogs{rows: map[int64]*domain.EmailLogEntry{}, nextID: 100}
	l.CreateFn = func(ctx context.Context, e *domain.EmailLogEntry) error {
		if l.createErr != nil {
			return l.createErr
		}
		l.nextID++
		e.ID = l.nextID
		cp := *e
		l.rows[e.ID] = &cp
		return nil
	}
	l.GetByIDFn = func(ctx context.Context, id int64) (*domain.EmailLogEntry, error) {
		e, ok := l.rows[id]
		if !ok {
			return nil, apperr.NotFound("email log entry")
		}
		cp := *e
		return &cp, nil
	}
	l.MarkSentFn = func(ctx context.Context, id int64, at time.Time) error {
		e, ok := l.rows[id]
		if !ok {
			return apperr.NotFound("email log entry")
		}
		e.Status = domain.EmailSent
		e.SentAt = &at
		e.Error = ""
		return nil
	}
	l.MarkFailedFn = func(ctx context.Context, id int64, msg string) error {
		e, ok := l.rows[id]
		if !ok {
			return apperr.NotFound("email log entry")
		}
		e.Status = domain.EmailFailed
		e.Error = msg
		return nil
	}
	return l
}

// templates returns a MockEmailTemplateRepo backed by the given rows keyed
// "key/locale".
func templates(rows map[string]*domain.EmailTemplate) *mocks.MockEmailTemplateRepo {
	return &mocks.MockEmailTemplateRepo{
		GetFn: func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
			if t, ok := rows[key+"/"+locale]; ok {
				cp := *t
				return &cp, nil
			}
			return nil, apperr.NotFound("email template")
		},
	}
}

type fixture struct {
	svc       *Service
	logs      *memLogs
	storage   *memStorage
	enqueuer  *mocks.MockEnqueuer
	mailer    *mocks.MockMailer
	audit     *mocks.MockAuditLogger
	templates *mocks.MockEmailTemplateRepo
	users     *mocks.MockUserRepo
	clients   *mocks.MockClientRepo
	settings  *mocks.MockSettingsRepo
	intLog    *mocks.MockIntegrationLogger
}

func newFixture(tplRows map[string]*domain.EmailTemplate) *fixture {
	f := &fixture{
		logs:      newMemLogs(),
		storage:   newMemStorage(),
		enqueuer:  &mocks.MockEnqueuer{},
		mailer:    &mocks.MockMailer{},
		audit:     &mocks.MockAuditLogger{},
		templates: templates(tplRows),
		settings:  &mocks.MockSettingsRepo{},
		intLog:    &mocks.MockIntegrationLogger{},
	}
	f.users = &mocks.MockUserRepo{
		GetByIDFn: func(ctx context.Context, id int64) (*domain.User, error) {
			if id != 7 {
				return nil, apperr.NotFound("user")
			}
			return &domain.User{ID: 7, Email: "budi@example.com", Locale: "id"}, nil
		},
	}
	f.clients = &mocks.MockClientRepo{
		GetByUserIDFn: func(ctx context.Context, userID int64) (*domain.Client, error) {
			if userID != 7 {
				return nil, apperr.NotFound("client")
			}
			return &domain.Client{ID: 3, UserID: 7, FirstName: "Budi", LastName: "Santoso"}, nil
		},
	}
	f.settings.GetStringFn = func(ctx context.Context, key, def string) (string, error) {
		switch key {
		case "company.name":
			return "Hosting Kita", nil
		case "mail.from_email":
			return "no-reply@hostingkita.id", nil
		case "mail.from_name":
			return "Hosting Kita", nil
		}
		return def, nil
	}
	f.svc = New(Deps{
		Templates:       f.templates,
		Logs:            f.logs,
		Users:           f.users,
		Clients:         f.clients,
		Settings:        f.settings,
		Storage:         f.storage,
		Mailer:          f.mailer,
		Enqueuer:        f.enqueuer,
		Audit:           f.audit,
		Clock:           &mocks.MockClock{FixedTime: fixedNow},
		IntegrationLog:  f.intLog,
		FrontendURL:     "https://panel.example.com",
		AdminAlertEmail: "ops@example.com",
	})
	return f
}

func invoiceTemplates() map[string]*domain.EmailTemplate {
	return map[string]*domain.EmailTemplate{
		"invoice_created/id": {
			ID: 1, Key: "invoice_created", Locale: "id",
			Subject:  "Tagihan {{.InvoiceNumber}} dari {{.CompanyName}}",
			BodyHTML: "<p>Halo {{.Name}}, total {{.Total}}. Bayar di {{.FrontendURL}}/billing</p>",
			BodyText: "Halo {{.Name}}, total {{.Total}}.",
		},
		"invoice_created/en": {
			ID: 2, Key: "invoice_created", Locale: "en",
			Subject:  "Invoice {{.InvoiceNumber}} from {{.CompanyName}}",
			BodyHTML: "<p>Hello {{.Name}}, total {{.Total}}.</p>",
			BodyText: "Hello {{.Name}}, total {{.Total}}.",
		},
		"admin_alert/id": {
			ID: 3, Key: "admin_alert", Locale: "id",
			Subject:  "[WHCMS] Peringatan: {{.Subject}}",
			BodyHTML: "<pre>{{.Detail}}</pre>",
			BodyText: "{{.Subject}} - {{.Detail}}",
		},
	}
}

// SendTemplate

func TestSendTemplateHappyPath(t *testing.T) {
	f := newFixture(invoiceTemplates())

	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "INV-202607-000001", "Total": "Rp150.000"})
	require.NoError(t, err)

	// email_log row created with rendered subject (id locale) and queued status.
	entry, err := f.logs.GetByID(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, "budi@example.com", entry.ToEmail)
	assert.Equal(t, "invoice_created", entry.TemplateKey)
	assert.Equal(t, "Tagihan INV-202607-000001 dari Hosting Kita", entry.Subject)
	assert.Equal(t, domain.EmailQueued, entry.Status)

	// rendered body persisted to storage with globals + Name default applied.
	raw, ok := f.storage.objects[EmailBodyKey(101)]
	require.True(t, ok, "rendered body stored")
	var body storedBody
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "Budi Santoso", body.ToName)
	assert.Contains(t, body.BodyHTML, "Halo Budi Santoso, total Rp150.000")
	assert.Contains(t, body.BodyHTML, "https://panel.example.com/billing")
	assert.Contains(t, body.BodyText, "Halo Budi Santoso")

	// mail:send enqueued with the email log id.
	require.Len(t, f.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeMailSend, f.enqueuer.Tasks[0].TaskType)
	assert.Equal(t, jobs.MailSendPayload{EmailLogID: 101}, f.enqueuer.Tasks[0].Payload)
}

func TestSendTemplateUsesUserLocale(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 7, Email: "budi@example.com", Locale: "en"}, nil
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, "Invoice X from Hosting Kita", entry.Subject)
}

func TestSendTemplateLocaleFallback(t *testing.T) {
	rows := invoiceTemplates()
	delete(rows, "invoice_created/en") // en missing -> fall back to id
	f := newFixture(rows)
	f.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 7, Email: "budi@example.com", Locale: "en"}, nil
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, "Tagihan X dari Hosting Kita", entry.Subject, "fell back to id variant")
}

func TestSendTemplateEmptyLocaleDefaultsToID(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 7, Email: "budi@example.com", Locale: ""}, nil
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Contains(t, entry.Subject, "Tagihan")
}

func TestSendTemplateCallerDataWinsOverDefaults(t *testing.T) {
	f := newFixture(invoiceTemplates())
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1", "Name": "Custom Name"}))
	var body storedBody
	require.NoError(t, json.Unmarshal(f.storage.objects[EmailBodyKey(101)], &body))
	assert.Contains(t, body.BodyHTML, "Halo Custom Name")
	assert.Equal(t, "Budi Santoso", body.ToName, "ToName still from client profile")
}

func TestSendTemplateNameFallsBackToEmail(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.clients.GetByUserIDFn = func(ctx context.Context, userID int64) (*domain.Client, error) {
		return nil, apperr.NotFound("client")
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	var body storedBody
	require.NoError(t, json.Unmarshal(f.storage.objects[EmailBodyKey(101)], &body))
	assert.Equal(t, "budi@example.com", body.ToName)
	assert.Contains(t, body.BodyHTML, "Halo budi@example.com")
}

func TestSendTemplateUserNotFound(t *testing.T) {
	f := newFixture(invoiceTemplates())
	err := f.svc.SendTemplate(context.Background(), 999, "invoice_created", nil)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestSendTemplateNilUserIsNotFound(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) { return nil, nil }
	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created", nil)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestSendTemplateMissingTemplateAllLocales(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	err := f.svc.SendTemplate(context.Background(), 7, "nope", nil)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestSendTemplateRepoErrorPropagates(t *testing.T) {
	f := newFixture(nil)
	f.templates.GetFn = func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
		return nil, errors.New("db down")
	}
	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created", nil)
	require.Error(t, err)
	assert.False(t, isNotFound(err))
}

// TestSendTemplateEscapesHTMLInCallerData is a regression test for
// body_html HTML/script injection via caller-supplied data (e.g. a client's
// ticket subject echoed back by the ticket_opened template). body_html must
// render through html/template so markup in the data is escaped; body_text
// is plain text and must stay unescaped.
func TestSendTemplateEscapesHTMLInCallerData(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{
		"ticket_opened/id": {
			ID: 9, Key: "ticket_opened", Locale: "id",
			Subject:  "Tiket dibuka: {{.Subject}}",
			BodyHTML: `<p>Halo {{.Name}},</p><p>Subjek: "{{.Subject}}"</p>`,
			BodyText: `Halo {{.Name}}, subjek: {{.Subject}}`,
		},
	})
	payload := `<img src=x onerror=alert(document.cookie)>`
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "ticket_opened",
		map[string]any{"Subject": payload}))

	var body storedBody
	require.NoError(t, json.Unmarshal(f.storage.objects[EmailBodyKey(101)], &body))

	assert.NotContains(t, body.BodyHTML, "<img src=x onerror=alert(document.cookie)>",
		"body_html must not contain the raw injected markup")
	assert.Contains(t, body.BodyHTML, "&lt;img src=x onerror=alert(document.cookie)&gt;",
		"body_html must HTML-escape caller-supplied data")

	// body_text is plain text (never rendered as HTML by a mail client), so
	// it stays unescaped.
	assert.Contains(t, body.BodyText, payload)
}

func TestSendTemplateRenderErrorIsInternal(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{
		"broken/id": {Key: "broken", Locale: "id",
			Subject: "ok", BodyHTML: "{{.Name.Missing}}", BodyText: ""},
	})
	err := f.svc.SendTemplate(context.Background(), 7, "broken", nil)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestSendTemplateLogCreateError(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.logs.createErr = errors.New("insert failed")
	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"})
	require.Error(t, err)
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestSendTemplateStoragePutFailureMarksFailed(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.storage.putErr = errors.New("s3 down")
	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"})
	require.Error(t, err)
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, domain.EmailFailed, entry.Status)
	assert.Contains(t, entry.Error, "store rendered body")
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestSendTemplateEnqueueFailureMarksFailed(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.enqueuer.EnqueueFn = func(ctx context.Context, taskType string, payload any, opts ...ports.JobOption) error {
		return errors.New("redis down")
	}
	err := f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"})
	require.Error(t, err)
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, domain.EmailFailed, entry.Status)
	assert.Contains(t, entry.Error, "enqueue mail:send")
}

// AlertAdmin

func TestAlertAdmin(t *testing.T) {
	f := newFixture(invoiceTemplates())
	require.NoError(t, f.svc.AlertAdmin(context.Background(), "Provisioning failed", "service 42: whm timeout"))

	entry, err := f.logs.GetByID(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, "ops@example.com", entry.ToEmail)
	assert.Equal(t, "admin_alert", entry.TemplateKey)
	assert.Equal(t, "[WHCMS] Peringatan: Provisioning failed", entry.Subject)

	var body storedBody
	require.NoError(t, json.Unmarshal(f.storage.objects[EmailBodyKey(101)], &body))
	assert.Contains(t, body.BodyHTML, "service 42: whm timeout")
	assert.Equal(t, "Administrator", body.ToName)

	require.Len(t, f.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeMailSend, f.enqueuer.Tasks[0].TaskType)
}

func TestAlertAdminMissingConfig(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.svc.d.AdminAlertEmail = ""
	err := f.svc.AlertAdmin(context.Background(), "x", "y")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	assert.Empty(t, f.enqueuer.Tasks)
}

func TestAlertAdminMissingTemplate(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	err := f.svc.AlertAdmin(context.Background(), "x", "y")
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestAlertAdminNoWebhookByDefault(t *testing.T) {
	f := newFixture(invoiceTemplates())
	require.NoError(t, f.svc.AlertAdmin(context.Background(), "x", "y"))
	assert.Empty(t, f.intLog.Calls, "no webhook URL configured, IntegrationLog must not be called")
}

func TestAlertAdminPostsWebhookWhenConfigured(t *testing.T) {
	var gotSubject, gotMessage string
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var payload alertWebhookPayload
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		gotSubject, gotMessage = payload.Subject, payload.Message
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := newFixture(invoiceTemplates())
	f.svc.d.AdminAlertWebhookURL = ts.URL + "/hook"

	require.NoError(t, f.svc.AlertAdmin(context.Background(), "Provisioning failed", "service 42: whm timeout"))

	assert.Equal(t, "/hook", gotPath)
	assert.Equal(t, "Provisioning failed", gotSubject)
	assert.Equal(t, "service 42: whm timeout", gotMessage)

	require.Len(t, f.intLog.Calls, 1)
	assert.Equal(t, "admin_alert_webhook", f.intLog.Calls[0].Provider)
	assert.True(t, f.intLog.Calls[0].Success)
	assert.Equal(t, http.StatusOK, f.intLog.Calls[0].StatusCode)

	// The email path must still run unaffected by the webhook.
	require.Len(t, f.enqueuer.Tasks, 1)
}

func TestAlertAdminWebhookFailureDoesNotFailCallOrBlockEmail(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.svc.d.AdminAlertWebhookURL = "http://127.0.0.1:1/unreachable"

	err := f.svc.AlertAdmin(context.Background(), "Provisioning failed", "service 42: whm timeout")
	require.NoError(t, err, "an unreachable webhook must not fail AlertAdmin")

	require.Len(t, f.intLog.Calls, 1)
	assert.False(t, f.intLog.Calls[0].Success)
	assert.NotEmpty(t, f.intLog.Calls[0].Error)

	// The email path must still have gone through.
	require.Len(t, f.enqueuer.Tasks, 1)
}

// DeliverEmail

// queueOne pushes one email through SendTemplate and returns its log id.
func queueOne(t *testing.T, f *fixture) int64 {
	t.Helper()
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	return f.logs.nextID
}

func TestDeliverEmailHappyPath(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)

	require.NoError(t, f.svc.DeliverEmail(context.Background(), id))

	require.Len(t, f.mailer.Sent, 1)
	msg := f.mailer.Sent[0]
	assert.Equal(t, "budi@example.com", msg.To)
	assert.Equal(t, "Budi Santoso", msg.ToName)
	assert.Equal(t, "no-reply@hostingkita.id", msg.From)
	assert.Equal(t, "Hosting Kita", msg.FromName)
	assert.Equal(t, "Tagihan X dari Hosting Kita", msg.Subject)
	assert.Contains(t, msg.HTML, "Halo Budi Santoso")
	assert.Contains(t, msg.Text, "Halo Budi Santoso")

	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailSent, entry.Status)
	require.NotNil(t, entry.SentAt)
	assert.Equal(t, fixedNow, *entry.SentAt)
}

func TestDeliverEmailAlreadySentIsNoop(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	require.NoError(t, f.svc.DeliverEmail(context.Background(), id))
	require.NoError(t, f.svc.DeliverEmail(context.Background(), id), "idempotent re-run")
	assert.Len(t, f.mailer.Sent, 1, "second run must not send again")
}

func TestDeliverEmailNotFound(t *testing.T) {
	f := newFixture(invoiceTemplates())
	err := f.svc.DeliverEmail(context.Background(), 9999)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestDeliverEmailNilEntryIsNotFound(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.logs.GetByIDFn = func(ctx context.Context, id int64) (*domain.EmailLogEntry, error) { return nil, nil }
	err := f.svc.DeliverEmail(context.Background(), 1)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestDeliverEmailMailerFailureMarksFailedAndReturnsError(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	f.mailer.SendFn = func(ctx context.Context, msg ports.MailMessage) error {
		return errors.New("smtp 554")
	}
	err := f.svc.DeliverEmail(context.Background(), id)
	require.Error(t, err, "error returned so asynq retries")
	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailFailed, entry.Status)
	assert.Contains(t, entry.Error, "smtp 554")
}

func TestDeliverEmailRetryAfterFailureSucceeds(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	f.mailer.SendFn = func(ctx context.Context, msg ports.MailMessage) error { return errors.New("boom") }
	require.Error(t, f.svc.DeliverEmail(context.Background(), id))

	f.mailer.SendFn = nil // recovers
	require.NoError(t, f.svc.DeliverEmail(context.Background(), id))
	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailSent, entry.Status)
	assert.Empty(t, entry.Error, "error cleared on success")
}

func TestDeliverEmailMissingBodyMarksFailed(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	delete(f.storage.objects, EmailBodyKey(id))
	err := f.svc.DeliverEmail(context.Background(), id)
	require.Error(t, err)
	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailFailed, entry.Status)
	assert.Contains(t, entry.Error, "load rendered body")
	assert.Empty(t, f.mailer.Sent)
}

func TestDeliverEmailCorruptBodyMarksFailed(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	f.storage.objects[EmailBodyKey(id)] = []byte("{not json")
	err := f.svc.DeliverEmail(context.Background(), id)
	require.Error(t, err)
	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailFailed, entry.Status)
}

func TestDeliverEmailNilStorageReader(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	f.storage.GetFn = func(ctx context.Context, key string) (io.ReadCloser, error) { return nil, nil }
	err := f.svc.DeliverEmail(context.Background(), id)
	require.Error(t, err)
	entry, _ := f.logs.GetByID(context.Background(), id)
	assert.Equal(t, domain.EmailFailed, entry.Status)
}

// Templates admin

func TestListTemplates(t *testing.T) {
	f := newFixture(nil)
	f.templates.ListFn = func(ctx context.Context) ([]domain.EmailTemplate, error) {
		return []domain.EmailTemplate{{Key: "a", Locale: "id"}}, nil
	}
	out, err := f.svc.ListTemplates(context.Background())
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestGetTemplateExactNoFallback(t *testing.T) {
	rows := invoiceTemplates()
	delete(rows, "invoice_created/en")
	f := newFixture(rows)

	got, err := f.svc.GetTemplate(context.Background(), "invoice_created", "id")
	require.NoError(t, err)
	assert.Equal(t, "id", got.Locale)

	_, err = f.svc.GetTemplate(context.Background(), "invoice_created", "en")
	require.Error(t, err, "GetTemplate must not apply locale fallback")
	assert.True(t, isNotFound(err))
}

func TestGetTemplateNilResult(t *testing.T) {
	f := newFixture(nil)
	f.templates.GetFn = func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
		return nil, nil
	}
	_, err := f.svc.GetTemplate(context.Background(), "x", "id")
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestSaveTemplateCreate(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	var upserted *domain.EmailTemplate
	f.templates.UpsertFn = func(ctx context.Context, tpl *domain.EmailTemplate) error {
		tpl.ID = 55
		upserted = tpl
		return nil
	}
	out, err := f.svc.SaveTemplate(context.Background(), 42, SaveTemplateInput{
		Key: "welcome", Locale: "id", Subject: "Halo {{.Name}}", BodyHTML: "<p>Hi</p>", BodyText: "Hi",
	})
	require.NoError(t, err)
	require.NotNil(t, upserted)
	assert.Equal(t, int64(55), out.ID)

	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "notifications.template_save", f.audit.Entries[0].Action)
	assert.Equal(t, int64(42), f.audit.Entries[0].ActorUserID)
	assert.Nil(t, f.audit.Entries[0].Before, "create -> no before snapshot")
}

func TestSaveTemplateUpdateKeepsBeforeSnapshot(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.templates.UpsertFn = func(ctx context.Context, tpl *domain.EmailTemplate) error {
		tpl.ID = 1
		return nil
	}
	_, err := f.svc.SaveTemplate(context.Background(), 42, SaveTemplateInput{
		Key: "invoice_created", Locale: "id", Subject: "Baru", BodyHTML: "<p>Baru</p>",
	})
	require.NoError(t, err)
	require.Len(t, f.audit.Entries, 1)
	before, ok := f.audit.Entries[0].Before.(*domain.EmailTemplate)
	require.True(t, ok)
	assert.Contains(t, before.Subject, "Tagihan")
}

func TestSaveTemplateFieldValidation(t *testing.T) {
	f := newFixture(nil)
	cases := []struct {
		name string
		in   SaveTemplateInput
	}{
		{"missing key", SaveTemplateInput{Locale: "id", Subject: "s", BodyHTML: "b"}},
		{"bad locale", SaveTemplateInput{Key: "k", Locale: "fr", Subject: "s", BodyHTML: "b"}},
		{"missing subject", SaveTemplateInput{Key: "k", Locale: "id", BodyHTML: "b"}},
		{"missing body", SaveTemplateInput{Key: "k", Locale: "id", Subject: "s"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.SaveTemplate(context.Background(), 1, tc.in)
			require.Error(t, err)
			assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
		})
	}
	assert.Empty(t, f.audit.Entries)
}

func TestSaveTemplateBadSyntaxRejected(t *testing.T) {
	f := newFixture(nil)
	_, err := f.svc.SaveTemplate(context.Background(), 1, SaveTemplateInput{
		Key: "k", Locale: "id", Subject: "ok", BodyHTML: "{{.Name", BodyText: "",
	})
	require.Error(t, err)
	e := apperr.From(err)
	assert.Equal(t, apperr.CodeValidation, e.Code)
	require.Len(t, e.Details, 1)
	assert.Equal(t, "body_html", e.Details[0].Field)
}

func TestSaveTemplateUpsertError(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	f.templates.UpsertFn = func(ctx context.Context, tpl *domain.EmailTemplate) error {
		return errors.New("db down")
	}
	_, err := f.svc.SaveTemplate(context.Background(), 1, SaveTemplateInput{
		Key: "k", Locale: "id", Subject: "s", BodyHTML: "b",
	})
	require.Error(t, err)
	assert.Empty(t, f.audit.Entries)
}

func TestDeleteTemplate(t *testing.T) {
	f := newFixture(invoiceTemplates())
	var deletedKey, deletedLocale string
	f.templates.DeleteFn = func(ctx context.Context, key, locale string) error {
		deletedKey, deletedLocale = key, locale
		return nil
	}
	require.NoError(t, f.svc.DeleteTemplate(context.Background(), 42, "invoice_created", "ID"))
	assert.Equal(t, "invoice_created", deletedKey)
	assert.Equal(t, "id", deletedLocale, "locale normalized")
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "notifications.template_delete", f.audit.Entries[0].Action)
}

func TestDeleteTemplateNilResultIsNotFound(t *testing.T) {
	f := newFixture(nil)
	f.templates.GetFn = func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
		return nil, nil
	}
	err := f.svc.DeleteTemplate(context.Background(), 42, "x", "id")
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestDeleteTemplateRepoError(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.templates.DeleteFn = func(ctx context.Context, key, locale string) error {
		return errors.New("db down")
	}
	err := f.svc.DeleteTemplate(context.Background(), 42, "invoice_created", "id")
	require.Error(t, err)
	assert.Empty(t, f.audit.Entries)
}

func TestDeleteTemplateNotFound(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	err := f.svc.DeleteTemplate(context.Background(), 42, "missing", "id")
	require.Error(t, err)
	assert.True(t, isNotFound(err))
	assert.Empty(t, f.audit.Entries)
}

// Preview

func TestPreviewRendersWithSampleDataAndGlobals(t *testing.T) {
	f := newFixture(invoiceTemplates())
	res, err := f.svc.Preview(context.Background(), PreviewInput{
		Key: "invoice_created", Locale: "id",
		SampleData: map[string]any{"InvoiceNumber": "INV-1", "Total": "Rp5.000", "Name": "Preview"},
	})
	require.NoError(t, err)
	assert.Equal(t, "invoice_created", res.Key)
	assert.Equal(t, "id", res.Locale)
	assert.Equal(t, "Tagihan INV-1 dari Hosting Kita", res.Subject)
	assert.Contains(t, res.BodyHTML, "Halo Preview")
	assert.Contains(t, res.BodyHTML, "https://panel.example.com")
}

func TestPreviewLocaleFallback(t *testing.T) {
	rows := invoiceTemplates()
	delete(rows, "invoice_created/en")
	f := newFixture(rows)
	res, err := f.svc.Preview(context.Background(), PreviewInput{Key: "invoice_created", Locale: "en"})
	require.NoError(t, err)
	assert.Equal(t, "id", res.Locale, "result reports the actually-used locale")
}

func TestPreviewUnknownKey(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{})
	_, err := f.svc.Preview(context.Background(), PreviewInput{Key: "nope"})
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestPreviewValidation(t *testing.T) {
	f := newFixture(nil)
	_, err := f.svc.Preview(context.Background(), PreviewInput{Locale: "id"})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

func TestPreviewRenderErrorIsValidation(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{
		"broken/id": {Key: "broken", Locale: "id",
			Subject: "{{.CompanyName.Missing}}", BodyHTML: "x", BodyText: ""},
	})
	_, err := f.svc.Preview(context.Background(), PreviewInput{Key: "broken", Locale: "id"})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

// Email log admin

func TestEmailLogsPassthrough(t *testing.T) {
	f := newFixture(nil)
	f.logs.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
		assert.Equal(t, "failed", p.Status)
		return []domain.EmailLogEntry{{ID: 1}}, 1, nil
	}
	rows, total, err := f.svc.EmailLogs(context.Background(), ports.ListParams{Status: "failed"})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

func TestRetryEmailFailedOnly(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	f.enqueuer.Tasks = nil

	// queued -> conflict
	_, err := f.svc.RetryEmail(context.Background(), 42, id)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	// failed -> re-enqueued
	require.NoError(t, f.logs.MarkFailed(context.Background(), id, "smtp down"))
	entry, err := f.svc.RetryEmail(context.Background(), 42, id)
	require.NoError(t, err)
	assert.Equal(t, id, entry.ID)
	require.Len(t, f.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.TypeMailSend, f.enqueuer.Tasks[0].TaskType)
	assert.Equal(t, jobs.MailSendPayload{EmailLogID: id}, f.enqueuer.Tasks[0].Payload)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "notifications.email_retry", f.audit.Entries[0].Action)

	// sent -> conflict
	require.NoError(t, f.logs.MarkSent(context.Background(), id, fixedNow))
	_, err = f.svc.RetryEmail(context.Background(), 42, id)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestRetryEmailNotFound(t *testing.T) {
	f := newFixture(nil)
	_, err := f.svc.RetryEmail(context.Background(), 42, 12345)
	require.Error(t, err)
	assert.True(t, isNotFound(err))
}

func TestRetryEmailEnqueueError(t *testing.T) {
	f := newFixture(invoiceTemplates())
	id := queueOne(t, f)
	require.NoError(t, f.logs.MarkFailed(context.Background(), id, "x"))
	f.enqueuer.EnqueueFn = func(ctx context.Context, taskType string, payload any, opts ...ports.JobOption) error {
		return errors.New("redis down")
	}
	_, err := f.svc.RetryEmail(context.Background(), 42, id)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

// Helpers

func TestLocaleCandidates(t *testing.T) {
	assert.Equal(t, []string{"id", "en"}, localeCandidates("id"))
	assert.Equal(t, []string{"en", "id"}, localeCandidates("en"))
	assert.Equal(t, []string{"id", "en"}, localeCandidates(""))
	assert.Equal(t, []string{"en", "id"}, localeCandidates(" EN "))
	assert.Equal(t, []string{"fr", "id", "en"}, localeCandidates("fr"))
}

func TestGlobalsSettingsFailureFallsBack(t *testing.T) {
	f := newFixture(invoiceTemplates())
	f.settings.GetStringFn = func(ctx context.Context, key, def string) (string, error) {
		return def, errors.New("db down")
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))
	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, "Tagihan X dari WHCMS", entry.Subject, "company name default used")
}

func TestNowWithoutClockFallsBack(t *testing.T) {
	s := New(Deps{})
	assert.WithinDuration(t, time.Now(), s.now(), time.Second)
}
