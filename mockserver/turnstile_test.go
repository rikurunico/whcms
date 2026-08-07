package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func postSiteverify(t *testing.T, baseURL, secret, response string) map[string]any {
	t.Helper()
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", response)
	resp, err := http.Post(baseURL+"/turnstile/v0/siteverify",
		"application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	wantStatus(t, resp, http.StatusOK)
	var m map[string]any
	if err := json.Unmarshal([]byte(bodyString(t, resp)), &m); err != nil {
		t.Fatalf("decode siteverify: %v", err)
	}
	return m
}

func TestTurnstileVerify(t *testing.T) {
	_, ts := newTestServer(t)

	t.Run("always-pass secret succeeds", func(t *testing.T) {
		m := postSiteverify(t, ts.URL, turnstileSecretAlwaysPass, "any-token")
		wantField(t, m, "success", true)
	})

	t.Run("arbitrary secret succeeds when response present", func(t *testing.T) {
		m := postSiteverify(t, ts.URL, "some-prod-secret", "token")
		wantField(t, m, "success", true)
	})

	t.Run("always-fail secret fails", func(t *testing.T) {
		m := postSiteverify(t, ts.URL, turnstileSecretAlwaysFail, "token")
		wantField(t, m, "success", false)
	})

	t.Run("token-spent secret fails", func(t *testing.T) {
		m := postSiteverify(t, ts.URL, turnstileSecretTokenSpent, "token")
		wantField(t, m, "success", false)
	})

	t.Run("empty response fails", func(t *testing.T) {
		m := postSiteverify(t, ts.URL, turnstileSecretAlwaysPass, "")
		wantField(t, m, "success", false)
	})
}
