package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoiceemail"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
)

// InvoiceService represents invoice service
type InvoiceService struct {
	ServiceUsingDB
	ServiceUsingUuid
	ServiceUsingConfig
	ServiceUsingMailer
}

// Initialize an invoice service singleton instance
var (
	Invoices = &InvoiceService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
		ServiceUsingConfig: ServiceUsingConfig{
			container: settings.Container,
		},
		ServiceUsingMailer: ServiceUsingMailer{
			container: mail.Container,
		},
	}
)

// InvoicePublicUrl returns the short public link of an invoice (<root url>i/<token>), the URL printed in the PDF
// footer and sent in the email; empty when the invoice has no public token
func (s *InvoiceService) InvoicePublicUrl(token string) string {
	if token == "" {
		return ""
	}

	return s.CurrentConfig().RootUrl + "i/" + url.PathEscape(token)
}

// invoiceEmailSubject is the default subject of a plain-text email: "Invoice <number> from <business name>" ("Estimate ..." for an
// estimate; without " from ..." when the profile has no business name)
func invoiceEmailSubject(kind string, number string, businessName string) string {
	title := "Invoice"

	if kind == models.INVOICE_KIND_ESTIMATE {
		title = "Estimate"
	}

	if businessName == "" {
		return title + " " + number
	}

	return title + " " + number + " from " + businessName
}

// SendInvoice preserves the legacy plain-text service contract.
func (s *InvoiceService) SendInvoice(c core.Context, uid int64, invoiceId int64, to string, subject string, message string) error {
	return s.SendInvoiceWithContent(c, uid, invoiceId, to, subject, &message, nil)
}

// GetProfile returns the invoice profile of user, or defaults when the user has not saved one yet
func (s *InvoiceService) GetProfile(c core.Context, uid int64) (*models.InvoiceProfile, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	profile := &models.InvoiceProfile{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Get(profile)

	if err != nil {
		return nil, err
	} else if !has {
		return defaultInvoiceProfile(uid), nil
	}

	return profile, nil
}

func defaultInvoiceProfile(uid int64) *models.InvoiceProfile {
	return &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", DefaultTermsDays: 7, RenewalDraftDays: models.InvoiceRenewalDraftDaysDefault,
		EstimateNumberPrefix: models.EstimateNumberPrefixDefault, EstimateValidDays: models.EstimateValidDaysDefault}
}

// SaveProfile creates or updates the invoice profile of user
func (s *InvoiceService) SaveProfile(c core.Context, profile *models.InvoiceProfile) error {
	return s.saveProfile(c, profile, false)
}

// saveProfile can omit template columns at the write boundary, avoiding stale business-form overwrites.
func (s *InvoiceService) saveProfile(c core.Context, profile *models.InvoiceProfile, preserveEmailTemplates bool) error {
	if profile.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	// a client from before estimates sends neither field
	if strings.TrimSpace(profile.EstimateNumberPrefix) == "" {
		profile.EstimateNumberPrefix = models.EstimateNumberPrefixDefault
	}

	if profile.EstimateValidDays <= 0 {
		profile.EstimateValidDays = models.EstimateValidDaysDefault
	}

	if !preserveEmailTemplates && profile.EmailSubjectTemplate != "" {
		if err := invoiceemail.ValidateSubject(profile.EmailSubjectTemplate); err != nil {
			return invoiceEmailTemplateError(err)
		}
		body, err := invoiceemail.SanitizeTemplate(profile.EmailBodyTemplate)
		if err != nil {
			return invoiceEmailTemplateError(err)
		}
		profile.EmailBodyTemplate = body
	}

	now := time.Now().Unix()
	profile.UpdatedUnixTime = now

	return s.UserDataDB(profile.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		exists, err := sess.Where("uid=?", profile.Uid).Exist(&models.InvoiceProfile{})

		if err != nil {
			return err
		}

		if exists {
			omittedColumns := []string{"created_unix_time"}
			if preserveEmailTemplates {
				omittedColumns = append(omittedColumns, "email_subject_template", "email_body_template")
			}
			_, err = sess.Where("uid=?", profile.Uid).AllCols().Omit(omittedColumns...).Update(profile)
			if err != nil {
				return err
			}
			if preserveEmailTemplates {
				// Return the template preserved by SQL, not the caller's earlier snapshot.
				saved := &models.InvoiceProfile{}
				has, err := sess.Where("uid=?", profile.Uid).Get(saved)
				if err != nil {
					return err
				}
				if !has {
					return errs.ErrOperationFailed
				}
				*profile = *saved
			}
			return nil
		}

		if preserveEmailTemplates {
			profile.EmailSubjectTemplate = ""
			profile.EmailBodyTemplate = ""
		}
		profile.CreatedUnixTime = now
		_, err = sess.Insert(profile)
		return err
	})
}

