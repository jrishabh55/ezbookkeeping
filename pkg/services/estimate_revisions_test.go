package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestEstimateRevisionNumbering(t *testing.T) {
	assert.Equal(t, "EST/2026-27/01-R2", estimateRevisionNumber("EST/2026-27/01", 2))
	assert.Equal(t, int32(1), nextEstimateRevisionNumber([]int32{0}))
	assert.Equal(t, int32(4), nextEstimateRevisionNumber([]int32{0, 3, 1})) // the highest wins, whatever was deleted in between
	assert.Equal(t, 2, nextInvoiceSequence([]string{"EST/2026-27/01", "EST/2026-27/01-R2"}, "EST/2026-27/"))
	assert.Equal(t, 2, nextInvoiceSequence([]string{"EST/2026-27/01-R2"}, "EST/2026-27/")) // a revision counts as the estimate it revises

	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01-R1", revision.Number)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/02", next)
}

func TestReviseCopiesEverythingAndLeavesTheSentVersionIntact(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	hosting := createTestProduct(t, uid, "Hosting", 0, "")

	estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31",
		Notes: "Valid for 30 days", TaxType: models.INVOICE_TAX_TYPE_IGST, TaxRate: 1800, Currency: "USD"}
	lines := testEstimateLines()
	lines[1].ProductId = hosting.ProductId
	assert.Nil(t, Invoices.CreateEstimate(ctx, estimate, lines, testEstimateMilestones()))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	reloaded, revisionLines, err := Invoices.GetInvoice(ctx, uid, revision.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, "EST/2026-27/01-R1", reloaded.Number)
	assert.Equal(t, estimate.InvoiceId, reloaded.RevisionOf)
	assert.Equal(t, int32(1), reloaded.RevisionNumber)
	assert.Equal(t, customer.CustomerId, reloaded.CustomerId)
	assert.Equal(t, "Quber Tech", reloaded.BillToName)
	assert.Equal(t, "USD", reloaded.Currency)
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, reloaded.TaxType)
	assert.Equal(t, int32(1800), reloaded.TaxRate)
	assert.Equal(t, estimate.Total, reloaded.Total)
	assert.Equal(t, "Valid for 30 days", reloaded.Notes)
	assert.Equal(t, "2026-10-30", reloaded.InvoiceDate)
	assert.Equal(t, "2026-11-29", reloaded.DueDate) // today + the profile's 30 days
	assert.Equal(t, "", reloaded.PublicToken)
	assert.Equal(t, int64(0), reloaded.SentUnixTime)
	assert.Equal(t, int64(0), reloaded.RespondedUnixTime) // the client agreed to another version
	assert.False(t, reloaded.RespondedByClient)
	assert.Equal(t, 2, len(revisionLines))
	assert.Equal(t, "Support", revisionLines[1].ItemName)
	assert.Equal(t, int64(300), revisionLines[1].Quantity)
	assert.Equal(t, int64(33333), revisionLines[1].Rate)
	assert.Equal(t, "998315", revisionLines[1].Sac)
	assert.Equal(t, hosting.ProductId, revisionLines[1].ProductId)

	oldMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	newMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, revision.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(newMilestones))

	for idx, milestone := range newMilestones {
		assert.Equal(t, oldMilestones[idx].Label, milestone.Label)
		assert.Equal(t, oldMilestones[idx].Percent, milestone.Percent)
		assert.Equal(t, int64(0), milestone.InvoiceId)
		assert.NotEqual(t, oldMilestones[idx].MilestoneId, milestone.MilestoneId)
	}

	// the version the client saw keeps everything, its answer included, and is now superseded
	old, oldLines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, old.EstimateStatus)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, old.SupersededFromStatus)
	assert.Greater(t, old.RespondedUnixTime, int64(0))
	assert.Equal(t, "EST/2026-27/01", old.Number)
	assert.Equal(t, "2099-12-31", old.DueDate)
	assert.Equal(t, int64(0), old.RevisionOf)
	assert.Equal(t, int32(0), old.RevisionNumber)
	assert.Equal(t, 2, len(oldLines))
	assert.Equal(t, 3, len(oldMilestones))
}

func TestReviseIsAllowedOnlyOnTheLatestSentOrAnsweredVersionWithNothingInvoiced(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	today := "2026-10-30"

	// a draft is edited in place
	draft := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	_, err := Invoices.ReviseEstimate(ctx, uid, draft.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	sent := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, sent.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, sent.InvoiceId, today)
	assert.Nil(t, err)

	// the superseded version and the new draft are not revisable
	_, err = Invoices.ReviseEstimate(ctx, uid, sent.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)
	_, err = Invoices.ReviseEstimate(ctx, uid, revision.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	// expired (sent, valid-until in the past), declined and accepted are revisable
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/03", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	shareTestEstimate(t, ctx, uid, expired.InvoiceId)
	_, err = Invoices.ReviseEstimate(ctx, uid, expired.InvoiceId, today)
	assert.Nil(t, err)

	declined := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/04", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, declined.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	_, err = Invoices.ReviseEstimate(ctx, uid, declined.InvoiceId, today)
	assert.Nil(t, err)

	accepted := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/05", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, accepted.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	_, err = Invoices.ReviseEstimate(ctx, uid, accepted.InvoiceId, today)
	assert.Nil(t, err)

	// never once anything is invoiced from it
	invoiced := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/06", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, invoiced.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, invoiced.InvoiceId)
	_, err = Invoices.ReviseEstimate(ctx, uid, invoiced.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoiced.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)

	// an invoice is not an estimate
	_, err = Invoices.ReviseEstimate(ctx, uid, invoice.InvoiceId, today)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
}

func TestConcurrentRevisesCreateOneRevision(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	// what a second click read just before the first one wrote
	stale, staleLines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	staleMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	_, err = Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	second, err := Invoices.reviseEstimate(ctx, stale, staleLines, staleMilestones, "2026-10-30")
	assert.Nil(t, second)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	all, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all)) // the first version and exactly one revision

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, reloaded.EstimateStatus)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.SupersededFromStatus)
}

func TestModifyEstimateRefusesAnythingButDraft(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	// the client has not seen a draft: it is edited in place
	estimate.Notes = "First draft"
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	estimate.Notes = "Changed after sending"
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "First draft", reloaded.Notes)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
}
