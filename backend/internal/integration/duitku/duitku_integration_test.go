//go:build integration

package duitku

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// TestAgainstLiveMockserver drives the adapter end-to-end against the real
// whcms-mock server (mockserver/README.md): methods -> inquiry -> pending
// status -> simulated payment -> signed callback verify -> paid status -> 404
// mapping. Skips when the mockserver is not reachable. Uses the mockserver's
// default credentials (DEMO/secretkey) and a unique merchantOrderId, and
// never calls /mock/reset (state may be shared with other agents).
func TestAgainstLiveMockserver(t *testing.T) {
	baseURL := os.Getenv("DUITKU_MOCK_URL")
	if baseURL == "" {
		baseURL = "http://localhost:9090"
	}
	if resp, err := http.Get(baseURL + "/healthz"); err != nil {
		t.Skipf("mockserver not reachable at %s: %v", baseURL, err)
	} else {
		resp.Body.Close()
	}

	// Local sink that captures the signed callback the mockserver delivers.
	callbackCh := make(chan ports.CallbackPayload, 1)
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		callbackCh <- ports.CallbackPayload{
			MerchantCode:    r.FormValue("merchantCode"),
			Amount:          r.FormValue("amount"),
			MerchantOrderID: r.FormValue("merchantOrderId"),
			ProductDetail:   r.FormValue("productDetail"),
			AdditionalParam: r.FormValue("additionalParam"),
			PaymentCode:     r.FormValue("paymentCode"),
			ResultCode:      r.FormValue("resultCode"),
			Reference:       r.FormValue("reference"),
			Signature:       r.FormValue("signature"),
		}
		_, _ = w.Write([]byte("OK"))
	}))
	defer sink.Close()

	cfg := Config{
		MerchantCode: envOr("DUITKU_MERCHANT_CODE", "DEMO"),
		APIKey:       envOr("DUITKU_API_KEY", "secretkey"),
		BaseURL:      baseURL,
		CallbackURL:  sink.URL,
		ReturnURL:    "",
	}
	ilog := &mocks.MockIntegrationLogger{}
	c := New(cfg, nil, nil, ilog, &mocks.MockClock{}) // MockClock zero value = real time.Now
	ctx := context.Background()

	// 1. Payment methods.
	methods, err := c.GetPaymentMethods(ctx, 150000)
	require.NoError(t, err)
	require.NotEmpty(t, methods)
	codes := make([]string, 0, len(methods))
	for _, m := range methods {
		codes = append(codes, m.Code)
	}
	assert.Contains(t, codes, "BC")

	// 2. Inquiry with a unique merchantOrderId.
	orderID := fmt.Sprintf("ITEST-%s-01", uuid.NewString()[:8])
	res, err := c.CreateTransaction(ctx, ports.CreateTxRequest{
		MerchantOrderID: orderID,
		Amount:          150000,
		Method:          "BC",
		ProductDetails:  "integration test",
		Email:           "itest@example.com",
		Phone:           "0800000000",
		CustomerName:    "Integration Test",
		ExpiryMinutes:   60,
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.Reference, "MOCKREF-"), "reference %q", res.Reference)
	assert.Contains(t, res.PaymentURL, "/payment/"+res.Reference)
	assert.Equal(t, int64(150000), res.Amount)
	assert.NotEmpty(t, res.VANumber)

	// 3. Status is pending before payment.
	st, err := c.CheckTransaction(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, ports.TxStatusPending, st.StatusCode)

	// 4. Simulate the user paying; mockserver POSTs the signed callback.
	payResp, err := http.Post(baseURL+"/mock/duitku/pay/"+res.Reference, "", nil)
	require.NoError(t, err)
	payResp.Body.Close()
	require.Equal(t, http.StatusOK, payResp.StatusCode)

	select {
	case p := <-callbackCh:
		assert.Equal(t, orderID, p.MerchantOrderID)
		assert.Equal(t, "00", p.ResultCode)
		assert.True(t, c.VerifyCallbackSignature(p), "mockserver callback signature must verify")
		p.Amount = "1"
		assert.False(t, c.VerifyCallbackSignature(p), "tampered callback must fail")
	case <-time.After(5 * time.Second):
		t.Fatal("callback not delivered by mockserver")
	}

	// 5. Status is paid after payment.
	st, err = c.CheckTransaction(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, ports.TxStatusSuccess, st.StatusCode)
	assert.Equal(t, int64(150000), st.Amount)
	assert.Equal(t, res.Reference, st.Reference)

	// 6. Unknown merchantOrderId maps HTTP 404 -> NOT_FOUND.
	_, err = c.CheckTransaction(ctx, "UNKNOWN-"+uuid.NewString())
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	// Every call hit the integration logger.
	assert.GreaterOrEqual(t, len(ilog.Calls), 5)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
