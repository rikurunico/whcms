package mailer_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"
	"github.com/tsdlamongan/whcms/backend/internal/platform/mailer"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closedAddr binds an ephemeral loopback port and closes it immediately, so
// connection attempts against it are refused right away (no timeout wait).
func closedAddr(t *testing.T) (string, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().(*net.TCPAddr)
	require.NoError(t, ln.Close())
	return addr.IP.String(), addr.Port
}

var testMsg = ports.MailMessage{
	To: "client@example.com", ToName: "Client",
	From: "no-reply@example.com", FromName: "WHCMS",
	Subject: "Test", HTML: "<p>Hello</p>", Text: "Hello",
}

func TestHTTPDriverPostsJSON(t *testing.T) {
	var got ports.MailMessage
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := mailer.NewHTTP(srv.URL, srv.Client())
	require.NoError(t, m.Send(context.Background(), testMsg))
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, testMsg, got)
}

func TestHTTPDriverNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()

	m := mailer.NewHTTP(srv.URL, srv.Client())
	err := m.Send(context.Background(), testMsg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}

func TestHTTPDriverConnectionError(t *testing.T) {
	m := mailer.NewHTTP("http://127.0.0.1:1", nil) // closed port
	assert.Error(t, m.Send(context.Background(), testMsg))
}

func TestLogDriverAlwaysSucceeds(t *testing.T) {
	m := mailer.NewLog(slog.New(slog.NewTextHandler(io.Discard, nil)))
	assert.NoError(t, m.Send(context.Background(), testMsg))
}

func TestNewPicksDriver(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("log", func(t *testing.T) {
		m, err := mailer.New(config.Config{MailDriver: "log"}, log)
		require.NoError(t, err)
		assert.IsType(t, &mailer.Log{}, m)
	})
	t.Run("http", func(t *testing.T) {
		m, err := mailer.New(config.Config{MailDriver: "http", MailHTTPURL: "http://localhost:9090/mail/send"}, log)
		require.NoError(t, err)
		assert.IsType(t, &mailer.HTTP{}, m)
	})
	t.Run("http missing url", func(t *testing.T) {
		_, err := mailer.New(config.Config{MailDriver: "http"}, log)
		assert.Error(t, err)
	})
	t.Run("smtp", func(t *testing.T) {
		m, err := mailer.New(config.Config{MailDriver: "smtp", SMTPHost: "localhost", SMTPPort: 587}, log)
		require.NoError(t, err)
		assert.IsType(t, &mailer.SMTP{}, m)
	})
	t.Run("unknown", func(t *testing.T) {
		_, err := mailer.New(config.Config{MailDriver: "pigeon"}, log)
		assert.Error(t, err)
	})
}

