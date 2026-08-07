// Package notifications implements the M-NOTIFICATIONS module (FR-NOTIF-001..005):
// template-based email notifications for every other module (via
// ports.NotificationSender), the mail:send worker entrypoint, and the admin
// email-template / email-log endpoints.
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET    /admin/email-templates                 list all templates              [perm: settings]
//	PUT    /admin/email-templates                 upsert one template             [perm: settings]
//	POST   /admin/email-templates/preview         render {key, locale, sample_data} [perm: settings]
//	GET    /admin/email-templates/:key/:locale    fetch one template              [perm: settings]
//	DELETE /admin/email-templates/:key/:locale    delete one template             [perm: settings]
//	POST   /admin/email/test                      synchronous SMTP smoke test     [perm: settings]
//	GET    /admin/email-log                       ?search&status&page&per_page    [perm: logs]
//	POST   /admin/email-log/:id/retry             re-enqueue a failed email       [perm: logs]
//
// Worker entrypoints (consumed by internal/worker):
//
//	DeliverEmail(ctx, emailLogID int64) error     // jobs.TypeMailSend
//
// Rendering model: text/template over subject + body_text, and html/template
// (auto-escaping) over body_html, with the caller's data map merged over the
// global variables (see dto.go for the documented variable set). body_html
// must use html/template because caller-supplied data (e.g. a client's ticket
// subject) flows into it unsanitized; text/template would emit it verbatim,
// allowing HTML/script injection into outbound HTML email. Because the
// email_log table stores only metadata
// (no body columns), the rendered body is persisted to object storage under
// `emails/<email_log_id>.json` at SendTemplate time; DeliverEmail loads it
// back, so deliveries and admin retries never re-render.
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// AdminAlertTemplateKey is the template used by AlertAdmin.
const AdminAlertTemplateKey = "admin_alert"

// LayoutTemplateKey is the global HTML wrapper (branded header/footer, mobile
// styles) rendered around every other template's body_html - the equivalent of
// WHMCS's global email header/footer. Its own body_html must contain
// {{.Content}}, the slot the per-template body is injected into. Editing it
// restyles every outbound email at once; deleting it degrades gracefully to
// the unwrapped body.
const LayoutTemplateKey = "_layout"

// DefaultLocale is used when the user has no locale preference.
const DefaultLocale = "id"

// UserGetter is the narrow slice of ports.UserRepo the module needs
// (provided by the auth module's users repository at wiring time).
type UserGetter interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
}

// ClientGetter is the narrow slice of ports.ClientRepo the module needs
// (used to resolve the recipient display name).
type ClientGetter interface {
	GetByUserID(ctx context.Context, userID int64) (*domain.Client, error)
}

// Deps are the service dependencies (small interfaces, wired by the
// composition root).
type Deps struct {
	Templates ports.EmailTemplateRepo
	Logs      ports.EmailLogRepo
	Users     UserGetter
	Clients   ClientGetter
	Settings  ports.SettingsRepo
	Storage   ports.Storage
	Mailer    ports.Mailer
	Enqueuer  ports.Enqueuer
	Audit     ports.AuditLogger
	Clock     ports.Clock
	// IntegrationLog records the AdminAlertWebhookURL POST, if configured.
	// Optional (nil-safe) - leave unset if no webhook egress is needed.
	IntegrationLog ports.IntegrationLogger

	// FrontendURL is the public frontend base URL (config FRONTEND_URL),
	// exposed to templates as {{.FrontendURL}}.
	FrontendURL string
	// AdminAlertEmail is the operator alert address (config ADMIN_ALERT_EMAIL).
	AdminAlertEmail string
	// AdminAlertWebhookURL, if set, receives a best-effort JSON POST
	// {"subject","message"} alongside every admin_alert email (FR-NOTIF-005).
	AdminAlertWebhookURL string
}

// Service implements ports.NotificationSender plus the admin template/log
// use-cases and the DeliverEmail worker entrypoint.
type Service struct {
	d          Deps
	v          *validate.Validator
	httpClient *http.Client
}

var _ ports.NotificationSender = (*Service)(nil)

