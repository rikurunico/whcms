package duitku

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Fixed vectors computed independently (openssl md5 / shasum -a 256) for
// merchantCode=DEMO apiKey=secretkey order=INV-202607-000001-01 amount=150000
// datetime="2026-07-03 10:00:00".
const (
	vecMerchantCode = "DEMO"
	vecAPIKey       = "secretkey"
	vecOrderID      = "INV-202607-000001-01"
	vecAmount       = "150000"
	vecDatetime     = "2026-07-03 10:00:00"

	vecSigGetMethod = "87286f4b1a2b6ba9ccaf83259b86278237c4d07f07038739890aea4010f5c9b0"
	vecSigInquiry   = "088a530aa7555d521f48167cb4fa82ef"
	vecSigCallback  = "862f4f3c58f4f6e676038789450a4ffd"
	vecSigCheck     = "42b381e1a6f4e3e837b7eab0663a42a7"
)

func TestSignatureVectors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, vecSigGetMethod,
		signGetPaymentMethod(vecMerchantCode, vecAmount, vecDatetime, vecAPIKey),
		"getpaymentmethod SHA256(merchantcode+amount+datetime+apiKey)")

	assert.Equal(t, vecSigInquiry,
		signInquiry(vecMerchantCode, vecOrderID, vecAmount, vecAPIKey),
		"inquiry MD5(merchantCode+merchantOrderId+paymentAmount+apiKey)")

	assert.Equal(t, vecSigCallback,
		signCallback(vecMerchantCode, vecAmount, vecOrderID, vecAPIKey),
		"callback MD5(merchantCode+amount+merchantOrderId+apiKey)")

	assert.Equal(t, vecSigCheck,
		signCheckTransaction(vecMerchantCode, vecOrderID, vecAPIKey),
		"checkTransaction MD5(merchantCode+merchantOrderId+apiKey)")
}

func TestSignatureInputOrderMatters(t *testing.T) {
	t.Parallel()

	// Inquiry and callback share the same components in different order; the
	// digests must differ.
	assert.NotEqual(t,
		signInquiry(vecMerchantCode, vecOrderID, vecAmount, vecAPIKey),
		signCallback(vecMerchantCode, vecAmount, vecOrderID, vecAPIKey))
}

func TestHexHelpers(t *testing.T) {
	t.Parallel()

	// Well-known digests of the empty string.
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", md5Hex(""))
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", sha256Hex(""))
}
