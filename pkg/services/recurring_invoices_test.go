package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func createTestRecurringInvoice(t *testing.T, uid int64, sourceInvoiceId int64, dayOfMonth int32, nextDate string) *models.RecurringInvoice {
	t.Helper()
	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: sourceInvoiceId, DayOfMonth: dayOfMonth, NextDate: nextDate, Enabled: true}
	assert.Nil(t, Invoices.CreateRecurringInvoice(core.NewNullContext(), recurring))
	return recurring
}

func TestNextRecurringDateAdvancesOneMonthAcrossYearBoundary(t *testing.T) {
	next, err := nextRecurringDate("2026-12-15", 15)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", next)

	next, err = nextRecurringDate("2026-01-31", 28)
	assert.Nil(t, err)
	assert.Equal(t, "2026-02-28", next)

	_, err = nextRecurringDate("2026-13-01", 1)
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

	_, err := Invoices.SetInvoicePublicToken(ctx, uid, source.InvoiceId, true)
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
	// a stale next date moves to the next occurrence of the day of month on or after today
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, "2026-10-01"))
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, "2026-10-15"))
	assert.Equal(t, "2026-11-15", catchUpRecurringDate("2026-07-15", 15, "2026-10-16"))
	assert.Equal(t, "2027-01-05", catchUpRecurringDate("2026-07-05", 5, "2026-12-31"))
	// a future next date is left alone
	assert.Equal(t, "2026-12-15", catchUpRecurringDate("2026-12-15", 15, "2026-10-01"))
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
