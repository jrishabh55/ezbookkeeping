package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCustomerOverviewShowsEstimateEventsButNeverCountsEstimatesAsBought(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	hosting := createTestProduct(t, uid, "Hosting", 0, "")
	hostingLines := func() []*models.InvoiceLine {
		return []*models.InvoiceLine{{ItemName: "Hosting", Quantity: 100, Rate: 150000, ProductId: hosting.ProductId}}
	}
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", hostingLines(), nil)

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Empty(t, overview.Products) // quoted is not bought
	assert.Equal(t, 1, len(overview.Timeline))
	assert.Equal(t, models.CUSTOMER_TIMELINE_KIND_ESTIMATE, overview.Timeline[0].Kind)
	assert.Equal(t, "EST/2026-27/01", overview.Timeline[0].InvoiceNumber)

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	invoice := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, invoice, hostingLines(), estimate.InvoiceId, 0))

	overview, err = Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(overview.Products))
	assert.Equal(t, 1, overview.Products[0].TimesBought)

	items := make(map[string]*models.CustomerTimelineItemResponse)

	for _, item := range overview.Timeline {
		items[item.Kind] = item
	}

	assert.NotNil(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE])
	assert.NotNil(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED])
	assert.False(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED].ByClient)
	assert.Equal(t, "INV/2026-27/01", items[models.CUSTOMER_TIMELINE_KIND_INVOICE].InvoiceNumber)
	assert.Equal(t, "EST/2026-27/01", items[models.CUSTOMER_TIMELINE_KIND_INVOICE].EstimateNumber)
}

func TestAcceptedEstimatesNotFullyInvoicedNeedAttention(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	whole := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	staged := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(),
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}})
	createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/03", testEstimateLines(), nil) // never accepted: never listed

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, whole.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, staged.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	estimates, progress, err := Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(estimates))
	assert.NotNil(t, progress[staged.InvoiceId])

	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01"), testEstimateLines(), whole.InvoiceId, 0))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates))
	assert.Equal(t, staged.InvoiceId, estimates[0].InvoiceId)

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, staged.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02"),
		[]*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 1}}, staged.InvoiceId, milestones[0].MilestoneId))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates)) // Delivery is still open

	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03"),
		[]*models.InvoiceLine{{ItemName: "Delivery", Quantity: 100, Rate: 1}}, staged.InvoiceId, milestones[1].MilestoneId))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, estimates)
}
