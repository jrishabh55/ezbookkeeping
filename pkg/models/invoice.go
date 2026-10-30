package models

import (
	"github.com/mayswind/ezbookkeeping/pkg/invoiceemail"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// Invoice kinds: an invoice, or an estimate (quote) that can be converted into invoices; estimates share the table and the editor
const (
	INVOICE_KIND_INVOICE  = "invoice"
	INVOICE_KIND_ESTIMATE = "estimate"
)

// INVOICE_DEFAULT_CURRENCY is the only invoice currency for now
const INVOICE_DEFAULT_CURRENCY = "INR"

// Invoice tax types: no GST, IGST (inter-state) or CGST + SGST (intra-state, the rate split evenly)
const (
	INVOICE_TAX_TYPE_NONE      = "none"
	INVOICE_TAX_TYPE_IGST      = "igst"
	INVOICE_TAX_TYPE_CGST_SGST = "cgst_sgst"
)

// Customer timeline entry kinds
const (
	CUSTOMER_TIMELINE_KIND_INVOICE = "invoice"
	CUSTOMER_TIMELINE_KIND_EMAIL   = "email"
	CUSTOMER_TIMELINE_KIND_PAYMENT = "payment"
)

// CustomerTimelineMaxItems is how many of the newest timeline entries the customer page shows
const CustomerTimelineMaxItems = 200

// CustomerProductResponse is a product the customer has bought, from the product-linked lines of their invoices
type CustomerProductResponse struct {
	ProductId      string `json:"productId"`
	Name           string `json:"name"`
	TimesBought    int    `json:"timesBought"` // invoices carrying the product
	LastBoughtDate string `json:"lastBoughtDate"`
	Archived       bool   `json:"archived"`
}

// CustomerTimelineItemResponse is one event on the customer page timeline
type CustomerTimelineItemResponse struct {
	Kind          string `json:"kind"`
	UnixTime      int64  `json:"unixTime"`
	InvoiceId     string `json:"invoiceId"`
	InvoiceNumber string `json:"invoiceNumber"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Text          string `json:"text"`    // the email subject for emails
	Renewal       bool   `json:"renewal"` // the invoice was created by a recurring schedule
}

// CustomerOverviewResponse is what the customer page shows besides the customer, invoices and recurring invoices it already has
type CustomerOverviewResponse struct {
	Products []*CustomerProductResponse      `json:"products"`
	Timeline []*CustomerTimelineItemResponse `json:"timeline"`
}

// INVOICE_TAX_RATE_MAX is the largest tax rate in basis points (100%)
const INVOICE_TAX_RATE_MAX = 10000

// InvoicePaidTolerance is the share of the total that may be missing (bank / forex charges)
// while a payment still counts as settling the invoice. The UI mirrors it in src/lib/invoice.ts.
const InvoicePaidTolerance = 0.03

// InvoicePaymentSuggestionDaysAfterDue is how long after the due date a payment is still suggested
const InvoicePaymentSuggestionDaysAfterDue = 60

// InvoicePaymentSuggestionMaxCount is the maximum number of suggested payments
const InvoicePaymentSuggestionMaxCount = 10

// InvoiceRenewalDraftDaysDefault is how many days before the renewal date a draft_ahead invoice is created, unless the profile says otherwise
const InvoiceRenewalDraftDaysDefault = 15

// InvoiceRenewalDraftDaysMax is the largest allowed renewal draft lead time
const InvoiceRenewalDraftDaysMax = 90

// InvoiceProfile represents the user's own business details printed on every invoice
type InvoiceProfile struct {
	EmailSubjectTemplate string `xorm:"VARCHAR(256) NOT NULL DEFAULT ''"`
	EmailBodyTemplate    string `xorm:"MEDIUMTEXT"`
	Uid                  int64  `xorm:"PK"`
	BusinessName         string `xorm:"VARCHAR(128) NOT NULL"`
	AddressLines         string `xorm:"VARCHAR(512) NOT NULL"`
	Phone                string `xorm:"VARCHAR(32) NOT NULL"`
	Website              string `xorm:"VARCHAR(128) NOT NULL"`
	LogoDataUrl          string `xorm:"MEDIUMTEXT"`
	HideLogo             bool   `xorm:"NOT NULL DEFAULT false"`
	DefaultNotes         string `xorm:"VARCHAR(2048) NOT NULL"`
	NumberPrefix         string `xorm:"VARCHAR(16) NOT NULL"`
	DefaultTermsDays     int32  `xorm:"NOT NULL"`
	Gstin                string `xorm:"VARCHAR(15) NOT NULL DEFAULT ''"`
	DefaultSac           string `xorm:"VARCHAR(8) NOT NULL DEFAULT ''"`
	CreatedUnixTime      int64
	UpdatedUnixTime      int64
	DefaultCurrency      string `xorm:"VARCHAR(3) NOT NULL DEFAULT ''"` // empty means INVOICE_DEFAULT_CURRENCY
	RenewalDraftDays     int32  `xorm:"NOT NULL DEFAULT 15"`
	EstimateNumberPrefix string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'EST'"`
	EstimateValidDays    int32  `xorm:"NOT NULL DEFAULT 30"`
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
	Phone           string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	Website         string `xorm:"VARCHAR(128) NOT NULL DEFAULT ''"`
	AccountNumber   string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	Pan             string `xorm:"VARCHAR(10) NOT NULL DEFAULT ''"`
	TermsDays       int32  `xorm:"NOT NULL DEFAULT 0"`
	Notes           string `xorm:"VARCHAR(2048) NOT NULL DEFAULT ''"`
	Gstin           string `xorm:"VARCHAR(15) NOT NULL DEFAULT ''"`
	TaxType         string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"` // empty means none
	TaxRate         int32  `xorm:"NOT NULL DEFAULT 0"`              // basis points
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// Invoice represents an invoice stored in database
type Invoice struct {
	InvoiceId          int64  `xorm:"PK"`
	Uid                int64  `xorm:"INDEX(IDX_invoice_uid_deleted) INDEX(IDX_invoice_uid_recurring_id) INDEX(IDX_invoice_uid_estimate_id) NOT NULL"`
	Deleted            bool   `xorm:"INDEX(IDX_invoice_uid_deleted) NOT NULL"`
	Kind               string `xorm:"VARCHAR(16) NOT NULL"`
	Number             string `xorm:"VARCHAR(64) NOT NULL"`
	CustomerId         int64  `xorm:"NOT NULL"`
	BillToName         string `xorm:"VARCHAR(128) NOT NULL"`
	BillToContactName  string `xorm:"VARCHAR(128) NOT NULL"`
	BillToAddressLines string `xorm:"VARCHAR(512) NOT NULL"`
	BillToEmail        string `xorm:"VARCHAR(128) NOT NULL"`
	BillToPhone        string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	BillToPan          string `xorm:"VARCHAR(10) NOT NULL DEFAULT ''"`
	BillToGstin        string `xorm:"VARCHAR(15) NOT NULL DEFAULT ''"`
	Currency           string `xorm:"VARCHAR(3) NOT NULL"`
	InvoiceDate        string `xorm:"VARCHAR(10) NOT NULL"`
	DueDate            string `xorm:"VARCHAR(10) NOT NULL"`
	Notes              string `xorm:"VARCHAR(2048) NOT NULL"`
	Total              int64  `xorm:"NOT NULL"`
	TaxType            string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"` // empty means none (rows saved before GST)
	TaxRate            int32  `xorm:"NOT NULL DEFAULT 0"`              // basis points
	Subtotal           int64  `xorm:"NOT NULL DEFAULT 0"`              // 0 on rows saved before GST; the document then uses Total
	TaxAmount          int64  `xorm:"NOT NULL DEFAULT 0"`
	ClosedManually     bool   `xorm:"NOT NULL"`
	CreatedUnixTime    int64
	UpdatedUnixTime    int64
	DeletedUnixTime    int64
	// ponytail: a plain (non-unique) index: empty means "no public link" and would collide under UNIQUE on every
	// database, while two equal 256-bit random tokens will not happen; works unchanged on SQLite, MySQL and Postgres
	PublicToken string `xorm:"VARCHAR(43) INDEX(IDX_invoice_public_token) NOT NULL DEFAULT ''"`
	// when the invoice was last emailed from the app; 0 = never sent
	SentUnixTime int64 `xorm:"NOT NULL DEFAULT 0"`
	// RecurringId is the recurring invoice that created this invoice; 0 for invoices made by hand
	RecurringId int64 `xorm:"INDEX(IDX_invoice_uid_recurring_id) NOT NULL DEFAULT 0"`
	// EstimateStatus is draft, sent, accepted or declined on an estimate and empty on an invoice
	EstimateStatus string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	// RespondedUnixTime is when an estimate was accepted or declined; 0 otherwise
	RespondedUnixTime int64 `xorm:"NOT NULL DEFAULT 0"`
	// RespondedByClient is true when the client answered through the public link
	RespondedByClient bool `xorm:"NOT NULL DEFAULT false"`
	// ResponseNote is the client's optional decline note, plain text, at most 500 characters
	ResponseNote string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	// EstimateId is the estimate this invoice was created from; 0 otherwise
	EstimateId int64 `xorm:"INDEX(IDX_invoice_uid_estimate_id) NOT NULL DEFAULT 0"`
}

// InvoicePublicTokenLength is the length of a public invoice token (32 random bytes, base64url without padding)
const InvoicePublicTokenLength = 43

// InvoiceLine represents one service line of an invoice stored in database
type InvoiceLine struct {
	LineId       int64  `xorm:"PK"`
	InvoiceId    int64  `xorm:"INDEX(IDX_invoice_line_invoice_id) NOT NULL"`
	Uid          int64  `xorm:"INDEX(IDX_invoice_line_uid_product_id) NOT NULL"`
	DisplayOrder int32  `xorm:"NOT NULL"`
	ItemName     string `xorm:"VARCHAR(128) NOT NULL"`
	Description  string `xorm:"VARCHAR(512) NOT NULL"`
	Quantity     int64  `xorm:"NOT NULL"` // hundredths: 100 = 1, 750 = 7.5
	Rate         int64  `xorm:"NOT NULL"`
	Amount       int64  `xorm:"NOT NULL"`
	Sac          string `xorm:"VARCHAR(8) NOT NULL DEFAULT ''"`
	ProductId    int64  `xorm:"INDEX(IDX_invoice_line_uid_product_id) NOT NULL DEFAULT 0"`
}

// InvoicePayment links an existing income transaction to an invoice.
// The paid amount is read from the transaction, so editing or deleting it keeps the invoice correct.
type InvoicePayment struct {
	PaymentId      int64 `xorm:"PK"`
	InvoiceId      int64 `xorm:"INDEX(IDX_invoice_payment_invoice_id) NOT NULL"`
	Uid            int64 `xorm:"NOT NULL"`
	TransactionId  int64 `xorm:"UNIQUE(UQE_invoice_payment_transaction_id) NOT NULL"`
	LinkedUnixTime int64
	SettledAmount  int64 `xorm:"NOT NULL DEFAULT 0"` // in invoice currency; 0 = same currency, the transaction amount counts
}

// InvoiceEmail is one sent invoice email, kept exactly as it went out
type InvoiceEmail struct {
	EmailId      int64  `xorm:"PK"`
	Uid          int64  `xorm:"NOT NULL"`
	InvoiceId    int64  `xorm:"INDEX(IDX_invoice_email_invoice_id) NOT NULL"`
	ToAddress    string `xorm:"VARCHAR(128) NOT NULL"`
	Subject      string `xorm:"VARCHAR(256) NOT NULL"`
	EmailHtml    string `xorm:"MEDIUMTEXT"`
	SentUnixTime int64  `xorm:"NOT NULL"`
}

// InvoiceEmailResponse is a sent email; EmailHtml is only filled when one email is requested
type InvoiceEmailResponse struct {
	Id           string `json:"id"`
	To           string `json:"to"`
	Subject      string `json:"subject"`
	SentUnixTime int64  `json:"sentUnixTime"`
	EmailHtml    string `json:"emailHtml,omitempty"`
}

// ToInvoiceEmailResponse returns a view-object of the sent email
func (e *InvoiceEmail) ToInvoiceEmailResponse() *InvoiceEmailResponse {
	return &InvoiceEmailResponse{Id: utils.Int64ToString(e.EmailId), To: e.ToAddress, Subject: e.Subject, SentUnixTime: e.SentUnixTime, EmailHtml: e.EmailHtml}
}

// InvoicePaymentInfo is a linked (or suggested) transaction with the account currency it was received in
// and, for a payment in another currency, the amount it settled in the invoice currency
type InvoicePaymentInfo struct {
	Transaction   *Transaction
	Currency      string // the account currency
	SettledAmount int64  // 0 when the account currency is the invoice currency
}

// ReceivedAmount returns what the payment counts for in the invoice currency
func (p *InvoicePaymentInfo) ReceivedAmount() int64 {
	if p.SettledAmount > 0 {
		return p.SettledAmount
	}

	return p.Transaction.Amount
}

// InvoiceReceivedAmount is the single place "received" is summed: the list, detail, public view, PDF and balance due all use it
func InvoiceReceivedAmount(payments []*InvoicePaymentInfo) int64 {
	received := int64(0)

	for _, payment := range payments {
		received += payment.ReceivedAmount()
	}

	return received
}

// InvoiceProfileUpdateRequest represents all parameters of invoice profile update request
type InvoiceProfileUpdateRequest struct {
	BusinessName         string `json:"businessName" binding:"max=128"`
	AddressLines         string `json:"addressLines" binding:"max=512"`
	Phone                string `json:"phone" binding:"max=32"`
	Website              string `json:"website" binding:"max=128"`
	LogoDataUrl          string `json:"logoDataUrl" binding:"max=350000"`
	HideLogo             bool   `json:"hideLogo"`
	DefaultNotes         string `json:"defaultNotes" binding:"max=2048"`
	NumberPrefix         string `json:"numberPrefix" binding:"required,notBlank,max=16"`
	DefaultTermsDays     int32  `json:"defaultTermsDays" binding:"min=0,max=365"`
	Gstin                string `json:"gstin" binding:"max=15"`
	DefaultSac           string `json:"defaultSac" binding:"max=8"`
	DefaultCurrency      string `json:"defaultCurrency" binding:"max=3"` // empty means INR; checked against the known currencies by the handler
	RenewalDraftDays     int32  `json:"renewalDraftDays" binding:"min=0,max=90"`
	EstimateNumberPrefix string `json:"estimateNumberPrefix" binding:"max=16"`     // empty means EST
	EstimateValidDays    int32  `json:"estimateValidDays" binding:"min=0,max=365"` // 0 means 30
}

// InvoiceProfileResponse represents a view-object of invoice profile
type InvoiceProfileResponse struct {
	EmailSubjectTemplate string `json:"emailSubjectTemplate"`
	EmailBodyTemplate    string `json:"emailBodyTemplate"`
	BusinessName         string `json:"businessName"`
	AddressLines         string `json:"addressLines"`
	Phone                string `json:"phone"`
	Website              string `json:"website"`
	LogoDataUrl          string `json:"logoDataUrl"`
	HideLogo             bool   `json:"hideLogo"`
	DefaultNotes         string `json:"defaultNotes"`
	NumberPrefix         string `json:"numberPrefix"`
	DefaultTermsDays     int32  `json:"defaultTermsDays"`
	Gstin                string `json:"gstin"`
	DefaultSac           string `json:"defaultSac"`
	DefaultCurrency      string `json:"defaultCurrency"`
	RenewalDraftDays     int32  `json:"renewalDraftDays"`
	EstimateNumberPrefix string `json:"estimateNumberPrefix"`
	EstimateValidDays    int32  `json:"estimateValidDays"`
}

// CustomerCreateRequest represents all parameters of customer creation request
type CustomerCreateRequest struct {
	Name          string `json:"name" binding:"required,notBlank,max=128"`
	ContactName   string `json:"contactName" binding:"max=128"`
	AddressLines  string `json:"addressLines" binding:"max=512"`
	Email         string `json:"email" binding:"max=128"`
	Phone         string `json:"phone" binding:"max=32"`
	Website       string `json:"website" binding:"max=128"`
	AccountNumber string `json:"accountNumber" binding:"max=64"`
	Pan           string `json:"pan" binding:"max=10"`
	TermsDays     int32  `json:"termsDays" binding:"min=0,max=365"`
	Notes         string `json:"notes" binding:"max=2048"`
	Gstin         string `json:"gstin" binding:"max=15"`
	TaxType       string `json:"taxType" binding:"max=16"`
	TaxRate       int32  `json:"taxRate" binding:"min=0,max=10000"`
}

// CustomerModifyRequest represents all parameters of customer modification request
type CustomerModifyRequest struct {
	Id            int64  `json:"id,string" binding:"required,min=1"`
	Name          string `json:"name" binding:"required,notBlank,max=128"`
	ContactName   string `json:"contactName" binding:"max=128"`
	AddressLines  string `json:"addressLines" binding:"max=512"`
	Email         string `json:"email" binding:"max=128"`
	Phone         string `json:"phone" binding:"max=32"`
	Website       string `json:"website" binding:"max=128"`
	AccountNumber string `json:"accountNumber" binding:"max=64"`
	Pan           string `json:"pan" binding:"max=10"`
	TermsDays     int32  `json:"termsDays" binding:"min=0,max=365"`
	Notes         string `json:"notes" binding:"max=2048"`
	Gstin         string `json:"gstin" binding:"max=15"`
	TaxType       string `json:"taxType" binding:"max=16"`
	TaxRate       int32  `json:"taxRate" binding:"min=0,max=10000"`
}

// CustomerResponse represents a view-object of customer
type CustomerResponse struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	ContactName   string `json:"contactName"`
	AddressLines  string `json:"addressLines"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Website       string `json:"website"`
	AccountNumber string `json:"accountNumber"`
	Pan           string `json:"pan"`
	TermsDays     int32  `json:"termsDays"`
	Notes         string `json:"notes"`
	Gstin         string `json:"gstin"`
	TaxType       string `json:"taxType"`
	TaxRate       int32  `json:"taxRate"`
}

// InvoiceLineRequest represents one line of invoice creation / modification request
type InvoiceLineRequest struct {
	ItemName    string `json:"itemName" binding:"required,notBlank,max=128"`
	Description string `json:"description" binding:"max=512"`
	Quantity    int64  `json:"quantity" binding:"required,min=1,max=1000000"` // hundredths: 0.01 to 10,000
	Rate        int64  `json:"rate" binding:"required,min=1,max=10000000000"`
	Sac         string `json:"sac" binding:"max=8"`
	ProductId   int64  `json:"productId,string" binding:"min=0"`
}

// InvoiceCreateRequest represents all parameters of invoice creation request
type InvoiceCreateRequest struct {
	Number      string                      `json:"number" binding:"required,notBlank,max=64"`
	CustomerId  int64                       `json:"customerId,string" binding:"required,min=1"`
	InvoiceDate string                      `json:"invoiceDate" binding:"required,len=10"`
	DueDate     string                      `json:"dueDate" binding:"required,len=10"`
	Notes       string                      `json:"notes" binding:"max=2048"`
	TaxType     string                      `json:"taxType" binding:"max=16"`
	TaxRate     int32                       `json:"taxRate" binding:"min=0,max=10000"`
	Lines       []*InvoiceLineRequest       `json:"lines" binding:"required,min=1,max=100,dive"`
	Currency    string                      `json:"currency" binding:"max=3"`                        // empty means the profile's default currency; not editable afterwards
	Kind        string                      `json:"kind" binding:"omitempty,oneof=invoice estimate"` // empty means invoice
	Milestones  []*EstimateMilestoneRequest `json:"milestones" binding:"omitempty,max=10,dive"`      // estimates only
	EstimateId  int64                       `json:"estimateId,string" binding:"min=0"`               // the invoice is made from this estimate; 0 = typed by hand
	MilestoneId int64                       `json:"milestoneId,string" binding:"min=0"`              // and from this milestone of it; 0 = the whole estimate
}

// InvoiceModifyRequest represents all parameters of invoice modification request
type InvoiceModifyRequest struct {
	Id          int64                       `json:"id,string" binding:"required,min=1"`
	Number      string                      `json:"number" binding:"required,notBlank,max=64"`
	CustomerId  int64                       `json:"customerId,string" binding:"required,min=1"`
	InvoiceDate string                      `json:"invoiceDate" binding:"required,len=10"`
	DueDate     string                      `json:"dueDate" binding:"required,len=10"`
	Notes       string                      `json:"notes" binding:"max=2048"`
	TaxType     string                      `json:"taxType" binding:"max=16"`
	TaxRate     int32                       `json:"taxRate" binding:"min=0,max=10000"`
	Lines       []*InvoiceLineRequest       `json:"lines" binding:"required,min=1,max=100,dive"`
	Kind        string                      `json:"kind" binding:"omitempty,oneof=invoice estimate"` // empty means invoice
	Milestones  []*EstimateMilestoneRequest `json:"milestones" binding:"omitempty,max=10,dive"`      // estimates only; replaces the stored ones
}

// InvoiceIdRequest represents a request that only carries an invoice / customer id (get, delete, close, reopen)
type InvoiceIdRequest struct {
	Id int64 `json:"id,string" form:"id,string" binding:"required,min=1"`
}

// InvoiceListRequest picks invoices (default) or estimates for the list
type InvoiceListRequest struct {
	Kind string `form:"kind" binding:"omitempty,oneof=invoice estimate"`
}

// InvoiceNextNumberRequest represents all parameters of next invoice number request
type InvoiceNextNumberRequest struct {
	InvoiceDate string `form:"invoice_date" binding:"required,len=10"`
	Kind        string `form:"kind" binding:"omitempty,oneof=invoice estimate"`
}

// InvoicePaymentLinkRequest represents all parameters of invoice payment link / unlink request
type InvoicePaymentLinkRequest struct {
	InvoiceId     int64 `json:"invoiceId,string" binding:"required,min=1"`
	TransactionId int64 `json:"transactionId,string" binding:"required,min=1"`
	SettledAmount int64 `json:"settledAmount" binding:"min=0"` // link only: required (> 0) when the account currency differs from the invoice currency
}

// InvoicePublicLinkUpdateRequest represents all parameters of enabling (a fresh token every time), or disabling, an invoice's public link
type InvoicePublicLinkUpdateRequest struct {
	Id      int64 `json:"id,string" binding:"required,min=1"`
	Enabled bool  `json:"enabled"`
}

// InvoicePublicLinkResponse carries the current public token of an invoice (empty when the link is disabled)
type InvoicePublicLinkResponse struct {
	PublicToken string `json:"publicToken"`
}

// InvoicePublicViewRequest represents all parameters of the unauthenticated invoice view request
type InvoicePublicViewRequest struct {
	Token string `form:"token" binding:"max=64"`
}

// InvoiceSendRequest supports saved templates, rich overrides and the legacy plain text message.
type InvoiceSendRequest struct {
	Id            int64   `json:"id,string" binding:"required,min=1"`
	To            string  `json:"to" binding:"required,notBlank,max=128,validEmail"`
	Subject       string  `json:"subject" binding:"max=256"`
	Message       *string `json:"message" binding:"omitempty,max=2048"`
	BodyHtml      *string `json:"bodyHtml" binding:"omitempty,max=32000"`
	CreateRenewal bool    `json:"createRenewal"`
}

// InvoiceEmailTemplateUpdateRequest saves a standalone template draft; an empty body must be explicit.
type InvoiceEmailTemplateUpdateRequest struct {
	Subject  string  `json:"subject" binding:"required,max=256"`
	BodyHtml *string `json:"bodyHtml" binding:"required,max=32000"`
}

// InvoiceEmailPreviewRequest accepts unsaved overrides; absence and an empty body are distinct.
type InvoiceEmailPreviewRequest struct {
	InvoiceId int64   `json:"invoiceId,string" binding:"min=0"`
	Subject   *string `json:"subject" binding:"omitempty,max=256"`
	BodyHtml  *string `json:"bodyHtml" binding:"omitempty,max=32000"`
}

// InvoiceEmailPreviewResponse is a rendered email and editor metadata, with no sending side effects.
type InvoiceEmailPreviewResponse struct {
	Subject                string                  `json:"subject"`
	BodyHtml               string                  `json:"bodyHtml"`
	EmailHtml              string                  `json:"emailHtml"`
	Variables              []invoiceemail.Variable `json:"variables"`
	DefaultSubjectTemplate string                  `json:"defaultSubjectTemplate"`
	DefaultBodyTemplate    string                  `json:"defaultBodyTemplate"`
	Sample                 bool                    `json:"sample"`
}

// InvoiceLineResponse represents a view-object of invoice line
type InvoiceLineResponse struct {
	ItemName    string `json:"itemName"`
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	Rate        int64  `json:"rate"`
	Amount      int64  `json:"amount"`
	Sac         string `json:"sac"`
	ProductId   string `json:"productId,omitempty"`
}

// InvoicePaymentResponse represents a view-object of a transaction linked (or suggested) as an invoice payment
type InvoicePaymentResponse struct {
	TransactionId   string `json:"transactionId"`
	Amount          int64  `json:"amount"`
	TransactionTime int64  `json:"transactionTime"`
	AccountId       string `json:"accountId"`
	Comment         string `json:"comment"`
	Currency        string `json:"currency"`      // the account currency: amount is realised in it
	SettledAmount   int64  `json:"settledAmount"` // in invoice currency; 0 when the currencies match
}

// InvoiceInfoResponse represents a view-object of invoice
type InvoiceInfoResponse struct {
	Id                 string                              `json:"id"`
	Kind               string                              `json:"kind"`
	Number             string                              `json:"number"`
	CustomerId         string                              `json:"customerId"`
	BillToName         string                              `json:"billToName"`
	BillToContactName  string                              `json:"billToContactName"`
	BillToAddressLines string                              `json:"billToAddressLines"`
	BillToEmail        string                              `json:"billToEmail"`
	BillToPhone        string                              `json:"billToPhone"`
	BillToPan          string                              `json:"billToPan"`
	BillToGstin        string                              `json:"billToGstin"`
	Currency           string                              `json:"currency"`
	InvoiceDate        string                              `json:"invoiceDate"`
	DueDate            string                              `json:"dueDate"`
	Notes              string                              `json:"notes"`
	Total              int64                               `json:"total"`
	TaxType            string                              `json:"taxType"`
	TaxRate            int32                               `json:"taxRate"`
	Subtotal           int64                               `json:"subtotal"`
	TaxAmount          int64                               `json:"taxAmount"`
	Received           int64                               `json:"received"`
	ClosedManually     bool                                `json:"closedManually"`
	Lines              []*InvoiceLineResponse              `json:"lines,omitempty"`
	Payments           []*InvoicePaymentResponse           `json:"payments,omitempty"`
	PublicToken        string                              `json:"publicToken,omitempty"`
	SentUnixTime       int64                               `json:"sentUnixTime,omitempty"`
	Emails             []*InvoiceEmailResponse             `json:"emails,omitempty"`
	RenewalOption      *InvoiceRenewalOption               `json:"renewalOption,omitempty"`
	EstimateStatus     string                              `json:"estimateStatus,omitempty"`
	RespondedUnixTime  int64                               `json:"respondedUnixTime,omitempty"`
	RespondedByClient  bool                                `json:"respondedByClient,omitempty"`
	ResponseNote       string                              `json:"responseNote,omitempty"`
	EstimateId         string                              `json:"estimateId,omitempty"`        // invoices made from an estimate
	MilestoneId        string                              `json:"milestoneId,omitempty"`       // conversion drafts of one milestone
	Milestones         []*EstimateMilestoneResponse        `json:"milestones,omitempty"`        // estimates
	ConvertedInvoices  []*EstimateConvertedInvoiceResponse `json:"convertedInvoices,omitempty"` // estimates: invoices made from it
	FullyInvoiced      bool                                `json:"fullyInvoiced,omitempty"`     // estimates
	EstimateNumber     string                              `json:"estimateNumber,omitempty"`    // invoices made from an estimate
}

// PublicInvoiceProfileResponse is the part of the invoice profile a recipient of a public link may see
type PublicInvoiceProfileResponse struct {
	BusinessName string `json:"businessName"`
	AddressLines string `json:"addressLines"`
	Phone        string `json:"phone"`
	Website      string `json:"website"`
	LogoDataUrl  string `json:"logoDataUrl"`
	Gstin        string `json:"gstin"`
}

// PublicInvoiceResponse is what a recipient of a public link sees: the printed document plus the amount received.
// It carries no ids, customer ids, payment details or the token itself.
type PublicInvoiceResponse struct {
	Number             string                        `json:"number"`
	BillToName         string                        `json:"billToName"`
	BillToContactName  string                        `json:"billToContactName"`
	BillToAddressLines string                        `json:"billToAddressLines"`
	BillToEmail        string                        `json:"billToEmail"`
	BillToPhone        string                        `json:"billToPhone"`
	BillToPan          string                        `json:"billToPan"`
	BillToGstin        string                        `json:"billToGstin"`
	Currency           string                        `json:"currency"`
	InvoiceDate        string                        `json:"invoiceDate"`
	DueDate            string                        `json:"dueDate"`
	Notes              string                        `json:"notes"`
	TaxType            string                        `json:"taxType"`
	TaxRate            int32                         `json:"taxRate"`
	Subtotal           int64                         `json:"subtotal"`
	TaxAmount          int64                         `json:"taxAmount"`
	Total              int64                         `json:"total"`
	Received           int64                         `json:"received"`
	ClosedManually     bool                          `json:"closedManually"`
	Lines              []*InvoiceLineResponse        `json:"lines"`
	Profile            *PublicInvoiceProfileResponse `json:"profile"`
}

// ToInvoiceProfileResponse returns a view-object according to database model
func (p *InvoiceProfile) ToInvoiceProfileResponse() *InvoiceProfileResponse {
	subject, body := invoiceemail.EffectiveTemplates(p.EmailSubjectTemplate, p.EmailBodyTemplate)
	return &InvoiceProfileResponse{
		EmailSubjectTemplate: subject,
		EmailBodyTemplate:    body,
		BusinessName:         p.BusinessName,
		AddressLines:         p.AddressLines,
		Phone:                p.Phone,
		Website:              p.Website,
		LogoDataUrl:          p.LogoDataUrl,
		HideLogo:             p.HideLogo,
		DefaultNotes:         p.DefaultNotes,
		NumberPrefix:         p.NumberPrefix,
		DefaultTermsDays:     p.DefaultTermsDays,
		Gstin:                p.Gstin,
		DefaultSac:           p.DefaultSac,
		DefaultCurrency:      p.DefaultCurrency,
		RenewalDraftDays:     p.RenewalDraftDays,
		EstimateNumberPrefix: p.EstimateNumberPrefix,
		EstimateValidDays:    p.EstimateValidDays,
	}
}

// ToCustomerResponse returns a view-object according to database model
func (c *Customer) ToCustomerResponse() *CustomerResponse {
	return &CustomerResponse{
		Id:            utils.Int64ToString(c.CustomerId),
		Name:          c.Name,
		ContactName:   c.ContactName,
		AddressLines:  c.AddressLines,
		Email:         c.Email,
		Phone:         c.Phone,
		Website:       c.Website,
		AccountNumber: c.AccountNumber,
		Pan:           c.Pan,
		TermsDays:     c.TermsDays,
		Notes:         c.Notes,
		Gstin:         c.Gstin,
		TaxType:       c.TaxType,
		TaxRate:       c.TaxRate,
	}
}

// ToInvoicePaymentResponse returns a view-object of a transaction used as an invoice payment
func ToInvoicePaymentResponse(p *InvoicePaymentInfo) *InvoicePaymentResponse {
	t := p.Transaction

	return &InvoicePaymentResponse{
		TransactionId:   utils.Int64ToString(t.TransactionId),
		Amount:          t.Amount,
		TransactionTime: utils.GetUnixTimeFromTransactionTime(t.TransactionTime),
		AccountId:       utils.Int64ToString(t.AccountId),
		Comment:         t.Comment,
		Currency:        p.Currency,
		SettledAmount:   p.SettledAmount,
	}
}

// ToInvoiceInfoResponse returns a view-object according to database model; lines and payments may be nil (list view)
func (i *Invoice) ToInvoiceInfoResponse(lines []*InvoiceLine, payments []*InvoicePaymentInfo) *InvoiceInfoResponse {
	resp := &InvoiceInfoResponse{
		Id:                 utils.Int64ToString(i.InvoiceId),
		Kind:               i.Kind,
		Number:             i.Number,
		CustomerId:         utils.Int64ToString(i.CustomerId),
		BillToName:         i.BillToName,
		BillToContactName:  i.BillToContactName,
		BillToAddressLines: i.BillToAddressLines,
		BillToEmail:        i.BillToEmail,
		BillToPhone:        i.BillToPhone,
		BillToPan:          i.BillToPan,
		BillToGstin:        i.BillToGstin,
		Currency:           i.Currency,
		InvoiceDate:        i.InvoiceDate,
		DueDate:            i.DueDate,
		Notes:              i.Notes,
		Total:              i.Total,
		TaxType:            i.TaxType,
		TaxRate:            i.TaxRate,
		Subtotal:           i.Subtotal,
		TaxAmount:          i.TaxAmount,
		ClosedManually:     i.ClosedManually,
		SentUnixTime:       i.SentUnixTime,
		Received:           InvoiceReceivedAmount(payments),
		EstimateStatus:     i.EstimateStatus,
		RespondedUnixTime:  i.RespondedUnixTime,
		RespondedByClient:  i.RespondedByClient,
		ResponseNote:       i.ResponseNote,
	}

	if i.EstimateId > 0 {
		resp.EstimateId = utils.Int64ToString(i.EstimateId)
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
				Sac:         line.Sac,
			}

			if line.ProductId > 0 {
				resp.Lines[idx].ProductId = utils.Int64ToString(line.ProductId)
			}
		}

		resp.Payments = make([]*InvoicePaymentResponse, len(payments))

		for idx, payment := range payments {
			resp.Payments[idx] = ToInvoicePaymentResponse(payment)
		}

		resp.PublicToken = i.PublicToken // detail view only; the list never needs the links
	}

	return resp
}

// ToPublicInvoiceResponse returns the view-object served to a recipient of the public link
func (i *Invoice) ToPublicInvoiceResponse(lines []*InvoiceLine, received int64, profile *InvoiceProfile) *PublicInvoiceResponse {
	logoDataUrl := profile.LogoDataUrl
	if profile.HideLogo {
		logoDataUrl = ""
	}
	resp := &PublicInvoiceResponse{
		Number:             i.Number,
		BillToName:         i.BillToName,
		BillToContactName:  i.BillToContactName,
		BillToAddressLines: i.BillToAddressLines,
		BillToEmail:        i.BillToEmail,
		BillToPhone:        i.BillToPhone,
		BillToPan:          i.BillToPan,
		BillToGstin:        i.BillToGstin,
		Currency:           i.Currency,
		InvoiceDate:        i.InvoiceDate,
		DueDate:            i.DueDate,
		Notes:              i.Notes,
		TaxType:            i.TaxType,
		TaxRate:            i.TaxRate,
		Subtotal:           i.Subtotal,
		TaxAmount:          i.TaxAmount,
		Total:              i.Total,
		Received:           received,
		ClosedManually:     i.ClosedManually,
		Lines:              make([]*InvoiceLineResponse, len(lines)),
		Profile: &PublicInvoiceProfileResponse{
			BusinessName: profile.BusinessName,
			AddressLines: profile.AddressLines,
			Phone:        profile.Phone,
			Website:      profile.Website,
			LogoDataUrl:  logoDataUrl,
			Gstin:        profile.Gstin,
		},
	}

	for idx, line := range lines {
		resp.Lines[idx] = &InvoiceLineResponse{
			ItemName:    line.ItemName,
			Description: line.Description,
			Sac:         line.Sac,
			Quantity:    line.Quantity,
			Rate:        line.Rate,
			Amount:      line.Amount,
		}
	}

	return resp
}

// SetEstimateProgress fills an estimate's milestones and the invoices made from it; nil (an invoice) changes nothing
func (r *InvoiceInfoResponse) SetEstimateProgress(progress *EstimateProgress) {
	if progress == nil {
		return
	}

	numbers := make(map[int64]string, len(progress.Invoices))
	r.ConvertedInvoices = make([]*EstimateConvertedInvoiceResponse, len(progress.Invoices))

	for idx, invoice := range progress.Invoices {
		numbers[invoice.InvoiceId] = invoice.Number
		r.ConvertedInvoices[idx] = &EstimateConvertedInvoiceResponse{Id: utils.Int64ToString(invoice.InvoiceId), Number: invoice.Number}
	}

	r.Milestones = make([]*EstimateMilestoneResponse, len(progress.Milestones))

	for idx, milestone := range progress.Milestones {
		r.Milestones[idx] = &EstimateMilestoneResponse{Id: utils.Int64ToString(milestone.MilestoneId), Label: milestone.Label, Percent: milestone.Percent}

		if milestone.InvoiceId > 0 {
			r.Milestones[idx].InvoiceId = utils.Int64ToString(milestone.InvoiceId)
			r.Milestones[idx].InvoiceNumber = numbers[milestone.InvoiceId]
		}
	}

	r.FullyInvoiced = progress.FullyInvoiced
}
