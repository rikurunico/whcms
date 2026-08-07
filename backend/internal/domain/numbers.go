package domain

import (
	"fmt"
	"time"
)

// Number formats (CONTRACTS.md §5):
//
//	invoice INV-YYYYMM-XXXXXX  (counter scope "invoice:YYYYMM")
//	order   ORD-YYYYMM-XXXXXX  (counter scope "order:YYYYMM")
//	ticket  TKT-XXXXXX         (counter scope "ticket")

// FormatInvoiceNumber renders an invoice number for the month of t and
// sequence n, e.g. INV-202607-000042.
func FormatInvoiceNumber(t time.Time, n int64) string {
	return fmt.Sprintf("INV-%s-%06d", t.UTC().Format("200601"), n)
}

// FormatOrderNumber renders an order number, e.g. ORD-202607-000042.
func FormatOrderNumber(t time.Time, n int64) string {
	return fmt.Sprintf("ORD-%s-%06d", t.UTC().Format("200601"), n)
}

// FormatTicketNumber renders a ticket number, e.g. TKT-000042.
func FormatTicketNumber(n int64) string {
	return fmt.Sprintf("TKT-%06d", n)
}

// InvoiceCounterScope returns the counters table scope for invoices in the
// month of t, e.g. "invoice:202607".
func InvoiceCounterScope(t time.Time) string {
	return "invoice:" + t.UTC().Format("200601")
}

// OrderCounterScope returns the counters table scope for orders in the month
// of t, e.g. "order:202607".
func OrderCounterScope(t time.Time) string {
	return "order:" + t.UTC().Format("200601")
}

// TicketCounterScope is the counters table scope for tickets.
const TicketCounterScope = "ticket"