// New builds the notifications Service.
func New(d Deps) *Service {
	return &Service{d: d, v: validate.New(), httpClient: &http.Client{Timeout: 5 * time.Second}}
}

// storedBody is the JSON document persisted to object storage for each queued
// email (the email_log table has no body columns).
type storedBody struct {
	ToName   string `json:"to_name"`
	BodyHTML string `json:"body_html"`
	BodyText string `json:"body_text"`
}

// EmailBodyKey is the object-storage key of the rendered body for one
// email_log row.
func EmailBodyKey(emailLogID int64) string {
	return fmt.Sprintf("emails/%d.json", emailLogID)
}

// ports.NotificationSender

// SendTemplate renders the template for the user (locale preference with
// id<->en fallback), inserts a queued email_log row, persists the rendered
// body to storage and enqueues the mail:send job.
func (s *Service) SendTemplate(ctx context.Context, userID int64, templateKey string, data map[string]any) error {
	user, err := s.d.Users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return apperr.NotFound("user")
	}

	tpl, err := s.loadTemplate(ctx, templateKey, user.Locale)
	if err != nil {
		return err
	}

	rd := s.globals(ctx)
	toName := s.displayName(ctx, user)
	rd["Name"] = toName
	rd["Email"] = user.Email
	for k, v := range data {
		rd[k] = v
	}

	subject, html, text, err := s.renderAll(ctx, tpl, rd)
	if err != nil {
		return apperr.Internal(fmt.Errorf("render template %s/%s: %w", tpl.Key, tpl.Locale, err))
	}
	return s.queue(ctx, &userID, user.Email, toName, templateKey, subject, html, text)
}

// AlertAdmin sends an operator alert to AdminAlertEmail via the admin_alert
// template ({{.Subject}} / {{.Detail}}), and, if AdminAlertWebhookURL is
// configured, best-effort POSTs the same alert as JSON (FR-NOTIF-005). The
// webhook is a secondary, non-critical channel: a failed/unreachable webhook
// never fails the call or blocks the email alert.
func (s *Service) AlertAdmin(ctx context.Context, subject, message string) error {
	if s.d.AdminAlertEmail == "" {
		return apperr.Internal(errors.New("notifications: ADMIN_ALERT_EMAIL is not configured"))
	}
	tpl, err := s.loadTemplate(ctx, AdminAlertTemplateKey, DefaultLocale)
	if err != nil {
		return err
	}
	rd := s.globals(ctx)
	rd["Subject"] = subject
	rd["Detail"] = message

	renderedSubject, html, text, err := s.renderAll(ctx, tpl, rd)
	if err != nil {
		return apperr.Internal(fmt.Errorf("render template %s/%s: %w", tpl.Key, tpl.Locale, err))
	}
	s.postAlertWebhook(ctx, subject, message)
	return s.queue(ctx, nil, s.d.AdminAlertEmail, "Administrator", AdminAlertTemplateKey, renderedSubject, html, text)
}

