package directadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// maxResponseBytes caps how much of a DirectAdmin response is read.
const maxResponseBytes = 1 << 20 // 1 MiB

// maxLoggedBody caps raw (unparseable) bodies persisted to integration logs.
const maxLoggedBody = 2048

// call performs one DirectAdmin command and interprets the response.
//
// expectDump marks commands whose success response is a raw url-encoded dump
// without an `error=0` marker (CMD_API_SHOW_USER_CONFIG). For every other
// command a response without an `error` key is treated as unexpected.
func (c *Client) call(ctx context.Context, s ports.ServerConfig, method, command string, params url.Values, expectDump bool) (url.Values, error) {
	endpoint := "/" + command
	start := c.clk.Now()

	logCall := func(status int, success bool, response any, errMsg string) {
		c.il.Log(ctx, ports.IntegrationCall{
			Provider:   providerName,
			Endpoint:   endpoint,
			Method:     method,
			StatusCode: status,
			Success:    success,
			LatencyMS:  c.clk.Now().Sub(start).Milliseconds(),
			Request:    requestLog(s, params),
			Response:   response,
			Error:      errMsg,
		})
	}

	if !c.breakerAllow() {
		err := apperr.New(apperr.CodeExternal, "directadmin: circuit open, failing fast")
		logCall(0, false, nil, err.Message)
		return nil, err
	}

	resp, body, err := c.do(ctx, s, method, endpoint, params) //nolint:bodyclose // do() drains and closes the body
	if err != nil {
		c.breakerFailure()
		e := apperr.External(providerName, err)
		logCall(0, false, nil, e.Error())
		return nil, e
	}
	if resp.StatusCode >= 500 {
		c.breakerFailure()
		e := apperr.Newf(apperr.CodeExternal, "directadmin: server error: http %d", resp.StatusCode)
		logCall(resp.StatusCode, false, truncated(body), e.Message)
		return nil, e
	}
	c.breakerSuccess()

	vals, perr := parseBody(resp.Header.Get("Content-Type"), body)
	if perr != nil {
		e := apperr.New(apperr.CodeExternal, "directadmin: unparseable response").WithCause(perr)
		logCall(resp.StatusCode, false, truncated(body), e.Error())
		return nil, e
	}

	if vals.Has("error") && vals.Get("error") != "0" {
		e := mapAPIError(command, vals.Get("text"), vals.Get("details"))
		logCall(resp.StatusCode, false, redactValues(vals), e.Error())
		return nil, e
	}
	if resp.StatusCode >= 400 {
		e := apperr.Newf(apperr.CodeExternal, "directadmin: http %d", resp.StatusCode)
		logCall(resp.StatusCode, false, redactValues(vals), e.Message)
		return nil, e
	}
	if !expectDump && !vals.Has("error") {
		e := apperr.New(apperr.CodeExternal, "directadmin: unexpected response format")
		logCall(resp.StatusCode, false, truncated(body), e.Message)
		return nil, e
	}

	logCall(resp.StatusCode, true, redactValues(vals), "")
	return vals, nil
}

// do executes the HTTP request. GETs are retried up to cfg.MaxRetries on
// network errors and 5xx responses; POSTs get exactly one attempt. The
// response body is fully read (capped) and the connection closed before
// returning.
func (c *Client) do(ctx context.Context, s ports.ServerConfig, method, endpoint string, params url.Values) (*http.Response, []byte, error) {
	scheme, port := schemeAndPort(s)
	base := fmt.Sprintf("%s://%s:%d%s", scheme, s.Hostname, port, endpoint)

	attempts := 1
	if method == http.MethodGet {
		attempts += c.cfg.MaxRetries
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(c.cfg.RetryBackoff):
			}
		}

		req, err := c.newRequest(ctx, s, method, base, params)
		if err != nil {
			return nil, nil, err
		}
		hc := c.hc
		if s.UseSSL && c.cfg.TLSInsecureSkipVerify {
			hc = c.hcInsecure
		}
		resp, err := hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		_ = resp.Body.Close()
		if rerr != nil {
			lastErr = rerr
			continue
		}
		if resp.StatusCode >= 500 && i < attempts-1 {
			lastErr = fmt.Errorf("http %d from directadmin", resp.StatusCode)
			continue
		}
		return resp, body, nil
	}
	return nil, nil, lastErr
}

