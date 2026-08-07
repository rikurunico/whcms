// Internal (white-box) test: package mailer, not mailer_test, so it can
// inspect buildMsg's unexported output directly instead of round-tripping
// through a live SMTP dial.
package mailer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawMIME serializes m the same way DialAndSendWithContext would put it on
// the wire, so the assertions below see exactly what a receiving mail server
// (and thus the recipient's client) sees.
func rawMIME(t *testing.T, msg ports.MailMessage) string {
	t.Helper()
	m, err := buildMsg("no-reply@example.com", msg)
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = m.WriteTo(&buf)
	require.NoError(t, err)
	return buf.String()
}

// A client that only understands MIME renders the LAST alternative it can -
// text/plain before text/html is what makes Gmail (and other RFC-2046-
// compliant clients) prefer the HTML part. Getting this backwards is exactly
// the 2026-08-04 bug: valid email, wrong part order, every client silently
// fell back to plain text instead of the designed HTML.
func TestBuildMsgOrdersPlainBeforeHTML(t *testing.T) {
	raw := rawMIME(t, ports.MailMessage{
		To: "client@example.com", From: "no-reply@example.com",
		Subject: "S", HTML: "<p>Hello HTML</p>", Text: "Hello text",
	})

	plainIdx := strings.Index(raw, `Content-Type: text/plain`)
	htmlIdx := strings.Index(raw, `Content-Type: text/html`)
	require.NotEqual(t, -1, plainIdx, "text/plain part present")
	require.NotEqual(t, -1, htmlIdx, "text/html part present")
	assert.Less(t, plainIdx, htmlIdx,
		"text/plain must come before text/html in a multipart/alternative body "+
			"(RFC 2046 §5.1.4) - compliant clients render the LAST understood part")
	assert.Contains(t, raw, "multipart/alternative")
}

func TestBuildMsgHTMLOnlyHasNoTextPart(t *testing.T) {
	raw := rawMIME(t, ports.MailMessage{
		To: "client@example.com", From: "no-reply@example.com",
		Subject: "S", HTML: "<p>Hello HTML</p>",
	})
	assert.Contains(t, raw, "Content-Type: text/html")
	assert.NotContains(t, raw, "Content-Type: text/plain")
}

func TestBuildMsgTextOnlyHasNoHTMLPart(t *testing.T) {
	raw := rawMIME(t, ports.MailMessage{
		To: "client@example.com", From: "no-reply@example.com",
		Subject: "S", Text: "Hello text",
	})
	assert.Contains(t, raw, "Content-Type: text/plain")
	assert.NotContains(t, raw, "Content-Type: text/html")
}

func TestBuildMsgInvalidFromIsRejected(t *testing.T) {
	_, err := buildMsg("not-an-email", ports.MailMessage{To: "client@example.com", Text: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: from")
}

func TestBuildMsgInvalidToIsRejected(t *testing.T) {
	_, err := buildMsg("no-reply@example.com", ports.MailMessage{To: "not-an-email", Text: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mailer: to")
}
