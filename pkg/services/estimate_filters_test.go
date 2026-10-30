package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

func TestEstimatesStayOutOfInvoiceOnlyFeatures(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01",
		[]*models.InvoiceLine{{ItemName: "Domain", Quantity: 100, Rate: 150000, ProductId: domain.ProductId}}, nil)
	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 150000, "2026-04-25")

	// no payments or suggestions on a quote
	assert.Equal(t, errs.ErrNotAvailableForEstimate, Invoices.LinkPayment(ctx, uid, estimate.InvoiceId, income.TransactionId, 0))
	_, err := Invoices.GetPaymentSuggestions(ctx, uid, estimate.InvoiceId)
	assert.Equal(t, errs.ErrNotAvailableForEstimate, err)

	// no Mark as Paid
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.SetInvoiceClosed(ctx, uid, estimate.InvoiceId, true))

	// never repeated or renewed, even with a renewable product on it
	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: estimate.InvoiceId, DayOfMonth: 15, NextDate: "2099-01-15", Enabled: true})
	assert.Equal(t, errs.ErrNotAvailableForEstimate, err)
	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, option)
	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)

	// a payment row pointing at an estimate (only possible by hand) never shows an invoice chip on the transaction
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Insert(&models.InvoicePayment{
		PaymentId: uuid.Container.GenerateUuid(uuid.UUID_TYPE_INVOICE), InvoiceId: estimate.InvoiceId, Uid: uid, TransactionId: income.TransactionId})
	assert.Nil(t, err)
	linked, err := Invoices.GetLinkedInvoicesByTransactionIds(ctx, uid, []int64{income.TransactionId})
	assert.Nil(t, err)
	assert.Empty(t, linked)

	// and it is not an invoice in the invoice list (outstanding and overdue are computed from it)
	invoices, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, invoices)
}
