package redact

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitive(t *testing.T) {
	t.Parallel()
	exact := Keys("password", "api_key", "Token")
	subs := Substrings("token", "secret")

	tests := []struct {
		name string
		r    *Redactor
		key  string
		want bool
	}{
		{"exact match", exact, "password", true},
		{"exact case-insensitive key", exact, "PASSWORD", true},
		{"exact case-insensitive name", exact, "token", true},
		{"exact no substring match", exact, "api_token", false},
		{"exact miss", exact, "user", false},
		{"substring match", subs, "api_token", true},
		{"substring case-insensitive", subs, "ClientSecret", true},
		{"substring exact key", subs, "token", true},
		{"substring miss", subs, "username", false},
		{"zero value matches nothing", &Redactor{}, "password", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.r.Sensitive(tt.key))
		})
	}
}

func TestValue(t *testing.T) {
	t.Parallel()
	r := Keys("password", "api_key", "signature")

	tests := []struct {
		name string
		in   any
		want any
	}{
		{"nil", nil, nil},
		{"string scalar", "plain", "plain"},
		{"number scalar", float64(7), float64(7)},
		{"bool scalar", true, true},
		{
			"flat map",
			map[string]any{"user": "jane", "password": "hunter2"},
			map[string]any{"user": "jane", "password": Placeholder},
		},
		{
			"case-insensitive key",
			map[string]any{"PassWord": "x", "API_KEY": "k"},
			map[string]any{"PassWord": Placeholder, "API_KEY": Placeholder},
		},
		{
			"nested map",
			map[string]any{"outer": map[string]any{"signature": "sig", "ok": "y"}},
			map[string]any{"outer": map[string]any{"signature": Placeholder, "ok": "y"}},
		},
		{
			"array of maps",
			[]any{map[string]any{"password": "x"}, "plain", float64(1)},
			[]any{map[string]any{"password": Placeholder}, "plain", float64(1)},
		},
		{
			"map with array value",
			map[string]any{"list": []any{map[string]any{"api_key": "k"}}},
			map[string]any{"list": []any{map[string]any{"api_key": Placeholder}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, r.Value(tt.in))
		})
	}
}

func TestValueDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	r := Keys("password")
	in := map[string]any{
		"password": "hunter2",
		"nested":   map[string]any{"password": "deep"},
		"list":     []any{map[string]any{"password": "inlist"}},
	}
	out := r.Value(in).(map[string]any)

	assert.Equal(t, Placeholder, out["password"])
	assert.Equal(t, "hunter2", in["password"])
	assert.Equal(t, "deep", in["nested"].(map[string]any)["password"])
	assert.Equal(t, "inlist", in["list"].([]any)[0].(map[string]any)["password"])
}

func TestParseBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		body  []byte
		limit int
		want  any
	}{
		{"nil body", nil, 2048, nil},
		{"empty body", []byte{}, 2048, nil},
		{"valid JSON object", []byte(`{"token":"abc","x":1}`), 2048, map[string]any{"token": "abc", "x": float64(1)}},
		{"valid JSON array", []byte(`[1,"a"]`), 2048, []any{float64(1), "a"}},
		{"non-JSON short", []byte("not json"), 2048, "not json"},
		{"non-JSON truncated", []byte(strings.Repeat("a", 100)), 10, strings.Repeat("a", 10)},
		{"non-JSON zero limit keeps all", []byte(strings.Repeat("a", 100)), 0, strings.Repeat("a", 100)},
		{"non-JSON negative limit keeps all", []byte(strings.Repeat("a", 100)), -1, strings.Repeat("a", 100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ParseBody(tt.body, tt.limit))
		})
	}
}

func TestBody(t *testing.T) {
	t.Parallel()
	r := Keys("signature", "token")

	tests := []struct {
		name  string
		body  []byte
		limit int
		want  any
	}{
		{"empty body is nil", nil, 2048, nil},
		{"non-JSON is truncated string", []byte(strings.Repeat("x", 3000)), 2048, strings.Repeat("x", 2048)},
		{
			"JSON is masked",
			[]byte(`{"signature":"abc","x":1}`),
			2048,
			map[string]any{"signature": Placeholder, "x": float64(1)},
		},
		{
			"nested JSON is masked",
			[]byte(`{"a":{"token":"t"},"b":[{"signature":"s"}]}`),
			2048,
			map[string]any{
				"a": map[string]any{"token": Placeholder},
				"b": []any{map[string]any{"signature": Placeholder}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, r.Body(tt.body, tt.limit))
		})
	}
}

func TestBodyWrapped(t *testing.T) {
	t.Parallel()
	r := Keys("password", "epp")

	tests := []struct {
		name  string
		body  []byte
		limit int
		want  any
	}{
		{"empty body wraps empty raw", nil, 512, map[string]any{"raw": ""}},
		{"non-JSON wraps raw", []byte("<html>"), 512, map[string]any{"raw": "<html>"}},
		{
			"non-JSON wraps truncated raw",
			[]byte(strings.Repeat("x", 600)),
			512,
			map[string]any{"raw": strings.Repeat("x", 512)},
		},
		{
			"JSON is masked",
			[]byte(`{"password":"p","name":"Jane","list":[{"epp":"E1"}]}`),
			512,
			map[string]any{
				"password": Placeholder,
				"name":     "Jane",
				"list":     []any{map[string]any{"epp": Placeholder}},
			},
		},
		{"JSON null stays nil", []byte(`null`), 512, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, r.BodyWrapped(tt.body, tt.limit))
		})
	}
}

func TestForm(t *testing.T) {
	t.Parallel()
	r := Keys("secret", "password")

	got := r.Form(url.Values{
		"email":    {"jane@example.test"},
		"secret":   {"tok"},
		"password": {"a", "b"},
		"tags":     {"x", "y"},
		"none":     {},
	})

	want := map[string]any{
		"email":    "jane@example.test",
		"secret":   Placeholder,
		"password": Placeholder,
		"tags":     []any{"x", "y"},
		"none":     "",
	}
	assert.Equal(t, want, got)
}

func TestFormEmpty(t *testing.T) {
	t.Parallel()
	r := Keys("secret")
	got := r.Form(url.Values{})
	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestFormJoined(t *testing.T) {
	t.Parallel()
	r := Substrings("password", "session")

	got := r.FormJoined(url.Values{
		"user":         {"alice"},
		"password":     {"x"},
		"session_id":   {"s"},
		"ns":           {"a", "b"},
		"empty":        {},
		"cookiesecret": {"unmatched"},
	})

	want := map[string]string{
		"user":         "alice",
		"password":     Placeholder,
		"session_id":   Placeholder,
		"ns":           "a,b",
		"empty":        "",
		"cookiesecret": "unmatched",
	}
	assert.Equal(t, want, got)
}
