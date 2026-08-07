package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// layoutTemplates returns invoiceTemplates plus a global _layout row.
func layoutTemplates() map[string]*domain.EmailTemplate {
	rows := invoiceTemplates()
	rows["_layout/id"] = &domain.EmailTemplate{
		ID: 90, Key: LayoutTemplateKey, Locale: "id",
		Subject:  "Layout email global",
		BodyHTML: `<html><body><header>{{.CompanyName}}</header><main>{{.Content}}</main><footer>{{.CompanyEmail}} {{.Year}}</footer></body></html>`,
	}
	rows["_layout/en"] = &domain.EmailTemplate{
		ID: 91, Key: LayoutTemplateKey, Locale: "en",
		Subject:  "Global email layout",
		BodyHTML: `<html><body><main>{{.Content}}</main></body></html>`,
	}
	return rows
}

// Layout wrapping

func TestSendTemplateWrapsBodyInLayout(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.settings.GetStringFn = func(ctx context.Context, key, def string) (string, error) {
		switch key {
		case "company.name":
			return "Hosting Kita", nil
		case "company.email":
			return "billing@hostingkita.id", nil
		}
		return def, nil
	}

	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "INV-1", "Total": "Rp1"}))

	body := storedBodyFor(t, f, 101)
	assert.True(t, strings.HasPrefix(body.BodyHTML, "<html>"), "layout wraps the body")
	assert.Contains(t, body.BodyHTML, "<header>Hosting Kita</header>")
	assert.Contains(t, body.BodyHTML, "Halo Budi Santoso, total Rp1", "body content is inside the layout")
	assert.Contains(t, body.BodyHTML, "billing@hostingkita.id 2026", "layout sees globals")

	// The plain-text part is never wrapped.
	assert.Equal(t, "Halo Budi Santoso, total Rp1.", body.BodyText)
}

func TestSendTemplateLayoutFollowsLocale(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.users.GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: 7, Email: "budi@example.com", Locale: "en"}, nil
	}
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))

	body := storedBodyFor(t, f, 101)
	assert.Contains(t, body.BodyHTML, "<main>")
	assert.NotContains(t, body.BodyHTML, "<header>", "en layout has no header")
}

func TestSendTemplateWithoutLayoutSendsBareBody(t *testing.T) {
	f := newFixture(invoiceTemplates()) // no _layout row at all
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": "Rp1"}))

	body := storedBodyFor(t, f, 101)
	assert.True(t, strings.HasPrefix(body.BodyHTML, "<p>"), "degrades to the unwrapped body")
}

func TestWrapHTMLSkipsBodyThatIsAlreadyADocument(t *testing.T) {
	f := newFixture(layoutTemplates())
	body := `<!DOCTYPE html><HTML><body>operator pasted a full document</body></html>`
	got := f.svc.wrapHTML(context.Background(), "id", body, map[string]any{})
	assert.Equal(t, body, got, "must not nest one document inside another")
}

func TestWrapHTMLBlankLayoutDegrades(t *testing.T) {
	rows := layoutTemplates()
	rows["_layout/id"].BodyHTML = "   "
	rows["_layout/en"].BodyHTML = ""
	f := newFixture(rows)

	got := f.svc.wrapHTML(context.Background(), "id", "<p>hi</p>", map[string]any{})
	assert.Equal(t, "<p>hi</p>", got)
}

func TestWrapHTMLUnparseableLayoutDegrades(t *testing.T) {
	rows := layoutTemplates()
	rows["_layout/id"].BodyHTML = "<html>{{.Content" // never closed
	rows["_layout/en"].BodyHTML = "<html>{{.Content"
	f := newFixture(rows)

	got := f.svc.wrapHTML(context.Background(), "id", "<p>hi</p>", map[string]any{})
	assert.Equal(t, "<p>hi</p>", got, "a broken layout must not block the send")
}

func TestWrapHTMLRepoErrorDegrades(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.templates.GetFn = func(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
		return nil, errors.New("db down")
	}
	got := f.svc.wrapHTML(context.Background(), "id", "<p>hi</p>", map[string]any{})
	assert.Equal(t, "<p>hi</p>", got)
}

func TestLayoutDoesNotDoubleEscapeOrUnescapeBody(t *testing.T) {
	f := newFixture(layoutTemplates())

	// A ticket subject containing markup must stay escaped exactly once after
	// the body is injected into the layout as template.HTML.
	require.NoError(t, f.svc.SendTemplate(context.Background(), 7, "invoice_created",
		map[string]any{"InvoiceNumber": "X", "Total": `<script>alert(1)</script>`}))

	body := storedBodyFor(t, f, 101)
	assert.NotContains(t, body.BodyHTML, "<script>")
	assert.Contains(t, body.BodyHTML, "&lt;script&gt;")
	assert.NotContains(t, body.BodyHTML, "&amp;lt;", "escaped once, not twice")
}

