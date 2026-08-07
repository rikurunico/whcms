package pdf_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/pdf"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleInvoice() ports.InvoicePDFData {
	paidAt := time.Date(2026, 7, 3, 10, 30, 0, 0, time.UTC)
	return ports.InvoicePDFData{
		InvoiceNumber: "INV-202607-000042",
		Status:        domain.InvoicePaid,
		IssuedAt:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		DueDate:       time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC),
		PaidAt:        &paidAt,
		CompanyName:   "WHCMS Hosting",
		CompanyAddr:   "Jl. Contoh No. 1\nLamongan, Jawa Timur",
		CompanyEmail:  "billing@example.com",
		ClientName:    "Budi Santoso",
		ClientCompany: "PT Maju Jaya",
		ClientAddr:    "Jl. Pelanggan No. 2\nSurabaya",
		ClientEmail:   "budi@example.com",
		Items: []ports.InvoicePDFItem{
			{Description: "Shared Hosting - Paket Personal (monthly)", Amount: 50000},
			{Description: "Domain Registration - example.co.id (1 year)", Amount: 150000},
			{Description: "Setup fee", Amount: 25000},
		},
		Subtotal:      225000,
		Discount:      25000,
		TaxRate:       11,
		TaxTotal:      22000,
		CreditApplied: 10000,
		Total:         212000,
		Notes:         "Terima kasih atas pembayaran Anda.",
	}
}

func TestInvoicePDFProducesValidPDF(t *testing.T) {
	g := pdf.New()
	out, err := g.InvoicePDF(context.Background(), sampleInvoice())
	require.NoError(t, err)

	// Valid PDF header + EOF marker.
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")), "must start with %%PDF- header")
	assert.Contains(t, string(out[len(out)-32:]), "%%EOF", "must end with EOF marker")
	assert.Greater(t, len(out), 1000, "non-trivial document")
}

func TestInvoicePDFMinimalData(t *testing.T) {
	g := pdf.New()
	out, err := g.InvoicePDF(context.Background(), ports.InvoicePDFData{
		InvoiceNumber: "INV-202607-000001",
		Status:        domain.InvoiceUnpaid,
		IssuedAt:      time.Now(),
		DueDate:       time.Now(),
		CompanyName:   "X",
		ClientName:    "Y",
		Total:         0,
	})
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")))
}

func TestInvoicePDFAllStatuses(t *testing.T) {
	g := pdf.New()
	for _, st := range []domain.InvoiceStatus{
		domain.InvoiceDraft, domain.InvoiceUnpaid, domain.InvoicePaid,
		domain.InvoiceOverdue, domain.InvoiceCancelled, domain.InvoiceRefunded,
	} {
		inv := sampleInvoice()
		inv.Status = st
		inv.PaidAt = nil
		out, err := g.InvoicePDF(context.Background(), inv)
		require.NoError(t, err, string(st))
		assert.True(t, bytes.HasPrefix(out, []byte("%PDF-")), string(st))
	}
}

func TestInvoicePDFDeterministicStructure(t *testing.T) {
	g := pdf.New()
	a, err := g.InvoicePDF(context.Background(), sampleInvoice())
	require.NoError(t, err)
	b, err := g.InvoicePDF(context.Background(), sampleInvoice())
	require.NoError(t, err)
	assert.Equal(t, len(a), len(b), "same input renders same-size document")
}