// alertWebhookPayload is the JSON body posted to AdminAlertWebhookURL.
type alertWebhookPayload struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// postAlertWebhook best-effort POSTs subject/message to AdminAlertWebhookURL
// when configured. Errors are recorded via IntegrationLogger (if set) and
// otherwise swallowed - this is an auxiliary egress path, not the source of
// truth for admin alerting (that's the queued email).
func (s *Service) postAlertWebhook(ctx context.Context, subject, message string) {
	url := s.d.AdminAlertWebhookURL
	if url == "" {
		return
	}
	start := time.Now()
	body, err := json.Marshal(alertWebhookPayload{Subject: subject, Message: message})
	if err != nil {
		s.logWebhookCall(ctx, 0, false, start, nil, err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logWebhookCall(ctx, 0, false, start, nil, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logWebhookCall(ctx, 0, false, start, nil, err)
		return
	}
	defer resp.Body.Close()
	s.logWebhookCall(ctx, resp.StatusCode, resp.StatusCode >= 200 && resp.StatusCode < 300, start, alertWebhookPayload{Subject: subject, Message: message}, nil)
}

func (s *Service) logWebhookCall(ctx context.Context, statusCode int, success bool, start time.Time, req any, err error) {
	if s.d.IntegrationLog == nil {
		return
	}
	call := ports.IntegrationCall{
		Provider:   "admin_alert_webhook",
		Endpoint:   s.d.AdminAlertWebhookURL,
		Method:     http.MethodPost,
		StatusCode: statusCode,
		Success:    success,
		LatencyMS:  time.Since(start).Milliseconds(),
		Request:    req,
	}
	if err != nil {
		call.Error = err.Error()
	}
	s.d.IntegrationLog.Log(ctx, call)
}

// persist inserts the email_log row (queued) and writes the rendered body to
// object storage, so any later delivery or retry can load it back without
// re-rendering. Shared by queue (async) and SendTestEmail (synchronous).
// userID is the recipient account holder when known (nil for
// AlertAdmin/SendTestEmail) - it's what lets that user view their own email
// history.
func (s *Service) persist(ctx context.Context, userID *int64, toEmail, toName, templateKey, subject, html, text string) (*domain.EmailLogEntry, error) {
	entry := &domain.EmailLogEntry{
		UserID:      userID,
		ToEmail:     toEmail,
		TemplateKey: templateKey,
		Subject:     subject,
		Status:      domain.EmailQueued,
	}
	if err := s.d.Logs.Create(ctx, entry); err != nil {
		return nil, apperr.Internal(err)
	}

	body, err := json.Marshal(storedBody{ToName: toName, BodyHTML: html, BodyText: text})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	key := EmailBodyKey(entry.ID)
	if err := s.d.Storage.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "application/json"); err != nil {
		_ = s.d.Logs.MarkFailed(ctx, entry.ID, "store rendered body: "+err.Error())
		return nil, apperr.Internal(fmt.Errorf("store rendered email body %s: %w", key, err))
	}
	return entry, nil
}

// queue persists the email_log row + rendered body and enqueues
// jobs.TypeMailSend{EmailLogID}.
func (s *Service) queue(ctx context.Context, userID *int64, toEmail, toName, templateKey, subject, html, text string) error {
	entry, err := s.persist(ctx, userID, toEmail, toName, templateKey, subject, html, text)
	if err != nil {
		return err
	}
	if err := s.d.Enqueuer.Enqueue(ctx, jobs.TypeMailSend, jobs.MailSendPayload{EmailLogID: entry.ID}); err != nil {
		_ = s.d.Logs.MarkFailed(ctx, entry.ID, "enqueue mail:send: "+err.Error())
		return apperr.Internal(fmt.Errorf("enqueue mail:send for email_log %d: %w", entry.ID, err))
	}
	return nil
}

// sender resolves the envelope sender from settings. Read failures degrade to
// empty strings on purpose - the mailer then applies its own fallback rather
// than blocking the send on a broken settings row.
func (s *Service) sender(ctx context.Context) (email, name string) {
	email, _ = s.d.Settings.GetString(ctx, "mail.from_email", "")
	name, _ = s.d.Settings.GetString(ctx, "mail.from_name", "")
	return email, name
}

// Worker entrypoint

// DeliverEmail loads the queued email_log row and its rendered body from
// storage, sends it through the Mailer and marks the row sent/failed.
// Idempotent: already-sent rows are a no-op. Errors are returned so asynq
// retries the task.
func (s *Service) DeliverEmail(ctx context.Context, emailLogID int64) error {
	entry, err := s.d.Logs.GetByID(ctx, emailLogID)
	if err != nil {
		return err
	}
	if entry == nil {
		return apperr.NotFound("email log entry")
	}
	if entry.Status == domain.EmailSent {
		return nil
	}

	body, err := s.loadBody(ctx, emailLogID)
	if err != nil {
		msg := "load rendered body: " + err.Error()
		_ = s.d.Logs.MarkFailed(ctx, emailLogID, msg)
		return apperr.Internal(fmt.Errorf("email_log %d: %s", emailLogID, msg))
	}

	fromEmail, fromName := s.sender(ctx)

	msg := ports.MailMessage{
		To:       entry.ToEmail,
		ToName:   body.ToName,
		From:     fromEmail,
		FromName: fromName,
		Subject:  entry.Subject,
		HTML:     body.BodyHTML,
		Text:     body.BodyText,
	}
	if err := s.d.Mailer.Send(ctx, msg); err != nil {
		_ = s.d.Logs.MarkFailed(ctx, emailLogID, err.Error())
		return err
	}
	return s.d.Logs.MarkSent(ctx, emailLogID, s.now())
}

// loadBody fetches and decodes the stored rendered body.
func (s *Service) loadBody(ctx context.Context, emailLogID int64) (*storedBody, error) {
	rc, err := s.d.Storage.Get(ctx, EmailBodyKey(emailLogID))
	if err != nil {
		return nil, err
	}
	if rc == nil {
		return nil, errors.New("object not found")
	}
	defer rc.Close()
	var b storedBody
	if err := json.NewDecoder(rc).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Admin use-cases: templates

// ListTemplates returns every stored template (all keys and locales).
func (s *Service) ListTemplates(ctx context.Context) ([]domain.EmailTemplate, error) {
	return s.d.Templates.List(ctx)
}

// GetTemplate fetches one template by exact key+locale (no fallback).
func (s *Service) GetTemplate(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
	t, err := s.d.Templates.Get(ctx, key, normalizeLocale(locale))
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperr.NotFound("email template")
	}
	return t, nil
}

// SaveTemplate validates the template syntax and upserts it, with an audit
// entry carrying the before/after snapshots.
func (s *Service) SaveTemplate(ctx context.Context, actorUserID int64, in SaveTemplateInput) (*domain.EmailTemplate, error) {
	if err := s.v.Struct(in); err != nil {
		return nil, err
	}
	if details := checkTemplateSyntax(in); len(details) > 0 {
		return nil, apperr.Validation("invalid template syntax", details...)
	}

	// Snapshot for the audit entry; NOT_FOUND simply means this is a create.
	var before *domain.EmailTemplate
	if prev, err := s.d.Templates.Get(ctx, in.Key, in.Locale); err == nil {
		before = prev
	} else if !isNotFound(err) {
		return nil, err
	}

	t := &domain.EmailTemplate{
		Key:      in.Key,
		Locale:   in.Locale,
		Subject:  in.Subject,
		BodyHTML: in.BodyHTML,
		BodyText: in.BodyText,
	}
	if err := s.d.Templates.Upsert(ctx, t); err != nil {
		return nil, err
	}
	s.audit(ctx, actorUserID, "notifications.template_save", "email_template", t.ID, before, t)
	return t, nil
}

// DeleteTemplate removes one template locale variant.
func (s *Service) DeleteTemplate(ctx context.Context, actorUserID int64, key, locale string) error {
	locale = normalizeLocale(locale)
	before, err := s.d.Templates.Get(ctx, key, locale)
	if err != nil {
		return err
	}
	if before == nil {
		return apperr.NotFound("email template")
	}
	if err := s.d.Templates.Delete(ctx, key, locale); err != nil {
		return err
	}
	s.audit(ctx, actorUserID, "notifications.template_delete", "email_template", before.ID, before, nil)
	return nil
}

// Preview renders a stored template (locale fallback applies) with the given
// sample data merged over the global variables.
func (s *Service) Preview(ctx context.Context, in PreviewInput) (*PreviewResult, error) {
	if err := s.v.Struct(in); err != nil {
		return nil, err
	}
	tpl, err := s.loadTemplate(ctx, in.Key, in.Locale)
	if err != nil {
		return nil, err
	}
	rd := s.globals(ctx)
	for k, v := range in.SampleData {
		rd[k] = v
	}
	// Previewing the layout itself has no per-template body to inject, so
	// stand in a representative one instead of rendering "<no value>".
	if tpl.Key == LayoutTemplateKey {
		if _, ok := rd["Content"]; !ok {
			rd["Content"] = htmltemplate.HTML(layoutPreviewContent)
		}
	}
	subject, html, text, err := s.renderAll(ctx, tpl, rd)
	if err != nil {
		return nil, apperr.Validation("template render failed: " + err.Error())
	}
	return &PreviewResult{
		Key:      tpl.Key,
		Locale:   tpl.Locale,
		Subject:  subject,
		BodyHTML: html,
		BodyText: text,
	}, nil
}

// Admin use-cases: test send

// TestEmailTemplateKey is the email_log template_key recorded for admin test
// sends. It is not a stored template: the body is built in code so the check
// still works on an installation whose templates were deleted or broken.
const TestEmailTemplateKey = "test_email"

const testEmailBodyHTML = `<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Konfigurasi email berhasil</h1>` +
	`<p style="margin:0 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">` +
	`Email uji coba ini dikirim dari <strong>{{.CompanyName}}</strong> ke <strong>{{.Email}}</strong>.</p>` +
	`<p style="margin:0 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">` +
	`Jika Anda menerimanya, pengiriman email keluar sudah berfungsi: relay SMTP, alamat pengirim, ` +
	`dan layout email semuanya terkonfigurasi dengan benar.</p>` +
	`<p style="margin:18px 0 0;font:400 13px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#8a8a8a;">` +
	`Dikirim dari Admin &gt; Settings &gt; Mail.</p>`

const testEmailBodyText = "Konfigurasi email berhasil.\n\n" +
	"Email uji coba ini dikirim dari {{.CompanyName}} ke {{.Email}}.\n" +
	"Jika Anda menerimanya, pengiriman email keluar sudah berfungsi.\n\n" +
	"Dikirim dari Admin > Settings > Mail.\n"

// SendTestEmail delivers a diagnostic message to `to` SYNCHRONOUSLY through
// the configured Mailer - deliberately bypassing the mail:send queue - so a
// bad SMTP host, port, TLS mode, credential or sender address surfaces in the
// admin's response instead of in a worker log. The attempt is recorded in the
// email log like any other send, and the upstream error text is returned
// verbatim because diagnosing the relay is the whole point of this endpoint.
func (s *Service) SendTestEmail(ctx context.Context, actorUserID int64, in SendTestEmailInput) (*TestEmailResult, error) {
	if err := s.v.Struct(in); err != nil {
		return nil, err
	}

	rd := s.globals(ctx)
	rd["Name"] = in.To
	rd["Email"] = in.To

	companyName, _ := rd["CompanyName"].(string)
	subject := fmt.Sprintf("[%s] Test email", companyName)

	html, err := renderHTML("test_email", testEmailBodyHTML, rd)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	html = s.wrapHTML(ctx, DefaultLocale, html, rd)
	text, err := render("test_email_text", testEmailBodyText, rd)
	if err != nil {
		return nil, apperr.Internal(err)
	}

	entry, err := s.persist(ctx, nil, in.To, in.To, TestEmailTemplateKey, subject, html, text)
	if err != nil {
		return nil, err
	}

	fromEmail, fromName := s.sender(ctx)
	sendErr := s.d.Mailer.Send(ctx, ports.MailMessage{
		To:       in.To,
		ToName:   in.To,
		From:     fromEmail,
		FromName: fromName,
		Subject:  subject,
		HTML:     html,
		Text:     text,
	})
	if sendErr != nil {
		_ = s.d.Logs.MarkFailed(ctx, entry.ID, sendErr.Error())
		s.audit(ctx, actorUserID, "notifications.test_email_failed", "email_log", entry.ID, nil, sendErr.Error())
		return nil, apperr.New(apperr.CodeExternal, "test email delivery failed: "+sendErr.Error())
	}

	if err := s.d.Logs.MarkSent(ctx, entry.ID, s.now()); err != nil {
		return nil, err
	}
	s.audit(ctx, actorUserID, "notifications.test_email", "email_log", entry.ID, nil, entry)
	return &TestEmailResult{EmailLogID: entry.ID, To: in.To, Subject: subject}, nil
}

// Admin use-cases: email log

// EmailLogs lists outbound email log rows (newest first; search by recipient,
// filter by status).
func (s *Service) EmailLogs(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
	return s.d.Logs.List(ctx, p)
}

// RetryEmail re-enqueues the mail:send job for a failed email. Only failed
// rows may be retried (queued rows are already in flight; sent are done).
func (s *Service) RetryEmail(ctx context.Context, actorUserID, emailLogID int64) (*domain.EmailLogEntry, error) {
	entry, err := s.d.Logs.GetByID(ctx, emailLogID)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, apperr.NotFound("email log entry")
	}
	if entry.Status != domain.EmailFailed {
		return nil, apperr.Conflict("only failed emails can be retried")
	}
	if err := s.d.Enqueuer.Enqueue(ctx, jobs.TypeMailSend, jobs.MailSendPayload{EmailLogID: entry.ID}); err != nil {
		return nil, apperr.Internal(fmt.Errorf("enqueue mail:send for email_log %d: %w", entry.ID, err))
	}
	s.audit(ctx, actorUserID, "notifications.email_retry", "email_log", entry.ID, nil, entry)
	return entry, nil
}

