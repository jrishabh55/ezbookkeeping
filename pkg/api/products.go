package api

import (
	"strings"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// productFromRequest builds a product of current user from a create / modify request
func productFromRequest(uid int64, productId int64, req *models.ProductCreateRequest) (*models.Product, error) {
	currency, err := normalizeInvoiceCurrency(req.Currency)

	if err != nil {
		return nil, err
	}

	return &models.Product{ProductId: productId, Uid: uid, Name: strings.TrimSpace(req.Name), Description: req.Description, Sac: normalizeTaxId(req.Sac),
		DefaultRate: req.DefaultRate, Currency: currency, RenewalMonths: req.RenewalMonths, DefaultRenewalMode: req.DefaultRenewalMode, Archived: req.Archived}, nil
}

// ProductListHandler returns all products of current user, archived included
func (a *InvoicesApi) ProductListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	products, err := a.invoices.GetAllProducts(c, uid)

	if err != nil {
		log.Errorf(c, "[products.ProductListHandler] failed to get products for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.ProductResponse, len(products))

	for idx, product := range products {
		resps[idx] = product.ToProductResponse()
	}

	return resps, nil
}

// ProductCreateHandler saves a new product for current user
func (a *InvoicesApi) ProductCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.ProductCreateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[products.ProductCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	product, err := productFromRequest(uid, 0, &req)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if err := a.invoices.CreateProduct(c, product); err != nil {
		log.Errorf(c, "[products.ProductCreateHandler] failed to create product for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return product.ToProductResponse(), nil
}

// ProductModifyHandler saves an existing product of current user
func (a *InvoicesApi) ProductModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.ProductModifyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[products.ProductModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	product, err := productFromRequest(uid, req.Id, &req.ProductCreateRequest)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if err := a.invoices.ModifyProduct(c, product); err != nil {
		log.Errorf(c, "[products.ProductModifyHandler] failed to modify product \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return product.ToProductResponse(), nil
}

// ProductDeleteHandler deletes a product of current user that no invoice uses
func (a *InvoicesApi) ProductDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[products.ProductDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.DeleteProduct(c, uid, req.Id); err != nil {
		log.Errorf(c, "[products.ProductDeleteHandler] failed to delete product \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// ProductUnlinkedLinesHandler returns the unlinked invoice lines of current user grouped by item name
func (a *InvoicesApi) ProductUnlinkedLinesHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	groups, err := a.invoices.GetUnlinkedInvoiceLineGroups(c, uid)

	if err != nil {
		log.Errorf(c, "[products.ProductUnlinkedLinesHandler] failed to get unlinked lines for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return groups, nil
}

// ProductLinkLinesHandler links one group of unlinked invoice lines of current user to a product and returns how many lines were linked
func (a *InvoicesApi) ProductLinkLinesHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.ProductLinkLinesRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[products.ProductLinkLinesHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	linked, err := a.invoices.LinkInvoiceLinesToProduct(c, uid, req.ItemName, req.ProductId)

	if err != nil {
		log.Errorf(c, "[products.ProductLinkLinesHandler] failed to link lines to product \"id:%d\" for user \"uid:%d\", because %s", req.ProductId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return linked, nil
}
