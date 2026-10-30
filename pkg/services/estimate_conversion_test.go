package services

import (
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// newTestInvoiceDraft is what the editor sends when saving an invoice made from an estimate
func newTestInvoiceDraft(uid int64, customerId int64, number string) *models.Invoice {
	return &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-10-30", DueDate: "2026-11-06"}
}

func TestEstimateMilestoneRateRoundsHalfUpAndLastOpenTakesTheRemainder(t *testing.T) {
	milestones := []*models.EstimateMilestone{{MilestoneId: 1, Percent: 3333}, {MilestoneId: 2, Percent: 3333}, {MilestoneId: 3, Percent: 3334}}

	rate, milestone, err := estimateMilestoneRate(199999, milestones, 1, nil)
	assert.Nil(t, err)
	assert.Equal(t, int64(66660), rate) // 66,659.67 rounds up
	assert.Equal(t, int64(1), milestone.MilestoneId)

	rate, _, err = estimateMilestoneRate(199999, milestones, 3, nil)
	assert.Nil(t, err)
	assert.Equal(t, int64(66680), rate) // not the last open one: its plain share

	// Go-live and Advance invoiced first: Design sign-off is the last open one and takes exactly what is left
	milestones[0].InvoiceId, milestones[2].InvoiceId = 101, 103
	rate, _, err = estimateMilestoneRate(199999, milestones, 2, map[int64]int64{1: 66660, 3: 66680})
	assert.Nil(t, err)
	assert.Equal(t, int64(66659), rate)
	assert.Equal(t, int64(199999), 66660+rate+66680)

	// an invoice edited after it was made: the remainder follows what was really invoiced
	rate, _, err = estimateMilestoneRate(199999, milestones, 2, map[int64]int64{1: 70000, 3: 66680})
	assert.Nil(t, err)
	assert.Equal(t, int64(63319), rate)

	_, _, err = estimateMilestoneRate(199999, milestones, 1, nil)
	assert.Equal(t, errs.ErrEstimateAlreadyConverted, err)
	_, _, err = estimateMilestoneRate(199999, milestones, 9, nil)
	assert.Equal(t, errs.ErrEstimateMilestoneNotFound, err)

	// no overflow on the largest amounts a line allows
	assert.Equal(t, int64(333300000000), estimateShare(1000000000000, 3333))
}

func TestEstimateMilestoneLineText(t *testing.T) {
	name, description := estimateMilestoneLineText("Advance", 3000, "EST/2026-27/01")
	assert.Equal(t, "Advance: 30% of Estimate EST/2026-27/01", name)
	assert.Equal(t, "", description)

	name, _ = estimateMilestoneLineText("Design sign-off", 3333, "EST/2026-27/01")
	assert.Equal(t, "Design sign-off: 33.33% of Estimate EST/2026-27/01", name)

	// a long label keeps the 128-character item name; the share moves to the description
	long := strings.Repeat("x", 120)
	name, description = estimateMilestoneLineText(long, 5000, "EST/2026-27/01")
	assert.Equal(t, long, name)
	assert.Equal(t, "50% of Estimate EST/2026-27/01", description)
}

func TestEstimateInvoiceDraftCopiesEveryLineForAFullConversion(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	support := createTestProduct(t, uid, "Support", 0, "")
	lines := testEstimateLines()
	lines[1].ProductId = support.ProductId
	estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31",
		Notes: "Valid for 30 days", TaxType: models.INVOICE_TAX_TYPE_IGST, TaxRate: 1800}
	assert.Nil(t, Invoices.CreateEstimate(ctx, estimate, lines, nil))

	draft, draftLines, err := Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, 0, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, draft.Kind)
	assert.Equal(t, estimate.InvoiceId, draft.EstimateId)
	assert.Equal(t, customer.CustomerId, draft.CustomerId)
	assert.Equal(t, "INV/2026-27/01", draft.Number)
	assert.Equal(t, "2026-10-30", draft.InvoiceDate)
	assert.Equal(t, "2026-11-06", draft.DueDate) // the profile's default terms: 7 days
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, draft.TaxType)
	assert.Equal(t, int32(1800), draft.TaxRate)
	assert.Equal(t, int64(235999), draft.Total)
	assert.Equal(t, estimate.Total, draft.Total)
	assert.Equal(t, 2, len(draftLines))
	assert.Equal(t, "Website", draftLines[0].ItemName)
	assert.Equal(t, "Design and build", draftLines[0].Description)
	assert.Equal(t, int64(300), draftLines[1].Quantity)
	assert.Equal(t, int64(33333), draftLines[1].Rate)
	assert.Equal(t, "998315", draftLines[1].Sac)
	assert.Equal(t, support.ProductId, draftLines[1].ProductId)
}

func TestEstimateInvoiceDraftOfAMilestoneIsOneLine(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	draft, draftLines, err := Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, milestones[0].MilestoneId, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(draftLines))
	assert.Equal(t, "Advance: 33.33% of Estimate EST/2026-27/01", draftLines[0].ItemName)
	assert.Equal(t, "998314", draftLines[0].Sac) // the estimate's first line
	assert.Equal(t, int64(100), draftLines[0].Quantity)
	assert.Equal(t, int64(66660), draftLines[0].Rate)
	assert.Equal(t, int64(66660), draft.Subtotal)

	// with milestones, the estimate is invoiced milestone by milestone
	_, _, err = Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, 0, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateMilestoneNotFound, err)

	// a declined estimate is not converted
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	_, _, err = Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, milestones[0].MilestoneId, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateStatusInvalid, err)
}

