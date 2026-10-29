package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// nextRecurringDate returns dayOfMonth of the month periodMonths after date; dayOfMonth is at most 28, so every month has it
func nextRecurringDate(date string, dayOfMonth int32, periodMonths int32) (string, error) {
	t, err := time.Parse(invoiceDateLayout, date)

	if err != nil {
		return "", errs.ErrInvoiceDateInvalid
	}

	return time.Date(t.Year(), t.Month()+time.Month(periodMonths), int(dayOfMonth), 0, 0, 0, 0, time.UTC).Format(invoiceDateLayout), nil
}

// catchUpRecurringDate steps a next date that is already in the past forward by whole periods until it is on or after today,
// so re-enabling a paused row does not back-fill one invoice per missed period and a yearly row keeps its anniversary
func catchUpRecurringDate(nextDate string, dayOfMonth int32, periodMonths int32, today string) string {
	candidate := nextDate

	for candidate < today {
		next, err := nextRecurringDate(candidate, dayOfMonth, periodMonths)

		if err != nil {
			return nextDate
		}

		candidate = next
	}

	return candidate
}

// validateRecurringInvoice fills the period and mode defaults (1 month, create) and checks every field
func validateRecurringInvoice(recurring *models.RecurringInvoice) error {
	if recurring.DayOfMonth < 1 || recurring.DayOfMonth > models.RecurringInvoiceMaxDayOfMonth {
		return errs.ErrRecurringInvoiceDayOfMonthInvalid
	}

	if _, err := time.Parse(invoiceDateLayout, recurring.NextDate); err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	if recurring.PeriodMonths == 0 {
		recurring.PeriodMonths = 1
	}

	if recurring.PeriodMonths < 1 || recurring.PeriodMonths > models.RecurringInvoiceMaxPeriodMonths {
		return errs.ErrRecurringInvoicePeriodInvalid
	}

	if recurring.Mode == "" {
		recurring.Mode = models.RECURRING_INVOICE_MODE_CREATE
	}

	if !models.IsValidRecurringInvoiceMode(recurring.Mode) {
		return errs.ErrRecurringInvoiceModeInvalid
	}

	return nil
}

// GetAllRecurringInvoices returns all recurring invoices of user, soonest next date first
func (s *InvoiceService) GetAllRecurringInvoices(c core.Context, uid int64) ([]*models.RecurringInvoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var recurrings []*models.RecurringInvoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("next_date asc, recurring_id asc").Find(&recurrings)

	return recurrings, err
}

// GetRecurringInvoice returns one recurring invoice of user
func (s *InvoiceService) GetRecurringInvoice(c core.Context, uid int64, recurringId int64) (*models.RecurringInvoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if recurringId <= 0 {
		return nil, errs.ErrRecurringInvoiceIdInvalid
	}

	recurring := &models.RecurringInvoice{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND recurring_id=?", uid, false, recurringId).Get(recurring)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrRecurringInvoiceNotFound
	}

	return recurring, nil
}

// CreateRecurringInvoice saves a new recurring invoice whose source invoice must exist
func (s *InvoiceService) CreateRecurringInvoice(c core.Context, recurring *models.RecurringInvoice) error {
	if recurring.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if err := validateRecurringInvoice(recurring); err != nil {
		return err
	}

	if _, _, err := s.GetInvoice(c, recurring.Uid, recurring.SourceInvoiceId); err != nil {
		return err
	}

	// a schedule started today never back-fills: the daily job creates one period per run, so a past next date would
	// produce (and, for auto_send, email) one back-dated invoice per night until it reached today
	if recurring.Enabled {
		recurring.NextDate = catchUpRecurringDate(recurring.NextDate, recurring.DayOfMonth, recurring.PeriodMonths, time.Now().Format(invoiceDateLayout))
	}

	recurring.RecurringId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

	if recurring.RecurringId < 1 {
		return errs.ErrSystemIsBusy
	}

	recurring.Deleted = false
	recurring.LastInvoiceId = 0
	recurring.LastRunUnixTime = 0
	recurring.CreatedUnixTime = time.Now().Unix()
	recurring.UpdatedUnixTime = recurring.CreatedUnixTime

	return s.UserDataDB(recurring.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(recurring)
		return err
	})
}

