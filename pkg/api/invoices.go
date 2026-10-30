package api

import (
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoicepdf"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
)

// InvoicesApi represents invoice api
type InvoicesApi struct {
	invoices *services.InvoiceService
}

// Initialize an invoice api singleton instance
var (
	Invoices = &InvoicesApi{
		invoices: services.Invoices,
	}
)

// isAllowedInvoiceLogo accepts an empty logo or a base64 png / jpeg data url; anything else (e.g. svg) is refused
func isAllowedInvoiceLogo(logoDataUrl string) bool {
	return logoDataUrl == "" ||
		strings.HasPrefix(logoDataUrl, "data:image/png;base64,") ||
		strings.HasPrefix(logoDataUrl, "data:image/jpeg;base64,")
}

// validateInvoiceTax checks a tax type / rate pair from a request and returns the normalized type (empty means none)
func validateInvoiceTax(taxType string, taxRate int32) (string, error) {
	if taxType == "" {
		taxType = models.INVOICE_TAX_TYPE_NONE
	}

	if taxType != models.INVOICE_TAX_TYPE_NONE && taxType != models.INVOICE_TAX_TYPE_IGST && taxType != models.INVOICE_TAX_TYPE_CGST_SGST {
		return "", errs.ErrInvoiceTaxTypeInvalid
	}

	if taxRate < 0 || taxRate > models.INVOICE_TAX_RATE_MAX || (taxType != models.INVOICE_TAX_TYPE_NONE && taxRate <= 0) {
		return "", errs.ErrInvoiceTaxRateInvalid
	}

	return taxType, nil
}

// normalizeTaxId upper-cases and trims a GSTIN / SAC as typed
func normalizeTaxId(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

// normalizeInvoiceCurrency upper-cases a currency code from a request; empty stays empty (the default applies)
// and anything else must be a known ISO 4217 code
func normalizeInvoiceCurrency(currency string) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))

	if currency == "" {
		return "", nil
	}

	if _, known := validators.AllCurrencyNames[currency]; !known {
		return "", errs.ErrInvoiceCurrencyInvalid
	}

	return currency, nil
}

func invoiceLinesFromRequest(lineReqs []*models.InvoiceLineRequest) []*models.InvoiceLine {
	lines := make([]*models.InvoiceLine, len(lineReqs))

	for idx, lineReq := range lineReqs {
		lines[idx] = &models.InvoiceLine{
			ItemName:    strings.TrimSpace(lineReq.ItemName),
			Description: lineReq.Description,
			Quantity:    lineReq.Quantity,
			Rate:        lineReq.Rate,
			Sac:         normalizeTaxId(lineReq.Sac),
			ProductId:   lineReq.ProductId,
		}
	}

	return lines
}

func estimateMilestonesFromRequest(milestoneReqs []*models.EstimateMilestoneRequest) []*models.EstimateMilestone {
	milestones := make([]*models.EstimateMilestone, len(milestoneReqs))

	for idx, milestoneReq := range milestoneReqs {
		milestones[idx] = &models.EstimateMilestone{Label: strings.TrimSpace(milestoneReq.Label), Percent: milestoneReq.Percent}
	}

	return milestones
}

// InvoiceProfileGetHandler returns the invoice profile of current user
func (a *InvoicesApi) InvoiceProfileGetHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	profile, err := a.invoices.GetProfile(c, uid)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceProfileGetHandler] failed to get invoice profile for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return profile.ToInvoiceProfileResponse(), nil
}

