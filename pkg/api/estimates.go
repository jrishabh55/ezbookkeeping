package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateResponse returns the estimate as the detail view shows it after an action
func (a *InvoicesApi) estimateResponse(c *core.WebContext, name string, uid int64, estimateId int64) (any, *errs.Error) {
	resp, err := a.getInvoiceResponse(c, uid, estimateId)

	if err != nil {
		log.Errorf(c, "[estimates.%s] failed to get estimate \"id:%d\" for user \"uid:%d\", because %s", name, estimateId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// EstimateRespondHandler records your own accept / decline of an estimate of current user (nothing is emailed)
func (a *InvoicesApi) EstimateRespondHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.EstimateRespondRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateRespondHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.SetEstimateResponse(c, uid, req.Id, req.Status); err != nil {
		log.Errorf(c, "[estimates.EstimateRespondHandler] failed to mark estimate \"id:%d\" %s for user \"uid:%d\", because %s", req.Id, req.Status, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.estimateResponse(c, "EstimateRespondHandler", uid, req.Id)
}

// EstimateReopenHandler returns an answered estimate of current user to sent
func (a *InvoicesApi) EstimateReopenHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateReopenHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.ReopenEstimate(c, uid, req.Id); err != nil {
		log.Errorf(c, "[estimates.EstimateReopenHandler] failed to reopen estimate \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.estimateResponse(c, "EstimateReopenHandler", uid, req.Id)
}

// EstimateToInvoiceListHandler returns the accepted, not fully invoiced estimates of current user for the home card
func (a *InvoicesApi) EstimateToInvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	estimates, progress, err := a.invoices.GetEstimatesToInvoice(c, uid)

	if err != nil {
		log.Errorf(c, "[estimates.EstimateToInvoiceListHandler] failed to get estimates to invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.InvoiceInfoResponse, len(estimates))

	for idx, estimate := range estimates {
		resps[idx] = estimate.ToInvoiceInfoResponse(nil, nil)
		resps[idx].SetEstimateProgress(progress[estimate.InvoiceId])
	}

	return resps, nil
}
