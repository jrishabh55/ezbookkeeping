package services

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// createEnabledTestSchedule saves an enabled schedule with exactly the given next date. CreateRecurringInvoice would catch a
// past date up to the real today, so the row is created paused and enabled in the database; the cron tests then drive
// CreateRecurringInvoices with explicit dates and stay independent of the day they run on.
func createEnabledTestSchedule(t *testing.T, recurring *models.RecurringInvoice) *models.RecurringInvoice {
	t.Helper()
	recurring.Enabled = false
	assert.Nil(t, Invoices.CreateRecurringInvoice(core.NewNullContext(), recurring))
	_, err := Invoices.UserDataDB(recurring.Uid).NewSession(core.NewNullContext()).Cols("enabled").Where("recurring_id=?", recurring.RecurringId).Update(&models.RecurringInvoice{Enabled: true})
	assert.Nil(t, err)
	recurring.Enabled = true
	return recurring
}

func createTestRecurringInvoice(t *testing.T, uid int64, sourceInvoiceId int64, dayOfMonth int32, nextDate string) *models.RecurringInvoice {
	t.Helper()
	return createEnabledTestSchedule(t, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: sourceInvoiceId, DayOfMonth: dayOfMonth, NextDate: nextDate})
}

func TestNextRecurringDateAdvancesByPeriod(t *testing.T) {
	next, err := nextRecurringDate("2026-12-15", 15, 1)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", next)

	next, err = nextRecurringDate("2026-01-31", 28, 1)
	assert.Nil(t, err)
	assert.Equal(t, "2026-02-28", next)

	next, err = nextRecurringDate("2026-11-10", 10, 3)
	assert.Nil(t, err)
	assert.Equal(t, "2027-02-10", next)

	next, err = nextRecurringDate("2026-03-28", 28, 12)
	assert.Nil(t, err)
	assert.Equal(t, "2027-03-28", next)

	_, err = nextRecurringDate("2026-13-01", 1, 1)
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)
}

func TestRecurringInvoiceCrudIsScopedToUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 37000000)

	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")
	assert.True(t, recurring.RecurringId > 0)

	_, err := Invoices.GetRecurringInvoice(ctx, otherUid, recurring.RecurringId)
	assert.Equal(t, errs.ErrRecurringInvoiceNotFound, err)

	recurring.DayOfMonth, recurring.NextDate, recurring.Enabled = 20, "2026-12-20", false
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, recurring))

	all, err := Invoices.GetAllRecurringInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))
	assert.Equal(t, int32(20), all[0].DayOfMonth)
	assert.Equal(t, "2026-12-20", all[0].NextDate)
	assert.False(t, all[0].Enabled)

	assert.Nil(t, Invoices.DeleteRecurringInvoice(ctx, uid, recurring.RecurringId))
	assert.Equal(t, errs.ErrRecurringInvoiceNotFound, Invoices.DeleteRecurringInvoice(ctx, uid, recurring.RecurringId))
	all, err = Invoices.GetAllRecurringInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))
}

func TestCreateRecurringInvoiceValidatesSourceAndDayOfMonth(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 37000000)

	err := Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId + 1, DayOfMonth: 15, NextDate: "2026-12-15", Enabled: true})
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 29, NextDate: "2026-12-29", Enabled: true})
	assert.Equal(t, errs.ErrRecurringInvoiceDayOfMonthInvalid, err)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "15-12-2026", Enabled: true})
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)
}

