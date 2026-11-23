package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func paidDateTestPayment(unixTime int64, utcOffset int16, amount int64) *InvoicePaymentInfo {
	return &InvoicePaymentInfo{Transaction: &Transaction{TransactionTime: unixTime * 1000, TimezoneUtcOffset: utcOffset, Amount: amount}}
}

func TestInvoicePaidDate(t *testing.T) {
	invoice := &Invoice{Kind: INVOICE_KIND_INVOICE, Total: 100000}
	// 2026-11-01T10:00Z and 2026-11-02T20:00Z; the second is 2026-11-03 01:30 in IST (+330)
	first, second := paidDateTestPayment(1793527200, 330, 50000), paidDateTestPayment(1793649600, 330, 50000)

	assert.Equal(t, "", InvoicePaidDate(invoice, nil))
	assert.Equal(t, "", InvoicePaidDate(invoice, []*InvoicePaymentInfo{first}), "partly paid")
	assert.Equal(t, "2026-11-03", InvoicePaidDate(invoice, []*InvoicePaymentInfo{second, first}), "latest payment in its own offset")
	assert.Equal(t, "", InvoicePaidDate(&Invoice{Kind: INVOICE_KIND_INVOICE, Total: 100000, ClosedManually: true}, nil), "closed without payments has no date")
	assert.Equal(t, "", InvoicePaidDate(&Invoice{Kind: INVOICE_KIND_ESTIMATE, Total: 100000}, []*InvoicePaymentInfo{first, second}), "estimates are never paid")
	// within the 3% tolerance
	assert.Equal(t, "2026-11-01", InvoicePaidDate(invoice, []*InvoicePaymentInfo{paidDateTestPayment(1793527200, 330, 97000)}))
}