// ModifyRecurringInvoice saves the day of month, next date and enabled flag of an existing recurring invoice
func (s *InvoiceService) ModifyRecurringInvoice(c core.Context, recurring *models.RecurringInvoice) error {
	if recurring.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if recurring.RecurringId <= 0 {
		return errs.ErrRecurringInvoiceIdInvalid
	}

	// an older client sends no period or mode: keep the stored ones instead of resetting them to the create defaults
	if recurring.PeriodMonths == 0 || recurring.Mode == "" {
		stored, err := s.GetRecurringInvoice(c, recurring.Uid, recurring.RecurringId)

		if err != nil {
			return err
		}

		if recurring.PeriodMonths == 0 {
			recurring.PeriodMonths = stored.PeriodMonths
		}

		if recurring.Mode == "" {
			recurring.Mode = stored.Mode
		}
	}

	if err := validateRecurringInvoice(recurring); err != nil {
		return err
	}

	if recurring.Enabled {
		recurring.NextDate = catchUpRecurringDate(recurring.NextDate, recurring.DayOfMonth, recurring.PeriodMonths, time.Now().Format(invoiceDateLayout))
	}

	recurring.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(recurring.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("day_of_month", "next_date", "enabled", "period_months", "mode", "updated_unix_time").
			Where("uid=? AND deleted=? AND recurring_id=?", recurring.Uid, false, recurring.RecurringId).Update(recurring)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrRecurringInvoiceNotFound
		}

		return nil
	})
}

// DeleteRecurringInvoice soft-deletes a recurring invoice; the invoices it created are untouched
func (s *InvoiceService) DeleteRecurringInvoice(c core.Context, uid int64, recurringId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if recurringId <= 0 {
		return errs.ErrRecurringInvoiceIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND recurring_id=?", uid, false, recurringId).
			Update(&models.RecurringInvoice{Deleted: true, DeletedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrRecurringInvoiceNotFound
		}

		return nil
	})
}

// CreateRecurringInvoices creates the next invoice of every enabled recurring invoice that is due: on the next date for create
// and auto_send rows, and up to the profile's renewal draft days earlier for draft_ahead rows. auto_send invoices are emailed
// to the customer right after they are created. A failing row is logged and left for the next run; it never fails the cron job.
func (s *InvoiceService) CreateRecurringInvoices(c core.Context, today string) error {
	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	// draft_ahead rows are read up to the longest lead time any user may set, then filtered by their own profile
	latestDraftDate := todayTime.AddDate(0, 0, models.InvoiceRenewalDraftDaysMax).Format(invoiceDateLayout)

	for i := 0; i < s.UserDataDBCount(); i++ {
		var recurrings []*models.RecurringInvoice
		err := s.UserDataDBByIndex(i).NewSession(c).
			Where("deleted=? AND enabled=? AND (next_date<=? OR (mode=? AND next_date<=?))", false, true, today, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD, latestDraftDate).
			Find(&recurrings)

		if err != nil {
			return err
		}

		draftDaysByUid := make(map[int64]int32)

		for _, recurring := range recurrings {
			if recurring.NextDate > today {
				draftDays, loaded := draftDaysByUid[recurring.Uid]

				if !loaded {
					profile, err := s.GetProfile(c, recurring.Uid)

					if err != nil {
						log.Errorf(c, "[recurring_invoices.CreateRecurringInvoices] failed to get invoice profile of user \"uid:%d\", because %s", recurring.Uid, err.Error())
						continue
					}

					draftDays = profile.RenewalDraftDays
					draftDaysByUid[recurring.Uid] = draftDays
				}

				if recurring.NextDate > todayTime.AddDate(0, 0, int(draftDays)).Format(invoiceDateLayout) {
					continue
				}
			}

			invoice, err := s.createRecurringInvoice(c, recurring)

			if err != nil {
				log.Errorf(c, "[recurring_invoices.CreateRecurringInvoices] failed to create invoice for recurring invoice \"id:%d\" of user \"uid:%d\", because %s", recurring.RecurringId, recurring.Uid, err.Error())
				continue
			}

			if invoice != nil && recurring.Mode == models.RECURRING_INVOICE_MODE_AUTO_SEND {
				s.autoSendRecurringInvoice(c, invoice)
			}
		}
	}

	return nil
}

