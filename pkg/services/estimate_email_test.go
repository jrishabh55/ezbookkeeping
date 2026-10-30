package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestSendingAnEstimateMarksItSentWithEstimateWording(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, estimate.InvoiceId, "accounts@qubertech.com", "", nil, nil))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
	assert.Greater(t, reloaded.SentUnixTime, int64(0))
	assert.NotEqual(t, "", reloaded.PublicToken)
	assert.Equal(t, "Estimate EST/2026-27/01", mailer.sent[0].Subject) // the profile has no business name
	assert.Contains(t, mailer.sent[0].Body, "View Estimate")
	assert.Equal(t, "Estimate_EST_2026-27_01_2099-12-31.pdf", mailer.sent[0].Attachments[0].Name)

	// sending again after the client answered never moves the status back
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, estimate.InvoiceId, "accounts@qubertech.com", "", nil, nil))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)

	// an invoice keeps its own wording
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "accounts@qubertech.com", "", nil, nil))
	assert.Equal(t, "Invoice INV/2026-27/01", mailer.sent[2].Subject)
}

func TestEmailingASupersededEstimateFailsAndSendsNothing(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	_, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SendInvoiceWithContent(ctx, uid, estimate.InvoiceId, "accounts@qubertech.com", "", nil, nil))
	assert.Empty(t, mailer.sent)
}
