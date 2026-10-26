package api

import (
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
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

func invoiceLinesFromRequest(lineReqs []*models.InvoiceLineRequest) []*models.InvoiceLine {
	lines := make([]*models.InvoiceLine, len(lineReqs))

	for idx, lineReq := range lineReqs {
		lines[idx] = &models.InvoiceLine{
			ItemName:    strings.TrimSpace(lineReq.ItemName),
			Description: lineReq.Description,
			Quantity:    lineReq.Quantity,
			Rate:        lineReq.Rate,
			Sac:         normalizeTaxId(lineReq.Sac),
		}
	}

	return lines
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

	uid := c.GetCurrentUid()
	profile := &models.InvoiceProfile{
		Uid:              uid,
		BusinessName:     req.BusinessName,
		AddressLines:     req.AddressLines,
		Phone:            req.Phone,
		Website:          req.Website,
		LogoDataUrl:      req.LogoDataUrl,
		DefaultNotes:     req.DefaultNotes,
		NumberPrefix:     strings.TrimSpace(req.NumberPrefix),
		DefaultTermsDays: req.DefaultTermsDays,
		Gstin:            normalizeTaxId(req.Gstin),
		DefaultSac:       normalizeTaxId(req.DefaultSac),
	}

	if err := a.invoices.SaveProfile(c, profile); err != nil {
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

// InvoiceListHandler returns all invoices of current user with received amounts
func (a *InvoicesApi) InvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	invoices, err := a.invoices.GetAllInvoices(c, uid)

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

	resps := make([]*models.InvoiceInfoResponse, len(invoices))

	for idx, invoice := range invoices {
		resps[idx] = invoice.ToInvoiceInfoResponse(nil, payments[invoice.InvoiceId])
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

	return invoice.ToInvoiceInfoResponse(lines, payments[invoiceId]), nil
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

	taxType, err := validateInvoiceTax(req.TaxType, req.TaxRate)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	uid := c.GetCurrentUid()
	invoice := &models.Invoice{Uid: uid, Number: strings.TrimSpace(req.Number), CustomerId: req.CustomerId, InvoiceDate: req.InvoiceDate, DueDate: req.DueDate, Notes: req.Notes,
		TaxType: taxType, TaxRate: req.TaxRate}

	if err := a.invoices.CreateInvoice(c, invoice, invoiceLinesFromRequest(req.Lines)); err != nil {
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

	if err := a.invoices.ModifyInvoice(c, invoice, invoiceLinesFromRequest(req.Lines)); err != nil {
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
	number, err := a.invoices.NextInvoiceNumber(c, uid, req.InvoiceDate)

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

// invoicePaymentAction binds {invoiceId, transactionId} and runs action for current user
func (a *InvoicesApi) invoicePaymentAction(c *core.WebContext, name string, action func(uid int64, invoiceId int64, transactionId int64) error) (any, *errs.Error) {
	var req models.InvoicePaymentLinkRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[invoices.%s] parse request failed, because %s", name, err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := action(uid, req.InvoiceId, req.TransactionId); err != nil {
		log.Errorf(c, "[invoices.%s] failed for invoice \"id:%d\" and transaction \"id:%d\" of user \"uid:%d\", because %s", name, req.InvoiceId, req.TransactionId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// InvoicePaymentLinkHandler links a transaction to an invoice of current user
func (a *InvoicesApi) InvoicePaymentLinkHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoicePaymentAction(c, "InvoicePaymentLinkHandler", func(uid int64, invoiceId int64, transactionId int64) error {
		return a.invoices.LinkPayment(c, uid, invoiceId, transactionId)
	})
}

// InvoicePaymentUnlinkHandler unlinks a transaction from an invoice of current user
func (a *InvoicesApi) InvoicePaymentUnlinkHandler(c *core.WebContext) (any, *errs.Error) {
	return a.invoicePaymentAction(c, "InvoicePaymentUnlinkHandler", func(uid int64, invoiceId int64, transactionId int64) error {
		return a.invoices.UnlinkPayment(c, uid, invoiceId, transactionId)
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
	token, err := a.invoices.SetInvoicePublicToken(c, uid, req.Id, req.Enabled)

	if err != nil {
		log.Errorf(c, "[invoices.InvoicePublicLinkUpdateHandler] failed to update public link of invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &models.InvoicePublicLinkResponse{PublicToken: token}, nil
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

	return invoice.ToPublicInvoiceResponse(lines, received, profile), nil
}
