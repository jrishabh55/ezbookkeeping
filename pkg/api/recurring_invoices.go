package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// getRecurringInvoiceResponses joins recurring invoices with their source invoices for the number and customer name
func (a *InvoicesApi) getRecurringInvoiceResponses(c *core.WebContext, uid int64, recurrings []*models.RecurringInvoice) ([]*models.RecurringInvoiceResponse, error) {
	// ponytail: the user's whole invoice list instead of a by-id query; it is already loaded for the invoice list page
	invoices, err := a.invoices.GetAllInvoices(c, uid)

	if err != nil {
		return nil, err
	}

	invoiceMap := make(map[int64]*models.Invoice, len(invoices))

	for _, invoice := range invoices {
		invoiceMap[invoice.InvoiceId] = invoice
	}

	resps := make([]*models.RecurringInvoiceResponse, len(recurrings))

	for idx, recurring := range recurrings {
		resps[idx] = recurring.ToRecurringInvoiceResponse(invoiceMap[recurring.SourceInvoiceId])
	}

	return resps, nil
}

// RecurringInvoiceListHandler returns all recurring invoices of current user
func (a *InvoicesApi) RecurringInvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	recurrings, err := a.invoices.GetAllRecurringInvoices(c, uid)

	if err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceListHandler] failed to get recurring invoices for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps, err := a.getRecurringInvoiceResponses(c, uid, recurrings)

	if err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceListHandler] failed to get source invoices for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resps, nil
}

// RecurringInvoiceCreateHandler saves a new recurring invoice for current user
func (a *InvoicesApi) RecurringInvoiceCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.RecurringInvoiceCreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[recurring_invoices.RecurringInvoiceCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: req.SourceInvoiceId, DayOfMonth: req.DayOfMonth, NextDate: req.NextDate, PeriodMonths: req.PeriodMonths, Mode: req.Mode, Enabled: true}

	if err := a.invoices.CreateRecurringInvoice(c, recurring); err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceCreateHandler] failed to create recurring invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.recurringInvoiceResponse(c, "RecurringInvoiceCreateHandler", uid, recurring)
}

// RecurringInvoiceModifyHandler saves the day of month, next date and enabled flag of a recurring invoice of current user
func (a *InvoicesApi) RecurringInvoiceModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.RecurringInvoiceModifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[recurring_invoices.RecurringInvoiceModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	recurring := &models.RecurringInvoice{RecurringId: req.Id, Uid: uid, DayOfMonth: req.DayOfMonth, NextDate: req.NextDate, PeriodMonths: req.PeriodMonths, Mode: req.Mode, Enabled: req.Enabled}

	if err := a.invoices.ModifyRecurringInvoice(c, recurring); err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceModifyHandler] failed to modify recurring invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	saved, err := a.invoices.GetRecurringInvoice(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceModifyHandler] failed to get recurring invoice \"id:%d\" after modifying it for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.recurringInvoiceResponse(c, "RecurringInvoiceModifyHandler", uid, saved)
}

// RecurringInvoiceDeleteHandler deletes a recurring invoice of current user
func (a *InvoicesApi) RecurringInvoiceDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[recurring_invoices.RecurringInvoiceDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.DeleteRecurringInvoice(c, uid, req.Id); err != nil {
		log.Errorf(c, "[recurring_invoices.RecurringInvoiceDeleteHandler] failed to delete recurring invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// InvoiceRenewalListHandler returns the renewals the home card lists for current user
func (a *InvoicesApi) InvoiceRenewalListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	renewals, err := a.invoices.GetInvoiceRenewals(c, uid, time.Now().Unix())

	if err != nil {
		log.Errorf(c, "[recurring_invoices.InvoiceRenewalListHandler] failed to get renewals for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	invoiceIds := make([]int64, len(renewals))

	for idx, renewal := range renewals {
		invoiceIds[idx] = renewal.Invoice.InvoiceId
	}

	payments, err := a.invoices.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		log.Errorf(c, "[recurring_invoices.InvoiceRenewalListHandler] failed to get payments for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.InvoiceRenewalResponse, len(renewals))

	for idx, renewal := range renewals {
		resps[idx] = &models.InvoiceRenewalResponse{
			State:       renewal.State,
			RecurringId: utils.Int64ToString(renewal.RecurringId),
			Invoice:     renewal.Invoice.ToInvoiceInfoResponse(nil, payments[renewal.Invoice.InvoiceId]),
		}
	}

	return resps, nil
}

// recurringInvoiceResponse returns one recurring invoice joined with its source invoice
func (a *InvoicesApi) recurringInvoiceResponse(c *core.WebContext, name string, uid int64, recurring *models.RecurringInvoice) (any, *errs.Error) {
	resps, err := a.getRecurringInvoiceResponses(c, uid, []*models.RecurringInvoice{recurring})

	if err != nil {
		log.Errorf(c, "[recurring_invoices.%s] failed to get source invoice of recurring invoice \"id:%d\" for user \"uid:%d\", because %s", name, recurring.RecurringId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resps[0], nil
}
