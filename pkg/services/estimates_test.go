package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// testEstimateLines come to 1,999.99 before tax: 1,000.00 plus three hours of 333.33
func testEstimateLines() []*models.InvoiceLine {
	return []*models.InvoiceLine{
		{ItemName: "Website", Description: "Design and build", Quantity: 100, Rate: 100000, Sac: "998314"},
		{ItemName: "Support", Quantity: 300, Rate: 33333, Sac: "998315"},
	}
}

// testEstimateMilestones are three thirds that need rounding: 33.33%, 33.33%, 33.34%
func testEstimateMilestones() []*models.EstimateMilestone {
	return []*models.EstimateMilestone{
		{Label: "Advance", Percent: 3333},
		{Label: "Design sign-off", Percent: 3333},
		{Label: "Go-live", Percent: 3334},
	}
}

// createTestEstimate saves a draft estimate dated 2026-04-23 and valid until 2099-12-31, so it never expires in a test
func createTestEstimate(t *testing.T, uid int64, customerId int64, number string, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) *models.Invoice {
	t.Helper()
	estimate := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
	assert.Nil(t, Invoices.CreateEstimate(core.NewNullContext(), estimate, lines, milestones))
	return estimate
}

func TestCreateEstimateStartsAsDraftAndListsApartFromInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	reloaded, lines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, int64(199999), reloaded.Subtotal)
	assert.Equal(t, "Quber Tech", reloaded.BillToName)
	assert.Equal(t, 2, len(lines))

	invoices, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(invoices))
	assert.Equal(t, "INV/2026-27/01", invoices[0].Number)

	estimates, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates))
	assert.Equal(t, estimate.InvoiceId, estimates[0].InvoiceId)

	// a plain invoice never carries estimate state, whatever the caller put in
	invoice := &models.Invoice{Uid: uid, Number: "INV/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30",
		EstimateStatus: models.ESTIMATE_STATUS_ACCEPTED, EstimateId: estimate.InvoiceId, ResponseNote: "x"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, testEstimateLines()))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, reloaded.Kind)
	assert.Equal(t, "", reloaded.EstimateStatus)
	assert.Equal(t, int64(0), reloaded.EstimateId)
	assert.Equal(t, "", reloaded.ResponseNote)
}

func TestEstimateNumberingIsSeparateFromInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/02", 100)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01", next)

	createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	// an estimate typed with an invoice number neither collides with the invoice nor moves the invoice series
	createTestEstimate(t, uid, customer.CustomerId, "INV/2026-27/02", testEstimateLines(), nil)
	createTestEstimate(t, uid, customer.CustomerId, "INV/2026-27/09", testEstimateLines(), nil)

	next, err = Invoices.NextEstimateNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/02", next)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "INV/2026-27/03", next)

	duplicate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, Invoices.CreateEstimate(ctx, duplicate, testEstimateLines(), nil))
}

func TestEstimateProfileSettingsDefaultAndPersist(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, models.EstimateNumberPrefixDefault, profile.EstimateNumberPrefix)
	assert.Equal(t, int32(models.EstimateValidDaysDefault), profile.EstimateValidDays)

	// a client from before estimates sends neither field: the defaults stay
	profile.EstimateNumberPrefix, profile.EstimateValidDays = "", 0
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, profile))
	reloaded, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "EST", reloaded.EstimateNumberPrefix)
	assert.Equal(t, int32(30), reloaded.EstimateValidDays)

	reloaded.EstimateNumberPrefix, reloaded.EstimateValidDays = "QT", 15
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, reloaded))
	reloaded, err = Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "QT", reloaded.ToInvoiceProfileResponse().EstimateNumberPrefix)
	assert.Equal(t, int32(15), reloaded.ToInvoiceProfileResponse().EstimateValidDays)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2027-01-10")
	assert.Nil(t, err)
	assert.Equal(t, "QT/2026-27/01", next)
}

func TestEstimateMilestonesMustBeOneToTenLabelledAndAddUpTo100Percent(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	eleven := make([]*models.EstimateMilestone, 11)
	for idx := range eleven {
		eleven[idx] = &models.EstimateMilestone{Label: "Part", Percent: 909}
	}
	eleven[10].Percent = 910

	for name, milestones := range map[string][]*models.EstimateMilestone{
		"99.99%":       {{Label: "Advance", Percent: 4999}, {Label: "Delivery", Percent: 5000}},
		"100.01%":      {{Label: "Advance", Percent: 5001}, {Label: "Delivery", Percent: 5000}},
		"blank label":  {{Label: "   ", Percent: 10000}},
		"zero percent": {{Label: "Advance", Percent: 0}, {Label: "Delivery", Percent: 10000}},
		"long label":   {{Label: strings.Repeat("x", 129), Percent: 10000}},
		"eleven":       eleven,
	} {
		estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
		assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.CreateEstimate(ctx, estimate, testEstimateLines(), milestones), name)
	}

	estimates, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, estimates) // nothing was saved by a refused create

	ten := make([]*models.EstimateMilestone, 10)
	for idx := range ten {
		ten[idx] = &models.EstimateMilestone{Label: "Part", Percent: 1000}
	}
	ten[0].Label = "  Advance  "

	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), ten)
	saved, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(saved))
	assert.Equal(t, "Advance", saved[0].Label)
	assert.Equal(t, int32(1), saved[0].DisplayOrder)
	assert.Equal(t, int32(10), saved[9].DisplayOrder)
	assert.Equal(t, int64(0), saved[9].InvoiceId)
	assert.Equal(t, estimate.InvoiceId, saved[9].EstimateId)

	none := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	saved, err = Invoices.GetEstimateMilestones(ctx, uid, none.InvoiceId)
	assert.Nil(t, err)
	assert.Empty(t, saved)
}