// GetAllCustomers returns all customers of user ordered by name
func (s *InvoiceService) GetAllCustomers(c core.Context, uid int64) ([]*models.Customer, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var customers []*models.Customer
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("name asc").Find(&customers)

	return customers, err
}

// GetCustomer returns one customer of user
func (s *InvoiceService) GetCustomer(c core.Context, uid int64, customerId int64) (*models.Customer, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if customerId <= 0 {
		return nil, errs.ErrCustomerIdInvalid
	}

	customer := &models.Customer{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Get(customer)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrCustomerNotFound
	}

	return customer, nil
}

// CreateCustomer saves a new customer to database
func (s *InvoiceService) CreateCustomer(c core.Context, customer *models.Customer) error {
	if customer.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	customer.CustomerId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

	if customer.CustomerId < 1 {
		return errs.ErrSystemIsBusy
	}

	customer.Deleted = false
	customer.CreatedUnixTime = time.Now().Unix()
	customer.UpdatedUnixTime = customer.CreatedUnixTime

	return s.UserDataDB(customer.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(customer)
		return err
	})
}

// ModifyCustomer saves an existing customer to database; existing invoices keep their bill-to snapshot
func (s *InvoiceService) ModifyCustomer(c core.Context, customer *models.Customer) error {
	if customer.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if customer.CustomerId <= 0 {
		return errs.ErrCustomerIdInvalid
	}

	customer.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(customer.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("name", "contact_name", "address_lines", "email", "phone", "website", "account_number", "pan", "terms_days", "notes", "gstin", "tax_type", "tax_rate", "updated_unix_time").
			Where("uid=? AND deleted=? AND customer_id=?", customer.Uid, false, customer.CustomerId).Update(customer)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrCustomerNotFound
		}

		return nil
	})
}

// DeleteCustomer soft-deletes a customer that no invoice uses
func (s *InvoiceService) DeleteCustomer(c core.Context, uid int64, customerId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if customerId <= 0 {
		return errs.ErrCustomerIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		inUse, err := sess.Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Exist(&models.Invoice{})

		if err != nil {
			return err
		} else if inUse {
			return errs.ErrCustomerInUse
		}

		updatedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).
			Update(&models.Customer{Deleted: true, DeletedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrCustomerNotFound
		}

		return nil
	})
}

const invoiceDateLayout = "2006-01-02"

// invoiceTotals is the result of computeInvoiceTotals; Cgst / Sgst are only meaningful for the cgst_sgst tax type
type invoiceTotals struct {
	Subtotal  int64
	TaxAmount int64
	Cgst      int64
	Sgst      int64
	Total     int64
}

// isInvoiceTaxable reports whether a tax type carries GST; empty (rows saved before GST) and none do not
func isInvoiceTaxable(taxType string) bool {
	return taxType == models.INVOICE_TAX_TYPE_IGST || taxType == models.INVOICE_TAX_TYPE_CGST_SGST
}

// computeInvoiceTotals fills each line's amount (quantity × rate, quantity in hundredths, rounded half up to the minor unit)
// and returns the subtotal, the tax (rate in basis points, rounded once, half up) and the total. This is the single place
// totals are computed; src/lib/invoice.ts mirrors it for the live preview.
func computeInvoiceTotals(lines []*models.InvoiceLine, taxType string, taxRate int32) invoiceTotals {
	totals := invoiceTotals{}

	for _, line := range lines {
		line.Amount = (line.Quantity*line.Rate + 50) / 100
		totals.Subtotal += line.Amount
	}

	if isInvoiceTaxable(taxType) && taxRate > 0 {
		// split the subtotal so subtotal × rate cannot overflow int64
		whole, rest := totals.Subtotal/models.INVOICE_TAX_RATE_MAX, totals.Subtotal%models.INVOICE_TAX_RATE_MAX
		totals.TaxAmount = whole*int64(taxRate) + (rest*int64(taxRate)+models.INVOICE_TAX_RATE_MAX/2)/models.INVOICE_TAX_RATE_MAX
	}

	totals.Cgst = totals.TaxAmount / 2
	totals.Sgst = totals.TaxAmount - totals.Cgst
	totals.Total = totals.Subtotal + totals.TaxAmount

	return totals
}

// invoiceFinancialYear returns the Indian financial year (April to March) of a YYYY-MM-DD date, e.g. "2026-27"
func invoiceFinancialYear(date string) (string, error) {
	t, err := time.Parse(invoiceDateLayout, date)

	if err != nil {
		return "", errs.ErrInvoiceDateInvalid
	}

	startYear := t.Year()

	if t.Month() < time.April {
		startYear--
	}

	return fmt.Sprintf("%d-%02d", startYear, (startYear+1)%100), nil
}

