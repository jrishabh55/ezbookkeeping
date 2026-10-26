package errs

import "net/http"

// Error codes related to invoices and customers
var (
	ErrInvoiceIdInvalid                 = NewNormalError(NormalSubcategoryInvoice, 0, http.StatusBadRequest, "invoice id is invalid")
	ErrInvoiceNotFound                  = NewNormalError(NormalSubcategoryInvoice, 1, http.StatusBadRequest, "invoice not found")
	ErrInvoiceNumberAlreadyExists       = NewNormalError(NormalSubcategoryInvoice, 2, http.StatusBadRequest, "invoice number already exists")
	ErrInvoiceDateInvalid               = NewNormalError(NormalSubcategoryInvoice, 3, http.StatusBadRequest, "invoice date is invalid")
	ErrInvoiceDueDateBeforeInvoiceDate  = NewNormalError(NormalSubcategoryInvoice, 4, http.StatusBadRequest, "due date cannot be before invoice date")
	ErrCustomerIdInvalid                = NewNormalError(NormalSubcategoryInvoice, 5, http.StatusBadRequest, "customer id is invalid")
	ErrCustomerNotFound                 = NewNormalError(NormalSubcategoryInvoice, 6, http.StatusBadRequest, "customer not found")
	ErrCustomerInUse                    = NewNormalError(NormalSubcategoryInvoice, 7, http.StatusBadRequest, "customer is used by invoices")
	ErrInvoicePaymentTransactionInvalid = NewNormalError(NormalSubcategoryInvoice, 8, http.StatusBadRequest, "transaction must be an income in the invoice currency")
	ErrInvoicePaymentAlreadyLinked      = NewNormalError(NormalSubcategoryInvoice, 9, http.StatusBadRequest, "transaction is already linked to an invoice")
	ErrInvoicePaymentExceedsBalanceDue  = NewNormalError(NormalSubcategoryInvoice, 10, http.StatusBadRequest, "payment exceeds the invoice balance due")
	ErrInvoicePaymentNotFound           = NewNormalError(NormalSubcategoryInvoice, 11, http.StatusBadRequest, "invoice payment not found")
	ErrInvoiceLogoInvalid               = NewNormalError(NormalSubcategoryInvoice, 12, http.StatusBadRequest, "invoice logo must be a png or jpeg image")
	ErrInvoiceTaxTypeInvalid            = NewNormalError(NormalSubcategoryInvoice, 13, http.StatusBadRequest, "invoice tax type is invalid")
	ErrInvoiceTaxRateInvalid            = NewNormalError(NormalSubcategoryInvoice, 14, http.StatusBadRequest, "invoice tax rate is invalid")
	// recurring invoices start at 40 so the codes do not collide with the other invoicing extensions
	ErrRecurringInvoiceIdInvalid         = NewNormalError(NormalSubcategoryInvoice, 40, http.StatusBadRequest, "recurring invoice id is invalid")
	ErrRecurringInvoiceNotFound          = NewNormalError(NormalSubcategoryInvoice, 41, http.StatusBadRequest, "recurring invoice not found")
	ErrRecurringInvoiceDayOfMonthInvalid = NewNormalError(NormalSubcategoryInvoice, 42, http.StatusBadRequest, "recurring invoice day of month must be between 1 and 28")
)