func TestCreateRecurringInvoicesCopiesSourceWithNextNumberAndFreshCustomer(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 37000000)
	source.Notes = "Pay by RTGS"
	assert.Nil(t, Invoices.ModifyInvoice(ctx, source, []*models.InvoiceLine{
		{ItemName: "Consulting", Description: "Software Consulting", Quantity: 750, Rate: 33333},
		{ItemName: "Support", Quantity: 100, Rate: 500000},
	}))

	customer.Name = "Quber Tech Ltd"
	customer.TermsDays = 10
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	_, _, err := Invoices.SetInvoicePublicToken(ctx, uid, source.InvoiceId, true)
	assert.Nil(t, err)
	_, err = Invoices.UserDataDB(uid).NewSession(ctx).Cols("sent_unix_time").Where("invoice_id=?", source.InvoiceId).Update(&models.Invoice{SentUnixTime: 1700000000})
	assert.Nil(t, err)

	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")

	// not due yet: nothing happens
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-14"))
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2026-12-15", reloaded.NextDate)
	assert.Equal(t, int64(0), reloaded.LastInvoiceId)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err = Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", reloaded.NextDate)
	assert.True(t, reloaded.LastInvoiceId > 0)
	assert.True(t, reloaded.LastRunUnixTime > 0)

	generated, lines, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "INV/2026-27/02", generated.Number)
	assert.Equal(t, "2026-12-15", generated.InvoiceDate)
	assert.Equal(t, "2026-12-25", generated.DueDate) // customer terms (10 days) win over the profile default
	assert.Equal(t, customer.CustomerId, generated.CustomerId)
	assert.Equal(t, "Quber Tech Ltd", generated.BillToName)
	assert.Equal(t, "Pay by RTGS", generated.Notes)
	assert.Equal(t, source.Currency, generated.Currency)
	assert.Equal(t, source.Total, generated.Total)
	assert.Equal(t, "", generated.PublicToken) // the source's public link and sent time are never inherited
	assert.Equal(t, int64(0), generated.SentUnixTime)
	assert.False(t, generated.ClosedManually)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "Consulting", lines[0].ItemName)
	assert.Equal(t, "Software Consulting", lines[0].Description)
	assert.Equal(t, int64(750), lines[0].Quantity)
	assert.Equal(t, int64(33333), lines[0].Rate)
	assert.Equal(t, "Support", lines[1].ItemName)
	assert.Equal(t, int32(2), lines[1].DisplayOrder)

	// the source keeps its old snapshot
	sourceReloaded, _, err := Invoices.GetInvoice(ctx, uid, source.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "Quber Tech", sourceReloaded.BillToName)

	// the same day again does not create a second invoice
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
}

func TestCreateRecurringInvoicesSkipsDisabledRow(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 37000000)

	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")
	recurring.Enabled = false
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, recurring))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-31"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2026-12-15", reloaded.NextDate)
	assert.Equal(t, int64(0), reloaded.LastInvoiceId)

	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))
}

func TestCreateRecurringInvoicesSkipsDeletedSourceWithoutError(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 37000000)

	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, source.InvoiceId))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2026-12-15", reloaded.NextDate)
	assert.Equal(t, int64(0), reloaded.LastInvoiceId)

	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))
}

func TestCatchUpRecurringDate(t *testing.T) {
	// monthly: a stale next date moves to the next occurrence of the day of month on or after today
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-01"))
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-15"))
	assert.Equal(t, "2026-11-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-16"))
	assert.Equal(t, "2027-01-05", catchUpRecurringDate("2026-07-05", 5, 1, "2026-12-31"))
	// a future next date is left alone
	assert.Equal(t, "2026-12-15", catchUpRecurringDate("2026-12-15", 15, 1, "2026-10-01"))
	// yearly: steps by whole years so the anniversary is kept
	assert.Equal(t, "2027-03-10", catchUpRecurringDate("2024-03-10", 10, 12, "2026-10-29"))
	// quarterly
	assert.Equal(t, "2026-11-01", catchUpRecurringDate("2026-02-01", 1, 3, "2026-10-29"))
}

func TestRecurringInvoicePeriodAndModeDefaultAndValidate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)

	// zero values mean today's behaviour
	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, int32(1), reloaded.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_CREATE, reloaded.Mode)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2026-12-15", PeriodMonths: 121, Enabled: true})
	assert.Equal(t, errs.ErrRecurringInvoicePeriodInvalid, err)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2026-12-15", Mode: "weekly", Enabled: true})
	assert.Equal(t, errs.ErrRecurringInvoiceModeInvalid, err)

	reloaded.PeriodMonths, reloaded.Mode = 12, models.RECURRING_INVOICE_MODE_AUTO_SEND
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, reloaded))
	reloaded, err = Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, int32(12), reloaded.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_AUTO_SEND, reloaded.Mode)
}

