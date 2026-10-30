package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// EstimateInvoiceDraftHandler returns, unsaved, the invoice an estimate of current user (or one of its milestones) turns into;
// the editor opens it and saves it through /invoices/add.json with estimateId and milestoneId
func (a *InvoicesApi) EstimateInvoiceDraftHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.EstimateInvoiceDraftRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateInvoiceDraftHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	draft, lines, err := a.invoices.GetEstimateInvoiceDraft(c, uid, req.Id, req.MilestoneId, time.Now().Format("2006-01-02"))

	if err != nil {
		log.Errorf(c, "[estimates.EstimateInvoiceDraftHandler] failed to build invoice from estimate \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp := draft.ToInvoiceInfoResponse(lines, nil)

	if req.MilestoneId > 0 {
		resp.MilestoneId = utils.Int64ToString(req.MilestoneId)
	}

	return resp, nil
}