// InvoiceProfileUpdateHandler saves the invoice profile of current user
func (a *InvoicesApi) InvoiceProfileUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceProfileUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceProfileUpdateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if !isAllowedInvoiceLogo(req.LogoDataUrl) {
		return nil, errs.ErrInvoiceLogoInvalid
	}

	defaultCurrency, err := normalizeInvoiceCurrency(req.DefaultCurrency)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	profile := &models.InvoiceProfile{
		Uid:                  uid,
		BusinessName:         req.BusinessName,
		AddressLines:         req.AddressLines,
		Phone:                req.Phone,
		Website:              req.Website,
		LogoDataUrl:          req.LogoDataUrl,
		HideLogo:             req.HideLogo,
		DefaultNotes:         req.DefaultNotes,
		NumberPrefix:         strings.TrimSpace(req.NumberPrefix),
		DefaultTermsDays:     req.DefaultTermsDays,
		Gstin:                normalizeTaxId(req.Gstin),
		DefaultSac:           normalizeTaxId(req.DefaultSac),
		DefaultCurrency:      defaultCurrency,
		RenewalDraftDays:     req.RenewalDraftDays,
		EstimateNumberPrefix: strings.TrimSpace(req.EstimateNumberPrefix),
		EstimateValidDays:    req.EstimateValidDays,
	}

	if err := a.invoices.SaveBusinessProfile(c, profile); err != nil {
		log.Errorf(c, "[invoices.InvoiceProfileUpdateHandler] failed to save invoice profile for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return profile.ToInvoiceProfileResponse(), nil
}

// CustomerListHandler returns all customers of current user
func (a *InvoicesApi) CustomerListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	customers, err := a.invoices.GetAllCustomers(c, uid)

	if err != nil {
		log.Errorf(c, "[invoices.CustomerListHandler] failed to get customers for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.CustomerResponse, len(customers))

	for idx, customer := range customers {
		resps[idx] = customer.ToCustomerResponse()
	}

	return resps, nil
}

// CustomerCreateHandler saves a new customer for current user
func (a *InvoicesApi) CustomerCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.CustomerCreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.CustomerCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	taxType, err := validateInvoiceTax(req.TaxType, req.TaxRate)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	customer := &models.Customer{Uid: uid, Name: strings.TrimSpace(req.Name), ContactName: req.ContactName, AddressLines: req.AddressLines, Email: req.Email,
		Phone: req.Phone, Website: req.Website, AccountNumber: req.AccountNumber, Pan: strings.ToUpper(strings.TrimSpace(req.Pan)), TermsDays: req.TermsDays, Notes: req.Notes,
		Gstin: normalizeTaxId(req.Gstin), TaxType: taxType, TaxRate: req.TaxRate}

	if err := a.invoices.CreateCustomer(c, customer); err != nil {
		log.Errorf(c, "[invoices.CustomerCreateHandler] failed to create customer for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return customer.ToCustomerResponse(), nil
}

// CustomerModifyHandler saves an existing customer for current user
func (a *InvoicesApi) CustomerModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.CustomerModifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.CustomerModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	taxType, err := validateInvoiceTax(req.TaxType, req.TaxRate)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	customer := &models.Customer{CustomerId: req.Id, Uid: uid, Name: strings.TrimSpace(req.Name), ContactName: req.ContactName, AddressLines: req.AddressLines, Email: req.Email,
		Phone: req.Phone, Website: req.Website, AccountNumber: req.AccountNumber, Pan: strings.ToUpper(strings.TrimSpace(req.Pan)), TermsDays: req.TermsDays, Notes: req.Notes,
		Gstin: normalizeTaxId(req.Gstin), TaxType: taxType, TaxRate: req.TaxRate}

	if err := a.invoices.ModifyCustomer(c, customer); err != nil {
		log.Errorf(c, "[invoices.CustomerModifyHandler] failed to modify customer \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return customer.ToCustomerResponse(), nil
}

// CustomerDeleteHandler deletes a customer of current user
func (a *InvoicesApi) CustomerDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.CustomerDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.DeleteCustomer(c, uid, req.Id); err != nil {
		log.Errorf(c, "[invoices.CustomerDeleteHandler] failed to delete customer \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// CustomerOverviewHandler returns the products bought and the timeline of a customer of current user
func (a *InvoicesApi) CustomerOverviewHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.CustomerOverviewHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	overview, err := a.invoices.GetCustomerOverview(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.CustomerOverviewHandler] failed to get overview of customer \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return overview, nil
}

// InvoiceListHandler returns all invoices (or, with kind=estimate, the latest version of every estimate; every version with
// include_superseded=true) of current user with received amounts
func (a *InvoicesApi) InvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceListRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	var invoices []*models.Invoice
	var err error

	if req.Kind == models.INVOICE_KIND_ESTIMATE && req.IncludeSuperseded {
		invoices, err = a.invoices.GetAllEstimates(c, uid)
	} else if req.Kind == models.INVOICE_KIND_ESTIMATE {
		invoices, err = a.invoices.GetLatestEstimates(c, uid)
	} else {
		invoices, err = a.invoices.GetAllInvoices(c, uid)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceListHandler] failed to get invoices for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	invoiceIds := make([]int64, len(invoices))

	for idx, invoice := range invoices {
		invoiceIds[idx] = invoice.InvoiceId
	}

	payments, err := a.invoices.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceListHandler] failed to get invoice payments for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	var progress map[int64]*models.EstimateProgress

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		progress, err = a.invoices.GetEstimateProgress(c, uid, invoiceIds)

		if err != nil {
			log.Errorf(c, "[invoices.InvoiceListHandler] failed to get estimate progress for user \"uid:%d\", because %s", uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	resps := make([]*models.InvoiceInfoResponse, len(invoices))

	for idx, invoice := range invoices {
		resps[idx] = invoice.ToInvoiceInfoResponse(nil, payments[invoice.InvoiceId])
		resps[idx].SetEstimateProgress(progress[invoice.InvoiceId])
	}

	return resps, nil
}

// getInvoiceResponse loads an invoice with its lines and payments
func (a *InvoicesApi) getInvoiceResponse(c *core.WebContext, uid int64, invoiceId int64) (*models.InvoiceInfoResponse, error) {
	invoice, lines, err := a.invoices.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	payments, err := a.invoices.GetInvoicePayments(c, uid, []int64{invoiceId})

	if err != nil {
		return nil, err
	}

	emails, err := a.invoices.GetInvoiceEmails(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	resp := invoice.ToInvoiceInfoResponse(lines, payments[invoiceId])
	resp.Emails = make([]*models.InvoiceEmailResponse, len(emails))

	for idx, email := range emails {
		resp.Emails[idx] = email.ToInvoiceEmailResponse()
	}

	resp.RenewalOption, err = a.invoices.GetInvoiceRenewalOption(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		// the chain always includes this estimate itself, so one progress query covers the detail and the Revisions list
		revisions, err := a.invoices.GetEstimateRevisions(c, uid, invoice)

		if err != nil {
			return nil, err
		}

		revisionIds := make([]int64, len(revisions))

		for idx, revision := range revisions {
			revisionIds[idx] = revision.InvoiceId
		}

		progress, err := a.invoices.GetEstimateProgress(c, uid, revisionIds)

		if err != nil {
			return nil, err
		}

		resp.SetEstimateProgress(progress[invoiceId])
		resp.SetEstimateRevisions(revisions, progress)
	} else if invoice.EstimateId > 0 {
		// an estimate cannot be deleted while this invoice exists, so it is always there
		estimate, _, err := a.invoices.GetInvoice(c, uid, invoice.EstimateId)

		if err != nil {
			return nil, err
		}

		resp.EstimateNumber = estimate.Number
	}

	return resp, nil
}

// InvoiceEmailGetHandler returns one sent invoice email of current user, with the HTML that was sent
func (a *InvoicesApi) InvoiceEmailGetHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceEmailGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	email, err := a.invoices.GetInvoiceEmail(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceEmailGetHandler] failed to get invoice email \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return email.ToInvoiceEmailResponse(), nil
}

// InvoiceGetHandler returns one invoice of current user with lines and payments
func (a *InvoicesApi) InvoiceGetHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	resp, err := a.getInvoiceResponse(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceGetHandler] failed to get invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// InvoiceCreateHandler saves a new invoice for current user
func (a *InvoicesApi) InvoiceCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceCreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	// a milestone only exists on an estimate, and an estimate is never made from another estimate
	if (req.MilestoneId > 0 && req.EstimateId == 0) || (req.EstimateId > 0 && req.Kind == models.INVOICE_KIND_ESTIMATE) {
		log.Warnf(c, "[invoices.InvoiceCreateHandler] estimate \"id:%d\" and milestone \"id:%d\" do not go with kind \"%s\"", req.EstimateId, req.MilestoneId, req.Kind)
		return nil, errs.ErrIncompleteOrIncorrectSubmission
	}

	taxType, err := validateInvoiceTax(req.TaxType, req.TaxRate)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	currency, err := normalizeInvoiceCurrency(req.Currency)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	invoice := &models.Invoice{Uid: uid, Number: strings.TrimSpace(req.Number), CustomerId: req.CustomerId, InvoiceDate: req.InvoiceDate, DueDate: req.DueDate, Notes: req.Notes,
		TaxType: taxType, TaxRate: req.TaxRate, Currency: currency}

	lines := invoiceLinesFromRequest(req.Lines)

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		err = a.invoices.CreateEstimate(c, invoice, lines, estimateMilestonesFromRequest(req.Milestones))
	} else if len(req.Milestones) > 0 {
		err = errs.ErrEstimateMilestonesInvalid // only an estimate has payment terms
	} else if req.EstimateId > 0 {
		err = a.invoices.CreateInvoiceFromEstimate(c, invoice, lines, req.EstimateId, req.MilestoneId)
	} else {
		err = a.invoices.CreateInvoice(c, invoice, lines)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceCreateHandler] failed to create invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp, err := a.getInvoiceResponse(c, uid, invoice.InvoiceId)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceCreateHandler] failed to get invoice \"id:%d\" after creating it for user \"uid:%d\", because %s", invoice.InvoiceId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// InvoiceModifyHandler saves an existing invoice for current user
func (a *InvoicesApi) InvoiceModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceModifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	taxType, err := validateInvoiceTax(req.TaxType, req.TaxRate)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	invoice := &models.Invoice{InvoiceId: req.Id, Uid: uid, Number: strings.TrimSpace(req.Number), CustomerId: req.CustomerId, InvoiceDate: req.InvoiceDate, DueDate: req.DueDate, Notes: req.Notes,
		TaxType: taxType, TaxRate: req.TaxRate}

	lines := invoiceLinesFromRequest(req.Lines)

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		err = a.invoices.ModifyEstimate(c, invoice, lines, estimateMilestonesFromRequest(req.Milestones))
	} else if len(req.Milestones) > 0 {
		err = errs.ErrEstimateMilestonesInvalid
	} else {
		err = a.invoices.ModifyInvoice(c, invoice, lines)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceModifyHandler] failed to modify invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp, err := a.getInvoiceResponse(c, uid, invoice.InvoiceId)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceModifyHandler] failed to get invoice \"id:%d\" after modifying it for user \"uid:%d\", because %s", invoice.InvoiceId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// invoiceIdAction binds an {id} body and runs action for current user
func (a *InvoicesApi) invoiceIdAction(c *core.WebContext, name string, action func(uid int64, invoiceId int64) error) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.%s] parse request failed, because %s", name, err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := action(uid, req.Id); err != nil {
		log.Errorf(c, "[invoices.%s] failed for invoice \"id:%d\" of user \"uid:%d\", because %s", name, req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// InvoiceDeleteHandler deletes an invoice of current user
func (a *InvoicesApi) InvoiceDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoiceIdAction(c, "InvoiceDeleteHandler", func(uid int64, invoiceId int64) error {
		return a.invoices.DeleteInvoice(c, uid, invoiceId)
	})
}