func TestCreateRecurringInvoicesAdvancesByPeriod(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, "")

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-12-15", reloaded.NextDate)
}

func TestModifyRecurringInvoiceEnablingCatchesUpAStaleNextDate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2020-07-15")

	recurring.Enabled = true
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, recurring))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.True(t, reloaded.NextDate >= time.Now().Format(invoiceDateLayout), reloaded.NextDate)
	assert.Equal(t, "15", reloaded.NextDate[8:])
}

type capturingMailer struct {
	sent []*mail.MailMessage
}

func (m *capturingMailer) SendMail(message *mail.MailMessage) error {
	m.sent = append(m.sent, message)
	return nil
}

// withCapturingMailer enables SMTP for one test and records every sent mail instead of sending it
func withCapturingMailer(t *testing.T) *capturingMailer {
	t.Helper()
	withSmtpEnabled(t)
	mailer := &capturingMailer{}
	previous := mail.Container.SetMailer(mailer)
	t.Cleanup(func() { mail.Container.SetMailer(previous) })
	return mailer
}

func createTestRenewal(t *testing.T, uid int64, sourceInvoiceId int64, nextDate string, periodMonths int32, mode string) *models.RecurringInvoice {
	t.Helper()
	return createEnabledTestSchedule(t, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: sourceInvoiceId, DayOfMonth: 15, NextDate: nextDate, PeriodMonths: periodMonths, Mode: mode})
}

func TestDraftAheadCreatesOneDraftInsideTheWindow(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	// 16 days before: outside the default 15-day window
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-11-29"))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))

	// exactly 15 days before: the draft is created, dated on the renewal date, and not sent
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-11-30"))
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-12-15", reloaded.NextDate)

	draft, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "2026-12-15", draft.InvoiceDate)
	assert.Equal(t, int64(0), draft.SentUnixTime)
	assert.Equal(t, recurring.RecurringId, draft.RecurringId)

	// the next days of the window do not create another one
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-01"))
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))
	all, err = Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
}

func TestDraftAheadUsesTheProfileDraftDays(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	profile.RenewalDraftDays = 3
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 1, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-11"))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-12"))
	all, err = Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
}

func TestAutoSendEmailsTheCustomerOnTheDate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	customer.Email = "accounts@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 6, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-14"))
	assert.Equal(t, 0, len(mailer.sent))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))
	assert.Equal(t, 1, len(mailer.sent))
	assert.Equal(t, "accounts@qubertech.com", mailer.sent[0].To)

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-06-15", reloaded.NextDate)
	sent, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.True(t, sent.SentUnixTime > 0)
}

func TestAutoSendFailureKeepsTheUnsentInvoiceAndAdvances(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withCapturingMailer(t)
	customer := createTestCustomer(t, uid) // no email address
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 1, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", reloaded.NextDate)
	unsent, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), unsent.SentUnixTime)
}

type failingMailer struct{}

func (m *failingMailer) SendMail(message *mail.MailMessage) error {
	return errors.New("smtp server unreachable")
}

// withFailingMailer enables SMTP for one test and makes every send fail, as if the SMTP server were down
func withFailingMailer(t *testing.T) {
	t.Helper()
	withSmtpEnabled(t)
	previous := mail.Container.SetMailer(&failingMailer{})
	t.Cleanup(func() { mail.Container.SetMailer(previous) })
}

func TestAutoSendSmtpFailureKeepsTheUnsentInvoiceAndAdvances(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withFailingMailer(t)
	customer := createTestCustomer(t, uid)
	customer.Email = "accounts@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 1, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", reloaded.NextDate)
	assert.True(t, reloaded.LastInvoiceId > 0)
	unsent, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), unsent.SentUnixTime)
}

