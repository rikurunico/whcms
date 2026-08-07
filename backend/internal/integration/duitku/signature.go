package duitku

import (
	"crypto/md5" //nolint:gosec // Duitku's signature scheme mandates MD5
	"crypto/sha256"
	"encoding/hex"
)

// md5Hex returns the lowercase hex MD5 digest of s.
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // required by the Duitku API
	return hex.EncodeToString(sum[:])
}

// sha256Hex returns the lowercase hex SHA-256 digest of s.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// signGetPaymentMethod computes SHA256(merchantCode + amount + datetime + apiKey).
// amount is the whole-IDR integer rendered without decimals; datetime uses
// the "2006-01-02 15:04:05" layout.
func signGetPaymentMethod(merchantCode, amount, datetime, apiKey string) string {
	return sha256Hex(merchantCode + amount + datetime + apiKey)
}

// signInquiry computes MD5(merchantCode + merchantOrderId + paymentAmount + apiKey).
func signInquiry(merchantCode, merchantOrderID, paymentAmount, apiKey string) string {
	return md5Hex(merchantCode + merchantOrderID + paymentAmount + apiKey)
}

// signCallback computes MD5(merchantCode + amount + merchantOrderId + apiKey)
// - the signature Duitku sends on webhook callbacks.
func signCallback(merchantCode, amount, merchantOrderID, apiKey string) string {
	return md5Hex(merchantCode + amount + merchantOrderID + apiKey)
}

// signCheckTransaction computes MD5(merchantCode + merchantOrderId + apiKey).
func signCheckTransaction(merchantCode, merchantOrderID, apiKey string) string {
	return md5Hex(merchantCode + merchantOrderID + apiKey)
}