// Internals

// loadTemplate resolves key for the preferred locale with fallback:
// en <-> id (unknown locales try id then en). NOT_FOUND only when no locale
// variant exists at all.
func (s *Service) loadTemplate(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
	for _, loc := range localeCandidates(locale) {
		t, err := s.d.Templates.Get(ctx, key, loc)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, err
		}
		if t != nil {
			return t, nil
		}
	}
	return nil, apperr.NotFound("email template " + key)
}

// globals are the variables available to every template. Settings read
// failures fall back to defaults on purpose: a broken settings row must not
// block notifications.
func (s *Service) globals(ctx context.Context) map[string]any {
	companyName, _ := s.d.Settings.GetString(ctx, "company.name", "WHCMS")
	companyLogo, _ := s.d.Settings.GetString(ctx, "company.logo_key", "")
	companyAddress, _ := s.d.Settings.GetString(ctx, "company.address", "")
	companyEmail, _ := s.d.Settings.GetString(ctx, "company.email", "")
	return map[string]any{
		"CompanyName":    companyName,
		"CompanyLogo":    companyLogo,
		"CompanyAddress": companyAddress,
		"CompanyEmail":   companyEmail,
		"FrontendURL":    s.d.FrontendURL,
		"Year":           s.now().Year(),
	}
}

