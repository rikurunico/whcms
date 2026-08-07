package directadmin

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

func TestSchemeAndPort(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ports.ServerConfig
		wantScheme string
		wantPort   int
	}{
		{"defaults", ports.ServerConfig{Hostname: "da"}, "http", 2222},
		{"ssl", ports.ServerConfig{Hostname: "da", UseSSL: true}, "https", 2222},
		{"custom port", ports.ServerConfig{Hostname: "da", Port: 3333}, "http", 3333},
		{"negative port defaults", ports.ServerConfig{Hostname: "da", Port: -1}, "http", 2222},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme, port := schemeAndPort(tt.cfg)
			assert.Equal(t, tt.wantScheme, scheme)
			assert.Equal(t, tt.wantPort, port)
		})
	}
}

func TestMapAPIError(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		text     string
		details  string
		wantCode apperr.Code
		wantMsg  string
	}{
		{"duplicate", "CMD_API_ACCOUNT_USER", "Error Creating User", "That username already exists", apperr.CodeConflict, "already exists"},
		{"missing user", "CMD_API_SELECT_USERS", "Error", "user bob does not exist", apperr.CodeNotFound, "does not exist"},
		{"no such user", "CMD_API_SELECT_USERS", "Error", "no such user bob", apperr.CodeNotFound, "no such user"},
		{"generic", "CMD_API_ACCOUNT_USER", "Error Creating User", "forced failure", apperr.CodeExternal, "forced failure"},
		{"login failed", "CMD_API_ACCOUNT_USER", "Login failed", "invalid credentials", apperr.CodeExternal, "Login failed"},
		{"text only", "CMD_API_ACCOUNT_USER", "Something broke", "", apperr.CodeExternal, "Something broke"},
		{"details only", "CMD_API_ACCOUNT_USER", "", "just details", apperr.CodeExternal, "just details"},
		{"both empty", "CMD_API_ACCOUNT_USER", "", "", apperr.CodeExternal, "api error (error=1)"},
		{"reserved username on create", "CMD_API_ACCOUNT_USER", "Error Creating User", "That username is reserved", apperr.CodeConflict, "reserved"},
		{"reserved text on unrelated command stays external", "CMD_API_SELECT_USERS", "Error", "That username is reserved", apperr.CodeExternal, "reserved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := mapAPIError(tt.command, tt.text, tt.details)
			assert.Equal(t, tt.wantCode, e.Code)
			assert.Contains(t, e.Message, tt.wantMsg)
			assert.True(t, strings.HasPrefix(e.Message, "directadmin: "))
		})
	}
}

func TestParseBodyLegacy(t *testing.T) {
	vals, err := parseBody("text/plain; charset=utf-8", []byte("error=0&text=Success&details=ok\n"))
	require.NoError(t, err)
	assert.Equal(t, "0", vals.Get("error"))
	assert.Equal(t, "Success", vals.Get("text"))
}

func TestParseBodyLegacyPartialInvalid(t *testing.T) {
	// Invalid escape in one pair: url.ParseQuery keeps the valid pairs.
	vals, err := parseBody("text/plain", []byte("error=0&bad=%zz"))
	require.NoError(t, err)
	assert.Equal(t, "0", vals.Get("error"))
}

func TestParseBodyLegacyFullyInvalid(t *testing.T) {
	_, err := parseBody("text/plain", []byte("%zz"))
	require.Error(t, err)
}

func TestParseBodyJSON(t *testing.T) {
	vals, err := parseBody("application/JSON", []byte(`{"error":1,"text":"Error","ok":true,"n":null,"f":2.5}`))
	require.NoError(t, err)
	assert.Equal(t, "1", vals.Get("error"))
	assert.Equal(t, "Error", vals.Get("text"))
	assert.Equal(t, "true", vals.Get("ok"))
	assert.True(t, vals.Has("n"))
	assert.Equal(t, "", vals.Get("n"))
	assert.Equal(t, "2.5", vals.Get("f"))
}

func TestValuesFromJSONNested(t *testing.T) {
	vals, err := valuesFromJSON([]byte(`{"list":[1,2],"obj":{"a":"b"}}`))
	require.NoError(t, err)
	assert.JSONEq(t, `[1,2]`, vals.Get("list"))
	assert.JSONEq(t, `{"a":"b"}`, vals.Get("obj"))
}

func TestValuesFromJSONInvalid(t *testing.T) {
	_, err := valuesFromJSON([]byte(`[1,2]`)) // not an object
	require.Error(t, err)
}

func TestIsSecretKey(t *testing.T) {
	for _, k := range []string{"passwd", "passwd2", "password", "PASSWORD", "api_key", "apiKey", "APIKEY", "token", "api_token", "signature", "Authorization"} {
		assert.True(t, isSecretKey(k), k)
	}
	for _, k := range []string{"username", "domain", "email", "package", "ip", "suspended"} {
		assert.False(t, isSecretKey(k), k)
	}
}

func TestRedactValues(t *testing.T) {
	out := redactValues(url.Values{
		"username": {"alice"},
		"passwd":   {"secret"},
		"passwd2":  {"secret"},
	})
	assert.Equal(t, "alice", out["username"])
	assert.Equal(t, "[REDACTED]", out["passwd"])
	assert.Equal(t, "[REDACTED]", out["passwd2"])
}

func TestRequestLogIncludesHost(t *testing.T) {
	out := requestLog(ports.ServerConfig{Hostname: "da.example.com"}, url.Values{"user": {"alice"}})
	assert.Equal(t, "da.example.com", out["host"])
	assert.Equal(t, "alice", out["user"])
}

func TestTruncated(t *testing.T) {
	long := strings.Repeat("x", maxLoggedBody+100)
	assert.Len(t, truncated([]byte(long)), maxLoggedBody)
	assert.Equal(t, "short", truncated([]byte("short")))
}

func TestParseDABool(t *testing.T) {
	for _, v := range []string{"yes", "YES", " y ", "1", "true", "on"} {
		assert.True(t, parseDABool(v), v)
	}
	for _, v := range []string{"no", "", "0", "false", "off", "whatever"} {
		assert.False(t, parseDABool(v), v)
	}
}

func TestNilDependencyFallbacks(t *testing.T) {
	// nopLogger and systemClock are the nil-dependency fallbacks used by New.
	assert.NotPanics(t, func() { nopLogger{}.Log(nil, ports.IntegrationCall{}) }) //nolint:staticcheck // nil ctx fine for no-op
	assert.False(t, systemClock{}.Now().IsZero())
}

func TestBasicPassword(t *testing.T) {
	assert.Equal(t, "pw", basicPassword(ports.ServerConfig{Password: "pw", APIToken: "tok"}))
	assert.Equal(t, "tok", basicPassword(ports.ServerConfig{APIToken: "tok"}))
	assert.Equal(t, "", basicPassword(ports.ServerConfig{}))
}