// newRequest builds the command request with Basic auth. GET commands carry
// params in the query string; POST commands as a form body.
func (c *Client) newRequest(ctx context.Context, s ports.ServerConfig, method, base string, params url.Values) (*http.Request, error) {
	var (
		reqURL = base
		body   io.Reader
	)
	if method == http.MethodGet {
		reqURL += "?" + params.Encode()
	} else {
		body = strings.NewReader(params.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.SetBasicAuth(s.Username, basicPassword(s))
	return req, nil
}

// basicPassword picks the Basic-auth secret: the server password, falling
// back to the API token (DirectAdmin login keys are passed as the password).
func basicPassword(s ports.ServerConfig) string {
	if s.Password != "" {
		return s.Password
	}
	return s.APIToken
}

// schemeAndPort resolves the URL scheme from use_ssl and defaults the port
// to 2222 when unset.
func schemeAndPort(s ports.ServerConfig) (string, int) {
	scheme := "http"
	if s.UseSSL {
		scheme = "https"
	}
	port := s.Port
	if port <= 0 {
		port = defaultPort
	}
	return scheme, port
}

// parseBody decodes a DirectAdmin response. Legacy URL-encoded is the primary
// format; JSON is tolerated when the Content-Type says so.
func parseBody(contentType string, body []byte) (url.Values, error) {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return valuesFromJSON(body)
	}
	vals, err := url.ParseQuery(strings.TrimSpace(string(body)))
	if err != nil && len(vals) == 0 {
		return nil, err
	}
	return vals, nil
}

// valuesFromJSON flattens a JSON object into url.Values so JSON responses
// share the legacy handling path. Scalars are stringified; nested values are
// re-encoded as JSON strings.
func valuesFromJSON(body []byte) (url.Values, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	vals := url.Values{}
	for k, v := range m {
		switch t := v.(type) {
		case string:
			vals.Set(k, t)
		case float64:
			vals.Set(k, strconv.FormatFloat(t, 'f', -1, 64))
		case bool:
			vals.Set(k, strconv.FormatBool(t))
		case nil:
			vals.Set(k, "")
		default:
			b, err := json.Marshal(t)
			if err != nil {
				return nil, err
			}
			vals.Set(k, string(b))
		}
	}
	return vals, nil
}

// mapAPIError converts an `error=1` response into an apperr. Known texts map
// to NOT_FOUND / CONFLICT; everything else is EXTERNAL. command is the
// CMD_API_* endpoint that produced the response, used to scope
// account-creation-specific text matches so they can't misfire on unrelated
// commands.
func mapAPIError(command, text, details string) *apperr.Error {
	text, details = strings.TrimSpace(text), strings.TrimSpace(details)
	msg := text
	switch {
	case msg == "":
		msg = details
	case details != "":
		msg += ": " + details
	}
	if msg == "" {
		msg = "api error (error=1)"
	}
	msg = "directadmin: " + msg

	lower := strings.ToLower(text + " " + details)
	switch {
	case strings.Contains(lower, "already exist"):
		return apperr.New(apperr.CodeConflict, msg)
	case strings.Contains(lower, "does not exist"), strings.Contains(lower, "no such user"):
		return apperr.New(apperr.CodeNotFound, msg)
	case command == "CMD_API_ACCOUNT_USER" && strings.Contains(lower, "reserved"):
		// DirectAdmin refuses certain system-significant usernames outright
		// (mirrors WHM's "reserved username" rejection) - not a collision
		// with another account, but ProvisionCreate's conflict handling
		// already does exactly what this needs: confirm no account was
		// actually created (AccountInfo returns NotFound) then retry under
		// domain.DisambiguateUsername instead of proposing the same doomed
		// username forever.
		return apperr.New(apperr.CodeConflict, msg)
	default:
		return apperr.New(apperr.CodeExternal, msg)
	}
}

// Circuit-breaker-lite: consecutive transport failures open the circuit for
// BreakerCooldown; any successfully received (parsed-or-not, non-5xx)
// response closes it.

func (c *Client) breakerAllow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.clk.Now().Before(c.openUntil)
}

func (c *Client) breakerFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	if c.failures >= c.cfg.BreakerThreshold {
		c.openUntil = c.clk.Now().Add(c.cfg.BreakerCooldown)
	}
}

func (c *Client) breakerSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
	c.openUntil = time.Time{}
}

// Integration-log redaction (CONTRACTS.md §3)

var secretKeySubstrings = []string{
	"passwd", "password", "api_key", "apikey", "token", "signature", "authorization",
}

// isSecretKey reports whether a request/response key must be redacted.
func isSecretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range secretKeySubstrings {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// redactValues flattens url.Values to a loggable map with secrets redacted.
func redactValues(v url.Values) map[string]string {
	out := make(map[string]string, len(v))
	for k := range v {
		if isSecretKey(k) {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v.Get(k)
	}
	return out
}

// requestLog builds the redacted request payload for the integration log.
func requestLog(s ports.ServerConfig, params url.Values) map[string]string {
	out := redactValues(params)
	out["host"] = s.Hostname
	return out
}

// truncated caps a raw body for logging.
func truncated(body []byte) string {
	if len(body) > maxLoggedBody {
		body = body[:maxLoggedBody]
	}
	return string(body)
}