// renewalModeRank orders modes from least to most hands-on; when products disagree, the most hands-on one wins
var renewalModeRank = map[string]int{
	models.RECURRING_INVOICE_MODE_AUTO_SEND:   1,
	models.RECURRING_INVOICE_MODE_CREATE:      2,
	models.RECURRING_INVOICE_MODE_DRAFT_AHEAD: 3,
}

// renewalOptionForLines returns the renewal the lines' renewable products ask for, or nil when none is renewable
func renewalOptionForLines(lines []*models.InvoiceLine, products map[int64]*models.Product) *models.InvoiceRenewalOption {
	var option *models.InvoiceRenewalOption

	for _, line := range lines {
		product := products[line.ProductId]

		if product == nil || product.RenewalMonths < 1 {
			continue
		}

		if option == nil {
			option = &models.InvoiceRenewalOption{PeriodMonths: product.RenewalMonths, Mode: product.DefaultRenewalMode}
			continue
		}

		if product.RenewalMonths != option.PeriodMonths {
			option.MixedPeriods = true
		}

		if renewalModeRank[product.DefaultRenewalMode] > renewalModeRank[option.Mode] {
			option.Mode = product.DefaultRenewalMode
		}
	}

	return option
}

// GetInvoiceRenewalOption returns the renewal sending this invoice can start, or nil when it has no renewable product
func (s *InvoiceService) GetInvoiceRenewalOption(c core.Context, uid int64, invoiceId int64) (*models.InvoiceRenewalOption, error) {
	invoice, lines, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	productIds := make([]int64, 0, len(lines))

	for _, line := range lines {
		if line.ProductId > 0 {
			productIds = append(productIds, line.ProductId)
		}
	}

	products, err := s.GetProductsByIds(c, uid, productIds)

	if err != nil {
		return nil, err
	}

	option := renewalOptionForLines(lines, products)

	if option == nil {
		return nil, nil
	}

	scheduled := invoice.RecurringId > 0

	if !scheduled {
		scheduled, err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND source_invoice_id=?", uid, false, invoiceId).Exist(&models.RecurringInvoice{})

		if err != nil {
			return nil, err
		}
	}

	option.Scheduled = scheduled
	return option, nil
}

// CreateRenewalForInvoice starts the renewal schedule of a sent invoice: every period months on its day of month (capped at 28),
// first one period after the invoice date, or the first such date on or after today for an older invoice (CreateRecurringInvoice catches up).
// It returns nil when there is nothing to create (no renewable product, mixed periods, already scheduled).
func (s *InvoiceService) CreateRenewalForInvoice(c core.Context, uid int64, invoiceId int64) (*models.RecurringInvoice, error) {
	option, err := s.GetInvoiceRenewalOption(c, uid, invoiceId)

	if err != nil || option == nil || option.MixedPeriods || option.Scheduled {
		return nil, err
	}

	invoice, err := s.getInvoiceHeader(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	invoiceDate, err := time.Parse(invoiceDateLayout, invoice.InvoiceDate)

	if err != nil {
		return nil, errs.ErrInvoiceDateInvalid
	}

	dayOfMonth := int32(min(invoiceDate.Day(), models.RecurringInvoiceMaxDayOfMonth))
	nextDate, err := nextRecurringDate(invoice.InvoiceDate, dayOfMonth, option.PeriodMonths)

	if err != nil {
		return nil, err
	}

	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: invoiceId, DayOfMonth: dayOfMonth, NextDate: nextDate, PeriodMonths: option.PeriodMonths, Mode: option.Mode, Enabled: true}

	if err := s.CreateRecurringInvoice(c, recurring); err != nil {
		return nil, err
	}

	return recurring, nil
}

