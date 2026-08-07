package config_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookupFrom(m map[string]string) config.Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":       "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable",
		"JWT_SECRET":         "test-secret",
		"APP_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32)),
	}
}

func TestLoadDefaults(t *testing.T) {
	c, err := config.LoadFrom(lookupFrom(validEnv()))
	require.NoError(t, err)

	assert.Equal(t, "development", c.AppEnv)
	assert.True(t, c.IsDevelopment())
	assert.False(t, c.IsProduction())
	assert.Equal(t, 8080, c.AppPort)
	assert.Equal(t, "http://localhost:8080", c.AppBaseURL)
	assert.Equal(t, "http://localhost:5173", c.FrontendURL)
	assert.Equal(t, "localhost:6379", c.RedisAddr)
	assert.Equal(t, 0, c.RedisDB)
	assert.Equal(t, "http://localhost:9000", c.RustFSEndpoint)
	assert.Equal(t, "whmcs", c.RustFSBucket)
	assert.False(t, c.RustFSUseSSL)
	assert.Equal(t, "sandbox", c.DuitkuEnv)
	assert.Equal(t, "https://sandbox.duitku.com", c.DuitkuBaseURL, "derived from sandbox env")
	assert.Equal(t, "", c.DuitkuBaseURLExplicit, "no explicit DUITKU_BASE_URL was set")
	assert.Equal(t, "https://api.dewabiz.co.id/v1", c.RDashBaseURL)
	assert.Equal(t, "log", c.MailDriver)
	assert.Equal(t, 587, c.SMTPPort)
	assert.Equal(t, 10, c.WorkerConcurrency)
	assert.Equal(t, "admin@example.com", c.AdminAlertEmail)
	assert.Equal(t, "", c.AdminAlertWebhookURL)
	assert.Len(t, c.EncryptionKey, 32)
}

func TestLoadExplicitValues(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "production"
	env["APP_PORT"] = "9999"
	env["REDIS_DB"] = "3"
	env["RUSTFS_USE_SSL"] = "true"
	env["DUITKU_ENV"] = "production"
	env["MAIL_DRIVER"] = "http"
	env["MAIL_HTTP_URL"] = "http://localhost:9090/mail/send"
	env["WORKER_CONCURRENCY"] = "25"
	env["ADMIN_ALERT_WEBHOOK_URL"] = "https://hooks.example.test/alert"

	c, err := config.LoadFrom(lookupFrom(env))
	require.NoError(t, err)
	assert.True(t, c.IsProduction())
	assert.Equal(t, 9999, c.AppPort)
	assert.Equal(t, 3, c.RedisDB)
	assert.True(t, c.RustFSUseSSL)
	assert.Equal(t, "https://passport.duitku.com", c.DuitkuBaseURL, "derived from production env")
	assert.Equal(t, "http", c.MailDriver)
	assert.Equal(t, 25, c.WorkerConcurrency)
	assert.Equal(t, "https://hooks.example.test/alert", c.AdminAlertWebhookURL)
}

func TestLoadExplicitDuitkuBaseURLWins(t *testing.T) {
	env := validEnv()
	env["DUITKU_BASE_URL"] = "http://localhost:9090"
	c, err := config.LoadFrom(lookupFrom(env))
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", c.DuitkuBaseURL)
	assert.Equal(t, "http://localhost:9090", c.DuitkuBaseURLExplicit,
		"raw env pin preserved separately so the live resolver can tell it apart from a mode-derived default")
}

func TestLoadMissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		drop    string
		wantMsg string
	}{
		{"missing db", "DATABASE_URL", "DATABASE_URL is required"},
		{"missing jwt", "JWT_SECRET", "JWT_SECRET is required"},
		{"missing enc key", "APP_ENCRYPTION_KEY", "APP_ENCRYPTION_KEY is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			delete(env, tt.drop)
			_, err := config.LoadFrom(lookupFrom(env))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestLoadInvalidEncryptionKey(t *testing.T) {
	t.Run("not base64", func(t *testing.T) {
		env := validEnv()
		env["APP_ENCRYPTION_KEY"] = "!!!not-base64!!!"
		_, err := config.LoadFrom(lookupFrom(env))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid base64")
	})
	t.Run("wrong length", func(t *testing.T) {
		env := validEnv()
		env["APP_ENCRYPTION_KEY"] = base64.StdEncoding.EncodeToString(make([]byte, 16))
		_, err := config.LoadFrom(lookupFrom(env))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "32 bytes")
	})
}

func TestLoadInvalidValues(t *testing.T) {
	env := validEnv()
	env["APP_PORT"] = "not-a-number"
	env["RUSTFS_USE_SSL"] = "maybe"
	env["MAIL_DRIVER"] = "pigeon"
	_, err := config.LoadFrom(lookupFrom(env))
	require.Error(t, err)
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "APP_PORT"), msg)
	assert.True(t, strings.Contains(msg, "RUSTFS_USE_SSL"), msg)
	assert.True(t, strings.Contains(msg, "MAIL_DRIVER"), msg)
}