// InvoiceCloseHandler marks an invoice of current user as paid regardless of received amount
func (a *InvoicesApi) InvoiceCloseHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoiceIdAction(c, "InvoiceCloseHandler", func(uid int64, invoiceId int64) error {
		return a.invoices.SetInvoiceClosed(c, uid, invoiceId, true)
	})
}

// InvoiceReopenHandler clears the manual paid flag of an invoice of current user
func (a *InvoicesApi) InvoiceReopenHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoiceIdAction(c, "InvoiceReopenHandler", func(uid int64, invoiceId int64) error {
		return a.invoices.SetInvoiceClosed(c, uid, invoiceId, false)
	})
}

// InvoiceNextNumberHandler returns the suggested number for a new invoice of current user
func (a *InvoicesApi) InvoiceNextNumberHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceNextNumberRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceNextNumberHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	var number string
	var err error

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		number, err = a.invoices.NextEstimateNumber(c, uid, req.InvoiceDate)
	} else {
		number, err = a.invoices.NextInvoiceNumber(c, uid, req.InvoiceDate)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceNextNumberHandler] failed to get next invoice number for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return number, nil
}

// InvoicePaymentSuggestionsHandler returns transactions that could pay an invoice of current user
func (a *InvoicesApi) InvoicePaymentSuggestionsHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoicePaymentSuggestionsHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	suggestions, err := a.invoices.GetPaymentSuggestions(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePaymentSuggestionsHandler] failed to get payment suggestions of invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.InvoicePaymentResponse, len(suggestions))

	for idx, suggestion := range suggestions {
		resps[idx] = models.ToInvoicePaymentResponse(suggestion)
	}

	return resps, nil
}