// GetInvoiceRenewals returns the generated invoices the home card lists, soonest invoice date first: unsent, unpaid draft_ahead
// invoices (due), unsent, unpaid auto_send invoices (send failed) and auto_send invoices sent in the last 7 days (auto sent).
// The schedule's current mode decides the state; create-mode invoices are never listed.
func (s *InvoiceService) GetInvoiceRenewals(c core.Context, uid int64, nowUnixTime int64) ([]*models.InvoiceRenewal, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND recurring_id>? AND closed_manually=? AND (sent_unix_time=? OR sent_unix_time>=?)", uid, false, 0, false, 0, nowUnixTime-models.InvoiceRenewalAutoSentVisibleSeconds).
		OrderBy("invoice_date asc, number asc").Find(&invoices)

	if err != nil || len(invoices) < 1 {
		return nil, err
	}

	recurringIds := make([]int64, 0, len(invoices))

	for _, invoice := range invoices {
		recurringIds = append(recurringIds, invoice.RecurringId)
	}

	var recurrings []*models.RecurringInvoice
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("recurring_id", recurringIds).Find(&recurrings)

	if err != nil {
		return nil, err
	}

	modes := make(map[int64]string, len(recurrings))

	for _, recurring := range recurrings {
		modes[recurring.RecurringId] = recurring.Mode
	}

	invoiceIds := make([]int64, 0, len(invoices))

	for _, invoice := range invoices {
		invoiceIds = append(invoiceIds, invoice.InvoiceId)
	}

	payments, err := s.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		return nil, err
	}

	renewals := make([]*models.InvoiceRenewal, 0, len(invoices))

	for _, invoice := range invoices {
		state := ""

		switch mode := modes[invoice.RecurringId]; {
		case mode == models.RECURRING_INVOICE_MODE_DRAFT_AHEAD && invoice.SentUnixTime == 0:
			state = models.INVOICE_RENEWAL_STATE_DUE
		case mode == models.RECURRING_INVOICE_MODE_AUTO_SEND && invoice.SentUnixTime == 0:
			state = models.INVOICE_RENEWAL_STATE_SEND_FAILED
		case mode == models.RECURRING_INVOICE_MODE_AUTO_SEND:
			state = models.INVOICE_RENEWAL_STATE_AUTO_SENT
		}

		// an unsent invoice that has been paid in full needs no sending (same paid rule as the rest of the app)
		if (state == models.INVOICE_RENEWAL_STATE_DUE || state == models.INVOICE_RENEWAL_STATE_SEND_FAILED) &&
			float64(invoice.Total-models.InvoiceReceivedAmount(payments[invoice.InvoiceId])) <= float64(invoice.Total)*models.InvoicePaidTolerance {
			continue
		}

		if state != "" {
			renewals = append(renewals, &models.InvoiceRenewal{State: state, RecurringId: invoice.RecurringId, Invoice: invoice})
		}
	}

	return renewals, nil
}

// autoSendRecurringInvoice emails a just-created auto_send invoice to its bill-to address with the saved template.
// A failure only logs: the invoice stays unsent and the home card offers to send it by hand.
func (s *InvoiceService) autoSendRecurringInvoice(c core.Context, invoice *models.Invoice) {
	if invoice.BillToEmail == "" {
		log.Warnf(c, "[recurring_invoices.autoSendRecurringInvoice] invoice \"id:%d\" of user \"uid:%d\" was not sent, because the customer has no email", invoice.InvoiceId, invoice.Uid)
		return
	}

	if err := s.SendInvoiceWithContent(c, invoice.Uid, invoice.InvoiceId, invoice.BillToEmail, "", nil, nil); err != nil {
		log.Warnf(c, "[recurring_invoices.autoSendRecurringInvoice] failed to send invoice \"id:%d\" of user \"uid:%d\", because %s", invoice.InvoiceId, invoice.Uid, err.Error())
	}
}

