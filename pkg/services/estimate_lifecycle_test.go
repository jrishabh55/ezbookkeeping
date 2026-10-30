package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// linkTestInvoiceToEstimate marks an invoice as made from an estimate, as converting does
func linkTestInvoiceToEstimate(t *testing.T, uid int64, invoiceId int64, estimateId int64) {
	t.Helper()
	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(core.NewNullContext()).Cols("estimate_id").
		Where("uid=? AND invoice_id=?", uid, invoiceId).Update(&models.Invoice{EstimateId: estimateId})
	assert.Nil(t, err)
}

func TestModifyEstimateReplacesLinesAndMilestones(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	estimate.Notes = "Valid for 30 days"
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, []*models.InvoiceLine{{ItemName: "Website", Quantity: 100, Rate: 500000}},
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}}))

	reloaded, lines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, int64(500000), reloaded.Subtotal)
	assert.Equal(t, "Valid for 30 days", reloaded.Notes)
	assert.Equal(t, 1, len(lines))

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(milestones))
	assert.Equal(t, "Delivery", milestones[1].Label)

	assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), []*models.EstimateMilestone{{Label: "Advance", Percent: 5000}}))

	// an estimate is never edited through the invoice path, nor an invoice through the estimate path
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.ModifyInvoice(ctx, reloaded, testEstimateLines()))
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.ModifyEstimate(ctx, invoice, testEstimateLines(), nil))
}

func TestAnsweredOrInvoicedEstimatesAreLocked(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(milestones)) // the refused edit left the payment terms alone

	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	// reopened means sent again: still never edited in place (it is revised instead)
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), testEstimateMilestones()))

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, estimate.InvoiceId)
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))
}

func TestEstimateResponseTransitions(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_SENT))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId)) // nothing to reopen

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_DECLINED, reloaded.EstimateStatus)
	assert.False(t, reloaded.RespondedByClient)
	assert.Greater(t, reloaded.RespondedUnixTime, int64(0))

	// an answered estimate takes no second answer until it is reopened
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
	assert.Equal(t, int64(0), reloaded.RespondedUnixTime)
	assert.Equal(t, "", reloaded.ResponseNote)

	// an expired estimate (valid-until in the past) can still be marked
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, expired.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	// reopening is refused once anything was invoiced from the estimate
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, expired.InvoiceId)
	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.ReopenEstimate(ctx, uid, expired.InvoiceId))

	// an invoice has no estimate status
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.SetEstimateResponse(ctx, uid, invoice.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
}

func TestDeletingEstimatesAndTheInvoicesMadeFromThem(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, estimate.InvoiceId)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("invoice_id").
		Where("milestone_id=?", milestones[0].MilestoneId).Update(&models.EstimateMilestone{InvoiceId: invoice.InvoiceId})
	assert.Nil(t, err)

	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	milestones, err = Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), milestones[0].InvoiceId)

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))
	_, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
}

func TestModifyingNumberToCollideIsCaughtPerKindButNotAcrossKinds(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	invoiceA := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	invoiceB := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/02", 100)
	invoiceB.Number = "INV/2026-27/01"
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, Invoices.ModifyInvoice(ctx, invoiceB, testEstimateLines()))

	estimateA := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	estimateB := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	estimateB.Number = "EST/2026-27/01"
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, Invoices.ModifyEstimate(ctx, estimateB, testEstimateLines(), nil))

	// an invoice and an estimate may share a number: neither modify call is blocked by the other kind
	invoiceA.Number = "SHARED/01"
	assert.Nil(t, Invoices.ModifyInvoice(ctx, invoiceA, testEstimateLines()))
	estimateA.Number = "SHARED/01"
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimateA, testEstimateLines(), nil))
}

func TestEstimateProgressCountsLiveInvoicesOnly(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	whole := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	staged := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(),
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}})

	progress, err := Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId, staged.InvoiceId})
	assert.Nil(t, err)
	assert.False(t, progress[whole.InvoiceId].FullyInvoiced)
	assert.Empty(t, progress[whole.InvoiceId].Invoices)
	assert.Equal(t, 2, len(progress[staged.InvoiceId].Milestones))

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, whole.InvoiceId)
	advance := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/02", 100)
	linkTestInvoiceToEstimate(t, uid, advance.InvoiceId, staged.InvoiceId)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("invoice_id").
		Where("milestone_id=?", progress[staged.InvoiceId].Milestones[0].MilestoneId).Update(&models.EstimateMilestone{InvoiceId: advance.InvoiceId})
	assert.Nil(t, err)

	progress, err = Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId, staged.InvoiceId})
	assert.Nil(t, err)
	assert.True(t, progress[whole.InvoiceId].FullyInvoiced)
	assert.Equal(t, "INV/2026-27/01", progress[whole.InvoiceId].Invoices[0].Number)
	assert.False(t, progress[staged.InvoiceId].FullyInvoiced) // Delivery is still open

	resp := whole.ToInvoiceInfoResponse(nil, nil)
	resp.SetEstimateProgress(progress[whole.InvoiceId])
	assert.Equal(t, "INV/2026-27/01", resp.ConvertedInvoices[0].Number)
	assert.True(t, resp.FullyInvoiced)

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	progress, err = Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId})
	assert.Nil(t, err)
	assert.False(t, progress[whole.InvoiceId].FullyInvoiced)
}