// nextInvoiceSequence returns one more than the highest numeric suffix among numbers starting with base
func nextInvoiceSequence(numbers []string, base string) int {
	maxSequence := 0

	for _, number := range numbers {
		if !strings.HasPrefix(number, base) {
			continue
		}

		sequence, err := strconv.Atoi(number[len(base):])

		if err == nil && sequence > maxSequence {
			maxSequence = sequence
		}
	}

	return maxSequence + 1
}

// validateInvoiceDates checks both dates parse and the due date is not before the invoice date
func validateInvoiceDates(invoice *models.Invoice) error {
	if _, err := time.Parse(invoiceDateLayout, invoice.InvoiceDate); err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	if _, err := time.Parse(invoiceDateLayout, invoice.DueDate); err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	if invoice.DueDate < invoice.InvoiceDate {
		return errs.ErrInvoiceDueDateBeforeInvoiceDate
	}

	return nil
}

// GetAllInvoices returns all invoices of user (never estimates), newest invoice date first
func (s *InvoiceService) GetAllInvoices(c core.Context, uid int64) ([]*models.Invoice, error) {
	return s.getAllOfKind(c, uid, models.INVOICE_KIND_INVOICE)
}

// GetAllEstimates returns all estimates of user, newest estimate date first
func (s *InvoiceService) GetAllEstimates(c core.Context, uid int64) ([]*models.Invoice, error) {
	return s.getAllOfKind(c, uid, models.INVOICE_KIND_ESTIMATE)
}

func (s *InvoiceService) getAllOfKind(c core.Context, uid int64, kind string) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=?", uid, false, kind).OrderBy("invoice_date desc, number desc").Find(&invoices)

	return invoices, err
}

// GetInvoice returns one invoice of user with its lines in display order
func (s *InvoiceService) GetInvoice(c core.Context, uid int64, invoiceId int64) (*models.Invoice, []*models.InvoiceLine, error) {
	if uid <= 0 {
		return nil, nil, errs.ErrUserIdInvalid
	}

	if invoiceId <= 0 {
		return nil, nil, errs.ErrInvoiceIdInvalid
	}

	invoice, err := s.getInvoiceHeader(c, uid, invoiceId)

	if err != nil {
		return nil, nil, err
	}

	var lines []*models.InvoiceLine
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND invoice_id=?", uid, invoiceId).OrderBy("display_order asc").Find(&lines)

	if err != nil {
		return nil, nil, err
	}

	return invoice, lines, nil
}

// getInvoiceHeader returns an owned invoice without its lines
func (s *InvoiceService) getInvoiceHeader(c core.Context, uid int64, invoiceId int64) (*models.Invoice, error) {
	invoice := &models.Invoice{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).Get(invoice)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrInvoiceNotFound
	}

	return invoice, nil
}

// prepareInvoice validates an invoice, copies the customer's bill-to details and computes the total
func (s *InvoiceService) prepareInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if err := validateInvoiceDates(invoice); err != nil {
		return err
	}

	if err := s.checkInvoiceLineProducts(c, invoice.Uid, lines); err != nil {
		return err
	}

	customer, err := s.GetCustomer(c, invoice.Uid, invoice.CustomerId)

	if err != nil {
		return err
	}

	invoice.BillToName = customer.Name
	invoice.BillToContactName = customer.ContactName
	invoice.BillToAddressLines = customer.AddressLines
	invoice.BillToEmail = customer.Email
	invoice.BillToPhone = customer.Phone
	invoice.BillToPan = customer.Pan
	invoice.BillToGstin = customer.Gstin

	if !isInvoiceTaxable(invoice.TaxType) {
		invoice.TaxType = models.INVOICE_TAX_TYPE_NONE
		invoice.TaxRate = 0
	}

	totals := computeInvoiceTotals(lines, invoice.TaxType, invoice.TaxRate)
	invoice.Subtotal = totals.Subtotal
	invoice.TaxAmount = totals.TaxAmount
	invoice.Total = totals.Total

	for idx, line := range lines {
		line.LineId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

		if line.LineId < 1 {
			return errs.ErrSystemIsBusy
		}

		line.InvoiceId = invoice.InvoiceId
		line.Uid = invoice.Uid
		line.DisplayOrder = int32(idx + 1)
	}

	return nil
}

// checkInvoiceNumberUnique returns ErrInvoiceNumberAlreadyExists if another non-deleted row of the same kind uses the number
func checkInvoiceNumberUnique(sess *xorm.Session, invoice *models.Invoice) error {
	exists, err := sess.Where("uid=? AND deleted=? AND kind=? AND number=? AND invoice_id<>?", invoice.Uid, false, invoice.Kind, invoice.Number, invoice.InvoiceId).Exist(&models.Invoice{})

	if err != nil {
		return err
	} else if exists {
		return errs.ErrInvoiceNumberAlreadyExists
	}

	return nil
}