// invoicePaymentAction binds {invoiceId, transactionId, settledAmount} and runs action for current user
func (a *InvoicesApi) invoicePaymentAction(c *core.WebContext, name string, action func(uid int64, req *models.InvoicePaymentLinkRequest) error) (any, *errs.Error) {
	var req models.InvoicePaymentLinkRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.%s] parse request failed, because %s", name, err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := action(uid, &req); err != nil {
		log.Errorf(c, "[invoices.%s] failed for invoice \"id:%d\" and transaction \"id:%d\" of user \"uid:%d\", because %s", name, req.InvoiceId, req.TransactionId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// InvoicePaymentLinkHandler links a transaction to an invoice of current user
func (a *InvoicesApi) InvoicePaymentLinkHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoicePaymentAction(c, "InvoicePaymentLinkHandler", func(uid int64, req *models.InvoicePaymentLinkRequest) error {
		return a.invoices.LinkPayment(c, uid, req.InvoiceId, req.TransactionId, req.SettledAmount)
	})
}

// InvoicePaymentUnlinkHandler unlinks a transaction from an invoice of current user
func (a *InvoicesApi) InvoicePaymentUnlinkHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoicePaymentAction(c, "InvoicePaymentUnlinkHandler", func(uid int64, req *models.InvoicePaymentLinkRequest) error {
		return a.invoices.UnlinkPayment(c, uid, req.InvoiceId, req.TransactionId)
	})
}

