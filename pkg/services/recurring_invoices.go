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

// nextRecurringDate returns dayOfMonth of the month after date; dayOfMonth is at most 28, so every month has it
func nextRecurringDate(date string, dayOfMonth int32) (string, error) {
	t, err := time.Parse(invoiceDateLayout, date)

	if err != nil {
		return "", errs.ErrInvoiceDateInvalid
	}

	return time.Date(t.Year(), t.Month()+1, int(dayOfMonth), 0, 0, 0, 0, time.UTC).Format(invoiceDateLayout), nil
}

// catchUpRecurringDate moves a next date that is already in the past to the next occurrence of the day of month
// on or after today, so re-enabling a paused row does not back-fill one invoice per missed month
func catchUpRecurringDate(nextDate string, dayOfMonth int32, today string) string {
	if nextDate >= today {
		return nextDate
	}

	t, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return nextDate
	}

	candidate := time.Date(t.Year(), t.Month(), int(dayOfMonth), 0, 0, 0, 0, time.UTC)

	if candidate.Before(t) {
		candidate = time.Date(t.Year(), t.Month()+1, int(dayOfMonth), 0, 0, 0, 0, time.UTC)
	}

	return candidate.Format(invoiceDateLayout)
}

// validateRecurringInvoice checks the day of month and the next date
func validateRecurringInvoice(recurring *models.RecurringInvoice) error {
	if recurring.DayOfMonth < 1 || recurring.DayOfMonth > models.RecurringInvoiceMaxDayOfMonth {
		return errs.ErrRecurringInvoiceDayOfMonthInvalid
	}

	if _, err := time.Parse(invoiceDateLayout, recurring.NextDate); err != nil {
		return errs.ErrInvoiceDateInvalid
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

	if err := validateRecurringInvoice(recurring); err != nil {
		return err
	}

	if recurring.Enabled {
		recurring.NextDate = catchUpRecurringDate(recurring.NextDate, recurring.DayOfMonth, time.Now().Format(invoiceDateLayout))
	}

	recurring.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(recurring.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("day_of_month", "next_date", "enabled", "updated_unix_time").
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

// CreateRecurringInvoices creates the next invoice of every enabled recurring invoice whose next date is today or earlier.
// A failing row is logged and left for the next run; it never fails the cron job.
func (s *InvoiceService) CreateRecurringInvoices(c core.Context, today string) error {
	if _, err := time.Parse(invoiceDateLayout, today); err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	for i := 0; i < s.UserDataDBCount(); i++ {
		var recurrings []*models.RecurringInvoice
		err := s.UserDataDBByIndex(i).NewSession(c).Where("deleted=? AND enabled=? AND next_date<=?", false, true, today).Find(&recurrings)

		if err != nil {
			return err
		}

		for _, recurring := range recurrings {
			if err := s.createRecurringInvoice(c, recurring); err != nil {
				log.Errorf(c, "[recurring_invoices.CreateRecurringInvoices] failed to create invoice for recurring invoice \"id:%d\" of user \"uid:%d\", because %s", recurring.RecurringId, recurring.Uid, err.Error())
			}
		}
	}

	return nil
}

// createRecurringInvoice creates one invoice from the recurring invoice's source and advances its next date
func (s *InvoiceService) createRecurringInvoice(c core.Context, recurring *models.RecurringInvoice) error {
	source, sourceLines, err := s.GetInvoice(c, recurring.Uid, recurring.SourceInvoiceId)

	if err == errs.ErrInvoiceNotFound {
		log.Warnf(c, "[recurring_invoices.createRecurringInvoice] source invoice \"id:%d\" of recurring invoice \"id:%d\" of user \"uid:%d\" is deleted, skipped", recurring.SourceInvoiceId, recurring.RecurringId, recurring.Uid)
		return nil
	} else if err != nil {
		return err
	}

	// ponytail: CreateInvoice always re-reads the customer for the bill-to snapshot and fails when it is gone,
	// so the spec's "fall back to the source snapshot" cannot be reached here; it also cannot happen, because
	// DeleteCustomer refuses while a non-deleted invoice (the source) still uses the customer
	customer, err := s.GetCustomer(c, recurring.Uid, source.CustomerId)

	if err != nil {
		return err
	}

	profile, err := s.GetProfile(c, recurring.Uid)

	if err != nil {
		return err
	}

	termsDays := profile.DefaultTermsDays

	if customer.TermsDays > 0 {
		termsDays = customer.TermsDays
	}

	number, err := s.NextInvoiceNumber(c, recurring.Uid, recurring.NextDate)

	if err != nil {
		return err
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

	nextDate, err := nextRecurringDate(recurring.NextDate, recurring.DayOfMonth)

	if err != nil {
		return err
	}

	if err := s.prepareNewInvoice(c, &invoice, lines); err != nil {
		return err
	}

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
		return err
	}

	log.Infof(c, "[recurring_invoices.createRecurringInvoice] recurring invoice \"id:%d\" of user \"uid:%d\" has created invoice \"id:%d\" (%s), next date %s", recurring.RecurringId, recurring.Uid, invoice.InvoiceId, invoice.Number, recurring.NextDate)
	return nil
}