// CreateInvoice saves a new invoice and its lines. Every invoice is created through this method
// (API today, recurring invoices later).
func (s *InvoiceService) CreateInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if err := s.prepareNewInvoice(c, invoice, lines); err != nil {
		return err
	}

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		return insertInvoice(sess, invoice, lines)
	})
}

// prepareNewInvoice validates, snapshots and resets everything an invoice carries before it is inserted;
// identity, status, link and sent state never come from a copied source
func (s *InvoiceService) prepareNewInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if invoice.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	invoice.InvoiceId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

	if invoice.InvoiceId < 1 {
		return errs.ErrSystemIsBusy
	}

	if err := s.prepareInvoice(c, invoice, lines); err != nil {
		return err
	}

	if invoice.Currency == "" {
		profile, err := s.GetProfile(c, invoice.Uid)

		if err != nil {
			return err
		}

		invoice.Currency = profile.DefaultCurrency
	}

	if invoice.Currency == "" {
		invoice.Currency = models.INVOICE_DEFAULT_CURRENCY
	}

	if _, known := validators.AllCurrencyNames[invoice.Currency]; !known {
		return errs.ErrInvoiceCurrencyInvalid
	}

	invoice.Kind = models.INVOICE_KIND_INVOICE
	invoice.Deleted = false
	invoice.ClosedManually = false
	invoice.PublicToken = ""
	invoice.SentUnixTime = 0
	invoice.RecurringId = 0
	invoice.EstimateStatus = ""
	invoice.RespondedUnixTime = 0
	invoice.RespondedByClient = false
	invoice.ResponseNote = ""
	invoice.EstimateId = 0
	invoice.CreatedUnixTime = time.Now().Unix()
	invoice.UpdatedUnixTime = invoice.CreatedUnixTime
	invoice.DeletedUnixTime = 0

	return nil
}

// insertInvoice writes a prepared invoice and its lines inside the caller's transaction
func insertInvoice(sess *xorm.Session, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if err := checkInvoiceNumberUnique(sess, invoice); err != nil {
		return err
	}

	if _, err := sess.Insert(invoice); err != nil {
		return err
	}

	_, err := sess.Insert(lines)
	return err
}

// invoiceModifyColumns are what editing rewrites; kind, status, answers, links and sent state never change on edit
var invoiceModifyColumns = []string{"number", "customer_id", "bill_to_name", "bill_to_contact_name", "bill_to_address_lines", "bill_to_email", "bill_to_phone", "bill_to_pan", "bill_to_gstin",
	"invoice_date", "due_date", "notes", "total", "tax_type", "tax_rate", "subtotal", "tax_amount", "updated_unix_time"}

// ModifyInvoice saves an existing invoice (never an estimate, see ModifyEstimate) and replaces all of its lines
func (s *InvoiceService) ModifyInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if invoice.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if invoice.InvoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	if err := s.prepareInvoice(c, invoice, lines); err != nil {
		return err
	}

	invoice.Kind = models.INVOICE_KIND_INVOICE
	invoice.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		// lock order (see lockInvoiceRow): the invoice row first, then its estimate. An invoice made from an estimate also locks the
		// estimate, so a conversion's remainder never reads a half-applied edit
		if err := lockInvoiceRow(sess, invoice.Uid, invoice.InvoiceId, invoice.UpdatedUnixTime); err != nil {
			return err
		}

		stored := &models.Invoice{}
		if _, err := sess.Cols("estimate_id").Where("uid=? AND deleted=? AND invoice_id=?", invoice.Uid, false, invoice.InvoiceId).Get(stored); err != nil {
			return err
		} else if stored.EstimateId > 0 {
			if err := lockInvoiceRow(sess, invoice.Uid, stored.EstimateId, invoice.UpdatedUnixTime); err != nil {
				return err
			}
		}

		if err := checkInvoiceNumberUnique(sess, invoice); err != nil {
			return err
		}

		updatedRows, err := sess.Cols(invoiceModifyColumns...).
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", invoice.Uid, false, models.INVOICE_KIND_INVOICE, invoice.InvoiceId).Update(invoice)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
		}

		if _, err := sess.Where("uid=? AND invoice_id=?", invoice.Uid, invoice.InvoiceId).Delete(&models.InvoiceLine{}); err != nil {
			return err
		}

		_, err = sess.Insert(lines)
		return err
	})
}

