// Package mailer implements ports.Mailer with three drivers picked by
// MAIL_DRIVER: smtp (wneessen/go-mail), http (POST MailMessage JSON to
// MAIL_HTTP_URL - the mockserver mail capture), and log (slog only).
//
// The smtp driver is tuned for real-world relays (CONTRACTS.md §11):
// SMTP_ENCRYPTION picks implicit TLS (port 465 / SMTPS) vs mandatory STARTTLS
// vs plaintext, SMTP_AUTH picks the AUTH mechanism (default: negotiate
// whatever the relay advertises, so LOGIN-only servers work), SMTP_TIMEOUT_SECONDS
// bounds a dial+send, and SMTP_INSECURE_SKIP_VERIFY exists for relays stuck on
// self-signed certificates. Encryption is never silently downgraded: a relay
// that does not offer STARTTLS fails the send instead of receiving the
// credentials in the clear.
package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	gomail "github.com/wneessen/go-mail"
)

// New picks the driver from cfg.MailDriver.
func New(cfg config.Config, log *slog.Logger) (ports.Mailer, error) {
	switch cfg.MailDriver {
	case "smtp":
		return NewSMTP(cfg)
	case "http":
		if cfg.MailHTTPURL == "" {
			return nil, fmt.Errorf("mailer: MAIL_HTTP_URL required for http driver")
		}
		return NewHTTP(cfg.MailHTTPURL, nil), nil
	case "log":
		return NewLog(log), nil
	default:
		return nil, fmt.Errorf("mailer: unknown driver %q", cfg.MailDriver)
	}
}

// SMTP driver

// SMTP sends mail through an SMTP relay using wneessen/go-mail.
type SMTP struct {
	client *gomail.Client
	// fallbackFrom is used when a message carries no From address (the
	// mail.from_email setting is blank or was cleared). Derived from
	// SMTP_USER when that looks like an address - relays almost always
	// require the envelope sender to be the authenticated mailbox anyway.
	fallbackFrom string
}

// resolveEncryption maps SMTP_ENCRYPTION ("auto" derives from the port) onto
// the go-mail options. Port 465 is implicit TLS (SMTPS) and must NOT be
// spoken in cleartext-then-STARTTLS, which is why "auto" special-cases it.
func resolveEncryption(mode string, port int) (opts []gomail.Option) {
	if mode == "auto" {
		if port == 465 {
			mode = "tls"
		} else {
			mode = "starttls"
		}
	}
	switch mode {
	case "tls":
		return []gomail.Option{gomail.WithSSL()}
	case "none":
		return []gomail.Option{gomail.WithTLSPolicy(gomail.NoTLS)}
	default: // starttls
		// Mandatory, not opportunistic: a relay that does not offer STARTTLS
		// must fail the send rather than receive the credentials in the clear.
		return []gomail.Option{gomail.WithTLSPolicy(gomail.TLSMandatory)}
	}
}

// resolveAuth maps SMTP_AUTH onto a go-mail auth type. "auto" lets go-mail
// negotiate whichever mechanism the relay advertises, which is what makes
// LOGIN-only and CRAM-MD5-only servers work out of the box.
func resolveAuth(mode string) gomail.SMTPAuthType {
	switch mode {
	case "plain":
		return gomail.SMTPAuthPlain
	case "login":
		return gomail.SMTPAuthLogin
	case "cram-md5":
		return gomail.SMTPAuthCramMD5
	default:
		return gomail.SMTPAuthAutoDiscover
	}
}

// defaultSMTPTimeout bounds one dial+send when SMTP_TIMEOUT_SECONDS is unset
// (or the Config was built programmatically without it).
const defaultSMTPTimeout = 20 * time.Second