func TestGetInvoiceRenewalsClassifiesGeneratedInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid) // no email: auto-send fails
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)

	draftAhead := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	autoSend := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)
	createOnly := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_CREATE)
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	now := time.Now().Unix()
	renewals, err := Invoices.GetInvoiceRenewals(ctx, uid, now)
	assert.Nil(t, err)
	states := map[int64]string{}
	for _, renewal := range renewals {
		states[renewal.RecurringId] = renewal.State
	}
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_DUE, states[draftAhead.RecurringId])
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_SEND_FAILED, states[autoSend.RecurringId])
	_, listed := states[createOnly.RecurringId]
	assert.False(t, listed)

	// once the failed one is sent it shows as auto sent for 7 days, then disappears
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, autoSend.RecurringId)
	assert.Nil(t, err)
	_, err = Invoices.UserDataDB(uid).NewSession(ctx).Cols("sent_unix_time").Where("invoice_id=?", reloaded.LastInvoiceId).Update(&models.Invoice{SentUnixTime: now - 3600})
	assert.Nil(t, err)
	renewals, err = Invoices.GetInvoiceRenewals(ctx, uid, now)
	assert.Nil(t, err)
	states = map[int64]string{}
	for _, renewal := range renewals {
		states[renewal.RecurringId] = renewal.State
	}
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_AUTO_SENT, states[autoSend.RecurringId])

	renewals, err = Invoices.GetInvoiceRenewals(ctx, uid, now+8*24*3600)
	assert.Nil(t, err)
	for _, renewal := range renewals {
		assert.NotEqual(t, autoSend.RecurringId, renewal.RecurringId)
	}
}

func TestRenewalOptionForLines(t *testing.T) {
	products := map[int64]*models.Product{
		1: {ProductId: 1, RenewalMonths: 12, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_AUTO_SEND},
		2: {ProductId: 2, RenewalMonths: 12, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_DRAFT_AHEAD},
		3: {ProductId: 3, RenewalMonths: 6, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_CREATE},
		4: {ProductId: 4},
	}

	assert.Nil(t, renewalOptionForLines([]*models.InvoiceLine{{ProductId: 4}, {ProductId: 0}}, products))

	option := renewalOptionForLines([]*models.InvoiceLine{{ProductId: 1}, {ProductId: 2}, {ProductId: 4}}, products)
	assert.Equal(t, int32(12), option.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD, option.Mode) // the most hands-on mode wins
	assert.False(t, option.MixedPeriods)

	option = renewalOptionForLines([]*models.InvoiceLine{{ProductId: 1}, {ProductId: 3}}, products)
	assert.True(t, option.MixedPeriods)
}

func TestCreateRenewalForInvoiceOnce(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)
	invoice := &models.Invoice{Uid: uid, Number: "INV/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-10-31", DueDate: "2026-11-07"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, []*models.InvoiceLine{{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000}}))

	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.False(t, option.Scheduled)

	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int32(12), recurring.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_AUTO_SEND, recurring.Mode)
	assert.Equal(t, int32(28), recurring.DayOfMonth)
	assert.Equal(t, catchUpRecurringDate("2027-10-28", 28, 12, time.Now().Format(invoiceDateLayout)), recurring.NextDate) // one year after the invoice, or its next anniversary

	option, err = Invoices.GetInvoiceRenewalOption(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.True(t, option.Scheduled)

	again, err := Invoices.CreateRenewalForInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, again)
}

func TestCreateRenewalForInvoiceSkipsMixedPeriodsAndGeneratedInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	amc := createTestProduct(t, uid, "AMC", 6, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	mixed := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
		{ItemName: "AMC", ProductId: amc.ProductId, Quantity: 100, Rate: 1},
	})
	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, mixed.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)

	// an invoice the scheduler generated is renewed by its own schedule
	single := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
	})
	_, err = Invoices.UserDataDB(uid).NewSession(ctx).Cols("recurring_id").Where("invoice_id=?", single.InvoiceId).Update(&models.Invoice{RecurringId: 42})
	assert.Nil(t, err)
	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, single.InvoiceId)
	assert.Nil(t, err)
	assert.True(t, option.Scheduled)
	recurring, err = Invoices.CreateRenewalForInvoice(ctx, uid, single.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)
}

