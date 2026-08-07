// Package config loads environment configuration (CONTRACTS.md §11) into a
// Config struct once at startup. No global state: pass Config by value / DI.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full application configuration.
type Config struct {
	// App
	AppEnv      string // development|production|test
	AppPort     int
	AppBaseURL  string
	FrontendURL string

	// Secrets
	JWTSecret     string
	EncryptionKey []byte // decoded 32 bytes from base64 APP_ENCRYPTION_KEY

	// Postgres
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// RustFS (S3)
	RustFSEndpoint  string
	RustFSAccessKey string
	RustFSSecretKey string
	RustFSBucket    string
	RustFSUseSSL    bool

	// Duitku
	DuitkuMerchantCode string
	DuitkuAPIKey       string
	DuitkuEnv          string // sandbox|production
	DuitkuBaseURL      string // final/effective value: DUITKU_BASE_URL if set, else derived from DuitkuEnv
	// DuitkuBaseURLExplicit is the RAW DUITKU_BASE_URL env var, before the
	// mode-derivation above collapses it - empty means "no explicit env
	// override was configured." Kept separate from DuitkuBaseURL so the live
	// resolver (internal/composition/build.go) can tell "an operator
	// explicitly pinned this (e.g. to the local mockserver) - that must win
	// over the mode setting" apart from "nothing was configured - the mode
	// setting should drive it live, no restart needed."
	DuitkuBaseURLExplicit string

	// RDash
	RDashResellerID string
	RDashAPIKey     string
	RDashBaseURL    string

	// Panels (cPanel/WHM, DirectAdmin)
	// PanelTLSInsecureSkipVerify disables TLS certificate verification when
	// talking to use_ssl panel servers. Default false (verify). Enable only
	// for panels stuck on self-signed certificates.
	PanelTLSInsecureSkipVerify bool

	// Turnstile (Cloudflare CAPTCHA). The secret key is ENV-only (never DB);
	// the on/off toggle and public site key live in the settings store
	// (security.captcha_*). VerifyURL defaults to Cloudflare's siteverify
	// endpoint and can point at the mockserver in dev/E2E.
	TurnstileSecretKey string
	TurnstileVerifyURL string

	// Mail
	MailDriver string // smtp|http|log
	SMTPHost   string
	SMTPPort   int
	SMTPUser   string
	SMTPPass   string
	// SMTPEncryption selects the transport security:
	//   auto     - derive from the port (465 => tls, otherwise starttls)
	//   tls      - implicit TLS from the first byte (SMTPS, usually port 465)
	//   starttls - plaintext connect, then a MANDATORY STARTTLS upgrade
	//   none     - no encryption at all (localhost relays / test harnesses only)
	// Default "auto". Never silently downgrades: an "starttls" server that does
	// not offer STARTTLS fails the send instead of leaking the credentials.
	SMTPEncryption string
	// SMTPAuth selects the SMTP AUTH mechanism: auto|plain|login|cram-md5|none.
	// Default "auto" (negotiate whatever the server advertises). "none" forces
	// an unauthenticated relay even when SMTP_USER is set.
	SMTPAuth string
	// SMTPTimeout bounds one dial+send attempt. Default 20s.
	SMTPTimeout time.Duration
	// SMTPInsecureSkipVerify disables TLS certificate verification for the
	// relay. Default false (verify). Enable only for relays stuck on
	// self-signed certificates (e.g. a cPanel box using its hostname cert).
	SMTPInsecureSkipVerify bool
	MailHTTPURL            string

	// Worker
	WorkerConcurrency int
	AdminAlertEmail   string
	// AdminAlertWebhookURL, if set, receives a JSON POST {"subject","message"}
	// alongside every admin_alert email (FR-NOTIF-005). Optional; empty disables it.
	AdminAlertWebhookURL string
}