// NewSMTP builds the SMTP driver.
func NewSMTP(cfg config.Config) (*SMTP, error) {
	timeout := cfg.SMTPTimeout
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	opts := []gomail.Option{
		gomail.WithPort(cfg.SMTPPort),
		gomail.WithTimeout(timeout),
	}
	opts = append(opts, resolveEncryption(cfg.SMTPEncryption, cfg.SMTPPort)...)

	if cfg.SMTPInsecureSkipVerify {
		opts = append(opts, gomail.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true, // #nosec G402 -- opt-in via SMTP_INSECURE_SKIP_VERIFY for self-signed relays
			ServerName:         cfg.SMTPHost,
		}))
	}

	if cfg.SMTPUser != "" && cfg.SMTPAuth != "none" {
		opts = append(opts,
			gomail.WithSMTPAuth(resolveAuth(cfg.SMTPAuth)),
			gomail.WithUsername(cfg.SMTPUser),
			gomail.WithPassword(cfg.SMTPPass),
		)
	}

	client, err := gomail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("mailer: smtp client: %w", err)
	}

	s := &SMTP{client: client}
	if strings.Contains(cfg.SMTPUser, "@") {
		s.fallbackFrom = cfg.SMTPUser
	}
	return s, nil
}

// Send delivers msg via SMTP.
func (s *SMTP) Send(ctx context.Context, msg ports.MailMessage) error {
	from := msg.From
	if from == "" {
		from = s.fallbackFrom
	}
	if from == "" {
		return fmt.Errorf("mailer: no sender address: set the mail.from_email setting (Admin > Settings > Mail) or SMTP_USER")
	}

	m, err := buildMsg(from, msg)
	if err != nil {
		return err
	}
	if err := s.client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("mailer: smtp send: %w", err)
	}
	return nil
}

// buildMsg assembles the go-mail message for msg, sent from the resolved
// from address. Split out from Send so the MIME part ordering below (which
// has no compile-time signal if it regresses - see the comment inline) is
// unit-testable without a live SMTP dial.
func buildMsg(from string, msg ports.MailMessage) (*gomail.Msg, error) {
	m := gomail.NewMsg()
	if err := m.FromFormat(msg.FromName, from); err != nil {
		return nil, fmt.Errorf("mailer: from %q: %w", from, err)
	}
	if err := m.AddToFormat(msg.ToName, msg.To); err != nil {
		return nil, fmt.Errorf("mailer: to: %w", err)
	}
	m.Subject(msg.Subject)
	// RFC 2046 §5.1.4: multipart/alternative parts must be ordered least to
	// most "faithful"; a MIME-aware client renders the LAST part it
	// understands. Plain text must come first and HTML last so HTML-capable
	// clients (Gmail, Outlook, ...) prefer it - SetBodyString always becomes
	// parts[0], AddAlternativeString always appends, so the call order below
	// IS the wire order. Getting this backwards doesn't error or warn; it
	// silently makes compliant clients render the plain-text fallback instead
	// of the designed HTML email.
	switch {
	case msg.HTML != "" && msg.Text != "":
		m.SetBodyString(gomail.TypeTextPlain, msg.Text)
		m.AddAlternativeString(gomail.TypeTextHTML, msg.HTML)
	case msg.HTML != "":
		m.SetBodyString(gomail.TypeTextHTML, msg.HTML)
	default:
		m.SetBodyString(gomail.TypeTextPlain, msg.Text)
	}
	return m, nil
}

// HTTP driver (mockserver mail capture)

// HTTP posts MailMessage JSON to a capture endpoint.
type HTTP struct {
	url    string
	client *http.Client
}

// NewHTTP builds the HTTP driver. client may be nil (default 10s timeout).
func NewHTTP(url string, client *http.Client) *HTTP {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTP{url: url, client: client}
}

// Send POSTs the message as JSON; non-2xx responses are errors.
func (h *HTTP) Send(ctx context.Context, msg ports.MailMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mailer: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailer: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("mailer: http send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mailer: http send: status %d: %s", resp.StatusCode, snippet)
	}
	return nil
}

// Log driver

// Log writes mail to the application log (development default).
type Log struct {
	log *slog.Logger
}

// NewLog builds the log driver.
func NewLog(log *slog.Logger) *Log { return &Log{log: log} }

// Send logs the message and succeeds.
func (l *Log) Send(ctx context.Context, msg ports.MailMessage) error {
	l.log.InfoContext(ctx, "mail (log driver)",
		"to", msg.To, "subject", msg.Subject, "text", msg.Text)
	return nil
}
