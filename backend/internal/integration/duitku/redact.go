package duitku

import "github.com/tsdlamongan/whcms/backend/internal/integration/redact"

// redactedPlaceholder replaces sensitive values in integration logs.
const redactedPlaceholder = redact.Placeholder

// maxRawBodyLog caps non-JSON response bodies persisted to integration logs.
const maxRawBodyLog = 2048

// redactor masks the key names whose values must never be persisted in
// integration logs (CONTRACTS.md section 3).
var redactor = redact.Keys(
	"apikey", "api_key", "password", "passwd", "token", "signature", "authorization",
)

// redactValue returns a deep copy of v with sensitive map keys masked.
func redactValue(v any) any { return redactor.Value(v) }

// redactBody parses a response body as JSON and redacts it; non-JSON bodies
// are logged as a (truncated) string. Empty bodies log as nil.
func redactBody(body []byte) any { return redactor.Body(body, maxRawBodyLog) }