// wellKnownDevSecrets are the JWT/encryption values committed to this
// repository for local development and E2E runs. Once the repo is public these
// are world-readable, so production refuses to start with any of them.
var wellKnownDevSecrets = map[string]bool{
	"dev-jwt-secret-change-me":                     true,
	"wXRw37nkVMhtvXKYd0msDNfPxAUJTEWb4a/4NtUssF8=": true,
	"2lhQfX4SZlR+sa1rt6gStNc8wIg1nDu27sngf0KcgW8=": true,
	"ZGV2LWVuY3J5cHRpb24ta2V5LTAxMjM0NTY3ODlhYmM=": true,
}

// IsDevelopment reports whether APP_ENV is development.
func (c Config) IsDevelopment() bool { return c.AppEnv == "development" }

// IsProduction reports whether APP_ENV is production.
func (c Config) IsProduction() bool { return c.AppEnv == "production" }

// Lookup resolves one env var; it mirrors os.LookupEnv for testability.
type Lookup func(key string) (string, bool)

// Load reads configuration from the process environment.
func Load() (Config, error) { return LoadFrom(os.LookupEnv) }

// LoadFrom reads configuration using the given lookup. Missing optional keys
// take documented defaults; invalid values return errors.
func LoadFrom(lookup Lookup) (Config, error) {
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && v != "" {
			return v
		}
		return def
	}
	var errs []error
	getInt := func(key string, def int) int {
		raw := get(key, "")
		if raw == "" {
			return def
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid integer %q", key, raw))
			return def
		}
		return n
	}
	getBool := func(key string, def bool) bool {
		raw := get(key, "")
		if raw == "" {
			return def
		}
		b, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid boolean %q", key, raw))
			return def
		}
		return b
	}

	c := Config{
		AppEnv:      get("APP_ENV", "development"),
		AppPort:     getInt("APP_PORT", 8080),
		AppBaseURL:  get("APP_BASE_URL", "http://localhost:8080"),
		FrontendURL: get("FRONTEND_URL", "http://localhost:5173"),

		JWTSecret: get("JWT_SECRET", ""),

		DatabaseURL: get("DATABASE_URL", ""),

		RedisAddr:     get("REDIS_ADDR", "localhost:6379"),
		RedisPassword: get("REDIS_PASSWORD", ""),
		RedisDB:       getInt("REDIS_DB", 0),

		RustFSEndpoint:  get("RUSTFS_ENDPOINT", "http://localhost:9000"),
		RustFSAccessKey: get("RUSTFS_ACCESS_KEY", ""),
		RustFSSecretKey: get("RUSTFS_SECRET_KEY", ""),
		RustFSBucket:    get("RUSTFS_BUCKET", "whmcs"),
		RustFSUseSSL:    getBool("RUSTFS_USE_SSL", false),

		DuitkuMerchantCode:    get("DUITKU_MERCHANT_CODE", ""),
		DuitkuAPIKey:          get("DUITKU_API_KEY", ""),
		DuitkuEnv:             get("DUITKU_ENV", "sandbox"),
		DuitkuBaseURL:         get("DUITKU_BASE_URL", ""),
		DuitkuBaseURLExplicit: get("DUITKU_BASE_URL", ""),

		RDashResellerID: get("RDASH_RESELLER_ID", ""),
		RDashAPIKey:     get("RDASH_API_KEY", ""),
		RDashBaseURL:    get("RDASH_BASE_URL", "https://api.dewabiz.co.id/v1"),

		TurnstileSecretKey: get("TURNSTILE_SECRET_KEY", ""),
		TurnstileVerifyURL: get("TURNSTILE_VERIFY_URL", "https://challenges.cloudflare.com/turnstile/v0/siteverify"),

		MailDriver:             get("MAIL_DRIVER", "log"),
		SMTPHost:               get("SMTP_HOST", ""),
		SMTPPort:               getInt("SMTP_PORT", 587),
		SMTPUser:               get("SMTP_USER", ""),
		SMTPPass:               get("SMTP_PASS", ""),
		SMTPEncryption:         strings.ToLower(get("SMTP_ENCRYPTION", "auto")),
		SMTPAuth:               strings.ToLower(get("SMTP_AUTH", "auto")),
		SMTPTimeout:            time.Duration(getInt("SMTP_TIMEOUT_SECONDS", 20)) * time.Second,
		SMTPInsecureSkipVerify: getBool("SMTP_INSECURE_SKIP_VERIFY", false),
		MailHTTPURL:            get("MAIL_HTTP_URL", ""),

		WorkerConcurrency:    getInt("WORKER_CONCURRENCY", 10),
		AdminAlertEmail:      get("ADMIN_ALERT_EMAIL", "admin@example.com"),
		AdminAlertWebhookURL: get("ADMIN_ALERT_WEBHOOK_URL", ""),
	}

	// Duitku base URL derives from env when unset.
	if c.DuitkuBaseURL == "" {
		if c.DuitkuEnv == "production" {
			c.DuitkuBaseURL = "https://passport.duitku.com"
		} else {
			c.DuitkuBaseURL = "https://sandbox.duitku.com"
		}
	}

	c.PanelTLSInsecureSkipVerify = getBool("PANEL_TLS_INSECURE_SKIP_VERIFY", false)

	// Required values.
	if c.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("DATABASE_URL is required"))
	}
	if c.JWTSecret == "" {
		errs = append(errs, fmt.Errorf("JWT_SECRET is required"))
	}

	// Refuse to boot production on the publicly-known dev/E2E secrets that
	// ship in this repository's compose files and scripts.
	if c.AppEnv == "production" {
		if wellKnownDevSecrets[c.JWTSecret] {
			errs = append(errs, fmt.Errorf("JWT_SECRET is a publicly-known development value; generate a real secret (openssl rand -base64 32)"))
		}
		if wellKnownDevSecrets[get("APP_ENCRYPTION_KEY", "")] {
			errs = append(errs, fmt.Errorf("APP_ENCRYPTION_KEY is a publicly-known development value; generate a real key (openssl rand -base64 32)"))
		}
	}

	// APP_ENCRYPTION_KEY: base64-encoded 32 bytes.
	rawKey := get("APP_ENCRYPTION_KEY", "")
	if rawKey == "" {
		errs = append(errs, fmt.Errorf("APP_ENCRYPTION_KEY is required"))
	} else {
		key, err := base64.StdEncoding.DecodeString(rawKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("APP_ENCRYPTION_KEY: invalid base64: %w", err))
		} else if len(key) != 32 {
			errs = append(errs, fmt.Errorf("APP_ENCRYPTION_KEY: must decode to 32 bytes, got %d", len(key)))
		} else {
			c.EncryptionKey = key
		}
	}

	switch c.MailDriver {
	case "smtp":
		// Fail loudly at boot rather than on the first queued email.
		if c.SMTPHost == "" {
			errs = append(errs, fmt.Errorf("SMTP_HOST is required when MAIL_DRIVER=smtp"))
		}
		if c.SMTPPort < 1 || c.SMTPPort > 65535 {
			errs = append(errs, fmt.Errorf("SMTP_PORT: must be 1-65535, got %d", c.SMTPPort))
		}
	case "http", "log":
	default:
		errs = append(errs, fmt.Errorf("MAIL_DRIVER: must be smtp|http|log, got %q", c.MailDriver))
	}

	switch c.SMTPEncryption {
	case "auto", "tls", "starttls", "none":
	default:
		errs = append(errs, fmt.Errorf("SMTP_ENCRYPTION: must be auto|tls|starttls|none, got %q", c.SMTPEncryption))
	}

	switch c.SMTPAuth {
	case "auto", "plain", "login", "cram-md5", "none":
	default:
		errs = append(errs, fmt.Errorf("SMTP_AUTH: must be auto|plain|login|cram-md5|none, got %q", c.SMTPAuth))
	}

	if c.SMTPTimeout <= 0 {
		errs = append(errs, fmt.Errorf("SMTP_TIMEOUT_SECONDS: must be > 0"))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", joinErrors(errs))
	}
	return c, nil
}

func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	msg := errs[0].Error()
	for _, e := range errs[1:] {
		msg += "; " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