// DeleteInvoice soft-deletes an invoice or estimate and removes its payment links; the linked transactions are untouched.
// An estimate with invoices made from it is refused; an invoice made from a milestone frees that milestone.
func (s *InvoiceService) DeleteInvoice(c core.Context, uid int64, invoiceId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if invoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		// an estimate's "nothing invoiced" check below must not race a conversion
		if err := lockInvoiceRow(sess, uid, invoiceId, now); err != nil {
			return err
		}

		invoice := &models.Invoice{}
		has, err := sess.Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).Get(invoice)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrInvoiceNotFound
		}

		// an invoice made from an estimate also locks the estimate, so a conversion of its last open milestone sees this delete
		// either fully (milestone freed) or not at all
		if invoice.EstimateId > 0 {
			if err := lockInvoiceRow(sess, uid, invoice.EstimateId, now); err != nil {
				return err
			}
		}

		if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
			invoiced, err := estimateHasInvoices(sess, uid, invoiceId)

			if err != nil {
				return err
			} else if invoiced {
				return errs.ErrEstimateHasInvoices
			}
		}

		updatedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).
			Update(&models.Invoice{Deleted: true, DeletedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
		}

		if invoice.EstimateId > 0 {
			if _, err := sess.Cols("invoice_id").Where("uid=? AND invoice_id=?", uid, invoiceId).Update(&models.EstimateMilestone{InvoiceId: 0}); err != nil {
				return err
			}
		}

		_, err = sess.Where("uid=? AND invoice_id=?", uid, invoiceId).Delete(&models.InvoicePayment{})
		return err
	})
}

// SetInvoiceClosed sets or clears the manual "mark as paid" flag
func (s *InvoiceService) SetInvoiceClosed(c core.Context, uid int64, invoiceId int64, closed bool) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if invoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("closed_manually", "updated_unix_time").Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_INVOICE, invoiceId).
			Update(&models.Invoice{ClosedManually: closed, UpdatedUnixTime: time.Now().Unix()})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
		}

		return nil
	})
}

// NextInvoiceNumber suggests "{prefix}/{financial year}/{sequence}" for an invoice dated invoiceDate; estimates never count
func (s *InvoiceService) NextInvoiceNumber(c core.Context, uid int64, invoiceDate string) (string, error) {
	return s.nextDocumentNumber(c, uid, invoiceDate, models.INVOICE_KIND_INVOICE)
}

// NextEstimateNumber suggests "{estimate prefix}/{financial year}/{sequence}" for an estimate, a series of its own
func (s *InvoiceService) NextEstimateNumber(c core.Context, uid int64, estimateDate string) (string, error) {
	return s.nextDocumentNumber(c, uid, estimateDate, models.INVOICE_KIND_ESTIMATE)
}

func (s *InvoiceService) nextDocumentNumber(c core.Context, uid int64, date string, kind string) (string, error) {
	financialYear, err := invoiceFinancialYear(date)

	if err != nil {
		return "", err
	}

	profile, err := s.GetProfile(c, uid)

	if err != nil {
		return "", err
	}

	prefix := profile.NumberPrefix

	if kind == models.INVOICE_KIND_ESTIMATE {
		prefix = profile.EstimateNumberPrefix

		if prefix == "" {
			prefix = models.EstimateNumberPrefixDefault
		}
	}

	base := prefix + "/" + financialYear + "/"

	var numbers []string
	err = s.UserDataDB(uid).NewSession(c).Table(&models.Invoice{}).Where("uid=? AND deleted=? AND kind=? AND number LIKE ?", uid, false, kind, base+"%").Cols("number").Find(&numbers)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%02d", base, nextInvoiceSequence(numbers, base)), nil
}

// rankInvoicePaymentSuggestions orders candidates: exact balance due first, then within the paid
// tolerance below it, then smaller (part-payments); oldest first inside each group; top N only
func rankInvoicePaymentSuggestions(transactions []*models.Transaction, balanceDue int64) []*models.Transaction {
	rank := func(amount int64) int {
		if amount == balanceDue {
			return 0
		} else if float64(amount) >= float64(balanceDue)*(1-models.InvoicePaidTolerance) {
			return 1
		}

		return 2
	}

	sort.SliceStable(transactions, func(i, j int) bool {
		rankI, rankJ := rank(transactions[i].Amount), rank(transactions[j].Amount)

		if rankI != rankJ {
			return rankI < rankJ
		}

		return transactions[i].TransactionTime < transactions[j].TransactionTime
	})

	if len(transactions) > models.InvoicePaymentSuggestionMaxCount {
		transactions = transactions[:models.InvoicePaymentSuggestionMaxCount]
	}

	return transactions
}

// GetInvoicePayments returns the linked, non-deleted transactions of each invoice, oldest first
func (s *InvoiceService) GetInvoicePayments(c core.Context, uid int64, invoiceIds []int64) (map[int64][]*models.InvoicePaymentInfo, error) {
	result := make(map[int64][]*models.InvoicePaymentInfo, len(invoiceIds))

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(invoiceIds) < 1 {
		return result, nil
	}

	return s.getInvoicePayments(s.UserDataDB(uid).NewSession(c), uid, invoiceIds)
}

