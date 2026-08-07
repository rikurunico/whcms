package payments

import (
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
)

// PayInvoiceRequest is the POST /invoices/:id/pay body.
type PayInvoiceRequest struct {
	// Method is a Duitku payment method code (e.g. "VA", "OV", "SP"),
	// ManualMethodBankTransfer ("bank_transfer"), or "credit" to pay from the
	// client's credit balance.
	Method string `json:"method" validate:"required,min=1,max=32"`
}

// PaymentInstruction is the response to POST /invoices/:id/pay. BankAccounts/
// Note are only ever populated for the manual bank-transfer gateway; empty
// for Duitku (and vice versa for payment_url/va_number/qr_string).
type PaymentInstruction struct {
	Status          string              `json:"status"` // "pending" (gateway) | "paid" (credit)
	TransactionID   int64               `json:"transaction_id,omitempty"`
	MerchantOrderID string              `json:"merchant_order_id,omitempty"`
	Reference       string              `json:"reference,omitempty"`
	PaymentURL      string              `json:"payment_url,omitempty"`
	VANumber        string              `json:"va_number,omitempty"`
	QRString        string              `json:"qr_string,omitempty"`
	BankAccounts    []ports.BankAccount `json:"bank_accounts,omitempty"`
	Note            string              `json:"note,omitempty"`
	// Amount is the TOTAL the customer must actually transfer via this
	// channel: the invoice's remaining balance plus Fee, when the gateway
	// passes its own channel fee on to the customer (Duitku reports this
	// gross figure itself on the VA/QRIS it creates - we never recompute a
	// fee from the getpaymentmethod catalogue ourselves). A channel with no
	// surcharge (or one the merchant's contract absorbs instead) has Fee=0
	// and Amount equal to the invoice's remaining balance.
	Amount    int64      `json:"amount"`
	Fee       int64      `json:"fee"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
