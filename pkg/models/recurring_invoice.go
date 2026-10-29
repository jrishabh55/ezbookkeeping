package models

import "github.com/mayswind/ezbookkeeping/pkg/utils"

// RecurringInvoiceMaxDayOfMonth is the last day of month a recurring invoice may repeat on, so every month has it
const RecurringInvoiceMaxDayOfMonth = 28

// RecurringInvoiceMaxPeriodMonths is the longest repeat period (10 years)
const RecurringInvoiceMaxPeriodMonths = 120

// Recurring invoice modes: create the invoice on the date (unsent), create it early as a draft, or create and email it on the date
const (
	RECURRING_INVOICE_MODE_CREATE      = "create"
	RECURRING_INVOICE_MODE_DRAFT_AHEAD = "draft_ahead"
	RECURRING_INVOICE_MODE_AUTO_SEND   = "auto_send"
)

// IsValidRecurringInvoiceMode reports whether mode is one of the recurring invoice modes
func IsValidRecurringInvoiceMode(mode string) bool {
	return mode == RECURRING_INVOICE_MODE_CREATE || mode == RECURRING_INVOICE_MODE_DRAFT_AHEAD || mode == RECURRING_INVOICE_MODE_AUTO_SEND
}

// Renewal states shown on the home card
const (
	INVOICE_RENEWAL_STATE_DUE         = "due"         // a draft_ahead invoice waiting to be sent
	INVOICE_RENEWAL_STATE_SEND_FAILED = "send_failed" // an auto_send invoice that could not be emailed
	INVOICE_RENEWAL_STATE_AUTO_SENT   = "auto_sent"   // an auto_send invoice emailed in the last few days
)

// InvoiceRenewalAutoSentVisibleSeconds is how long an auto-sent renewal stays on the home card (7 days)
const InvoiceRenewalAutoSentVisibleSeconds = 7 * 24 * 3600

// InvoiceRenewalOption is the renewal schedule sending an invoice can start, from the renewable products on its lines
type InvoiceRenewalOption struct {
	PeriodMonths int32  `json:"periodMonths"`
	Mode         string `json:"mode"`
	MixedPeriods bool   `json:"mixedPeriods"` // renewable products with different periods: split the invoice to track them
	Scheduled    bool   `json:"scheduled"`    // a schedule already renews this invoice
}

// InvoiceRenewal is one generated invoice that needs (or recently got) attention
type InvoiceRenewal struct {
	State       string
	RecurringId int64
	Invoice     *Invoice
}

// InvoiceRenewalResponse represents a view-object of a renewal on the home card
type InvoiceRenewalResponse struct {
	State       string               `json:"state"`
	RecurringId string               `json:"recurringId"`
	Invoice     *InvoiceInfoResponse `json:"invoice"`
}

// RecurringInvoice repeats a source invoice every PeriodMonths months; the cron job creates the next invoice once NextDate (minus the draft days for draft_ahead) is reached
type RecurringInvoice struct {
	RecurringId     int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_recurring_invoice_uid_deleted) NOT NULL"`
	Deleted         bool   `xorm:"INDEX(IDX_recurring_invoice_uid_deleted) INDEX(IDX_recurring_invoice_deleted_enabled_next_date) NOT NULL"`
	SourceInvoiceId int64  `xorm:"NOT NULL"`
	DayOfMonth      int32  `xorm:"NOT NULL"`
	Enabled         bool   `xorm:"INDEX(IDX_recurring_invoice_deleted_enabled_next_date) NOT NULL"`
	NextDate        string `xorm:"VARCHAR(10) INDEX(IDX_recurring_invoice_deleted_enabled_next_date) NOT NULL"`
	LastInvoiceId   int64  `xorm:"NOT NULL"`
	LastRunUnixTime int64
	PeriodMonths    int32  `xorm:"NOT NULL DEFAULT 1"`
	Mode            string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'create'"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// RecurringInvoiceCreateRequest represents all parameters of recurring invoice creation request
type RecurringInvoiceCreateRequest struct {
	SourceInvoiceId int64  `json:"sourceInvoiceId,string" binding:"required,min=1"`
	DayOfMonth      int32  `json:"dayOfMonth" binding:"required,min=1,max=28"`
	NextDate        string `json:"nextDate" binding:"required,len=10"`
	PeriodMonths    int32  `json:"periodMonths" binding:"min=0,max=120"` // 0 means 1 (clients before renewals)
	Mode            string `json:"mode" binding:"max=16"`                // empty means create
}

// RecurringInvoiceModifyRequest represents all parameters of recurring invoice modification request
type RecurringInvoiceModifyRequest struct {
	Id           int64  `json:"id,string" binding:"required,min=1"`
	DayOfMonth   int32  `json:"dayOfMonth" binding:"required,min=1,max=28"`
	NextDate     string `json:"nextDate" binding:"required,len=10"`
	Enabled      bool   `json:"enabled"`
	PeriodMonths int32  `json:"periodMonths" binding:"min=0,max=120"` // 0 means 1 (clients before renewals)
	Mode         string `json:"mode" binding:"max=16"`                // empty means create
}

// RecurringInvoiceResponse represents a view-object of recurring invoice; the source number and customer name
// come from the source invoice and are blank when it has been deleted
type RecurringInvoiceResponse struct {
	Id                  string `json:"id"`
	SourceInvoiceId     string `json:"sourceInvoiceId"`
	SourceInvoiceNumber string `json:"sourceInvoiceNumber"`
	CustomerId          string `json:"customerId"`
	CustomerName        string `json:"customerName"`
	DayOfMonth          int32  `json:"dayOfMonth"`
	NextDate            string `json:"nextDate"`
	Enabled             bool   `json:"enabled"`
	LastInvoiceId       string `json:"lastInvoiceId"`
	LastRunUnixTime     int64  `json:"lastRunUnixTime"`
	PeriodMonths        int32  `json:"periodMonths"`
	Mode                string `json:"mode"`
}

// ToRecurringInvoiceResponse returns a view-object according to database model; source may be nil (deleted)
func (r *RecurringInvoice) ToRecurringInvoiceResponse(source *Invoice) *RecurringInvoiceResponse {
	resp := &RecurringInvoiceResponse{
		Id:              utils.Int64ToString(r.RecurringId),
		SourceInvoiceId: utils.Int64ToString(r.SourceInvoiceId),
		DayOfMonth:      r.DayOfMonth,
		NextDate:        r.NextDate,
		Enabled:         r.Enabled,
		LastInvoiceId:   utils.Int64ToString(r.LastInvoiceId),
		LastRunUnixTime: r.LastRunUnixTime,
		PeriodMonths:    r.PeriodMonths,
		Mode:            r.Mode,
	}

	if source != nil {
		resp.SourceInvoiceNumber = source.Number
		resp.CustomerId = utils.Int64ToString(source.CustomerId)
		resp.CustomerName = source.BillToName
	}

	return resp
}
