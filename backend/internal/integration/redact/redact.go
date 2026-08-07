// Package redact masks secret values in request and response payloads before
// integration adapters persist them via ports.IntegrationLogger, so that
// credentials never reach integration_logs (CONTRACTS.md section 3).
//
// Each adapter constructs a Redactor with its own sensitive-key set (Keys for
// exact names, Substrings for fragment matching) and passes payloads through
// Value, Body, BodyWrapped, Form, or FormJoined depending on the payload
// shape it logs.
package redact

import (
	"encoding/json"
	"net/url"
	"strings"
)

// Placeholder is the value written in place of a masked secret.
const Placeholder = "[REDACTED]"

// A Redactor decides which map keys are sensitive and rewrites their values
// to Placeholder. The zero value matches no keys; construct one with Keys or
// Substrings.
type Redactor struct {
	exact     map[string]struct{}
	fragments []string
}

// Keys returns a Redactor that masks map keys equal to one of the given
// names, compared case-insensitively.
func Keys(names ...string) *Redactor {
	r := &Redactor{exact: make(map[string]struct{}, len(names))}
	for _, n := range names {
		r.exact[strings.ToLower(n)] = struct{}{}
	}
	return r
}

// Substrings returns a Redactor that masks map keys containing any of the
// given fragments, compared case-insensitively. It matches more broadly than
// Keys: the fragment "token" also masks "api_token".
func Substrings(fragments ...string) *Redactor {
	r := &Redactor{fragments: make([]string, len(fragments))}
	for i, f := range fragments {
		r.fragments[i] = strings.ToLower(f)
	}
	return r
}

// Sensitive reports whether values stored under key must be masked.
func (r *Redactor) Sensitive(key string) bool {
	k := strings.ToLower(key)
	if _, hit := r.exact[k]; hit {
		return true
	}
	for _, f := range r.fragments {
		if strings.Contains(k, f) {
			return true
		}
	}
	return false
}

// Value returns a deep copy of a decoded JSON value with every sensitive map
// key replaced by Placeholder, recursing through nested maps and slices.
// Scalars pass through unchanged and the input is never mutated.
func (r *Redactor) Value(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if r.Sensitive(k) {
				out[k] = Placeholder
				continue
			}
			out[k] = r.Value(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = r.Value(item)
		}
		return out
	default:
		return v
	}
}

// ParseBody prepares a raw HTTP body for logging without masking anything:
// an empty body returns nil, valid JSON returns the decoded value, and any
// other body returns its text truncated to limit bytes (a non-positive limit
// disables truncation). Only for payloads known to carry no secrets; callers
// with sensitive payloads use Redactor.Body instead.
func ParseBody(body []byte, limit int) any {
	if len(body) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return truncate(string(body), limit)
	}
	return v
}

// Body prepares a raw HTTP body for logging: like ParseBody, but decoded
// JSON is additionally passed through Value so sensitive keys are masked.
func (r *Redactor) Body(body []byte, limit int) any {
	return r.Value(ParseBody(body, limit))
}

// BodyWrapped is a Body variant that always logs something JSON-shaped:
// decoded JSON is masked via Value, while any body that fails to parse,
// including an empty body, is wrapped as a map under the key "raw" with its
// text truncated to limit bytes (a non-positive limit disables truncation).
func (r *Redactor) BodyWrapped(body []byte, limit int) any {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return map[string]any{"raw": truncate(string(body), limit)}
	}
	return r.Value(v)
}

// Form flattens a url.Values body into a loggable map with sensitive keys
// masked. Single-value fields flatten to the plain string, repeated fields
// to a []any of strings, and a key with no values to an empty string.
func (r *Redactor) Form(form url.Values) map[string]any {
	out := make(map[string]any, len(form))
	for k, vals := range form {
		if r.Sensitive(k) {
			out[k] = Placeholder
			continue
		}
		switch len(vals) {
		case 0:
			out[k] = ""
		case 1:
			out[k] = vals[0]
		default:
			list := make([]any, len(vals))
			for i, val := range vals {
				list[i] = val
			}
			out[k] = list
		}
	}
	return out
}

// FormJoined flattens a url.Values body into a string-valued map with
// sensitive keys masked; repeated fields are joined with commas.
func (r *Redactor) FormJoined(form url.Values) map[string]string {
	out := make(map[string]string, len(form))
	for k, vals := range form {
		if r.Sensitive(k) {
			out[k] = Placeholder
			continue
		}
		out[k] = strings.Join(vals, ",")
	}
	return out
}

// truncate caps s at limit bytes; a non-positive limit leaves s unchanged.
func truncate(s string, limit int) string {
	if limit > 0 && len(s) > limit {
		return s[:limit]
	}
	return s
}