// displayName resolves the recipient name: client full name when the user has
// a client profile, otherwise the email address.
func (s *Service) displayName(ctx context.Context, user *domain.User) string {
	client, err := s.d.Clients.GetByUserID(ctx, user.ID)
	if err == nil && client != nil {
		if name := client.FullName(); name != "" {
			return name
		}
	}
	return user.Email
}

func (s *Service) audit(ctx context.Context, actorUserID int64, action, entity string, entityID int64, before, after any) {
	if s.d.Audit != nil {
		s.d.Audit.Log(ctx, actorUserID, action, entity, entityID, before, after)
	}
}

func (s *Service) now() time.Time {
	if s.d.Clock != nil {
		return s.d.Clock.Now()
	}
	return time.Now()
}

// normalizeLocale lower-cases and defaults empty to DefaultLocale.
func normalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" {
		return DefaultLocale
	}
	return locale
}

// localeCandidates returns the locale lookup order (preference first,
// then the id<->en counterpart).
func localeCandidates(locale string) []string {
	switch normalizeLocale(locale) {
	case "en":
		return []string{"en", "id"}
	case "id":
		return []string{"id", "en"}
	default:
		return []string{normalizeLocale(locale), "id", "en"}
	}
}

// renderAll renders subject, body_html and body_text of the template, then
// wraps the HTML part in the global layout (see wrapHTML).
// subject and body_text are plain text (mail header / text part) and use
// text/template. body_html is real HTML sent as the message's HTML part and
// MUST use html/template so caller-supplied data (which may contain
// attacker-controlled markup, e.g. a client's ticket subject) is
// context-aware escaped instead of interpolated verbatim.
func (s *Service) renderAll(ctx context.Context, t *domain.EmailTemplate, data map[string]any) (subject, html, text string, err error) {
	if subject, err = render("subject", t.Subject, data); err != nil {
		return "", "", "", err
	}
	if html, err = renderHTML("body_html", t.BodyHTML, data); err != nil {
		return "", "", "", err
	}
	if t.Key != LayoutTemplateKey {
		html = s.wrapHTML(ctx, t.Locale, html, data)
	}
	if text, err = render("body_text", t.BodyText, data); err != nil {
		return "", "", "", err
	}
	return subject, html, text, nil
}

