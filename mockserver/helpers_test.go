package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	testMerchantCode = "DEMO"
	testAPIKey       = "secretkey"

	// Precomputed signature vectors (see PRD §8.1 formulas):
	//   MD5("DEMO" + "ORD-1" + "150000" + "secretkey")               - inquiry
	//   SHA256("DEMO" + "150000" + "2026-07-03 10:00:00" + "secretkey") - get payment method
	//   MD5("DEMO" + "150000" + "ORD-1" + "secretkey")               - callback
	//   MD5("DEMO" + "ORD-1" + "secretkey")                          - check transaction
	vecInquirySig   = "5f1e64d6d34b3a84fece16fadeea9305"
	vecGetMethodSig = "87286f4b1a2b6ba9ccaf83259b86278237c4d07f07038739890aea4010f5c9b0"
	vecCallbackSig  = "8e5d1dff3a33d96e39bd98ef17e269da"
	vecStatusSig    = "6427ea79abe680b3bdf1f6ebff24b6d7"
	vecDatetime     = "2026-07-03 10:00:00"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // silence per-request logging in tests
	os.Exit(m.Run())
}

// newTestServer boots the mock behind httptest and points BaseURL at it so
// paymentUrl / SSO URLs are actually reachable during tests.
func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer(Config{
		Port:               "9090",
		BaseURL:            "http://localhost:9090",
		DuitkuMerchantCode: testMerchantCode,
		DuitkuAPIKey:       testAPIKey,
	})
	s.sleep = func(time.Duration) {} // no real backoff waits in tests
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	s.cfg.BaseURL = ts.URL
	return s, ts
}

// postJSON POSTs body as JSON and decodes the JSON response into a map.
func postJSON(t *testing.T, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp, decodeBody(t, resp)
}

// jsonReq issues an arbitrary-method JSON request with optional Basic auth.
func jsonReq(t *testing.T, method, url string, body any, basicUser, basicPass string) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if basicUser != "" || basicPass != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp, decodeBody(t, resp)
}

// formReq issues an arbitrary-method application/x-www-form-urlencoded
// request with optional Basic auth (used by the RDash mock, which - like the
// real Dewabiz API - takes form fields rather than JSON).
func formReq(t *testing.T, method, u string, form url.Values, basicUser, basicPass string) (*http.Response, map[string]any) {
	t.Helper()
	var reader io.Reader
	if form != nil {
		reader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, u, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if basicUser != "" || basicPass != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, u, err)
	}
	return resp, decodeBody(t, resp)
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("decode JSON body %q: %v", raw, err)
		}
	}
	return m
}

func wantStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func wantField(t *testing.T, m map[string]any, key string, want any) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Fatalf("field %q missing in %v", key, m)
	}
	// JSON numbers decode as float64; normalize for comparison.
	if wi, isInt := want.(int); isInt {
		want = float64(wi)
	}
	if got != want {
		t.Fatalf("field %q = %v, want %v", key, got, want)
	}
}

// wantDuitkuBadSignature asserts the canonical 400 invalid-signature body.
func wantDuitkuBadSignature(t *testing.T, resp *http.Response, m map[string]any) {
	t.Helper()
	wantStatus(t, resp, http.StatusBadRequest)
	wantField(t, m, "statusCode", "XX")
	wantField(t, m, "statusMessage", "invalid signature")
}

// bodyString reads the full response body as a string.
func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(raw)
}

// noRedirectClient never follows redirects, so Location can be asserted.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// formValues parses a legacy URL-encoded (DirectAdmin style) response body.
func formValues(t *testing.T, body string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(strings.TrimSpace(body))
	if err != nil {
		t.Fatalf("parse url-encoded body %q: %v", body, err)
	}
	return v
}
