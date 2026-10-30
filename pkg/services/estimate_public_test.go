package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// shareTestEstimate marks an estimate sent and gives it a public link, as emailing it does; it returns the token
func shareTestEstimate(t *testing.T, ctx core.Context, uid int64, estimateId int64) string {
	t.Helper()
	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("estimate_status").
		Where("uid=? AND invoice_id=?", uid, estimateId).Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT})
	assert.Nil(t, err)
	token, err := Invoices.SetInvoicePublicToken(ctx, uid, estimateId, true)
	assert.Nil(t, err)
	return token
}

func TestClientAcceptsEstimateThroughPublicLinkAndOwnerIsEmailed(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "a note is only kept on decline", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	assert.True(t, reloaded.RespondedByClient)
	assert.Greater(t, reloaded.RespondedUnixTime, int64(0))
	assert.Equal(t, "", reloaded.ResponseNote)

	user, err := Users.GetUserById(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(mailer.sent))
	assert.Equal(t, user.Email, mailer.sent[0].To)
	assert.Equal(t, "Quber Tech accepted Estimate EST/2026-27/01 (₹1,999.99)", mailer.sent[0].Subject)
	assert.Contains(t, mailer.sent[0].Body, "https://books.example/desktop#/invoice/list?invoiceId="+utils.Int64ToString(estimate.InvoiceId))

	// a second click, or the other button in another tab, records nothing and sends nothing
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))
	assert.Equal(t, 1, len(mailer.sent))
}

func TestConcurrentPublicAnswersRecordOnlyOne(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	// what a second request read just before the first one wrote
	stale, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	recorded, err := Invoices.recordEstimateClientResponse(ctx, stale, token, models.ESTIMATE_STATUS_DECLINED, "too late")
	assert.Nil(t, err)
	assert.False(t, recorded)

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	assert.Equal(t, "", reloaded.ResponseNote)
}

func TestPublicEstimateAnswerGuards(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	// never sent
	draft := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	draftToken, err := Invoices.SetInvoicePublicToken(ctx, uid, draft.InvoiceId, true)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, draftToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	// valid until April 30: refused the day after, accepted on the day itself
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	expiredToken := shareTestEstimate(t, ctx, uid, expired.InvoiceId)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, expiredToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-05-01"))
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, expiredToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-04-30"))

	sent := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/03", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, sent.InvoiceId)
	assert.Equal(t, errs.ErrEstimateResponseNoteTooLong, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, strings.Repeat("é", 501), "2026-10-30"))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_SENT, "", "2026-10-30"))

	// a disabled (or reset) link is a plain not-found
	_, err = Invoices.SetInvoicePublicToken(ctx, uid, sent.InvoiceId, false)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.RespondToEstimateByPublicToken(ctx, "short", models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))

	// an invoice's link answers nothing
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	invoiceToken, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, invoiceToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, sent.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
}

// TestPublicDeclineIsRefusedOnceInvoiced carries over the SetEstimateResponse rule to the public path: a decline is refused
// once any non-deleted invoice references the estimate, but an accept through the same link still goes through
func TestPublicDeclineIsRefusedOnceInvoiced(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, estimate.InvoiceId)

	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "too late", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
	assert.Equal(t, "", reloaded.ResponseNote)

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	reloaded, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
}

func TestClientDeclineKeepsThePlainTextNote(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	note := "<b>Too expensive</b>\nMaybe next quarter"

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "  "+note+"  ", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_DECLINED, reloaded.EstimateStatus)
	assert.Equal(t, note, reloaded.ResponseNote)
	assert.Equal(t, "Quber Tech declined Estimate EST/2026-27/01 (₹1,999.99)", mailer.sent[0].Subject)
	assert.Contains(t, mailer.sent[0].Body, "&lt;b&gt;Too expensive&lt;/b&gt;<br/>Maybe next quarter")
	assert.NotContains(t, mailer.sent[0].Body, "<b>Too")
}

func TestClientAnswerCountsWhenTheOwnerEmailFails(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	// SMTP off
	first := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, shareTestEstimate(t, ctx, uid, first.InvoiceId), models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	// SMTP on, sending fails
	withFailingMailer(t)
	second := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, shareTestEstimate(t, ctx, uid, second.InvoiceId), models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	for _, estimate := range []*models.Invoice{first, second} {
		reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
		assert.Nil(t, err)
		assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	}
}