// accountCurrencies reads the currency of each account with the given session (same database as the transactions)
func accountCurrencies(sess *xorm.Session, uid int64, accountIds []int64) (map[int64]string, error) {
	var accounts []*models.Account
	err := sess.Where("uid=?", uid).In("account_id", accountIds).Find(&accounts)

	if err != nil {
		return nil, err
	}

	currencies := make(map[int64]string, len(accounts))

	for _, account := range accounts {
		currencies[account.AccountId] = account.Currency
	}

	return currencies, nil
}

// getInvoicePayments reads the linked, non-deleted transactions of the invoices with the given session
func (s *InvoiceService) getInvoicePayments(sess *xorm.Session, uid int64, invoiceIds []int64) (map[int64][]*models.InvoicePaymentInfo, error) {
	result := make(map[int64][]*models.InvoicePaymentInfo, len(invoiceIds))

	var links []*models.InvoicePayment
	err := sess.Where("uid=?", uid).In("invoice_id", invoiceIds).Find(&links)

	if err != nil || len(links) < 1 {
		return result, err
	}

	linkByTransactionId := make(map[int64]*models.InvoicePayment, len(links))
	transactionIds := make([]int64, 0, len(links))

	for _, link := range links {
		linkByTransactionId[link.TransactionId] = link
		transactionIds = append(transactionIds, link.TransactionId)
	}

	var transactions []*models.Transaction
	err = sess.Where("uid=? AND deleted=?", uid, false).In("transaction_id", transactionIds).OrderBy("transaction_time asc").Find(&transactions)

	if err != nil {
		return nil, err
	}

	accountIds := make([]int64, len(transactions))

	for idx, transaction := range transactions {
		accountIds[idx] = transaction.AccountId
	}

	currencies, err := accountCurrencies(sess, uid, accountIds)

	if err != nil {
		return nil, err
	}

	for _, transaction := range transactions {
		link := linkByTransactionId[transaction.TransactionId]
		result[link.InvoiceId] = append(result[link.InvoiceId], &models.InvoicePaymentInfo{
			Transaction:   transaction,
			Currency:      currencies[transaction.AccountId],
			SettledAmount: link.SettledAmount,
		})
	}

	return result, nil
}

// linkedInvoiceRow is one invoice_payment joined with its invoice
type linkedInvoiceRow struct {
	TransactionId int64          `xorm:"transaction_id"`
	Invoice       models.Invoice `xorm:"extends"`
}

// GetLinkedInvoicesByTransactionIds returns the non-deleted invoice each of the given transactions is linked to, keyed by transaction id
func (s *InvoiceService) GetLinkedInvoicesByTransactionIds(c core.Context, uid int64, transactionIds []int64) (map[int64]*models.Invoice, error) {
	result := make(map[int64]*models.Invoice, len(transactionIds))

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(transactionIds) < 1 {
		return result, nil
	}

	var rows []*linkedInvoiceRow
	err := s.UserDataDB(uid).NewSession(c).Table("invoice_payment").
		Select("invoice_payment.transaction_id, invoice.*").
		Join("INNER", "invoice", "invoice.invoice_id = invoice_payment.invoice_id").
		Where("invoice_payment.uid=? AND invoice.uid=? AND invoice.deleted=? AND invoice.kind=?", uid, uid, false, models.INVOICE_KIND_INVOICE).
		In("invoice_payment.transaction_id", transactionIds).
		Find(&rows)

	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		invoice := row.Invoice
		result[row.TransactionId] = &invoice
	}

	return result, nil
}

// invoiceBalanceDue returns total minus what the invoice's linked, non-deleted transactions settled
func (s *InvoiceService) invoiceBalanceDue(sess *xorm.Session, uid int64, invoice *models.Invoice) (int64, error) {
	payments, err := s.getInvoicePayments(sess, uid, []int64{invoice.InvoiceId})

	if err != nil {
		return 0, err
	}

	return invoice.Total - models.InvoiceReceivedAmount(payments[invoice.InvoiceId]), nil
}

