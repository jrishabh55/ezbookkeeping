package models

import "github.com/mayswind/ezbookkeeping/pkg/utils"

// INVOICE_KIND_INVOICE is the only invoice kind for now; quotes and credit notes can be added later
const INVOICE_KIND_INVOICE = "invoice"

// INVOICE_DEFAULT_CURRENCY is the only invoice currency for now
const INVOICE_DEFAULT_CURRENCY = "INR"

// InvoicePaidTolerance is the share of the total that may be missing (bank / forex charges)
// while a payment still counts as settling the invoice. The UI mirrors it in src/lib/invoice.ts.
const InvoicePaidTolerance = 0.03

// InvoicePaymentSuggestionDaysAfterDue is how long after the due date a payment is still suggested
const InvoicePaymentSuggestionDaysAfterDue = 60

// InvoicePaymentSuggestionMaxCount is the maximum number of suggested payments
const InvoicePaymentSuggestionMaxCount = 10

// InvoiceProfile represents the user's own business details printed on every invoice
type InvoiceProfile struct {
	Uid              int64  `xorm:"PK"`
	BusinessName     string `xorm:"VARCHAR(128) NOT NULL"`
	AddressLines     string `xorm:"VARCHAR(512) NOT NULL"`
	Phone            string `xorm:"VARCHAR(32) NOT NULL"`
	Website          string `xorm:"VARCHAR(128) NOT NULL"`
	LogoDataUrl      string `xorm:"MEDIUMTEXT"`
	DefaultNotes     string `xorm:"VARCHAR(2048) NOT NULL"`
	NumberPrefix     string `xorm:"VARCHAR(16) NOT NULL"`
	DefaultTermsDays int32  `xorm:"NOT NULL"`
	CreatedUnixTime  int64
	UpdatedUnixTime  int64
}

