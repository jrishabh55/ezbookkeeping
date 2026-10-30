package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// EstimatePublicAcceptHandler records the client's acceptance through a public link; no login, so only true is returned
func (a *InvoicesApi) EstimatePublicAcceptHandler(c *core.WebContext) (any, *errs.Error) {
	return a.estimatePublicRespond(c, "EstimatePublicAcceptHandler", models.ESTIMATE_STATUS_ACCEPTED)
}

// EstimatePublicDeclineHandler records the client's decline (with an optional note) through a public link
func (a *InvoicesApi) EstimatePublicDeclineHandler(c *core.WebContext) (any, *errs.Error) {
	return a.estimatePublicRespond(c, "EstimatePublicDeclineHandler", models.ESTIMATE_STATUS_DECLINED)
}

func (a *InvoicesApi) estimatePublicRespond(c *core.WebContext, name string, status string) (any, *errs.Error) {
	var req models.EstimatePublicRespondRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.%s] parse request failed, because %s", name, err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if err := a.invoices.RespondToEstimateByPublicToken(c, req.Token, status, req.Note, time.Now().Format("2006-01-02")); err != nil {
		// the token is a secret: never log it
		log.Warnf(c, "[estimates.%s] failed to answer an estimate by public token, because %s", name, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}