// GetPaymentSuggestions returns unlinked income transactions that could pay the invoice: those in the invoice
// currency ranked by amount first, then those in other currencies (any amount) newest first, at most 10 in total
func (s *InvoiceService) GetPaymentSuggestions(c core.Context, uid int64, invoiceId int64) ([]*models.InvoicePaymentInfo, error) {
	invoice, _, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return nil, errs.ErrNotAvailableForEstimate
	}

	balanceDue, err := s.invoiceBalanceDue(s.UserDataDB(uid).NewSession(c), uid, invoice)

	if err != nil {
		return nil, err
	}

	if balanceDue <= 0 {
		return []*models.InvoicePaymentInfo{}, nil
	}

	invoiceDate, _ := time.Parse(invoiceDateLayout, invoice.InvoiceDate)
	dueDate, _ := time.Parse(invoiceDateLayout, invoice.DueDate)
	// ponytail: one day of padding on each side instead of per-user timezone math
	minTime := utils.GetMinTransactionTimeFromUnixTime(invoiceDate.Add(-24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(dueDate.Add(time.Duration(models.InvoicePaymentSuggestionDaysAfterDue+1) * 24 * time.Hour).Unix())

	// the balance-due cap only applies to the invoice currency, so it is checked below once the account currency is known
	var candidates []*models.Transaction
	err = s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND type=? AND transaction_time>=? AND transaction_time<=? AND amount>?",
			uid, false, models.TRANSACTION_DB_TYPE_INCOME, minTime, maxTime, 0).
		Find(&candidates)

	if err != nil || len(candidates) < 1 {
		return []*models.InvoicePaymentInfo{}, err
	}

	transactionIds := make([]int64, len(candidates))
	accountIds := make([]int64, len(candidates))

	for idx, candidate := range candidates {
		transactionIds[idx] = candidate.TransactionId
		accountIds[idx] = candidate.AccountId
	}

	var linked []*models.InvoicePayment
	if err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("transaction_id", transactionIds).Find(&linked); err != nil {
		return nil, err
	}

	linkedIds := make(map[int64]bool, len(linked))

	for _, link := range linked {
		linkedIds[link.TransactionId] = true
	}

	accounts, err := Accounts.GetAccountsByAccountIds(c, uid, accountIds)

	if err != nil {
		return nil, err
	}

	sameCurrency := make([]*models.Transaction, 0, len(candidates))
	otherCurrency := make([]*models.InvoicePaymentInfo, 0, len(candidates))

	for _, candidate := range candidates {
		account := accounts[candidate.AccountId]

		if linkedIds[candidate.TransactionId] || account == nil {
			continue
		}

		if account.Currency == invoice.Currency {
			if candidate.Amount <= balanceDue {
				sameCurrency = append(sameCurrency, candidate)
			}
		} else {
			otherCurrency = append(otherCurrency, &models.InvoicePaymentInfo{Transaction: candidate, Currency: account.Currency})
		}
	}

	suggestions := make([]*models.InvoicePaymentInfo, 0, models.InvoicePaymentSuggestionMaxCount)

	for _, transaction := range rankInvoicePaymentSuggestions(sameCurrency, balanceDue) {
		suggestions = append(suggestions, &models.InvoicePaymentInfo{Transaction: transaction, Currency: invoice.Currency})
	}

	sort.SliceStable(otherCurrency, func(i, j int) bool {
		return otherCurrency[i].Transaction.TransactionTime > otherCurrency[j].Transaction.TransactionTime
	})

	suggestions = append(suggestions, otherCurrency...)

	if len(suggestions) > models.InvoicePaymentSuggestionMaxCount {
		suggestions = suggestions[:models.InvoicePaymentSuggestionMaxCount]
	}

	return suggestions, nil
}

// LinkPayment links an existing income transaction of user to an invoice. settledAmount is the amount in the
// invoice currency that the transaction settled: required when the account currency differs from the invoice
// currency, and refused (the transaction amount counts as-is) when the currencies match.
func (s *InvoiceService) LinkPayment(c core.Context, uid int64, invoiceId int64, transactionId int64, settledAmount int64) error {
	invoice, _, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return err
	}

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return errs.ErrNotAvailableForEstimate
	}

	transaction, err := Transactions.GetTransactionByTransactionId(c, uid, transactionId)

	if err != nil {
		return err
	}

	// Check if already linked before other validations
	alreadyLinked, err := s.UserDataDB(uid).NewSession(c).Where("transaction_id=?", transactionId).Exist(&models.InvoicePayment{})

	if err != nil {
		return err
	} else if alreadyLinked {
		return errs.ErrInvoicePaymentAlreadyLinked
	}

	accounts, err := Accounts.GetAccountsByAccountIds(c, uid, []int64{transaction.AccountId})

	if err != nil {
		return err
	}

	account := accounts[transaction.AccountId]

	if transaction.Type != models.TRANSACTION_DB_TYPE_INCOME || account == nil {
		return errs.ErrInvoicePaymentTransactionInvalid
	}

	if account.Currency == invoice.Currency {
		if settledAmount != 0 {
			return errs.ErrInvoicePaymentSettledAmountNotAllowed
		}
	} else if settledAmount <= 0 {
		return errs.ErrInvoicePaymentSettledAmountRequired
	}

	payment := &models.InvoicePayment{
		PaymentId:      s.GenerateUuid(uuid.UUID_TYPE_INVOICE),
		InvoiceId:      invoiceId,
		Uid:            uid,
		TransactionId:  transactionId,
		LinkedUnixTime: time.Now().Unix(),
		SettledAmount:  settledAmount,
	}

	if payment.PaymentId < 1 {
		return errs.ErrSystemIsBusy
	}

	received := (&models.InvoicePaymentInfo{Transaction: transaction, SettledAmount: settledAmount}).ReceivedAmount()

	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		alreadyLinked, err := sess.Where("transaction_id=?", transactionId).Exist(&models.InvoicePayment{})

		if err != nil {
			return err
		} else if alreadyLinked {
			return errs.ErrInvoicePaymentAlreadyLinked
		}

		// read the balance due in the same transaction as the insert, so a concurrent link is counted
		balanceDue, err := s.invoiceBalanceDue(sess, uid, invoice)

		if err != nil {
			return err
		} else if received > balanceDue {
			return errs.ErrInvoicePaymentExceedsBalanceDue
		}

		_, err = sess.Insert(payment)
		return err
	})

	if err != nil && !errs.IsCustomError(err) {
		// the unique index on transaction_id is the backstop for two concurrent links of the same transaction
		if alreadyLinked, existErr := s.UserDataDB(uid).NewSession(c).Where("transaction_id=?", transactionId).Exist(&models.InvoicePayment{}); existErr == nil && alreadyLinked {
			return errs.ErrInvoicePaymentAlreadyLinked
		}
	}

	return err
}