func TestCreateInvoiceFromMilestoneRecomputesTheRateAndInvoicesEachMilestoneOnce(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	other := &models.Customer{Uid: uid, Name: "Someone Else"}
	assert.Nil(t, Invoices.CreateCustomer(ctx, other))
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	// the client sends a tampered rate, quantity and customer; the server bills the estimate's customer the milestone's share
	advance := newTestInvoiceDraft(uid, other.CustomerId, "INV/2026-27/01")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, advance, []*models.InvoiceLine{{ItemName: "Advance", Quantity: 500, Rate: 1}}, estimate.InvoiceId, milestones[0].MilestoneId))
	saved, lines, err := Invoices.GetInvoice(ctx, uid, advance.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(100), lines[0].Quantity)
	assert.Equal(t, int64(66660), lines[0].Rate)
	assert.Equal(t, int64(66660), saved.Subtotal)
	assert.Equal(t, customer.CustomerId, saved.CustomerId)
	assert.Equal(t, estimate.InvoiceId, saved.EstimateId)

	// a second click on the same milestone is refused
	again := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Equal(t, errs.ErrEstimateAlreadyConverted,
		Invoices.CreateInvoiceFromEstimate(ctx, again, []*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 66660}}, estimate.InvoiceId, milestones[0].MilestoneId))

	// Go-live next; Design sign-off is then the last open one and takes the remainder
	goLive := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, goLive, []*models.InvoiceLine{{ItemName: "Go-live", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[2].MilestoneId))
	signOff := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, signOff, []*models.InvoiceLine{{ItemName: "Sign-off", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[1].MilestoneId))
	assert.Equal(t, int64(66680), goLive.Subtotal)
	assert.Equal(t, int64(66659), signOff.Subtotal)
	assert.Equal(t, estimate.Subtotal, advance.Subtotal+goLive.Subtotal+signOff.Subtotal)

	milestones, err = Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, signOff.InvoiceId, milestones[1].InvoiceId)

	// deleting a milestone invoice frees the milestone, which can be invoiced again for the same remainder
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, signOff.InvoiceId))
	redo := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/04")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, redo, []*models.InvoiceLine{{ItemName: "Sign-off", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[1].MilestoneId))
	assert.Equal(t, int64(66659), redo.Subtotal)

	// a milestone invoice has exactly one line
	twoLines := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/05")
	assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.CreateInvoiceFromEstimate(ctx, twoLines, testEstimateLines(), estimate.InvoiceId, milestones[0].MilestoneId))
}

func TestFullConversionHappensOnceWhileItsInvoiceExists(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	// the editor may adjust the copied lines before saving
	invoice := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01")
	adjusted := testEstimateLines()
	adjusted[1].Quantity = 200
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, invoice, adjusted, estimate.InvoiceId, 0))
	assert.Equal(t, estimate.InvoiceId, invoice.EstimateId)
	assert.Equal(t, int64(166666), invoice.Subtotal)

	second := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Equal(t, errs.ErrEstimateAlreadyConverted, Invoices.CreateInvoiceFromEstimate(ctx, second, testEstimateLines(), estimate.InvoiceId, 0))
	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, second, testEstimateLines(), estimate.InvoiceId, 0))

	// an invoice id is not an estimate
	third := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03")
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.CreateInvoiceFromEstimate(ctx, third, testEstimateLines(), second.InvoiceId, 0))
}

func TestConcurrentConversionsOfTheSameMilestoneOrEstimateMakeAtMostOneInvoice(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	withMilestones := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	whole := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, withMilestones.InvoiceId)
	assert.Nil(t, err)

	convert := func(estimateId int64, milestoneId int64, prefix string) int {
		const attempts = 4
		results := make(chan error, attempts)
		var wg sync.WaitGroup

		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				lines := testEstimateLines()
				if milestoneId > 0 {
					lines = []*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 1}}
				}
				results <- Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, prefix+strconv.Itoa(i)), lines, estimateId, milestoneId)
			}(i)
		}

		wg.Wait()
		close(results)
		succeeded := 0

		for err := range results {
			if err == nil {
				succeeded++
			}
		}

		return succeeded
	}

	// the losers fail (already converted, or the database being busy); never two invoices for one milestone or one estimate
	milestoneWins := convert(withMilestones.InvoiceId, milestones[0].MilestoneId, "INV/A/")
	wholeWins := convert(whole.InvoiceId, 0, "INV/B/")
	assert.LessOrEqual(t, milestoneWins, 1)
	assert.LessOrEqual(t, wholeWins, 1)

	progress, err := Invoices.GetEstimateProgress(ctx, uid, []int64{withMilestones.InvoiceId, whole.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, milestoneWins, len(progress[withMilestones.InvoiceId].Invoices))
	assert.Equal(t, wholeWins, len(progress[whole.InvoiceId].Invoices))
}