// createRecurringInvoice creates one invoice from the recurring invoice's source and advances its next date;
// it returns nil when the source invoice has been deleted
func (s *InvoiceService) createRecurringInvoice(c core.Context, recurring *models.RecurringInvoice) (*models.Invoice, error) {
	source, sourceLines, err := s.GetInvoice(c, recurring.Uid, recurring.SourceInvoiceId)

	if err == errs.ErrInvoiceNotFound {
		log.Warnf(c, "[recurring_invoices.createRecurringInvoice] source invoice \"id:%d\" of recurring invoice \"id:%d\" of user \"uid:%d\" is deleted, skipped", recurring.SourceInvoiceId, recurring.RecurringId, recurring.Uid)
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// ponytail: CreateInvoice always re-reads the customer for the bill-to snapshot and fails when it is gone,
	// so the spec's "fall back to the source snapshot" cannot be reached here; it also cannot happen, because
	// DeleteCustomer refuses while a non-deleted invoice (the source) still uses the customer
	customer, err := s.GetCustomer(c, recurring.Uid, source.CustomerId)

	if err != nil {
		return nil, err
	}

	profile, err := s.GetProfile(c, recurring.Uid)

	if err != nil {
		return nil, err
	}

	termsDays := profile.DefaultTermsDays

	if customer.TermsDays > 0 {
		termsDays = customer.TermsDays
	}

	number, err := s.NextInvoiceNumber(c, recurring.Uid, recurring.NextDate)

	if err != nil {
		return nil, err
	}

	invoiceDate, _ := time.Parse(invoiceDateLayout, recurring.NextDate)

	// everything the source carries (notes, currency, tax, ...) is copied; CreateInvoice resets identity,
	// status, timestamps and the bill-to snapshot, so only number and dates are set here
	invoice := *source
	invoice.InvoiceId = 0
	invoice.Number = number
	invoice.InvoiceDate = recurring.NextDate
	invoice.DueDate = invoiceDate.AddDate(0, 0, int(termsDays)).Format(invoiceDateLayout)

	lines := make([]*models.InvoiceLine, len(sourceLines))

	for idx, sourceLine := range sourceLines {
		line := *sourceLine
		line.LineId = 0
		lines[idx] = &line
	}

	nextDate, err := nextRecurringDate(recurring.NextDate, recurring.DayOfMonth, max(recurring.PeriodMonths, 1))

	if err != nil {
		return nil, err
	}

	if err := s.prepareNewInvoice(c, &invoice, lines); err != nil {
		return nil, err
	}

	invoice.RecurringId = recurring.RecurringId

	recurring.LastInvoiceId = invoice.InvoiceId
	recurring.LastRunUnixTime = time.Now().Unix()
	recurring.NextDate = nextDate
	recurring.UpdatedUnixTime = recurring.LastRunUnixTime

	// the invoice and the row advance commit together, so a crash in between cannot create the same period twice
	err = s.UserDataDB(recurring.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := insertInvoice(sess, &invoice, lines); err != nil {
			return err
		}

		_, err := sess.Cols("last_invoice_id", "last_run_unix_time", "next_date", "updated_unix_time").
			Where("uid=? AND recurring_id=?", recurring.Uid, recurring.RecurringId).Update(recurring)
		return err
	})

	if err != nil {
		return nil, err
	}

	log.Infof(c, "[recurring_invoices.createRecurringInvoice] recurring invoice \"id:%d\" of user \"uid:%d\" has created invoice \"id:%d\" (%s), next date %s", recurring.RecurringId, recurring.Uid, invoice.InvoiceId, invoice.Number, recurring.NextDate)
	return &invoice, nil
}
