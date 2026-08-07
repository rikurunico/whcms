package billing

import (
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
)

// InvoiceDetail is an invoice with its line items and payment transaction
// history (oldest first - ports.TransactionRepo.ListByInvoice's own order).
type InvoiceDetail struct {
	Invoice      domain.Invoice       `json:"invoice"`
	Items        []domain.InvoiceItem `json:"items"`
	Transactions []domain.Transaction `json:"transactions"`
}

// ManualInvoiceItemInput is one line of an admin-created manual invoice.
type ManualInvoiceItemInput struct {
	Description string `json:"description" validate:"required,max=500"`
	Amount      int64  `json:"amount" validate:"required,gt=0"`
	Taxed       bool   `json:"taxed"`
}

// ManualInvoiceInput creates a manual invoice (admin).
type ManualInvoiceInput struct {
	ClientID int64                    `json:"client_id" validate:"required,gt=0"`
	Items    []ManualInvoiceItemInput `json:"items" validate:"required,min=1,max=50,dive"`
	Discount int64                    `json:"discount" validate:"gte=0"`
	DueDate  string                   `json:"due_date" validate:"omitempty,datetime=2006-01-02"`
	Notes    string                   `json:"notes" validate:"max=2000"`
}

// ManualPaymentInput records an admin manual payment against an invoice.
type ManualPaymentInput struct {
	Amount int64  `json:"amount" validate:"required,gt=0"`
	Method string `json:"method" validate:"required,max=50"`
}

// RefundInput marks a paid invoice refunded; ToCredit optionally returns the
// amount to the client's credit balance.
type RefundInput struct {
	ToCredit bool `json:"to_credit"`
}

// UpdateInvoiceInput patches the due date and/or notes of an open invoice.
type UpdateInvoiceInput struct {
	DueDate *string `json:"due_date" validate:"omitempty,datetime=2006-01-02"`
	Notes   *string `json:"notes" validate:"omitempty,max=2000"`
}

// GenerateSelectedInvoicesInput lists explicit service/domain IDs to
// force-generate a renewal invoice for right now (admin "Invoice Selected
// Items"), regardless of billing.renewal_lead_days - mirrors WHMCS's
// same-named action. At least one of ServiceIDs/DomainIDs must be
// non-empty (checked in the service, like AdminUpdateServiceInput's
// "nothing to update" guard).
type GenerateSelectedInvoicesInput struct {
	ServiceIDs []int64 `json:"service_ids" validate:"omitempty,dive,gt=0"`
	DomainIDs  []int64 `json:"domain_ids" validate:"omitempty,dive,gt=0"`
}

// GeneratedInvoiceSummary describes one invoice created by
// GenerateSelectedRenewalInvoices.
type GeneratedInvoiceSummary struct {
	Type          string `json:"type"` // "service" | "domain"
	ID            int64  `json:"id"`
	InvoiceID     int64  `json:"invoice_id"`
	InvoiceNumber string `json:"invoice_number"`
}

// SkippedItem describes one service/domain GenerateSelectedRenewalInvoices
// declined to invoice, and why (e.g. "already invoiced", "not eligible for
// renewal invoicing", "not found").
type SkippedItem struct {
	Type   string `json:"type"`
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// GenerateSelectedInvoicesResult is the response of
// GenerateSelectedRenewalInvoices.
type GenerateSelectedInvoicesResult struct {
	Created []GeneratedInvoiceSummary `json:"created"`
	Skipped []SkippedItem             `json:"skipped"`
}

// parseDate parses a YYYY-MM-DD string into a UTC midnight time.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