// Layout validation and preview

func TestSaveLayoutWithoutContentSlotRejected(t *testing.T) {
	f := newFixture(layoutTemplates())
	_, err := f.svc.SaveTemplate(context.Background(), 1, SaveTemplateInput{
		Key: LayoutTemplateKey, Locale: "id", Subject: "Layout",
		BodyHTML: "<html><body>no slot here</body></html>",
	})

	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, apperr.CodeValidation, e.Code)
	require.NotEmpty(t, e.Details)
	assert.Equal(t, "body_html", e.Details[0].Field)
	assert.Contains(t, e.Details[0].Message, "{{.Content}}")
}

func TestSaveLayoutWithContentSlotAccepted(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.templates.UpsertFn = func(ctx context.Context, tpl *domain.EmailTemplate) error { return nil }

	_, err := f.svc.SaveTemplate(context.Background(), 1, SaveTemplateInput{
		Key: LayoutTemplateKey, Locale: "id", Subject: "Layout",
		BodyHTML: "<html><body>{{.Content}}</body></html>",
	})
	require.NoError(t, err)
}

func TestContentSlotOnlyRequiredForTheLayoutKey(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.templates.UpsertFn = func(ctx context.Context, tpl *domain.EmailTemplate) error { return nil }

	_, err := f.svc.SaveTemplate(context.Background(), 1, SaveTemplateInput{
		Key: "invoice_created", Locale: "id", Subject: "S", BodyHTML: "<p>no slot</p>",
	})
	require.NoError(t, err)
}

func TestPreviewOfLayoutInjectsSampleContent(t *testing.T) {
	f := newFixture(layoutTemplates())
	res, err := f.svc.Preview(context.Background(), PreviewInput{Key: LayoutTemplateKey, Locale: "id"})
	require.NoError(t, err)

	assert.Contains(t, res.BodyHTML, "Contoh Judul Email")
	assert.NotContains(t, res.BodyHTML, "<no value>")
	assert.Contains(t, res.BodyHTML, "<h1", "sample content is injected as HTML, not escaped text")
}

func TestPreviewOfLayoutHonoursSuppliedContent(t *testing.T) {
	f := newFixture(layoutTemplates())
	res, err := f.svc.Preview(context.Background(), PreviewInput{
		Key: LayoutTemplateKey, Locale: "id",
		SampleData: map[string]any{"Content": "custom slot"},
	})
	require.NoError(t, err)
	assert.Contains(t, res.BodyHTML, "custom slot")
	assert.NotContains(t, res.BodyHTML, "Contoh Judul Email")
}

func TestPreviewOfNormalTemplateIsWrapped(t *testing.T) {
	f := newFixture(layoutTemplates())
	res, err := f.svc.Preview(context.Background(), PreviewInput{
		Key: "invoice_created", Locale: "id",
		SampleData: map[string]any{"Name": "Preview", "Total": "Rp1", "InvoiceNumber": "X"},
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.BodyHTML, "<html>"),
		"preview must show what actually gets sent, layout included")
}

// Globals

func TestGlobalsIncludeCompanyContactAndYear(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.settings.GetStringFn = func(ctx context.Context, key, def string) (string, error) {
		switch key {
		case "company.name":
			return "Hosting Kita", nil
		case "company.address":
			return "Jl. Merdeka 10", nil
		case "company.email":
			return "billing@hostingkita.id", nil
		case "company.logo_key":
			return "branding/logo.png", nil
		}
		return def, nil
	}

	g := f.svc.globals(context.Background())
	assert.Equal(t, "Hosting Kita", g["CompanyName"])
	assert.Equal(t, "Jl. Merdeka 10", g["CompanyAddress"])
	assert.Equal(t, "billing@hostingkita.id", g["CompanyEmail"])
	assert.Equal(t, "branding/logo.png", g["CompanyLogo"])
	assert.Equal(t, "https://panel.example.com", g["FrontendURL"])
	assert.Equal(t, 2026, g["Year"])
}

// SendTestEmail