// Customer represents an invoice customer stored in database
type Customer struct {
	CustomerId      int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_customer_uid_deleted) NOT NULL"`
	Deleted         bool   `xorm:"INDEX(IDX_customer_uid_deleted) NOT NULL"`
	Name            string `xorm:"VARCHAR(128) NOT NULL"`
	ContactName     string `xorm:"VARCHAR(128) NOT NULL"`
	AddressLines    string `xorm:"VARCHAR(512) NOT NULL"`
	Email           string `xorm:"VARCHAR(128) NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// Invoice represents an invoice stored in database
type Invoice struct {
	InvoiceId          int64  `xorm:"PK"`
	Uid                int64  `xorm:"INDEX(IDX_invoice_uid_deleted) NOT NULL"`
	Deleted            bool   `xorm:"INDEX(IDX_invoice_uid_deleted) NOT NULL"`
	Kind               string `xorm:"VARCHAR(16) NOT NULL"`
	Number             string `xorm:"VARCHAR(64) NOT NULL"`
	CustomerId         int64  `xorm:"NOT NULL"`
	BillToName         string `xorm:"VARCHAR(128) NOT NULL"`
	BillToContactName  string `xorm:"VARCHAR(128) NOT NULL"`
	BillToAddressLines string `xorm:"VARCHAR(512) NOT NULL"`
	BillToEmail        string `xorm:"VARCHAR(128) NOT NULL"`
	Currency           string `xorm:"VARCHAR(3) NOT NULL"`
	InvoiceDate        string `xorm:"VARCHAR(10) NOT NULL"`
	DueDate            string `xorm:"VARCHAR(10) NOT NULL"`
	Notes              string `xorm:"VARCHAR(2048) NOT NULL"`
	Total              int64  `xorm:"NOT NULL"`
	ClosedManually     bool   `xorm:"NOT NULL"`
	CreatedUnixTime    int64
	UpdatedUnixTime    int64
	DeletedUnixTime    int64
}

// InvoiceLine represents one service line of an invoice stored in database
type InvoiceLine struct {
	LineId       int64  `xorm:"PK"`
	InvoiceId    int64  `xorm:"INDEX(IDX_invoice_line_invoice_id) NOT NULL"`
	Uid          int64  `xorm:"NOT NULL"`
	DisplayOrder int32  `xorm:"NOT NULL"`
	ItemName     string `xorm:"VARCHAR(128) NOT NULL"`
	Description  string `xorm:"VARCHAR(512) NOT NULL"`
	Quantity     int64  `xorm:"NOT NULL"`
	Rate         int64  `xorm:"NOT NULL"`
	Amount       int64  `xorm:"NOT NULL"`
}

// InvoicePayment links an existing income transaction to an invoice.
// The paid amount is read from the transaction, so editing or deleting it keeps the invoice correct.
type InvoicePayment struct {
	PaymentId      int64 `xorm:"PK"`
	InvoiceId      int64 `xorm:"INDEX(IDX_invoice_payment_invoice_id) NOT NULL"`
	Uid            int64 `xorm:"NOT NULL"`
	TransactionId  int64 `xorm:"UNIQUE(UQE_invoice_payment_transaction_id) NOT NULL"`
	LinkedUnixTime int64
}

// InvoiceProfileUpdateRequest represents all parameters of invoice profile update request
type InvoiceProfileUpdateRequest struct {
	BusinessName     string `json:"businessName" binding:"max=128"`
	AddressLines     string `json:"addressLines" binding:"max=512"`
	Phone            string `json:"phone" binding:"max=32"`
	Website          string `json:"website" binding:"max=128"`
	LogoDataUrl      string `json:"logoDataUrl" binding:"max=350000"`
	DefaultNotes     string `json:"defaultNotes" binding:"max=2048"`
	NumberPrefix     string `json:"numberPrefix" binding:"required,notBlank,max=16"`
	DefaultTermsDays int32  `json:"defaultTermsDays" binding:"min=0,max=365"`
}

// InvoiceProfileResponse represents a view-object of invoice profile
type InvoiceProfileResponse struct {
	BusinessName     string `json:"businessName"`
	AddressLines     string `json:"addressLines"`
	Phone            string `json:"phone"`
	Website          string `json:"website"`
	LogoDataUrl      string `json:"logoDataUrl"`
	DefaultNotes     string `json:"defaultNotes"`
	NumberPrefix     string `json:"numberPrefix"`
	DefaultTermsDays int32  `json:"defaultTermsDays"`
}

// CustomerCreateRequest represents all parameters of customer creation request
type CustomerCreateRequest struct {
	Name         string `json:"name" binding:"required,notBlank,max=128"`
	ContactName  string `json:"contactName" binding:"max=128"`
	AddressLines string `json:"addressLines" binding:"max=512"`
	Email        string `json:"email" binding:"max=128"`
}

// CustomerModifyRequest represents all parameters of customer modification request
type CustomerModifyRequest struct {
	Id           int64  `json:"id,string" binding:"required,min=1"`
	Name         string `json:"name" binding:"required,notBlank,max=128"`
	ContactName  string `json:"contactName" binding:"max=128"`
	AddressLines string `json:"addressLines" binding:"max=512"`
	Email        string `json:"email" binding:"max=128"`
}

// CustomerResponse represents a view-object of customer
type CustomerResponse struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	ContactName  string `json:"contactName"`
	AddressLines string `json:"addressLines"`
	Email        string `json:"email"`
}

// InvoiceLineRequest represents one line of invoice creation / modification request
type InvoiceLineRequest struct {
	ItemName    string `json:"itemName" binding:"required,notBlank,max=128"`
	Description string `json:"description" binding:"max=512"`
	Quantity    int64  `json:"quantity" binding:"required,min=1,max=10000"`
	Rate        int64  `json:"rate" binding:"required,min=1,max=10000000000"`
}

// InvoiceCreateRequest represents all parameters of invoice creation request
type InvoiceCreateRequest struct {
	Number      string                `json:"number" binding:"required,notBlank,max=64"`
	CustomerId  int64                 `json:"customerId,string" binding:"required,min=1"`
	InvoiceDate string                `json:"invoiceDate" binding:"required,len=10"`
	DueDate     string                `json:"dueDate" binding:"required,len=10"`
	Notes       string                `json:"notes" binding:"max=2048"`
	Lines       []*InvoiceLineRequest `json:"lines" binding:"required,min=1,max=100,dive"`
}

// InvoiceModifyRequest represents all parameters of invoice modification request
type InvoiceModifyRequest struct {
	Id          int64                 `json:"id,string" binding:"required,min=1"`
	Number      string                `json:"number" binding:"required,notBlank,max=64"`
	CustomerId  int64                 `json:"customerId,string" binding:"required,min=1"`
	InvoiceDate string                `json:"invoiceDate" binding:"required,len=10"`
	DueDate     string                `json:"dueDate" binding:"required,len=10"`
	Notes       string                `json:"notes" binding:"max=2048"`
	Lines       []*InvoiceLineRequest `json:"lines" binding:"required,min=1,max=100,dive"`
}

// InvoiceIdRequest represents a request that only carries an invoice / customer id (get, delete, close, reopen)
type InvoiceIdRequest struct {
	Id int64 `json:"id,string" form:"id,string" binding:"required,min=1"`
}

// InvoiceNextNumberRequest represents all parameters of next invoice number request
type InvoiceNextNumberRequest struct {
	InvoiceDate string `form:"invoice_date" binding:"required,len=10"`
}

// InvoicePaymentLinkRequest represents all parameters of invoice payment link / unlink request
type InvoicePaymentLinkRequest struct {
	InvoiceId     int64 `json:"invoiceId,string" binding:"required,min=1"`
	TransactionId int64 `json:"transactionId,string" binding:"required,min=1"`
}

// InvoiceLineResponse represents a view-object of invoice line
type InvoiceLineResponse struct {
	ItemName    string `json:"itemName"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	Rate        int64  `json:"rate"`
	Amount      int64  `json:"amount"`
}