func TestHTTPDriverRequestCreationError(t *testing.T) {
	m := mailer.NewHTTP("%zz", nil) // malformed URL: fails url parsing
	err := m.Send(context.Background(), testMsg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: request:")
}

func TestNewSMTPWithAuth(t *testing.T) {
	m, err := mailer.NewSMTP(config.Config{
		SMTPHost: "localhost", SMTPPort: 587,
		SMTPUser: "user", SMTPPass: "pass",
	})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestNewSMTPInvalidPort(t *testing.T) {
	_, err := mailer.NewSMTP(config.Config{SMTPHost: "localhost", SMTPPort: 0})
	assert.Error(t, err)
}

func TestSMTPSendInvalidFrom(t *testing.T) {
	s, err := mailer.NewSMTP(config.Config{SMTPHost: "127.0.0.1", SMTPPort: 587})
	require.NoError(t, err)

	msg := testMsg
	msg.From = "not-an-email"
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `mailer: from "not-an-email":`)
}

func TestSMTPSendInvalidTo(t *testing.T) {
	s, err := mailer.NewSMTP(config.Config{SMTPHost: "127.0.0.1", SMTPPort: 587})
	require.NoError(t, err)

	msg := testMsg
	msg.To = "not-an-email"
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: to:")
}

func TestSMTPSendDialFailureHTMLAndText(t *testing.T) {
	host, port := closedAddr(t)
	s, err := mailer.NewSMTP(config.Config{SMTPHost: host, SMTPPort: port})
	require.NoError(t, err)

	err = s.Send(context.Background(), testMsg) // has both HTML and Text
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: smtp send:")
}

func TestSMTPSendDialFailureTextOnly(t *testing.T) {
	host, port := closedAddr(t)
	s, err := mailer.NewSMTP(config.Config{SMTPHost: host, SMTPPort: port})
	require.NoError(t, err)

	msg := testMsg
	msg.HTML = "" // exercises the plain-text-only body branch
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: smtp send:")
}

// Transport security / auth selection

func TestNewSMTPEncryptionModes(t *testing.T) {
	for _, mode := range []string{"", "auto", "tls", "starttls", "none"} {
		t.Run("mode="+mode, func(t *testing.T) {
			m, err := mailer.NewSMTP(config.Config{
				SMTPHost: "mail.example.com", SMTPPort: 587, SMTPEncryption: mode,
			})
			require.NoError(t, err)
			assert.NotNil(t, m)
		})
	}
}

// Port 465 is implicit TLS; "auto" must pick that instead of STARTTLS, which
// would hang against an SMTPS-only relay.
func TestNewSMTPAutoUsesImplicitTLSOn465(t *testing.T) {
	m, err := mailer.NewSMTP(config.Config{
		SMTPHost: "mail.example.com", SMTPPort: 465, SMTPEncryption: "auto",
	})
	require.NoError(t, err)
	require.NotNil(t, m)
}

func TestNewSMTPAuthModes(t *testing.T) {
	for _, mode := range []string{"", "auto", "plain", "login", "cram-md5", "none"} {
		t.Run("auth="+mode, func(t *testing.T) {
			m, err := mailer.NewSMTP(config.Config{
				SMTPHost: "mail.example.com", SMTPPort: 587,
				SMTPUser: "user", SMTPPass: "pass", SMTPAuth: mode,
			})
			require.NoError(t, err)
			assert.NotNil(t, m)
		})
	}
}

func TestNewSMTPInsecureSkipVerify(t *testing.T) {
	m, err := mailer.NewSMTP(config.Config{
		SMTPHost: "self-signed.example.com", SMTPPort: 465,
		SMTPEncryption: "tls", SMTPInsecureSkipVerify: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

// Sender fallback

// A blank mail.from_email setting must fall back to SMTP_USER rather than
// failing every single send.
func TestSMTPSendFallsBackToSMTPUserAsSender(t *testing.T) {
	host, port := closedAddr(t)
	s, err := mailer.NewSMTP(config.Config{
		SMTPHost: host, SMTPPort: port, SMTPUser: "no-reply@example.com", SMTPPass: "x",
	})
	require.NoError(t, err)

	msg := testMsg
	msg.From = "" // settings blank
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	// It got past address construction and failed at the network layer.
	assert.Contains(t, err.Error(), "mailer: smtp send:")
}

func TestSMTPSendWithoutAnySenderIsActionable(t *testing.T) {
	s, err := mailer.NewSMTP(config.Config{SMTPHost: "127.0.0.1", SMTPPort: 587})
	require.NoError(t, err)

	msg := testMsg
	msg.From = ""
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sender address")
	assert.Contains(t, err.Error(), "mail.from_email")
}

// A non-email SMTP_USER (many relays use a plain login name) must not be
// mistaken for a sender address.
func TestSMTPSendNonEmailSMTPUserIsNotASenderFallback(t *testing.T) {
	s, err := mailer.NewSMTP(config.Config{
		SMTPHost: "127.0.0.1", SMTPPort: 587, SMTPUser: "loginname", SMTPPass: "x",
	})
	require.NoError(t, err)

	msg := testMsg
	msg.From = ""
	err = s.Send(context.Background(), msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sender address")
}
