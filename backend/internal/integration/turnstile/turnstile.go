// Package turnstile implements ports.CaptchaVerifier against Cloudflare
// Turnstile's siteverify API (mirrored by the local mockserver in dev/E2E).
//
// Endpoint:
//
//	POST https://challenges.cloudflare.com/turnstile/v0/siteverify
//	  (application/x-www-form-urlencoded: secret, response[, remoteip])
//	  -> {"success": bool, "error-codes": [...], "hostname": "...", ...}
//
// The secret key is ENV-only (TURNSTILE_SECRET_KEY); the on/off toggle and the
// public site key live in the settings store (security.captcha_*). Every HTTP
// attempt is recorded through ports.IntegrationLogger with the secret redacted.
// A siteverify {"success": false} (e.g. an expired or forged token) is a normal
// negative result (ok=false, err=nil); only transport/HTTP/decoding failures
// return an EXTERNAL apperr.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

const providerName = "turnstile"

const (
	// DefaultVerifyURL is Cloudflare's production siteverify endpoint.
	DefaultVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	defaultTimeout   = 10 * time.Second
	maxResponseBytes = 1 << 20
)

// Config carries the secret key and the siteverify URL. Wiring fills it from
// env (TURNSTILE_SECRET_KEY, TURNSTILE_VERIFY_URL); an empty VerifyURL falls
// back to Cloudflare's production endpoint.
type Config struct {
	SecretKey string
	VerifyURL string
}

// Client is the Turnstile siteverify adapter.
type Client struct {
	cfg   Config
	http  *http.Client
	ilog  ports.IntegrationLogger
	clock ports.Clock
}

// Compile-time interface assertion.
var _ ports.CaptchaVerifier = (*Client)(nil)

// New builds a Client. A nil httpClient gets a default 10s-timeout client; a
// caller-provided client without a timeout is shallow-copied and given one.
func New(cfg Config, httpClient *http.Client, ilog ports.IntegrationLogger, clock ports.Clock) *Client {
	if cfg.VerifyURL == "" {
		cfg.VerifyURL = DefaultVerifyURL
	}
	hc := &http.Client{Timeout: defaultTimeout}
	if httpClient != nil {
		cp := *httpClient
		if cp.Timeout == 0 {
			cp.Timeout = defaultTimeout
		}
		hc = &cp
	}
	return &Client{cfg: cfg, http: hc, ilog: ilog, clock: clock}
}

// verifyResponse is the siteverify JSON payload (subset).
type verifyResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	Hostname    string   `json:"hostname"`
	ChallengeTS string   `json:"challenge_ts"`
	Action      string   `json:"action"`
}

// Verify posts the token to the siteverify endpoint and reports whether it is
// valid. An empty token is invalid without a network call. remoteIP is optional.
func (c *Client) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" {
		return false, nil
	}

	form := url.Values{}
	form.Set("secret", c.cfg.SecretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	call := ports.IntegrationCall{
		Provider: providerName,
		Endpoint: "/turnstile/v0/siteverify",
		Method:   http.MethodPost,
		Request:  redactForm(form),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.VerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		call.Error = err.Error()
		c.ilog.Log(ctx, call)
		return false, apperr.External(providerName, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	start := c.clock.Now()
	resp, err := c.http.Do(req)
	call.LatencyMS = c.clock.Now().Sub(start).Milliseconds()
	if err != nil {
		call.Error = err.Error()
		c.ilog.Log(ctx, call)
		return false, apperr.External(providerName, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	call.StatusCode = resp.StatusCode
	call.Response = redactBody(body)
	if readErr != nil {
		call.Error = readErr.Error()
		c.ilog.Log(ctx, call)
		return false, apperr.External(providerName, readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.ilog.Log(ctx, call)
		return false, apperr.External(providerName, fmt.Errorf("HTTP %d", resp.StatusCode))
	}

	var out verifyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		call.Error = err.Error()
		c.ilog.Log(ctx, call)
		return false, apperr.External(providerName, fmt.Errorf("decode siteverify response: %w", err))
	}
	call.Success = out.Success
	if !out.Success && len(out.ErrorCodes) > 0 {
		call.Error = strings.Join(out.ErrorCodes, ",")
	}
	c.ilog.Log(ctx, call)
	return out.Success, nil
}
