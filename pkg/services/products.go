package services

import (
	"sort"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// validateProduct checks the renewal period and mode; a one-off product (RenewalMonths 0) drops its mode
func validateProduct(product *models.Product) error {
	if product.RenewalMonths == 0 {
		product.DefaultRenewalMode = ""
		return nil
	}

	if product.RenewalMonths < 0 || product.RenewalMonths > models.RecurringInvoiceMaxPeriodMonths {
		return errs.ErrRecurringInvoicePeriodInvalid
	}

	if !models.IsValidRecurringInvoiceMode(product.DefaultRenewalMode) {
		return errs.ErrRecurringInvoiceModeInvalid
	}

	return nil
}

// GetAllProducts returns all products of user (archived included) ordered by name
func (s *InvoiceService) GetAllProducts(c core.Context, uid int64) ([]*models.Product, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var products []*models.Product
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("name asc").Find(&products)

	return products, err
}

// GetProductsByIds returns the user's non-deleted products with the given ids, keyed by id
func (s *InvoiceService) GetProductsByIds(c core.Context, uid int64, productIds []int64) (map[int64]*models.Product, error) {
	result := make(map[int64]*models.Product, len(productIds))

	if len(productIds) < 1 {
		return result, nil
	}

	var products []*models.Product
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("product_id", productIds).Find(&products)

	if err != nil {
		return nil, err
	}

	for _, product := range products {
		result[product.ProductId] = product
	}

	return result, nil
}

// CreateProduct saves a new product to database
func (s *InvoiceService) CreateProduct(c core.Context, product *models.Product) error {
	if product.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if err := validateProduct(product); err != nil {
		return err
	}

	product.ProductId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

	if product.ProductId < 1 {
		return errs.ErrSystemIsBusy
	}

	product.Deleted = false
	product.CreatedUnixTime = time.Now().Unix()
	product.UpdatedUnixTime = product.CreatedUnixTime

	return s.UserDataDB(product.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(product)
		return err
	})
}

// ModifyProduct saves an existing product; invoice lines keep their own copied text
func (s *InvoiceService) ModifyProduct(c core.Context, product *models.Product) error {
	if product.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if product.ProductId <= 0 {
		return errs.ErrProductIdInvalid
	}

	if err := validateProduct(product); err != nil {
		return err
	}

	product.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(product.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("name", "description", "sac", "default_rate", "currency", "renewal_months", "default_renewal_mode", "archived", "updated_unix_time").
			Where("uid=? AND deleted=? AND product_id=?", product.Uid, false, product.ProductId).Update(product)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrProductNotFound
		}

		return nil
	})
}

// DeleteProduct soft-deletes a product that no line of a non-deleted invoice uses
func (s *InvoiceService) DeleteProduct(c core.Context, uid int64, productId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if productId <= 0 {
		return errs.ErrProductIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		inUse, err := sess.Table("invoice_line").Join("INNER", "invoice", "invoice.invoice_id=invoice_line.invoice_id").
			Where("invoice_line.uid=? AND invoice_line.product_id=? AND invoice.deleted=?", uid, productId, false).Count()

		if err != nil {
			return err
		} else if inUse > 0 {
			return errs.ErrProductInUse
		}

		updatedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND product_id=?", uid, false, productId).
			Update(&models.Product{Deleted: true, DeletedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrProductNotFound
		}

		return nil
	})
}

// checkInvoiceLineProducts refuses lines that point at a product the user does not own (archived products are allowed)
func (s *InvoiceService) checkInvoiceLineProducts(c core.Context, uid int64, lines []*models.InvoiceLine) error {
	productIds := make([]int64, 0)
	seen := make(map[int64]bool)

	for _, line := range lines {
		if line.ProductId > 0 && !seen[line.ProductId] {
			seen[line.ProductId] = true
			productIds = append(productIds, line.ProductId)
		}
	}

	products, err := s.GetProductsByIds(c, uid, productIds)

	if err != nil {
		return err
	} else if len(products) != len(productIds) {
		return errs.ErrInvoiceLineProductInvalid
	}

	return nil
}

// invoiceLineGroupKey is how past lines are matched to a product: the item name, trimmed and lower-cased
func invoiceLineGroupKey(itemName string) string {
	return strings.ToLower(strings.TrimSpace(itemName))
}

// getUnlinkedInvoiceLines returns the lines of non-deleted invoices that have no product yet
func (s *InvoiceService) getUnlinkedInvoiceLines(c core.Context, uid int64) ([]*models.InvoiceLine, error) {
	var lines []*models.InvoiceLine
	err := s.UserDataDB(uid).NewSession(c).Table("invoice_line").Select("invoice_line.*").
		Join("INNER", "invoice", "invoice.invoice_id=invoice_line.invoice_id").
		Where("invoice_line.uid=? AND invoice_line.product_id=? AND invoice.deleted=?", uid, 0, false).Find(&lines)

	return lines, err
}

// GetUnlinkedInvoiceLineGroups groups the unlinked lines by item name, most used first; the first spelling seen names the group
func (s *InvoiceService) GetUnlinkedInvoiceLineGroups(c core.Context, uid int64) ([]*models.InvoiceLineGroupResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	lines, err := s.getUnlinkedInvoiceLines(c, uid)

	if err != nil {
		return nil, err
	}

	groups := make(map[string]*models.InvoiceLineGroupResponse)
	result := make([]*models.InvoiceLineGroupResponse, 0)

	for _, line := range lines {
		key := invoiceLineGroupKey(line.ItemName)
		group, exists := groups[key]

		if !exists {
			group = &models.InvoiceLineGroupResponse{ItemName: strings.TrimSpace(line.ItemName)}
			groups[key] = group
			result = append(result, group)
		}

		group.Count++
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}

		return result[i].ItemName < result[j].ItemName
	})

	return result, nil
}

// LinkInvoiceLinesToProduct links every unlinked line whose item name matches itemName (trimmed, case-insensitive) and returns how many
func (s *InvoiceService) LinkInvoiceLinesToProduct(c core.Context, uid int64, itemName string, productId int64) (int64, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	products, err := s.GetProductsByIds(c, uid, []int64{productId})

	if err != nil {
		return 0, err
	} else if products[productId] == nil {
		return 0, errs.ErrProductNotFound
	}

	lines, err := s.getUnlinkedInvoiceLines(c, uid)

	if err != nil {
		return 0, err
	}

	key := invoiceLineGroupKey(itemName)
	lineIds := make([]int64, 0)

	for _, line := range lines {
		if invoiceLineGroupKey(line.ItemName) == key {
			lineIds = append(lineIds, line.LineId)
		}
	}

	if len(lineIds) < 1 {
		return 0, nil
	}

	var linked int64

	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		linked, err = sess.Cols("product_id").Where("uid=? AND product_id=?", uid, 0).In("line_id", lineIds).Update(&models.InvoiceLine{ProductId: productId})
		return err
	})

	return linked, err
}
