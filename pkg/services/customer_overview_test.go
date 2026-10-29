package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCustomerOverviewListsProductsAndTimeline(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	otherCustomer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	first := &models.Invoice{Uid: uid, Number: "INV/2025-26/01", CustomerId: customer.CustomerId, InvoiceDate: "2025-10-01", DueDate: "2025-10-08"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, first, []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
		{ItemName: "Setup", Quantity: 100, Rate: 500000},
	}))
	second := &models.Invoice{Uid: uid, Number: "INV/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-10-01", DueDate: "2026-10-08"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, second, []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 160000},
	}))
	createTestInvoiceWithLines(t, uid, otherCustomer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
	})

	accountId := createTestAccount(t, ctx, uid, "HDFC", "INR")
	payment := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 650000, "2025-10-05")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, first.InvoiceId, payment.TransactionId, 0))

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(overview.Products))
	assert.Equal(t, "Domain", overview.Products[0].Name)
	assert.Equal(t, 2, overview.Products[0].TimesBought)
	assert.Equal(t, "2026-10-01", overview.Products[0].LastBoughtDate)

	kinds := map[string]int{}
	for _, item := range overview.Timeline {
		kinds[item.Kind]++
	}
	assert.Equal(t, 2, kinds[models.CUSTOMER_TIMELINE_KIND_INVOICE]) // the other customer's invoice is not here
	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_PAYMENT])

	for i := 1; i < len(overview.Timeline); i++ {
		assert.True(t, overview.Timeline[i-1].UnixTime >= overview.Timeline[i].UnixTime) // newest first
	}

	_, err = Invoices.GetCustomerOverview(ctx, otherUid, customer.CustomerId)
	assert.Equal(t, errs.ErrCustomerNotFound, err)
}