func TestSendTestEmailHappyPath(t *testing.T) {
	f := newFixture(layoutTemplates())

	res, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.NoError(t, err)
	assert.Equal(t, "ops@example.com", res.To)
	assert.Equal(t, "[Hosting Kita] Test email", res.Subject)
	assert.Equal(t, int64(101), res.EmailLogID)

	// Delivered synchronously - never queued.
	require.Len(t, f.mailer.Sent, 1)
	sent := f.mailer.Sent[0]
	assert.Equal(t, "ops@example.com", sent.To)
	assert.Equal(t, "no-reply@hostingkita.id", sent.From)
	assert.Equal(t, "Hosting Kita", sent.FromName)
	assert.Contains(t, sent.HTML, "ops@example.com")
	assert.Contains(t, sent.Text, "Konfigurasi email berhasil")
	assert.Empty(t, f.enqueuer.Tasks, "test sends bypass the mail:send queue")

	// Recorded in the email log as sent, with the body kept for retries.
	entry, err := f.logs.GetByID(context.Background(), 101)
	require.NoError(t, err)
	assert.Equal(t, domain.EmailSent, entry.Status)
	assert.Equal(t, TestEmailTemplateKey, entry.TemplateKey)
	require.NotNil(t, entry.SentAt)
	assert.Equal(t, fixedNow, *entry.SentAt)
	_, stored := f.storage.objects[EmailBodyKey(101)]
	assert.True(t, stored)

	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "notifications.test_email", f.audit.Entries[0].Action)
}

func TestSendTestEmailIsWrappedInLayout(t *testing.T) {
	f := newFixture(layoutTemplates())
	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(f.mailer.Sent[0].HTML, "<html>"))
}

func TestSendTestEmailWorksWithoutStoredTemplates(t *testing.T) {
	f := newFixture(map[string]*domain.EmailTemplate{}) // every template deleted
	res, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.NoError(t, err)
	assert.Equal(t, "ops@example.com", res.To)
	require.Len(t, f.mailer.Sent, 1)
}

func TestSendTestEmailSurfacesRelayError(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.mailer.SendFn = func(ctx context.Context, msg ports.MailMessage) error {
		return errors.New("mailer: smtp send: dial tcp 1.2.3.4:465: i/o timeout")
	}

	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, apperr.CodeExternal, e.Code)
	assert.Contains(t, e.Message, "dial tcp 1.2.3.4:465: i/o timeout",
		"the relay's own words are the whole point of the diagnostic")

	entry, _ := f.logs.GetByID(context.Background(), 101)
	assert.Equal(t, domain.EmailFailed, entry.Status)
	assert.Contains(t, entry.Error, "i/o timeout")
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "notifications.test_email_failed", f.audit.Entries[0].Action)
}

func TestSendTestEmailValidatesRecipient(t *testing.T) {
	f := newFixture(layoutTemplates())
	for _, to := range []string{"", "not-an-email"} {
		_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: to})
		var e *apperr.Error
		require.ErrorAs(t, err, &e, "to=%q", to)
		assert.Equal(t, apperr.CodeValidation, e.Code, "to=%q", to)
		assert.Empty(t, f.mailer.Sent)
	}
}

func TestSendTestEmailStorageFailurePropagates(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.storage.putErr = errors.New("bucket unreachable")

	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.Error(t, err)
	assert.Empty(t, f.mailer.Sent, "no send attempted when the body could not be persisted")
}

func TestSendTestEmailLogCreateFailurePropagates(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.logs.createErr = errors.New("insert failed")

	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.Error(t, err)
	assert.Empty(t, f.mailer.Sent)
}

func TestSendTestEmailMarkSentFailurePropagates(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.logs.MarkSentFn = func(ctx context.Context, id int64, at time.Time) error {
		return errors.New("update failed")
	}

	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.Error(t, err)
	assert.Len(t, f.mailer.Sent, 1, "the mail did go out; only the bookkeeping failed")
}

func TestSendTestEmailUsesEmptySenderWhenSettingsAreBlank(t *testing.T) {
	f := newFixture(layoutTemplates())
	f.settings.GetStringFn = func(ctx context.Context, key, def string) (string, error) {
		return def, nil // every setting unset
	}

	_, err := f.svc.SendTestEmail(context.Background(), 42, SendTestEmailInput{To: "ops@example.com"})
	require.NoError(t, err)
	require.Len(t, f.mailer.Sent, 1)
	assert.Empty(t, f.mailer.Sent[0].From, "left to the mailer's SMTP_USER fallback")
	assert.Equal(t, "[WHCMS] Test email", f.mailer.Sent[0].Subject, "company.name default applies")
}

// storedBodyFor decodes the rendered body persisted for one email_log row.
func storedBodyFor(t *testing.T, f *fixture, id int64) storedBody {
	t.Helper()
	raw, ok := f.storage.objects[EmailBodyKey(id)]
	require.True(t, ok, "rendered body stored for email_log %d", id)
	var b storedBody
	require.NoError(t, json.Unmarshal(raw, &b))
	return b
}
