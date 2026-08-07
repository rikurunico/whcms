package turnstile

import (
	"net/url"

	"github.com/tsdlamongan/whcms/backend/internal/integration/redact"
)

// redactedPlaceholder replaces sensitive values in integration logs.
const redactedPlaceholder = redact.Placeholder

// maxRawBodyLog caps non-JSON response bodies persisted to integration logs.
const maxRawBodyLog = 2048

// redactor masks the form-field names whose values must never be persisted in
// integration logs (CONTRACTS.md section 3). "secret" is the Turnstile
// siteverify secret.
var redactor = redact.Keys(
	"secret", "apikey", "api_key", "token", "password", "signature", "authorization",
)

// redactForm converts a url.Values request body into a loggable map with
// sensitive keys masked. Single-value fields flatten to a scalar.
func redactForm(form url.Values) map[string]any { return redactor.Form(form) }

// redactBody parses a siteverify response body as JSON for logging; non-JSON
// bodies are logged as a (truncated) string. The response carries no secrets,
// so parsed JSON is logged unmasked.
func redactBody(body []byte) any { return redact.ParseBody(body, maxRawBodyLog) }