// UnlinkPayment removes a transaction from an invoice's payments
func (s *InvoiceService) UnlinkPayment(c core.Context, uid int64, invoiceId int64, transactionId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.Where("uid=? AND invoice_id=? AND transaction_id=?", uid, invoiceId, transactionId).Delete(&models.InvoicePayment{})

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrInvoicePaymentNotFound
		}

		return nil
	})
}

// generateInvoicePublicToken returns 32 bytes from crypto/rand as base64url without padding (43 characters)
func generateInvoicePublicToken() (string, error) {
	buf := make([]byte, 32)

	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// SetInvoicePublicToken enables the public link of an invoice with a fresh token (every call invalidates the old one)
// or disables it (enable=false clears the token). It returns the token now stored, empty when disabled.
func (s *InvoiceService) SetInvoicePublicToken(c core.Context, uid int64, invoiceId int64, enable bool) (string, error) {
	if uid <= 0 {
		return "", errs.ErrUserIdInvalid
	}

	if invoiceId <= 0 {
		return "", errs.ErrInvoiceIdInvalid
	}

	token := ""

	if enable {
		generated, err := generateInvoicePublicToken()

		if err != nil {
			return "", err
		}

		token = generated
	}

	if err := s.saveInvoicePublicToken(c, uid, invoiceId, token); err != nil {
		return "", err
	}

	return token, nil
}

func (s *InvoiceService) saveInvoicePublicToken(c core.Context, uid int64, invoiceId int64, token string) error {
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("public_token", "updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).
			Update(&models.Invoice{PublicToken: token, UpdatedUnixTime: time.Now().Unix()})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
		}

		return nil
	})
}

// GetInvoiceByPublicToken returns the invoice behind a public link with its lines, the amount received and the
// owner's invoice profile. An empty, unknown or disabled token is a plain not-found, with nothing to tell them apart.
func (s *InvoiceService) GetInvoiceByPublicToken(c core.Context, token string) (*models.Invoice, []*models.InvoiceLine, int64, *models.InvoiceProfile, error) {
	if len(token) != models.InvoicePublicTokenLength {
		return nil, nil, 0, nil, errs.ErrInvoiceNotFound
	}

	// ponytail: a public link carries no uid, so the lookup goes to the first user-data store; Choose() ignores
	// its key today (a single store), so this is the same database every user's invoices live in
	sess := s.UserDataDB(0).NewSession(c)

	invoice := &models.Invoice{}
	has, err := sess.Where("public_token=? AND deleted=?", token, false).Get(invoice)

	if err != nil {
		return nil, nil, 0, nil, err
	} else if !has {
		return nil, nil, 0, nil, errs.ErrInvoiceNotFound
	}

	invoice, lines, err := s.GetInvoice(c, invoice.Uid, invoice.InvoiceId)

	if err != nil {
		return nil, nil, 0, nil, err
	}

	payments, err := s.getInvoicePayments(s.UserDataDB(invoice.Uid).NewSession(c), invoice.Uid, []int64{invoice.InvoiceId})

	if err != nil {
		return nil, nil, 0, nil, err
	}

	profile, err := s.GetProfile(c, invoice.Uid)

	if err != nil {
		return nil, nil, 0, nil, err
	}

	return invoice, lines, models.InvoiceReceivedAmount(payments[invoice.InvoiceId]), profile, nil
}