// InvoicePublicLinkUpdateHandler enables (with a fresh token) or disables the public link of an invoice of current user
func (a *InvoicesApi) InvoicePublicLinkUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoicePublicLinkUpdateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.InvoicePublicLinkUpdateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	token, estimateStatus, err := a.invoices.SetInvoicePublicToken(c, uid, req.Id, req.Enabled)

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePublicLinkUpdateHandler] failed to update public link of invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &models.InvoicePublicLinkResponse{PublicToken: token, EstimateStatus: estimateStatus}, nil
}

// InvoicePublicViewHandler returns the invoice behind a public link; no login, so the response carries no ids
func (a *InvoicesApi) InvoicePublicViewHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoicePublicViewRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoicePublicViewHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	invoice, lines, received, profile, err := a.invoices.GetInvoiceByPublicToken(c, req.Token)

	if err != nil {
		// the token is a secret: never log it
		log.Warnf(c, "[invoices.InvoicePublicViewHandler] failed to get invoice by public token, because %s", err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp := invoice.ToPublicInvoiceResponse(lines, received, profile)

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		milestones, err := a.invoices.GetEstimateMilestones(c, invoice.Uid, invoice.InvoiceId)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePublicViewHandler] failed to get milestones of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}

		resp.Milestones = models.ToPublicEstimateMilestoneResponses(milestones)
		resp.SupersededBy, err = a.invoices.GetEstimateSupersededBy(c, invoice)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePublicViewHandler] failed to get the latest version of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	return resp, nil
}