// wrapHTML renders the global LayoutTemplateKey template around an
// already-rendered body, exposing it as {{.Content}}. It degrades to the bare
// body - never an error - when no layout is stored, when the layout is blank
// or unparseable, or when the body is already a full HTML document (an
// operator who pasted their own <html> wrapper must not get it nested).
//
// body is injected as template.HTML because renderHTML already escaped every
// interpolated value in it; re-escaping here would emit the markup literally.
func (s *Service) wrapHTML(ctx context.Context, locale, body string, data map[string]any) string {
	if strings.Contains(strings.ToLower(body), "<html") {
		return body
	}
	layout, err := s.loadTemplate(ctx, LayoutTemplateKey, locale)
	if err != nil || layout == nil || strings.TrimSpace(layout.BodyHTML) == "" {
		return body
	}

	wd := make(map[string]any, len(data)+1)
	for k, v := range data {
		wd[k] = v
	}
	wd["Content"] = htmltemplate.HTML(body) //nolint:gosec // G203: already escaped by renderHTML above

	out, err := renderHTML("layout", layout.BodyHTML, wd)
	if err != nil {
		return body
	}
	return out
}

// layoutPreviewContent stands in for {{.Content}} when an admin previews the
// layout template itself.
// Mirrors the typography the seeded templates use (migration 000020), so the
// preview reflects the real design.
const layoutPreviewContent = `<h1 class="em-h1" style="margin:0 0 16px;font:600 22px/1.3 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#222222;">Contoh Judul Email</h1>` +
	`<p style="margin:0 0 14px;font:400 15px/1.65 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;color:#3c3c3c;">` +
	`Ini adalah contoh isi email. Bagian ini diganti oleh body_html masing-masing template; ` +
	`header, footer, dan gaya di sekelilingnya berasal dari layout ini.</p>`

