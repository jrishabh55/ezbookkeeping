package models

import "github.com/mayswind/ezbookkeeping/pkg/utils"

// RecurringInvoiceMaxDayOfMonth is the last day of month a recurring invoice may repeat on, so every month has it
const RecurringInvoiceMaxDayOfMonth = 28

// RecurringInvoice repeats a source invoice every month; the cron job creates the next invoice once NextDate is reached
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
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// RecurringInvoiceCreateRequest represents all parameters of recurring invoice creation request
type RecurringInvoiceCreateRequest struct {
	SourceInvoiceId int64  `json:"sourceInvoiceId,string" binding:"required,min=1"`
	DayOfMonth      int32  `json:"dayOfMonth" binding:"required,min=1,max=28"`
	NextDate        string `json:"nextDate" binding:"required,len=10"`
}

// RecurringInvoiceModifyRequest represents all parameters of recurring invoice modification request
type RecurringInvoiceModifyRequest struct {
	Id         int64  `json:"id,string" binding:"required,min=1"`
	DayOfMonth int32  `json:"dayOfMonth" binding:"required,min=1,max=28"`
	NextDate   string `json:"nextDate" binding:"required,len=10"`
	Enabled    bool   `json:"enabled"`
}

// RecurringInvoiceResponse represents a view-object of recurring invoice; the source number and customer name
// come from the source invoice and are blank when it has been deleted
type RecurringInvoiceResponse struct {
	Id                  string `json:"id"`
	SourceInvoiceId     string `json:"sourceInvoiceId"`
	SourceInvoiceNumber string `json:"sourceInvoiceNumber"`
	CustomerName        string `json:"customerName"`
	DayOfMonth          int32  `json:"dayOfMonth"`
	NextDate            string `json:"nextDate"`
	Enabled             bool   `json:"enabled"`
	LastInvoiceId       string `json:"lastInvoiceId"`
	LastRunUnixTime     int64  `json:"lastRunUnixTime"`
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
	}

	if source != nil {
		resp.SourceInvoiceNumber = source.Number
		resp.CustomerName = source.BillToName
	}

	return resp
}