// InvoicePdfHandler returns the PDF document of one invoice of current user (GET /api/v1/invoices/pdf/:id).
func (a *InvoicesApi) InvoicePdfHandler(c *core.WebContext) ([]byte, string, *errs.Error) {
	invoiceId, err := utils.StringToInt64(c.Param("id"))

	if err != nil || invoiceId <= 0 {
		return nil, "", errs.ErrInvoiceIdInvalid
	}

	uid := c.GetCurrentUid()
	invoice, lines, err := a.invoices.GetInvoice(c, uid, invoiceId)

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePdfHandler] failed to get invoice \"id:%d\" for user \"uid:%d\", because %s", invoiceId, uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	payments, err := a.invoices.GetInvoicePayments(c, uid, []int64{invoiceId})

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePdfHandler] failed to get payments of invoice \"id:%d\" for user \"uid:%d\", because %s", invoiceId, uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	profile, err := a.invoices.GetProfile(c, uid)

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePdfHandler] failed to get invoice profile for user \"uid:%d\", because %s", uid, err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	received := models.InvoiceReceivedAmount(payments[invoiceId])

	var milestones []*models.EstimateMilestone

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		milestones, err = a.invoices.GetEstimateMilestones(c, invoice.Uid, invoice.InvoiceId)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePdfHandler] failed to get milestones of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, "", errs.Or(err, errs.ErrOperationFailed)
		}
	}

	// the footer prints the public link when the invoice has one; InvoicePublicUrl returns "" otherwise
	pdf, err := invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, a.invoices.InvoicePublicUrl(invoice.PublicToken))

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePdfHandler] failed to render invoice \"id:%d\" for user \"uid:%d\", because %s", invoiceId, uid, err.Error())
		return nil, "", errs.ErrOperationFailed
	}

	return pdf, invoicepdf.DocumentFileName(invoice), nil
}

// InvoiceSendHandler emails an invoice of current user (public link in the body, PDF attached) and returns the invoice with its sent time
func (a *InvoicesApi) InvoiceSendHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceSendRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceSendHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.SendInvoiceWithContent(c, uid, req.Id, req.To, req.Subject, req.Message, req.BodyHtml); err != nil {
		log.Errorf(c, "[invoices.InvoiceSendHandler] failed to send invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	// the email is out; a failure to start the renewal is logged, never reported as a failed send
	if req.CreateRenewal {
		if _, err := a.invoices.CreateRenewalForInvoice(c, uid, req.Id); err != nil {
			log.Errorf(c, "[invoices.InvoiceSendHandler] failed to create renewal for invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		}
	}

	resp, err := a.getInvoiceResponse(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceSendHandler] failed to get invoice \"id:%d\" for user \"uid:%d\" after sending, because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[invoices.InvoiceSendHandler] user \"uid:%d\" has sent invoice \"id:%d\"", uid, req.Id)

	return resp, nil
}

// InvoicePublicPdfHandler returns the PDF document behind a public link (GET /api/public/invoices/pdf?token=); no login
func (a *InvoicesApi) InvoicePublicPdfHandler(c *core.WebContext) ([]byte, string, *errs.Error) {
	var req models.InvoicePublicViewRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoicePublicPdfHandler] parse request failed, because %s", err.Error())
		return nil, "", errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	invoice, lines, received, profile, err := a.invoices.GetInvoiceByPublicToken(c, req.Token)

	if err != nil {
		// the token is a secret: never log it
		log.Warnf(c, "[invoices.InvoicePublicPdfHandler] failed to get invoice by public token, because %s", err.Error())
		return nil, "", errs.Or(err, errs.ErrOperationFailed)
	}

	var milestones []*models.EstimateMilestone

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		milestones, err = a.invoices.GetEstimateMilestones(c, invoice.Uid, invoice.InvoiceId)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePublicPdfHandler] failed to get milestones of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, "", errs.Or(err, errs.ErrOperationFailed)
		}
	}

	pdf, err := invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, a.invoices.InvoicePublicUrl(invoice.PublicToken))

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePublicPdfHandler] failed to render invoice \"id:%d\", because %s", invoice.InvoiceId, err.Error())
		return nil, "", errs.ErrOperationFailed
	}

	return pdf, invoicepdf.DocumentFileName(invoice), nil
}

// InvoiceEmailPreviewHandler renders an unsaved draft using owned invoice data or deterministic samples.
func (a *InvoicesApi) InvoiceEmailPreviewHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceEmailPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}
	preview, err := a.invoices.PreviewInvoiceEmail(c, c.GetCurrentUid(), req.InvoiceId, req.Subject, req.BodyHtml)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return preview, nil
}

// InvoiceEmailTemplateUpdateHandler saves the standalone template editor's draft.
func (a *InvoicesApi) InvoiceEmailTemplateUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceEmailTemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}
	profile, err := a.invoices.SaveInvoiceEmailTemplate(c, c.GetCurrentUid(), req.Subject, *req.BodyHtml)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
	return profile.ToInvoiceProfileResponse(), nil
}