func TestLoadRejectsKnownDevSecretsInProduction(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "production"
	env["JWT_SECRET"] = "dev-jwt-secret-change-me"
	env["APP_ENCRYPTION_KEY"] = "ZGV2LWVuY3J5cHRpb24ta2V5LTAxMjM0NTY3ODlhYmM="

	_, err := config.LoadFrom(lookupFrom(env))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET is a publicly-known development value")
	assert.Contains(t, err.Error(), "APP_ENCRYPTION_KEY is a publicly-known development value")
}

func TestLoadAllowsKnownDevSecretsOutsideProduction(t *testing.T) {
	env := validEnv()
	env["APP_ENV"] = "development"
	env["JWT_SECRET"] = "dev-jwt-secret-change-me"
	env["APP_ENCRYPTION_KEY"] = "ZGV2LWVuY3J5cHRpb24ta2V5LTAxMjM0NTY3ODlhYmM="

	c, err := config.LoadFrom(lookupFrom(env))
	require.NoError(t, err)
	assert.Equal(t, "dev-jwt-secret-change-me", c.JWTSecret)
}

func TestLoadPanelTLSInsecureSkipVerify(t *testing.T) {
	c, err := config.LoadFrom(lookupFrom(validEnv()))
	require.NoError(t, err)
	assert.False(t, c.PanelTLSInsecureSkipVerify, "TLS verification must be on by default")

	env := validEnv()
	env["PANEL_TLS_INSECURE_SKIP_VERIFY"] = "true"
	c, err = config.LoadFrom(lookupFrom(env))
	require.NoError(t, err)
	assert.True(t, c.PanelTLSInsecureSkipVerify)
}

func TestSMTPDefaults(t *testing.T) {
	c, err := config.LoadFrom(lookupFrom(validEnv()))
	require.NoError(t, err)
	assert.Equal(t, "auto", c.SMTPEncryption)
	assert.Equal(t, "auto", c.SMTPAuth)
	assert.Equal(t, 20*time.Second, c.SMTPTimeout)
	assert.False(t, c.SMTPInsecureSkipVerify)
}

func TestSMTPExplicitValues(t *testing.T) {
	env := validEnv()
	env["MAIL_DRIVER"] = "smtp"
	env["SMTP_HOST"] = "mail.example.co.id"
	env["SMTP_PORT"] = "465"
	env["SMTP_ENCRYPTION"] = "TLS" // case-insensitive
	env["SMTP_AUTH"] = "LOGIN"
	env["SMTP_TIMEOUT_SECONDS"] = "45"
	env["SMTP_INSECURE_SKIP_VERIFY"] = "true"

	c, err := config.LoadFrom(lookupFrom(env))
	require.NoError(t, err)
	assert.Equal(t, "tls", c.SMTPEncryption)
	assert.Equal(t, "login", c.SMTPAuth)
	assert.Equal(t, 45*time.Second, c.SMTPTimeout)
	assert.True(t, c.SMTPInsecureSkipVerify)
}

// MAIL_DRIVER=smtp without a host must fail at boot, not on the first email.
func TestSMTPDriverRequiresHost(t *testing.T) {
	env := validEnv()
	env["MAIL_DRIVER"] = "smtp"

	_, err := config.LoadFrom(lookupFrom(env))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP_HOST is required")
}

func TestSMTPInvalidValuesRejected(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"port out of range", map[string]string{
			"MAIL_DRIVER": "smtp", "SMTP_HOST": "h", "SMTP_PORT": "70000",
		}, "SMTP_PORT"},
		{"unknown encryption", map[string]string{"SMTP_ENCRYPTION": "ssl-ish"}, "SMTP_ENCRYPTION"},
		{"unknown auth", map[string]string{"SMTP_AUTH": "kerberos"}, "SMTP_AUTH"},
		{"zero timeout", map[string]string{"SMTP_TIMEOUT_SECONDS": "0"}, "SMTP_TIMEOUT_SECONDS"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			for k, v := range tc.env {
				env[k] = v
			}
			_, err := config.LoadFrom(lookupFrom(env))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