// render executes one text/template source with the data map. Missing map
// keys render as "<no value>" (text/template default), which keeps sends
// robust and makes gaps visible in previews. Only for plain-text contexts
// (subject, body_text) - never use this for HTML output.
func render(name, src string, data map[string]any) (string, error) {
	tmpl, err := template.New(name).Parse(src)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// renderHTML executes one html/template source with the data map. Unlike
// render, this auto-escapes interpolated values based on their position in
// the HTML (text node, attribute, URL, ...), which is what makes body_html
// safe against caller-supplied data containing markup.
func renderHTML(name, src string, data map[string]any) (string, error) {
	tmpl, err := htmltemplate.New(name).Parse(src)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}

// checkTemplateSyntax parse-checks each template field and returns per-field
// errors for the VALIDATION envelope. body_html is checked with html/template
// (the engine that actually renders it at send time); the other fields are
// plain text and use text/template.
func checkTemplateSyntax(in SaveTemplateInput) []apperr.FieldError {
	var details []apperr.FieldError
	for field, src := range map[string]string{
		"subject":   in.Subject,
		"body_text": in.BodyText,
	} {
		if _, err := template.New(field).Parse(src); err != nil {
			details = append(details, apperr.FieldError{Field: field, Message: err.Error()})
		}
	}
	if _, err := htmltemplate.New("body_html").Parse(in.BodyHTML); err != nil {
		details = append(details, apperr.FieldError{Field: "body_html", Message: err.Error()})
	}
	// A layout without its content slot would silently blank out every
	// outbound email, so refuse to store one.
	if in.Key == LayoutTemplateKey && !strings.Contains(in.BodyHTML, ".Content") {
		details = append(details, apperr.FieldError{
			Field:   "body_html",
			Message: "the layout template must contain {{.Content}}, the slot each email body is rendered into",
		})
	}
	return details
}

func isNotFound(err error) bool {
	var e *apperr.Error
	return errors.As(err, &e) && e.Code == apperr.CodeNotFound
}