func TestCreateRenewalForAnOldInvoiceStartsFromTodayAndBackFillsNothing(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	customer.Email = "accounts@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))
	hosting := createTestProduct(t, uid, "Hosting", 1, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	now := time.Now()
	today := now.Format(invoiceDateLayout)
	invoiceDate := time.Date(now.Year(), now.Month()-14, 10, 0, 0, 0, 0, time.UTC).Format(invoiceDateLayout)
	invoice := &models.Invoice{Uid: uid, Number: "INV/2025-26/01", CustomerId: customer.CustomerId, InvoiceDate: invoiceDate, DueDate: invoiceDate}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, []*models.InvoiceLine{{ItemName: "Hosting", ProductId: hosting.ProductId, Quantity: 100, Rate: 150000}}))

	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.True(t, recurring.NextDate >= today, recurring.NextDate)
	assert.True(t, recurring.NextDate <= now.AddDate(0, 1, 0).Format(invoiceDateLayout), recurring.NextDate)
	assert.Equal(t, "10", recurring.NextDate[8:])

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, recurring.NextDate, reloaded.NextDate)

	// the cron job today creates (and emails) at most one invoice, never one per missed month
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, today))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.True(t, len(all) <= 2, len(all))
	assert.True(t, len(mailer.sent) <= 1, len(mailer.sent))
}

func TestCreateRecurringInvoiceCatchesUpAPastNextDate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)

	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2020-07-15", PeriodMonths: 12, Enabled: true}
	assert.Nil(t, Invoices.CreateRecurringInvoice(ctx, recurring))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.True(t, reloaded.NextDate >= time.Now().Format(invoiceDateLayout), reloaded.NextDate)
	assert.Equal(t, "07-15", reloaded.NextDate[5:]) // whole years, so the anniversary is kept

	// a paused schedule keeps the date it was given
	paused := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2020-07-15", Enabled: false}
	assert.Nil(t, Invoices.CreateRecurringInvoice(ctx, paused))
	reloaded, err = Invoices.GetRecurringInvoice(ctx, uid, paused.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2020-07-15", reloaded.NextDate)
}

func TestModifyRecurringInvoiceKeepsStoredPeriodAndModeWhenOmitted(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2099-12-15", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	// an older client only knows day of month, next date and enabled
	stale := &models.RecurringInvoice{Uid: uid, RecurringId: recurring.RecurringId, DayOfMonth: 15, NextDate: "2099-12-15", Enabled: false}
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, stale))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.False(t, reloaded.Enabled)
	assert.Equal(t, int32(12), reloaded.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_AUTO_SEND, reloaded.Mode)

	// another user's id is still not found
	_, otherUid := newAlertTestUser(t)
	assert.Equal(t, errs.ErrRecurringInvoiceNotFound, Invoices.ModifyRecurringInvoice(ctx, &models.RecurringInvoice{Uid: otherUid, RecurringId: recurring.RecurringId, DayOfMonth: 15, NextDate: "2099-12-15"}))
}

func TestGetInvoiceRenewalsSkipsPaidUnsentInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid) // no email: auto-send fails
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 10000000)

	draftAhead := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	autoSend := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	draftAhead, err := Invoices.GetRecurringInvoice(ctx, uid, draftAhead.RecurringId)
	assert.Nil(t, err)
	autoSend, err = Invoices.GetRecurringInvoice(ctx, uid, autoSend.RecurringId)
	assert.Nil(t, err)

	// the draft is paid within the bank-charge tolerance, the failed auto-send only in part
	paid := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 9800000, "2026-12-16")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, draftAhead.LastInvoiceId, paid.TransactionId, 0))
	part := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 5000000, "2026-12-16")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, autoSend.LastInvoiceId, part.TransactionId, 0))

	renewals, err := Invoices.GetInvoiceRenewals(ctx, uid, time.Now().Unix())
	assert.Nil(t, err)
	states := map[int64]string{}
	for _, renewal := range renewals {
		states[renewal.RecurringId] = renewal.State
	}
	_, listed := states[draftAhead.RecurringId]
	assert.False(t, listed)
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_SEND_FAILED, states[autoSend.RecurringId])

	// once the rest arrives the failed one drops off too
	rest := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 5000000, "2026-12-17")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, autoSend.LastInvoiceId, rest.TransactionId, 0))
	renewals, err = Invoices.GetInvoiceRenewals(ctx, uid, time.Now().Unix())
	assert.Nil(t, err)
	assert.Equal(t, 0, len(renewals))
}