// InvoicePaymentResponse represents a view-object of a transaction linked (or suggested) as an invoice payment
type InvoicePaymentResponse struct {
	TransactionId   string `json:"transactionId"`
	Amount          int64  `json:"amount"`
	TransactionTime int64  `json:"transactionTime"`
	AccountId       string `json:"accountId"`
	Comment         string `json:"comment"`
}

// InvoiceInfoResponse represents a view-object of invoice
type InvoiceInfoResponse struct {
	Id                 string                    `json:"id"`
	Kind               string                    `json:"kind"`
	Number             string                    `json:"number"`
	CustomerId         string                    `json:"customerId"`
	BillToName         string                    `json:"billToName"`
	BillToContactName  string                    `json:"billToContactName"`
	BillToAddressLines string                    `json:"billToAddressLines"`
	BillToEmail        string                    `json:"billToEmail"`
	Currency           string                    `json:"currency"`
	InvoiceDate        string                    `json:"invoiceDate"`
	DueDate            string                    `json:"dueDate"`
	Notes              string                    `json:"notes"`
	Total              int64                     `json:"total"`
	Received           int64                     `json:"received"`
	ClosedManually     bool                      `json:"closedManually"`
	Lines              []*InvoiceLineResponse    `json:"lines,omitempty"`
	Payments           []*InvoicePaymentResponse `json:"payments,omitempty"`
}

// ToInvoiceProfileResponse returns a view-object according to database model
func (p *InvoiceProfile) ToInvoiceProfileResponse() *InvoiceProfileResponse {
	return &InvoiceProfileResponse{
		BusinessName:     p.BusinessName,
		AddressLines:     p.AddressLines,
		Phone:            p.Phone,
		Website:          p.Website,
		LogoDataUrl:      p.LogoDataUrl,
		DefaultNotes:     p.DefaultNotes,
		NumberPrefix:     p.NumberPrefix,
		DefaultTermsDays: p.DefaultTermsDays,
	}
}

// ToCustomerResponse returns a view-object according to database model
func (c *Customer) ToCustomerResponse() *CustomerResponse {
	return &CustomerResponse{
		Id:           utils.Int64ToString(c.CustomerId),
		Name:         c.Name,
		ContactName:  c.ContactName,
		AddressLines: c.AddressLines,
		Email:        c.Email,
	}
}

// ToInvoicePaymentResponse returns a view-object of a transaction used as an invoice payment
func ToInvoicePaymentResponse(t *Transaction) *InvoicePaymentResponse {
	return &InvoicePaymentResponse{
		TransactionId:   utils.Int64ToString(t.TransactionId),
		Amount:          t.Amount,
		TransactionTime: utils.GetUnixTimeFromTransactionTime(t.TransactionTime),
		AccountId:       utils.Int64ToString(t.AccountId),
		Comment:         t.Comment,
	}
}

// ToInvoiceInfoResponse returns a view-object according to database model; lines and payments may be nil (list view)
func (i *Invoice) ToInvoiceInfoResponse(lines []*InvoiceLine, payments []*Transaction) *InvoiceInfoResponse {
	resp := &InvoiceInfoResponse{
		Id:                 utils.Int64ToString(i.InvoiceId),
		Kind:               i.Kind,
		Number:             i.Number,
		CustomerId:         utils.Int64ToString(i.CustomerId),
		BillToName:         i.BillToName,
		BillToContactName:  i.BillToContactName,
		BillToAddressLines: i.BillToAddressLines,
		BillToEmail:        i.BillToEmail,
		Currency:           i.Currency,
		InvoiceDate:        i.InvoiceDate,
		DueDate:            i.DueDate,
		Notes:              i.Notes,
		Total:              i.Total,
		ClosedManually:     i.ClosedManually,
	}

	for _, payment := range payments {
		resp.Received += payment.Amount
	}

	if lines != nil {
		resp.Lines = make([]*InvoiceLineResponse, len(lines))

		for idx, line := range lines {
			resp.Lines[idx] = &InvoiceLineResponse{
				ItemName:    line.ItemName,
				Description: line.Description,
				Quantity:    line.Quantity,
				Rate:        line.Rate,
				Amount:      line.Amount,
			}
		}

		resp.Payments = make([]*InvoicePaymentResponse, len(payments))

		for idx, payment := range payments {
			resp.Payments[idx] = ToInvoicePaymentResponse(payment)
		}
	}

	return resp
}
