package turnstile

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

const testSecret = "1x0000000000000000000000000000000AA"

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *mocks.MockIntegrationLogger) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	ilog := &mocks.MockIntegrationLogger{}
	clock := &mocks.MockClock{FixedTime: time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)}
	return New(Config{SecretKey: testSecret, VerifyURL: srv.URL}, srv.Client(), ilog, clock), ilog
}

func TestNewDefaults(t *testing.T) {
	c := New(Config{SecretKey: "s"}, nil, &mocks.MockIntegrationLogger{}, &mocks.MockClock{})
	assert.Equal(t, DefaultVerifyURL, c.cfg.VerifyURL)
	assert.Equal(t, defaultTimeout, c.http.Timeout)

	// A caller client without a timeout is given one.
	c2 := New(Config{SecretKey: "s", VerifyURL: "http://x"}, &http.Client{}, &mocks.MockIntegrationLogger{}, &mocks.MockClock{})
	assert.Equal(t, defaultTimeout, c2.http.Timeout)
}

func TestVerifySuccess(t *testing.T) {
	var gotForm url.Values
	client, ilog := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"hostname":"example.com"}`))
	})

	ok, err := client.Verify(context.Background(), "good-token", "203.0.113.7")
	require.NoError(t, err)
	assert.True(t, ok)

	// Request carried secret + token + remoteip.
	assert.Equal(t, testSecret, gotForm.Get("secret"))
	assert.Equal(t, "good-token", gotForm.Get("response"))
	assert.Equal(t, "203.0.113.7", gotForm.Get("remoteip"))

	// The secret is redacted in the integration log; response recorded.
	require.Len(t, ilog.Calls, 1)
	logged := ilog.Calls[0]
	assert.Equal(t, providerName, logged.Provider)
	assert.True(t, logged.Success)
	reqMap, _ := logged.Request.(map[string]any)
	assert.Equal(t, redactedPlaceholder, reqMap["secret"])
	assert.Equal(t, "good-token", reqMap["response"])
}

func TestVerifyFailureReturnsFalseNoError(t *testing.T) {
	client, ilog := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	})

	ok, err := client.Verify(context.Background(), "bad-token", "")
	require.NoError(t, err)
	assert.False(t, ok)
	require.Len(t, ilog.Calls, 1)
	assert.False(t, ilog.Calls[0].Success)
	assert.Contains(t, ilog.Calls[0].Error, "invalid-input-response")
}

func TestVerifyEmptyTokenShortCircuits(t *testing.T) {
	called := false
	client, ilog := newTestClient(t, func(http.ResponseWriter, *http.Request) { called = true })

	ok, err := client.Verify(context.Background(), "   ", "1.2.3.4")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.False(t, called, "empty token must not hit the network")
	assert.Empty(t, ilog.Calls)
}

func TestVerifyHTTPErrorIsExternal(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	ok, err := client.Verify(context.Background(), "t", "")
	assert.False(t, ok)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeExternal, ""))
}

func TestVerifyBadJSONIsExternal(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	})
	ok, err := client.Verify(context.Background(), "t", "")
	assert.False(t, ok)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeExternal, ""))
}

func TestVerifyTransportErrorIsExternal(t *testing.T) {
	// Point at a closed server to force a transport error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	url := srv.URL
	srv.Close()
	client := New(Config{SecretKey: testSecret, VerifyURL: url}, &http.Client{Timeout: time.Second}, &mocks.MockIntegrationLogger{}, &mocks.MockClock{})
	ok, err := client.Verify(context.Background(), "t", "")
	assert.False(t, ok)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeExternal, ""))
}
