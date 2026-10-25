# Invoicing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create customer invoices in ezBookkeeping, print them to PDF in the Wave layout, and track each one until it is paid by linking existing income transactions.

**Architecture:** Five new tables (profile, customer, invoice, invoice line, invoice payment) behind a new `InvoiceService` and `InvoicesApi`, following the existing buckets pattern (`pkg/models/bucket.go`, `pkg/services/buckets.go`, `pkg/api/buckets.go`). The desktop frontend gets a thin store, pure helpers in `src/lib/invoice.ts` (status, formatting), a list page, an edit dialog, a view page that renders a print-styled `InvoiceDocument` component, and an invoice profile settings page. Budgets, Buckets and Invoices move into the desktop top header.

**Tech Stack:** Go 1.x + gin + xorm (sqlite/mysql/postgres), Vue 3 + Vuetify 3 + Pinia + TypeScript, vitest, testify.

**Spec:** `docs/superpowers/specs/2026-10-25-invoicing-design.md`

## Global Constraints

- INR only, no GST. `Invoice.Currency` is always `"INR"` and `Invoice.Kind` is always `"invoice"` in this phase.
- Amounts are `int64` paise (₹1 = 100), same as `Transaction.Amount`.
- Every table and query is scoped by `uid`; a user can never read or link another user's data.
- Invoicing never creates or modifies transactions.
- Matching is suggested, never automatic.
- Paid tolerance is `0.03` (3%): an invoice is Paid when `closedManually` or `total - received <= total * 0.03`.
- Payment suggestions: income transactions, same currency account, not linked, not deleted, dated from invoice date to due date + 60 days, amount ≤ balance due; ranked exact, then `[97%, 100%)`, then smaller; top 10.
- A transaction links to at most one invoice (DB unique constraint on `invoice_payment.transaction_id`).
- PDF is produced by the browser's print dialog; no PDF library. No new npm or Go dependencies at all.
- Invoice number format suggestion: `{NumberPrefix}/{FY}/{seq:02}`, FY April–March, e.g. `CA/2026-27/01`.
- Desktop UI only. No mobile pages.
- Commit with `git commit --no-verify` (repo convention; the pre-commit hook is slow and unrelated).
- UI strings go through `tt('...')` and every new string is added to `src/locales/en.json` (top-level flat keys); every new backend error message is added under `"error"` in `en.json`.

## Review Focus

1. **Short payment exactly at the 3% boundary** (₹3,70,000 invoice, ₹3,58,900 received = exactly 3% short) must show Paid with ₹11,100 bank charges, while ₹3,58,899 must show Partially paid. Pinned in Task 6 vitest.
2. **A linked transaction is later deleted** in the transactions page: the invoice's received amount must drop and the invoice must go back to Unpaid/Partially paid, not error. Pinned in Task 4 Go test `TestInvoiceReceivedIgnoresDeletedTransaction`.
3. **Linking another user's transaction ID** (crafted request) must be refused as not found, never linked. Pinned in Task 4 Go test `TestLinkPaymentRejectsOtherUsersTransaction`.
4. **Invoice dated in March vs April** must get the right financial year (`2025-26` vs `2026-27`), and a gap in numbering (01, 03) must suggest 04. Pinned in Task 3 Go tests.
5. **Editing an invoice down below what was already received** (₹3,70,000 invoice fully paid, edited to ₹3,00,000) must still load and show Paid with no negative bank charges. Pinned in Task 6 vitest (`balance due never negative`).

---

## File map

Backend (create):
- `pkg/models/invoice.go`: table structs, request/response structs, `ToInfoResponse` converters, `InvoicePaidTolerance`.
- `pkg/errs/invoice.go`: error values.
- `pkg/services/invoices.go`: `InvoiceService` (profile, customers, invoices, numbering, payments).
- `pkg/services/invoices_test.go`: service tests (uses the shared sqlite helper in `alerts_testutil_test.go`).
- `pkg/api/invoices.go`: `InvoicesApi` handlers.

Backend (modify):
- `pkg/errs/error.go`: add `NormalSubcategoryInvoice = 23`.
- `pkg/uuid/uuid_type.go`: add `UUID_TYPE_INVOICE UuidType = 15`.
- `cmd/database.go`: `SyncStructs` for the five tables.
- `cmd/webserver.go`: routes.
- `pkg/services/alerts_testutil_test.go`: sync invoice tables in the shared test DB.

Frontend (create):
- `src/models/invoice.ts`: types.
- `src/lib/invoice.ts` + `src/lib/__tests__/invoice.test.ts`: pure helpers and tests.
- `src/stores/invoice.ts`: API wrappers.
- `src/views/desktop/common/NoNavbarPageLayout.vue`: route wrapper for header-only pages.
- `src/views/desktop/invoices/ListPage.vue`
- `src/views/desktop/invoices/ViewPage.vue`
- `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`
- `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`
- `src/components/desktop/InvoiceDocument.vue`
- `src/views/desktop/settings/InvoiceProfileSettingsPage.vue`

Frontend (modify):
- `src/lib/services.ts`: API calls.
- `src/router/desktop.ts`: routes.
- `src/components/desktop/MainPageLayout.vue`: three header buttons.
- `src/views/desktop/settings/SettingsPageLayout.vue`: remove Planning section, add Invoice Profile item.
- `src/locales/en.json`: strings and errors.

---

### Task 1: Backend models, errors, UUID type, table registration

**Files:**
- Create: `pkg/models/invoice.go`, `pkg/errs/invoice.go`
- Modify: `pkg/errs/error.go:50` (after `NormalSubcategoryBucket = 22`), `pkg/uuid/uuid_type.go:22`, `cmd/database.go:232-238` (after the AlertMessage block)

**Interfaces:**
- Produces: `models.InvoiceProfile`, `models.Customer`, `models.Invoice`, `models.InvoiceLine`, `models.InvoicePayment`, all request/response structs below, `models.InvoicePaidTolerance`, `models.INVOICE_KIND_INVOICE`, `models.INVOICE_DEFAULT_CURRENCY`, `uuid.UUID_TYPE_INVOICE`, all `errs.ErrInvoice*` / `errs.ErrCustomer*` values.

- [ ] **Step 1: Add the error subcategory and UUID type**

In `pkg/errs/error.go`, after `NormalSubcategoryBucket = 22` add:

```go
	NormalSubcategoryInvoice                = 23
```

In `pkg/uuid/uuid_type.go`, after `UUID_TYPE_ALERT_MESSAGE UuidType = 14` add:

```go
	// UUID_TYPE_INVOICE is shared by customers, invoices, invoice lines and invoice payments:
	// the type field is only 4 bits wide and 15 is the last free value. Sequence numbers are
	// kept per type, so ids generated for different invoice tables never collide.
	UUID_TYPE_INVOICE UuidType = 15
```

- [ ] **Step 2: Create `pkg/errs/invoice.go`**

```go
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
)
```

- [ ] **Step 3: Create `pkg/models/invoice.go`**

```go
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
	LogoDataUrl      string `xorm:"TEXT"`
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
```

Note: payments are only expanded in the detail view (when `lines != nil`); the list view gets `received` only.

- [ ] **Step 4: Register the tables in `cmd/database.go`**

After the AlertMessage block (just before `return nil` in `updateAllDatabaseTablesStructure`):

```go
	for _, invoiceTable := range []any{new(models.InvoiceProfile), new(models.Customer), new(models.Invoice), new(models.InvoiceLine), new(models.InvoicePayment)} {
		err = datastore.Container.UserDataStore.SyncStructs(invoiceTable)

		if err != nil {
			return err
		}
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] invoice tables maintained successfully")
```

- [ ] **Step 5: Build**

Run: `go build ./... && go vet ./pkg/models ./pkg/errs ./pkg/uuid ./cmd`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add pkg/models/invoice.go pkg/errs/invoice.go pkg/errs/error.go pkg/uuid/uuid_type.go cmd/database.go
git commit --no-verify -m "invoices: models, errors and table registration"
```

---

### Task 2: Service: profile and customers

**Files:**
- Create: `pkg/services/invoices.go`, `pkg/services/invoices_test.go`
- Modify: `pkg/services/alerts_testutil_test.go` (the `for _, m := range []any{...}` list in `newAlertTestUser`)

**Interfaces:**
- Consumes: Task 1 models and errors.
- Produces (on `services.Invoices`, type `*InvoiceService`):
  - `GetProfile(c core.Context, uid int64) (*models.InvoiceProfile, error)`: returns defaults (`NumberPrefix: "INV"`, `DefaultTermsDays: 7`) when the user has no row.
  - `SaveProfile(c core.Context, profile *models.InvoiceProfile) error`: upsert by `Uid`.
  - `GetAllCustomers(c core.Context, uid int64) ([]*models.Customer, error)`: ordered by name.
  - `GetCustomer(c core.Context, uid int64, customerId int64) (*models.Customer, error)`
  - `CreateCustomer(c core.Context, customer *models.Customer) error`
  - `ModifyCustomer(c core.Context, customer *models.Customer) error`
  - `DeleteCustomer(c core.Context, uid int64, customerId int64) error`: `ErrCustomerInUse` if any non-deleted invoice uses it.

- [ ] **Step 1: Sync invoice tables in the shared test DB**

In `pkg/services/alerts_testutil_test.go`, extend the list inside `newAlertTestUser`:

```go
		for _, m := range []any{new(models.User), new(models.Account), new(models.Transaction), new(models.TransactionCategory), new(models.TransactionTag),
			new(models.TransactionTagIndex), new(models.AlertMessage), new(models.TransactionPictureInfo),
			new(models.InvoiceProfile), new(models.Customer), new(models.Invoice), new(models.InvoiceLine), new(models.InvoicePayment)} {
```

- [ ] **Step 2: Write the failing tests** in `pkg/services/invoices_test.go`

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestInvoiceProfileDefaultsThenSave(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "INV", profile.NumberPrefix)
	assert.Equal(t, int32(7), profile.DefaultTermsDays)

	profile.BusinessName = "Rishabh Jain"
	profile.NumberPrefix = "CA"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	profile.BusinessName = "Rishabh Jain (Codeation)"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	saved, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "Rishabh Jain (Codeation)", saved.BusinessName)
	assert.Equal(t, "CA", saved.NumberPrefix)
}

func TestCustomerCrudIsScopedToUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)

	customer := &models.Customer{Uid: uid, Name: "Quber Tech", ContactName: "Kanika Batra"}
	assert.Nil(t, Invoices.CreateCustomer(ctx, customer))
	assert.True(t, customer.CustomerId > 0)

	_, err := Invoices.GetCustomer(ctx, otherUid, customer.CustomerId)
	assert.Equal(t, errs.ErrCustomerNotFound, err)

	customer.Email = "kanika.batra@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	all, err := Invoices.GetAllCustomers(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))
	assert.Equal(t, "kanika.batra@qubertech.com", all[0].Email)

	assert.Nil(t, Invoices.DeleteCustomer(ctx, uid, customer.CustomerId))
	all, err = Invoices.GetAllCustomers(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))
}
```

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./pkg/services -run 'TestInvoiceProfile|TestCustomerCrud' -count=1`
Expected: FAIL to compile, `undefined: Invoices`.

- [ ] **Step 4: Create `pkg/services/invoices.go`** with the service and the profile/customer methods

```go
package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// InvoiceService represents invoice service
type InvoiceService struct {
	ServiceUsingDB
	ServiceUsingUuid
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
	}
)

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
		return &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", DefaultTermsDays: 7}, nil
	}

	return profile, nil
}

// SaveProfile creates or updates the invoice profile of user
func (s *InvoiceService) SaveProfile(c core.Context, profile *models.InvoiceProfile) error {
	if profile.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()
	profile.UpdatedUnixTime = now

	return s.UserDataDB(profile.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		exists, err := sess.Where("uid=?", profile.Uid).Exist(&models.InvoiceProfile{})

		if err != nil {
			return err
		}

		if exists {
			_, err = sess.Where("uid=?", profile.Uid).AllCols().Omit("created_unix_time").Update(profile)
			return err
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
		updatedRows, err := sess.Cols("name", "contact_name", "address_lines", "email", "updated_unix_time").
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
```

- [ ] **Step 5: Run the tests**

Run: `go test ./pkg/services -run 'TestInvoiceProfile|TestCustomerCrud' -count=1 -v`
Expected: PASS for both.

- [ ] **Step 6: Commit**

```bash
git add pkg/services/invoices.go pkg/services/invoices_test.go pkg/services/alerts_testutil_test.go
git commit --no-verify -m "invoices: profile and customer service"
```

---

### Task 3: Service: invoices CRUD, totals and numbering

**Files:**
- Modify: `pkg/services/invoices.go`, `pkg/services/invoices_test.go`

**Interfaces:**
- Consumes: Task 2 `GetCustomer`, `GetProfile`.
- Produces:
  - `GetAllInvoices(c core.Context, uid int64) ([]*models.Invoice, error)`: newest invoice date first.
  - `GetInvoice(c core.Context, uid int64, invoiceId int64) (*models.Invoice, []*models.InvoiceLine, error)`
  - `CreateInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error`: sets id, kind, currency, bill-to snapshot from the customer, line amounts, total.
  - `ModifyInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error`: replaces all lines; keeps `ClosedManually`, kind and currency.
  - `DeleteInvoice(c core.Context, uid int64, invoiceId int64) error`: soft-deletes and hard-deletes its payment links.
  - `SetInvoiceClosed(c core.Context, uid int64, invoiceId int64, closed bool) error`
  - `NextInvoiceNumber(c core.Context, uid int64, invoiceDate string) (string, error)`
  - unexported pure helpers `computeInvoiceTotal(lines []*models.InvoiceLine) int64` (also fills `Amount`), `invoiceFinancialYear(date string) (string, error)`, `nextInvoiceSequence(numbers []string, base string) int`.

- [ ] **Step 1: Write the failing tests** (append to `pkg/services/invoices_test.go`)

```go
func TestComputeInvoiceTotalFillsLineAmounts(t *testing.T) {
	lines := []*models.InvoiceLine{{Quantity: 1, Rate: 37000000}, {Quantity: 3, Rate: 150000}}
	assert.Equal(t, int64(37450000), computeInvoiceTotal(lines))
	assert.Equal(t, int64(450000), lines[1].Amount)
}

func TestInvoiceFinancialYear(t *testing.T) {
	fy, err := invoiceFinancialYear("2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "2026-27", fy)

	fy, err = invoiceFinancialYear("2026-03-31")
	assert.Nil(t, err)
	assert.Equal(t, "2025-26", fy)

	fy, err = invoiceFinancialYear("2099-12-01")
	assert.Nil(t, err)
	assert.Equal(t, "2099-00", fy)

	_, err = invoiceFinancialYear("23/04/2026")
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)
}

func TestNextInvoiceSequenceSkipsGapsAndOtherSeries(t *testing.T) {
	numbers := []string{"CA/2026-27/01", "CA/2026-27/03", "CA/2025-26/09", "XX/2026-27/07", "CA/2026-27/draft"}
	assert.Equal(t, 4, nextInvoiceSequence(numbers, "CA/2026-27/"))
	assert.Equal(t, 1, nextInvoiceSequence(nil, "CA/2026-27/"))
}
```

Add these helpers to the same file:

```go
func createTestInvoice(t *testing.T, uid int64, customerId int64, number string, rate int64) *models.Invoice {
	t.Helper()
	invoice := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	lines := []*models.InvoiceLine{{ItemName: "Consulting", Description: "Software Consulting\nQuber Tech", Quantity: 1, Rate: rate}}
	assert.Nil(t, Invoices.CreateInvoice(core.NewNullContext(), invoice, lines))
	return invoice
}

func createTestCustomer(t *testing.T, uid int64) *models.Customer {
	t.Helper()
	customer := &models.Customer{Uid: uid, Name: "Quber Tech", ContactName: "Kanika Batra", AddressLines: "167-169 Great Portland Street\nLondon, W1W 5PF\nUnited Kingdom"}
	assert.Nil(t, Invoices.CreateCustomer(core.NewNullContext(), customer))
	return customer
}

func TestCreateInvoiceSnapshotsCustomerAndRejectsDuplicateNumber(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	assert.Equal(t, int64(37000000), invoice.Total)
	assert.Equal(t, models.INVOICE_DEFAULT_CURRENCY, invoice.Currency)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, invoice.Kind)

	customer.AddressLines = "New address"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	saved, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "167-169 Great Portland Street\nLondon, W1W 5PF\nUnited Kingdom", saved.BillToAddressLines)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, int64(37000000), lines[0].Amount)

	duplicate := &models.Invoice{Uid: uid, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-05-01", DueDate: "2026-05-08"}
	err = Invoices.CreateInvoice(ctx, duplicate, []*models.InvoiceLine{{ItemName: "x", Quantity: 1, Rate: 100}})
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, err)
}

func TestCreateInvoiceValidatesDatesAndCustomer(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	otherCustomer := createTestCustomer(t, otherUid)
	lines := []*models.InvoiceLine{{ItemName: "x", Quantity: 1, Rate: 100}}

	err := Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "A", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-30", DueDate: "2026-04-23"}, lines)
	assert.Equal(t, errs.ErrInvoiceDueDateBeforeInvoiceDate, err)

	err = Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "B", CustomerId: customer.CustomerId, InvoiceDate: "2026-02-30", DueDate: "2026-04-23"}, lines)
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)

	err = Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "C", CustomerId: otherCustomer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}, lines)
	assert.Equal(t, errs.ErrCustomerNotFound, err)
}

func TestModifyInvoiceReplacesLinesAndKeepsClosedFlag(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	assert.Nil(t, Invoices.SetInvoiceClosed(ctx, uid, invoice.InvoiceId, true))

	modified := &models.Invoice{Uid: uid, InvoiceId: invoice.InvoiceId, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-05-07"}
	newLines := []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 1, Rate: 30000000}, {ItemName: "Support", Quantity: 2, Rate: 100000}}
	assert.Nil(t, Invoices.ModifyInvoice(ctx, modified, newLines))

	saved, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "Support", lines[1].ItemName)
	assert.Equal(t, int64(30200000), saved.Total)
	assert.Equal(t, "2026-05-07", saved.DueDate)
	assert.True(t, saved.ClosedManually)
}

func TestNextInvoiceNumberUsesProfilePrefixAndFinancialYear(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile, _ := Invoices.GetProfile(ctx, uid)
	profile.NumberPrefix = "CA"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	next, err := Invoices.NextInvoiceNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2026-27/01", next)

	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/03", 100)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2026-06-01")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2026-27/04", next)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2027-04-01")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2027-28/01", next)
}

func TestDeleteInvoiceHidesIt(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)

	assert.Equal(t, errs.ErrCustomerInUse, Invoices.DeleteCustomer(ctx, uid, customer.CustomerId))
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))

	_, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))

	// a deleted invoice frees its number
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)
}
```

Add `"github.com/mayswind/ezbookkeeping/pkg/core"` to the test file imports.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./pkg/services -run 'Invoice' -count=1`
Expected: FAIL to compile, `undefined: computeInvoiceTotal` (and the other new names).

- [ ] **Step 3: Implement** (append to `pkg/services/invoices.go`; add imports `"fmt"`, `"strconv"`, `"strings"`)

```go
const invoiceDateLayout = "2006-01-02"

// computeInvoiceTotal fills each line's amount (quantity × rate) and returns the invoice total.
// This is the single place totals are computed; GST would add tax lines here.
func computeInvoiceTotal(lines []*models.InvoiceLine) int64 {
	total := int64(0)

	for _, line := range lines {
		line.Amount = line.Quantity * line.Rate
		total += line.Amount
	}

	return total
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

// GetAllInvoices returns all invoices of user, newest invoice date first
func (s *InvoiceService) GetAllInvoices(c core.Context, uid int64) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("invoice_date desc, number desc").Find(&invoices)

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

	invoice := &models.Invoice{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).Get(invoice)

	if err != nil {
		return nil, nil, err
	} else if !has {
		return nil, nil, errs.ErrInvoiceNotFound
	}

	var lines []*models.InvoiceLine
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND invoice_id=?", uid, invoiceId).OrderBy("display_order asc").Find(&lines)

	if err != nil {
		return nil, nil, err
	}

	return invoice, lines, nil
}

// prepareInvoice validates an invoice, copies the customer's bill-to details and computes the total
func (s *InvoiceService) prepareInvoice(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine) error {
	if err := validateInvoiceDates(invoice); err != nil {
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
	invoice.Total = computeInvoiceTotal(lines)

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

// checkInvoiceNumberUnique returns ErrInvoiceNumberAlreadyExists if another non-deleted invoice uses the number
func checkInvoiceNumberUnique(sess *xorm.Session, invoice *models.Invoice) error {
	exists, err := sess.Where("uid=? AND deleted=? AND number=? AND invoice_id<>?", invoice.Uid, false, invoice.Number, invoice.InvoiceId).Exist(&models.Invoice{})

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

	invoice.Kind = models.INVOICE_KIND_INVOICE
	invoice.Currency = models.INVOICE_DEFAULT_CURRENCY
	invoice.Deleted = false
	invoice.ClosedManually = false
	invoice.CreatedUnixTime = time.Now().Unix()
	invoice.UpdatedUnixTime = invoice.CreatedUnixTime

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := checkInvoiceNumberUnique(sess, invoice); err != nil {
			return err
		}

		if _, err := sess.Insert(invoice); err != nil {
			return err
		}

		_, err := sess.Insert(lines)
		return err
	})
}

// ModifyInvoice saves an existing invoice and replaces all of its lines
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

	invoice.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := checkInvoiceNumberUnique(sess, invoice); err != nil {
			return err
		}

		updatedRows, err := sess.Cols("number", "customer_id", "bill_to_name", "bill_to_contact_name", "bill_to_address_lines", "bill_to_email",
			"invoice_date", "due_date", "notes", "total", "updated_unix_time").
			Where("uid=? AND deleted=? AND invoice_id=?", invoice.Uid, false, invoice.InvoiceId).Update(invoice)

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

// DeleteInvoice soft-deletes an invoice and removes its payment links; the linked transactions are untouched
func (s *InvoiceService) DeleteInvoice(c core.Context, uid int64, invoiceId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if invoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).
			Update(&models.Invoice{Deleted: true, DeletedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
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
		updatedRows, err := sess.Cols("closed_manually", "updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).
			Update(&models.Invoice{ClosedManually: closed, UpdatedUnixTime: time.Now().Unix()})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrInvoiceNotFound
		}

		return nil
	})
}

// NextInvoiceNumber suggests "{prefix}/{financial year}/{sequence}" for an invoice dated invoiceDate
func (s *InvoiceService) NextInvoiceNumber(c core.Context, uid int64, invoiceDate string) (string, error) {
	financialYear, err := invoiceFinancialYear(invoiceDate)

	if err != nil {
		return "", err
	}

	profile, err := s.GetProfile(c, uid)

	if err != nil {
		return "", err
	}

	base := profile.NumberPrefix + "/" + financialYear + "/"

	var numbers []string
	err = s.UserDataDB(uid).NewSession(c).Table(&models.Invoice{}).Where("uid=? AND deleted=? AND number LIKE ?", uid, false, base+"%").Cols("number").Find(&numbers)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%02d", base, nextInvoiceSequence(numbers, base)), nil
}
```

Note on xorm: `Update` with a struct skips zero values unless the column is listed in `Cols(...)`; every update above lists its columns explicitly, which is what makes `ClosedManually: false` and `Deleted: true` work.

- [ ] **Step 4: Run the tests**

Run: `go test ./pkg/services -run 'Invoice|Customer' -count=1 -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/services/invoices.go pkg/services/invoices_test.go
git commit --no-verify -m "invoices: invoice CRUD, totals and numbering"
```

---

### Task 4: Service: payments (received, suggestions, link, unlink)

**Files:**
- Modify: `pkg/services/invoices.go`, `pkg/services/invoices_test.go`

**Interfaces:**
- Consumes: Task 3 `GetInvoice`; existing `Accounts.GetAccountsByAccountIds(c, uid, ids) (map[int64]*models.Account, error)`.
- Produces:
  - `GetInvoicePayments(c core.Context, uid int64, invoiceIds []int64) (map[int64][]*models.Transaction, error)`: linked, non-deleted transactions per invoice, oldest first.
  - `GetPaymentSuggestions(c core.Context, uid int64, invoiceId int64) ([]*models.Transaction, error)`
  - `LinkPayment(c core.Context, uid int64, invoiceId int64, transactionId int64) error`
  - `UnlinkPayment(c core.Context, uid int64, invoiceId int64, transactionId int64) error`
  - unexported `rankInvoicePaymentSuggestions(transactions []*models.Transaction, balanceDue int64) []*models.Transaction`

- [ ] **Step 1: Write the failing tests** (append; add imports `"time"`, `"github.com/mayswind/ezbookkeeping/pkg/datastore"`, `"github.com/mayswind/ezbookkeeping/pkg/utils"`, `"github.com/mayswind/ezbookkeeping/pkg/uuid"`)

```go
// insertTestTransaction writes a transaction row directly (no category or balance bookkeeping needed for invoice tests)
func insertTestTransaction(t *testing.T, uid int64, accountId int64, typ models.TransactionDbType, amount int64, date string) *models.Transaction {
	t.Helper()
	at, err := time.Parse("2006-01-02", date)
	assert.Nil(t, err)
	transaction := &models.Transaction{
		TransactionId: uuid.Container.GenerateUuid(uuid.UUID_TYPE_TRANSACTION), Uid: uid, Type: typ, AccountId: accountId,
		Amount: amount, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Add(12 * time.Hour).Unix()), TimezoneUtcOffset: 330,
	}
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(core.NewNullContext()).Insert(transaction)
	assert.Nil(t, err)
	return transaction
}

func TestRankInvoicePaymentSuggestions(t *testing.T) {
	small := &models.Transaction{TransactionId: 1, Amount: 10000000, TransactionTime: 3}
	short := &models.Transaction{TransactionId: 2, Amount: 36000000, TransactionTime: 2}
	exact := &models.Transaction{TransactionId: 3, Amount: 37000000, TransactionTime: 1}
	ranked := rankInvoicePaymentSuggestions([]*models.Transaction{small, short, exact}, 37000000)
	assert.Equal(t, []int64{3, 2, 1}, []int64{ranked[0].TransactionId, ranked[1].TransactionId, ranked[2].TransactionId})
}

func TestLinkPaymentsAccumulatesReceived(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	first := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 20000000, "2026-04-28")
	second := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 16850000, "2026-05-03")

	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(payments[invoice.InvoiceId]))
	assert.Equal(t, first.TransactionId, payments[invoice.InvoiceId][0].TransactionId)

	assert.Equal(t, errs.ErrInvoicePaymentAlreadyLinked, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId))

	assert.Nil(t, Invoices.UnlinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))
	assert.Equal(t, errs.ErrInvoicePaymentNotFound, Invoices.UnlinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))

	payments, err = Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(payments[invoice.InvoiceId]))
}

func TestLinkPaymentValidatesTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	expense := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_EXPENSE, 1000, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentTransactionInvalid, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, expense.TransactionId))

	tooBig := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000001, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentExceedsBalanceDue, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, tooBig.TransactionId))
}

func TestLinkPaymentRejectsOtherUsersTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	otherCtx, otherUid := newAlertTestUser(t, "Other Savings")
	otherAccountId := accountIdByName(t, otherCtx, otherUid, "Other Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	othersIncome := insertTestTransaction(t, otherUid, otherAccountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Equal(t, errs.ErrTransactionNotFound, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, othersIncome.TransactionId))
}

func TestInvoiceReceivedIgnoresDeletedTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, income.TransactionId))

	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).ID(income.TransactionId).Cols("deleted").Update(&models.Transaction{Deleted: true})
	assert.Nil(t, err)

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(payments[invoice.InvoiceId]))
}

func TestPaymentSuggestionsFilterAndRank(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	other := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/02", 100000)

	exact := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-05-02")
	short := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 36850000, "2026-04-29")
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-03-01")   // before invoice date
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-07-30")   // more than 60 days after due
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 40000000, "2026-05-02")   // more than balance due
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_EXPENSE, 37000000, "2026-05-02")  // not income
	linkedElsewhere := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 100000, "2026-05-02")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, other.InvoiceId, linkedElsewhere.TransactionId))

	suggestions, err := Invoices.GetPaymentSuggestions(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(suggestions))
	assert.Equal(t, exact.TransactionId, suggestions[0].TransactionId)
	assert.Equal(t, short.TransactionId, suggestions[1].TransactionId)
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./pkg/services -run 'Payment|Received' -count=1`
Expected: FAIL to compile, `undefined: rankInvoicePaymentSuggestions`.

- [ ] **Step 3: Implement** (append to `pkg/services/invoices.go`; add import `"sort"`)

```go
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
func (s *InvoiceService) GetInvoicePayments(c core.Context, uid int64, invoiceIds []int64) (map[int64][]*models.Transaction, error) {
	result := make(map[int64][]*models.Transaction, len(invoiceIds))

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(invoiceIds) < 1 {
		return result, nil
	}

	var links []*models.InvoicePayment
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", invoiceIds).Find(&links)

	if err != nil || len(links) < 1 {
		return result, err
	}

	invoiceIdByTransactionId := make(map[int64]int64, len(links))
	transactionIds := make([]int64, 0, len(links))

	for _, link := range links {
		invoiceIdByTransactionId[link.TransactionId] = link.InvoiceId
		transactionIds = append(transactionIds, link.TransactionId)
	}

	var transactions []*models.Transaction
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("transaction_id", transactionIds).OrderBy("transaction_time asc").Find(&transactions)

	if err != nil {
		return nil, err
	}

	for _, transaction := range transactions {
		invoiceId := invoiceIdByTransactionId[transaction.TransactionId]
		result[invoiceId] = append(result[invoiceId], transaction)
	}

	return result, nil
}

// invoiceBalanceDue returns total minus the amounts of the invoice's linked, non-deleted transactions
func (s *InvoiceService) invoiceBalanceDue(c core.Context, uid int64, invoice *models.Invoice) (int64, error) {
	payments, err := s.GetInvoicePayments(c, uid, []int64{invoice.InvoiceId})

	if err != nil {
		return 0, err
	}

	balanceDue := invoice.Total

	for _, payment := range payments[invoice.InvoiceId] {
		balanceDue -= payment.Amount
	}

	return balanceDue, nil
}

// GetPaymentSuggestions returns unlinked income transactions in the invoice currency that could pay the invoice
func (s *InvoiceService) GetPaymentSuggestions(c core.Context, uid int64, invoiceId int64) ([]*models.Transaction, error) {
	invoice, _, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	balanceDue, err := s.invoiceBalanceDue(c, uid, invoice)

	if err != nil {
		return nil, err
	}

	if balanceDue <= 0 {
		return []*models.Transaction{}, nil
	}

	invoiceDate, _ := time.Parse(invoiceDateLayout, invoice.InvoiceDate)
	dueDate, _ := time.Parse(invoiceDateLayout, invoice.DueDate)
	// ponytail: one day of padding on each side instead of per-user timezone math
	minTime := utils.GetMinTransactionTimeFromUnixTime(invoiceDate.Add(-24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(dueDate.Add(time.Duration(models.InvoicePaymentSuggestionDaysAfterDue+1) * 24 * time.Hour).Unix())

	var candidates []*models.Transaction
	err = s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND type=? AND transaction_time>=? AND transaction_time<=? AND amount>? AND amount<=?",
			uid, false, models.TRANSACTION_DB_TYPE_INCOME, minTime, maxTime, 0, balanceDue).
		Find(&candidates)

	if err != nil || len(candidates) < 1 {
		return []*models.Transaction{}, err
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

	suggestions := make([]*models.Transaction, 0, len(candidates))

	for _, candidate := range candidates {
		account := accounts[candidate.AccountId]

		if linkedIds[candidate.TransactionId] || account == nil || account.Currency != invoice.Currency {
			continue
		}

		suggestions = append(suggestions, candidate)
	}

	return rankInvoicePaymentSuggestions(suggestions, balanceDue), nil
}

// LinkPayment links an existing income transaction of user to an invoice
func (s *InvoiceService) LinkPayment(c core.Context, uid int64, invoiceId int64, transactionId int64) error {
	invoice, _, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return err
	}

	transaction, err := Transactions.GetTransactionByTransactionId(c, uid, transactionId)

	if err != nil {
		return err
	}

	accounts, err := Accounts.GetAccountsByAccountIds(c, uid, []int64{transaction.AccountId})

	if err != nil {
		return err
	}

	account := accounts[transaction.AccountId]

	if transaction.Type != models.TRANSACTION_DB_TYPE_INCOME || account == nil || account.Currency != invoice.Currency {
		return errs.ErrInvoicePaymentTransactionInvalid
	}

	balanceDue, err := s.invoiceBalanceDue(c, uid, invoice)

	if err != nil {
		return err
	}

	if transaction.Amount > balanceDue {
		return errs.ErrInvoicePaymentExceedsBalanceDue
	}

	payment := &models.InvoicePayment{
		PaymentId:      s.GenerateUuid(uuid.UUID_TYPE_INVOICE),
		InvoiceId:      invoiceId,
		Uid:            uid,
		TransactionId:  transactionId,
		LinkedUnixTime: time.Now().Unix(),
	}

	if payment.PaymentId < 1 {
		return errs.ErrSystemIsBusy
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		alreadyLinked, err := sess.Where("transaction_id=?", transactionId).Exist(&models.InvoicePayment{})

		if err != nil {
			return err
		} else if alreadyLinked {
			return errs.ErrInvoicePaymentAlreadyLinked
		}

		// the unique index on transaction_id is the backstop for two concurrent links
		_, err = sess.Insert(payment)
		return err
	})
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
```

Add `"github.com/mayswind/ezbookkeeping/pkg/utils"` to the service imports.

- [ ] **Step 4: Run all service tests**

Run: `go test ./pkg/services -count=1`
Expected: `ok`. (The existing alerts tests share the DB and must still pass.)

- [ ] **Step 5: Commit**

```bash
git add pkg/services/invoices.go pkg/services/invoices_test.go
git commit --no-verify -m "invoices: payment linking and suggestions"
```

---

### Task 5: API handlers and routes

**Files:**
- Create: `pkg/api/invoices.go`
- Modify: `cmd/webserver.go:550` (after the Buckets route block)

**Interfaces:**
- Consumes: all `services.Invoices` methods from Tasks 2–4.
- Produces HTTP endpoints (all under `/api/v1`, JSON responses in the standard `{success, result}` envelope):

| Method + path | Request | Result |
|---|---|---|
| `GET invoice_profile/get.json` | – | `InvoiceProfileResponse` |
| `POST invoice_profile/update.json` | `InvoiceProfileUpdateRequest` | `InvoiceProfileResponse` |
| `GET customers/list.json` | – | `[]CustomerResponse` |
| `POST customers/add.json` | `CustomerCreateRequest` | `CustomerResponse` |
| `POST customers/modify.json` | `CustomerModifyRequest` | `CustomerResponse` |
| `POST customers/delete.json` | `{id}` | `true` |
| `GET invoices/list.json` | – | `[]InvoiceInfoResponse` (no lines/payments) |
| `GET invoices/get.json?id=` | – | `InvoiceInfoResponse` with lines and payments |
| `POST invoices/add.json` | `InvoiceCreateRequest` | `InvoiceInfoResponse` |
| `POST invoices/modify.json` | `InvoiceModifyRequest` | `InvoiceInfoResponse` |
| `POST invoices/delete.json`, `close.json`, `reopen.json` | `{id}` | `true` |
| `GET invoices/next_number.json?invoice_date=` | – | `string` |
| `GET invoices/payment/suggestions.json?id=` | – | `[]InvoicePaymentResponse` |
| `POST invoices/payment/link.json`, `unlink.json` | `{invoiceId, transactionId}` | `true` |

- [ ] **Step 1: Create `pkg/api/invoices.go`**

```go
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

func invoiceLinesFromRequest(lineReqs []*models.InvoiceLineRequest) []*models.InvoiceLine {
	lines := make([]*models.InvoiceLine, len(lineReqs))

	for idx, lineReq := range lineReqs {
		lines[idx] = &models.InvoiceLine{
			ItemName:    strings.TrimSpace(lineReq.ItemName),
			Description: lineReq.Description,
			Quantity:    lineReq.Quantity,
			Rate:        lineReq.Rate,
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

	uid := c.GetCurrentUid()
	customer := &models.Customer{Uid: uid, Name: strings.TrimSpace(req.Name), ContactName: req.ContactName, AddressLines: req.AddressLines, Email: req.Email}

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

	uid := c.GetCurrentUid()
	customer := &models.Customer{CustomerId: req.Id, Uid: uid, Name: strings.TrimSpace(req.Name), ContactName: req.ContactName, AddressLines: req.AddressLines, Email: req.Email}

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

	uid := c.GetCurrentUid()
	invoice := &models.Invoice{Uid: uid, Number: strings.TrimSpace(req.Number), CustomerId: req.CustomerId, InvoiceDate: req.InvoiceDate, DueDate: req.DueDate, Notes: req.Notes}

	if err := a.invoices.CreateInvoice(c, invoice, invoiceLinesFromRequest(req.Lines)); err != nil {
		log.Errorf(c, "[invoices.InvoiceCreateHandler] failed to create invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp, err := a.getInvoiceResponse(c, uid, invoice.InvoiceId)

	if err != nil {
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

	uid := c.GetCurrentUid()
	invoice := &models.Invoice{InvoiceId: req.Id, Uid: uid, Number: strings.TrimSpace(req.Number), CustomerId: req.CustomerId, InvoiceDate: req.InvoiceDate, DueDate: req.DueDate, Notes: req.Notes}

	if err := a.invoices.ModifyInvoice(c, invoice, invoiceLinesFromRequest(req.Lines)); err != nil {
		log.Errorf(c, "[invoices.InvoiceModifyHandler] failed to modify invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp, err := a.getInvoiceResponse(c, uid, invoice.InvoiceId)

	if err != nil {
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
```

- [ ] **Step 2: Add the routes** in `cmd/webserver.go`, right after the `// Buckets` block:

```go
			// Invoices
			apiV1Route.GET("/invoice_profile/get.json", bindApi(api.Invoices.InvoiceProfileGetHandler, config))
			apiV1Route.POST("/invoice_profile/update.json", bindApi(api.Invoices.InvoiceProfileUpdateHandler, config))
			apiV1Route.GET("/customers/list.json", bindApi(api.Invoices.CustomerListHandler, config))
			apiV1Route.POST("/customers/add.json", bindApi(api.Invoices.CustomerCreateHandler, config))
			apiV1Route.POST("/customers/modify.json", bindApi(api.Invoices.CustomerModifyHandler, config))
			apiV1Route.POST("/customers/delete.json", bindApi(api.Invoices.CustomerDeleteHandler, config))
			apiV1Route.GET("/invoices/list.json", bindApi(api.Invoices.InvoiceListHandler, config))
			apiV1Route.GET("/invoices/get.json", bindApi(api.Invoices.InvoiceGetHandler, config))
			apiV1Route.POST("/invoices/add.json", bindApi(api.Invoices.InvoiceCreateHandler, config))
			apiV1Route.POST("/invoices/modify.json", bindApi(api.Invoices.InvoiceModifyHandler, config))
			apiV1Route.POST("/invoices/delete.json", bindApi(api.Invoices.InvoiceDeleteHandler, config))
			apiV1Route.POST("/invoices/close.json", bindApi(api.Invoices.InvoiceCloseHandler, config))
			apiV1Route.POST("/invoices/reopen.json", bindApi(api.Invoices.InvoiceReopenHandler, config))
			apiV1Route.GET("/invoices/next_number.json", bindApi(api.Invoices.InvoiceNextNumberHandler, config))
			apiV1Route.GET("/invoices/payment/suggestions.json", bindApi(api.Invoices.InvoicePaymentSuggestionsHandler, config))
			apiV1Route.POST("/invoices/payment/link.json", bindApi(api.Invoices.InvoicePaymentLinkHandler, config))
			apiV1Route.POST("/invoices/payment/unlink.json", bindApi(api.Invoices.InvoicePaymentUnlinkHandler, config))
```

- [ ] **Step 3: Unit-test the logo guard** (create `pkg/api/invoices_test.go`)

```go
package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedInvoiceLogo(t *testing.T) {
	assert.True(t, isAllowedInvoiceLogo(""))
	assert.True(t, isAllowedInvoiceLogo("data:image/png;base64,iVBORw0KGgo="))
	assert.True(t, isAllowedInvoiceLogo("data:image/jpeg;base64,/9j/4AAQ"))
	assert.False(t, isAllowedInvoiceLogo("data:image/svg+xml;base64,PHN2Zz4="))
	assert.False(t, isAllowedInvoiceLogo("https://example.com/logo.png"))
}
```

- [ ] **Step 4: Build, test and smoke-test**

Run: `go build ./... && go test ./pkg/api -run TestIsAllowedInvoiceLogo -count=1`
Expected: `ok`.

Smoke test against a local server (it migrates the new tables on start):

```bash
go run . server run &   # or the repo's usual dev command; wait for "will run at"
TOKEN=<a token from logging in to the local instance>
curl -s -H "Authorization: Bearer $TOKEN" 'http://127.0.0.1:8080/api/v1/invoices/next_number.json?invoice_date=2026-04-23'
```

Expected: `{"success":true,"result":"INV/2026-27/01"}`. Stop the server afterwards.

- [ ] **Step 5: Commit**

```bash
git add pkg/api/invoices.go pkg/api/invoices_test.go cmd/webserver.go
git commit --no-verify -m "invoices: API handlers and routes"
```

---

### Task 6: Frontend models, pure helpers, API calls, store

**Files:**
- Create: `src/models/invoice.ts`, `src/lib/invoice.ts`, `src/lib/__tests__/invoice.test.ts`, `src/stores/invoice.ts`
- Modify: `src/lib/services.ts` (imports near line 213; methods after `getAllBuckets` block near line 1015)

**Interfaces:**
- Consumes: Task 5 endpoints.
- Produces:
  - Types: `InvoiceProfile`, `Customer`, `CustomerSaveRequest`, `InvoiceLine`, `InvoicePayment`, `Invoice`, `InvoiceLineRequest`, `InvoiceSaveRequest`, `InvoiceStatus`.
  - Helpers in `src/lib/invoice.ts`: `INVOICE_PAID_TOLERANCE`, `computeInvoiceTotals(lines)`, `getInvoiceBalanceDue(invoice)`, `getInvoiceStatus(invoice)`, `isInvoiceOverdue(invoice, today)`, `getInvoiceBankCharges(invoice)`, `formatInvoiceAmount(paise)`, `formatInvoiceDate(date)`, `getInvoicePdfFileName(number, dueDate)`, `addDaysToDate(date, days)`, `getTodayDateString()`, `rupeesToPaise(value)`, `paiseToRupees(value)`.
  - Store `useInvoicesStore()` with methods named exactly like the service calls below.

- [ ] **Step 1: Create `src/models/invoice.ts`**

```ts
export interface InvoiceProfile {
    businessName: string;
    addressLines: string;
    phone: string;
    website: string;
    logoDataUrl: string;
    defaultNotes: string;
    numberPrefix: string;
    defaultTermsDays: number;
}

export interface Customer {
    id: string;
    name: string;
    contactName: string;
    addressLines: string;
    email: string;
}

export interface CustomerSaveRequest {
    id?: string;
    name: string;
    contactName: string;
    addressLines: string;
    email: string;
}

export interface InvoiceLine {
    itemName: string;
    description: string;
    quantity: number;
    rate: number;
    amount: number;
}

export interface InvoicePayment {
    transactionId: string;
    amount: number;
    transactionTime: number;
    accountId: string;
    comment: string;
}

export interface Invoice {
    id: string;
    kind: string;
    number: string;
    customerId: string;
    billToName: string;
    billToContactName: string;
    billToAddressLines: string;
    billToEmail: string;
    currency: string;
    invoiceDate: string;
    dueDate: string;
    notes: string;
    total: number;
    received: number;
    closedManually: boolean;
    lines?: InvoiceLine[];
    payments?: InvoicePayment[];
}

export interface InvoiceLineRequest {
    itemName: string;
    description: string;
    quantity: number;
    rate: number;
}

export interface InvoiceSaveRequest {
    id?: string;
    number: string;
    customerId: string;
    invoiceDate: string;
    dueDate: string;
    notes: string;
    lines: InvoiceLineRequest[];
}

export type InvoiceStatus = 'unpaid' | 'partial' | 'paid';
```

- [ ] **Step 2: Write the failing vitest** `src/lib/__tests__/invoice.test.ts`

```ts
import { describe, expect, test } from 'vitest';

import {
    computeInvoiceTotals,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    isInvoiceOverdue,
    getInvoiceBankCharges,
    formatInvoiceAmount,
    formatInvoiceDate,
    getInvoicePdfFileName,
    addDaysToDate,
    rupeesToPaise,
    paiseToRupees
} from '@/lib/invoice.ts';

const invoice = (total: number, received: number, closedManually = false, dueDate = '2026-04-30') =>
    ({ total, received, closedManually, dueDate });

describe('computeInvoiceTotals', () => {
    test('sums quantity × rate', () => {
        expect(computeInvoiceTotals([{ quantity: 1, rate: 37000000 }, { quantity: 3, rate: 150000 }])).toEqual({ subtotal: 37450000, total: 37450000 });
        expect(computeInvoiceTotals([])).toEqual({ subtotal: 0, total: 0 });
    });
});

describe('getInvoiceStatus', () => {
    test('unpaid with nothing received', () => {
        expect(getInvoiceStatus(invoice(37000000, 0))).toBe('unpaid');
    });

    test('paid when exactly 3% short', () => {
        expect(getInvoiceStatus(invoice(37000000, 35890000))).toBe('paid');
        expect(getInvoiceBankCharges(invoice(37000000, 35890000))).toBe(1110000);
    });

    test('partial when one paisa more than 3% short', () => {
        expect(getInvoiceStatus(invoice(37000000, 35889999))).toBe('partial');
        expect(getInvoiceBankCharges(invoice(37000000, 35889999))).toBe(0);
    });

    test('paid when closed manually even with a big shortfall', () => {
        expect(getInvoiceStatus(invoice(37000000, 10000000, true))).toBe('paid');
        expect(getInvoiceBankCharges(invoice(37000000, 10000000, true))).toBe(27000000);
    });

    test('balance due never negative after editing the total below what was received', () => {
        expect(getInvoiceBalanceDue(invoice(30000000, 37000000))).toBe(0);
        expect(getInvoiceStatus(invoice(30000000, 37000000))).toBe('paid');
        expect(getInvoiceBankCharges(invoice(30000000, 37000000))).toBe(0);
    });
});

describe('isInvoiceOverdue', () => {
    test('only unpaid or partial invoices past the due date', () => {
        expect(isInvoiceOverdue(invoice(100, 0), '2026-05-01')).toBe(true);
        expect(isInvoiceOverdue(invoice(100, 0), '2026-04-30')).toBe(false);
        expect(isInvoiceOverdue(invoice(100, 100), '2026-05-01')).toBe(false);
    });
});

describe('formatting', () => {
    test('amount like the Wave sample', () => {
        expect(formatInvoiceAmount(37000000)).toBe('₹370,000.00');
    });

    test('date like the Wave sample', () => {
        expect(formatInvoiceDate('2026-04-23')).toBe('April 23, 2026');
    });

    test('pdf file name like the Wave sample', () => {
        expect(getInvoicePdfFileName('CA/2026-27/01', '2026-04-30')).toBe('Invoice_CA_2026-27_01_2026-04-30');
    });

    test('add days across month end', () => {
        expect(addDaysToDate('2026-04-23', 7)).toBe('2026-04-30');
        expect(addDaysToDate('2026-04-28', 7)).toBe('2026-05-05');
    });

    test('rupees and paise round trip without float drift', () => {
        expect(rupeesToPaise(370000)).toBe(37000000);
        expect(rupeesToPaise(0.29)).toBe(29);
        expect(paiseToRupees(37000000)).toBe(370000);
    });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: FAIL, cannot resolve `@/lib/invoice.ts`.

- [ ] **Step 4: Create `src/lib/invoice.ts`**

```ts
import type { InvoiceStatus } from '@/models/invoice.ts';

// mirrors models.InvoicePaidTolerance in pkg/models/invoice.go
export const INVOICE_PAID_TOLERANCE = 0.03;

interface InvoiceAmounts {
    total: number;
    received: number;
    closedManually: boolean;
}

export function computeInvoiceTotals(lines: { quantity: number, rate: number }[]): { subtotal: number, total: number } {
    const subtotal = lines.reduce((sum, line) => sum + line.quantity * line.rate, 0);
    // GST would add tax lines between subtotal and total here
    return { subtotal, total: subtotal };
}

export function getInvoiceBalanceDue(invoice: InvoiceAmounts): number {
    return Math.max(invoice.total - invoice.received, 0);
}

export function getInvoiceStatus(invoice: InvoiceAmounts): InvoiceStatus {
    if (invoice.closedManually || getInvoiceBalanceDue(invoice) <= invoice.total * INVOICE_PAID_TOLERANCE) {
        return 'paid';
    }

    return invoice.received > 0 ? 'partial' : 'unpaid';
}

export function isInvoiceOverdue(invoice: InvoiceAmounts & { dueDate: string }, today: string): boolean {
    return getInvoiceStatus(invoice) !== 'paid' && invoice.dueDate < today;
}

// the shortfall written off on a paid invoice (bank / forex charges, or a manual close)
export function getInvoiceBankCharges(invoice: InvoiceAmounts): number {
    return getInvoiceStatus(invoice) === 'paid' ? getInvoiceBalanceDue(invoice) : 0;
}

const invoiceAmountFormat = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'INR' });

export function formatInvoiceAmount(paise: number): string {
    return invoiceAmountFormat.format(paise / 100);
}

export function formatInvoiceDate(date: string): string {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day!)).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' });
}

export function getInvoicePdfFileName(number: string, dueDate: string): string {
    return `Invoice_${number.replace(/[\\/:*?"<>|\s]+/g, '_')}_${dueDate}`;
}

export function addDaysToDate(date: string, days: number): string {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day! + days)).toISOString().substring(0, 10);
}

export function getTodayDateString(): string {
    const now = new Date();
    return new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())).toISOString().substring(0, 10);
}

export function rupeesToPaise(value: number): number {
    return Math.round(value * 100);
}

export function paiseToRupees(value: number): number {
    return value / 100;
}
```

- [ ] **Step 5: Run the vitest**

Run: `npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: all PASS.

- [ ] **Step 6: Add API calls to `src/lib/services.ts`**

Add to the imports block (next to the bucket imports):

```ts
import type {
    InvoiceProfile,
    Customer,
    CustomerSaveRequest,
    Invoice,
    InvoicePayment,
    InvoiceSaveRequest
} from '@/models/invoice.ts';
```

Add after the bucket methods (same object literal):

```ts
    getInvoiceProfile: (): ApiResponsePromise<InvoiceProfile> => {
        return axios.get<ApiResponse<InvoiceProfile>>('v1/invoice_profile/get.json');
    },
    updateInvoiceProfile: (req: InvoiceProfile): ApiResponsePromise<InvoiceProfile> => {
        return axios.post<ApiResponse<InvoiceProfile>>('v1/invoice_profile/update.json', req);
    },
    getAllCustomers: (): ApiResponsePromise<Customer[]> => {
        return axios.get<ApiResponse<Customer[]>>('v1/customers/list.json');
    },
    addCustomer: (req: CustomerSaveRequest): ApiResponsePromise<Customer> => {
        return axios.post<ApiResponse<Customer>>('v1/customers/add.json', req);
    },
    modifyCustomer: (req: CustomerSaveRequest): ApiResponsePromise<Customer> => {
        return axios.post<ApiResponse<Customer>>('v1/customers/modify.json', req);
    },
    deleteCustomer: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/customers/delete.json', { id });
    },
    getAllInvoices: (): ApiResponsePromise<Invoice[]> => {
        return axios.get<ApiResponse<Invoice[]>>('v1/invoices/list.json');
    },
    getInvoice: ({ id }: { id: string }): ApiResponsePromise<Invoice> => {
        return axios.get<ApiResponse<Invoice>>('v1/invoices/get.json?id=' + encodeURIComponent(id));
    },
    addInvoice: (req: InvoiceSaveRequest): ApiResponsePromise<Invoice> => {
        return axios.post<ApiResponse<Invoice>>('v1/invoices/add.json', req);
    },
    modifyInvoice: (req: InvoiceSaveRequest): ApiResponsePromise<Invoice> => {
        return axios.post<ApiResponse<Invoice>>('v1/invoices/modify.json', req);
    },
    deleteInvoice: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/delete.json', { id });
    },
    closeInvoice: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/close.json', { id });
    },
    reopenInvoice: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/reopen.json', { id });
    },
    getNextInvoiceNumber: ({ invoiceDate }: { invoiceDate: string }): ApiResponsePromise<string> => {
        return axios.get<ApiResponse<string>>('v1/invoices/next_number.json?invoice_date=' + encodeURIComponent(invoiceDate));
    },
    getInvoicePaymentSuggestions: ({ id }: { id: string }): ApiResponsePromise<InvoicePayment[]> => {
        return axios.get<ApiResponse<InvoicePayment[]>>('v1/invoices/payment/suggestions.json?id=' + encodeURIComponent(id));
    },
    linkInvoicePayment: (req: { invoiceId: string, transactionId: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/payment/link.json', req);
    },
    unlinkInvoicePayment: (req: { invoiceId: string, transactionId: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/payment/unlink.json', req);
    },
```

- [ ] **Step 7: Create `src/stores/invoice.ts`**

```ts
import { defineStore } from 'pinia';

import type {
    InvoiceProfile,
    Customer,
    CustomerSaveRequest,
    Invoice,
    InvoicePayment,
    InvoiceSaveRequest
} from '@/models/invoice.ts';

import services, { type ApiResponsePromise } from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

const FAILED_MESSAGE = 'Unable to process invoice request';

// unwraps the standard API envelope and normalises errors the same way the other stores do
function call<T>(request: ApiResponsePromise<T>): Promise<T> {
    return new Promise((resolve, reject) => {
        request.then(response => {
            const data = response.data;

            if (!data || !data.success || data.result === undefined || data.result === null) {
                reject({ message: FAILED_MESSAGE });
                return;
            }

            resolve(data.result);
        }).catch(error => {
            logger.error(FAILED_MESSAGE, error);

            if (error.response && error.response.data && error.response.data.errorMessage) {
                reject({ error: error.response.data });
            } else if (!error.processed) {
                reject({ message: FAILED_MESSAGE });
            } else {
                reject(error);
            }
        });
    });
}

// ponytail: no client-side cache; each page loads what it shows (a handful of invoices and customers)
export const useInvoicesStore = defineStore('invoices', () => {
    return {
        getInvoiceProfile: (): Promise<InvoiceProfile> => call(services.getInvoiceProfile()),
        updateInvoiceProfile: (profile: InvoiceProfile): Promise<InvoiceProfile> => call(services.updateInvoiceProfile(profile)),
        getAllCustomers: (): Promise<Customer[]> => call(services.getAllCustomers()),
        saveCustomer: (customer: CustomerSaveRequest): Promise<Customer> => call(customer.id ? services.modifyCustomer(customer) : services.addCustomer(customer)),
        deleteCustomer: (id: string): Promise<boolean> => call(services.deleteCustomer({ id })),
        getAllInvoices: (): Promise<Invoice[]> => call(services.getAllInvoices()),
        getInvoice: (id: string): Promise<Invoice> => call(services.getInvoice({ id })),
        saveInvoice: (invoice: InvoiceSaveRequest): Promise<Invoice> => call(invoice.id ? services.modifyInvoice(invoice) : services.addInvoice(invoice)),
        deleteInvoice: (id: string): Promise<boolean> => call(services.deleteInvoice({ id })),
        closeInvoice: (id: string): Promise<boolean> => call(services.closeInvoice({ id })),
        reopenInvoice: (id: string): Promise<boolean> => call(services.reopenInvoice({ id })),
        getNextInvoiceNumber: (invoiceDate: string): Promise<string> => call(services.getNextInvoiceNumber({ invoiceDate })),
        getInvoicePaymentSuggestions: (id: string): Promise<InvoicePayment[]> => call(services.getInvoicePaymentSuggestions({ id })),
        linkInvoicePayment: (invoiceId: string, transactionId: string): Promise<boolean> => call(services.linkInvoicePayment({ invoiceId, transactionId })),
        unlinkInvoicePayment: (invoiceId: string, transactionId: string): Promise<boolean> => call(services.unlinkInvoicePayment({ invoiceId, transactionId }))
    };
});
```

If `ApiResponsePromise` is not exported by name from the default-export module import, import it as `import services, { type ApiResponsePromise } from '@/lib/services.ts';` (it is declared `export type` at `src/lib/services.ts:252`).

- [ ] **Step 8: Type-check and test**

Run: `npx vue-tsc --noEmit && npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: no type errors; tests PASS.

- [ ] **Step 9: Commit**

```bash
git add src/models/invoice.ts src/lib/invoice.ts src/lib/__tests__/invoice.test.ts src/lib/services.ts src/stores/invoice.ts
git commit --no-verify -m "invoices: frontend models, helpers, API calls and store"
```

---

### Task 7: Navigation and invoice profile settings page

**Files:**
- Create: `src/views/desktop/common/NoNavbarPageLayout.vue`, `src/views/desktop/settings/InvoiceProfileSettingsPage.vue`
- Modify: `src/router/desktop.ts` (imports ~line 50; move `/budget/list` and `/bucket/list` at lines 206-215; add new routes), `src/components/desktop/MainPageLayout.vue` (after the Accounts `router-link` ending ~line 89; `@mdi/js` import ~line 203-225), `src/views/desktop/settings/SettingsPageLayout.vue` (remove the "Planning" section lines 28-44; add Invoice Profile item; drop now-unused icon imports)

**Interfaces:**
- Consumes: `useInvoicesStore().getInvoiceProfile/updateInvoiceProfile`.
- Produces routes: `/budget/list`, `/bucket/list`, `/invoice/list`, `/invoice/view?id=` (under `NoNavbarPageLayout`), `/settings/invoice_profile` (under `SettingsPageLayout`). Tasks 8 and 9 create the `invoices/ListPage.vue` and `invoices/ViewPage.vue` components these routes import; until then, create them as one-line placeholders so the router compiles:

```vue
<template><div/></template>
```

- [ ] **Step 1: Create `src/views/desktop/common/NoNavbarPageLayout.vue`**

```vue
<template>
    <main-page-layout no-navbar>
        <template #content>
            <router-view />
        </template>
    </main-page-layout>
</template>
```

- [ ] **Step 2: Create placeholder `src/views/desktop/invoices/ListPage.vue` and `src/views/desktop/invoices/ViewPage.vue`** with `<template><div/></template>` (replaced in Tasks 8 and 9).

- [ ] **Step 3: Update `src/router/desktop.ts`**

Add imports next to `BucketListPage`:

```ts
import NoNavbarPageLayout from '@/views/desktop/common/NoNavbarPageLayout.vue';
import InvoiceListPage from '@/views/desktop/invoices/ListPage.vue';
import InvoiceViewPage from '@/views/desktop/invoices/ViewPage.vue';
import InvoiceProfileSettingsPage from '@/views/desktop/settings/InvoiceProfileSettingsPage.vue';
```

Delete the `/budget/list` and `/bucket/list` entries from the `SettingsPageLayout` children. Insert this new top-level route right after the `/account/list` route (before the `SettingsPageLayout` route):

```ts
        {
            path: '/',
            component: NoNavbarPageLayout,
            beforeEnter: checkLogin,
            children: [
                {
                    path: '/budget/list',
                    component: BudgetListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/bucket/list',
                    component: BucketListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/invoice/list',
                    component: InvoiceListPage,
                    beforeEnter: checkLogin
                },
                {
                    path: '/invoice/view',
                    component: InvoiceViewPage,
                    beforeEnter: checkLogin,
                    props: route => ({ id: route.query['id'] })
                }
            ]
        },
```

Add to the `SettingsPageLayout` children, after `/settings/sms_capture`:

```ts
                {
                    path: '/settings/invoice_profile',
                    component: InvoiceProfileSettingsPage,
                    beforeEnter: checkLogin
                },
```

- [ ] **Step 4: Add header buttons in `src/components/desktop/MainPageLayout.vue`**

After the Accounts `</router-link>` and before the Add Transaction `router-link`, insert:

```vue
                                <router-link to="/budget/list"
                                             :aria-current="isTopNavigationActive('/budget/list') ? 'page' : undefined">
                                    <v-btn class="top-navigation-button ms-1" density="comfortable" variant="text"
                                           :aria-label="tt('Budgets')" :icon="true"
                                           :active="isTopNavigationActive('/budget/list')"
                                           :color="isTopNavigationActive('/budget/list') ? 'primary' : 'default'">
                                        <v-icon :icon="isTopNavigationActive('/budget/list') ? mdiWallet : mdiWalletOutline" size="24" />
                                        <v-tooltip activator="parent">{{ tt('Budgets') }}</v-tooltip>
                                    </v-btn>
                                </router-link>

                                <router-link to="/bucket/list"
                                             :aria-current="isTopNavigationActive('/bucket/list') ? 'page' : undefined">
                                    <v-btn class="top-navigation-button ms-1" density="comfortable" variant="text"
                                           :aria-label="tt('Buckets')" :icon="true"
                                           :active="isTopNavigationActive('/bucket/list')"
                                           :color="isTopNavigationActive('/bucket/list') ? 'primary' : 'default'">
                                        <v-icon :icon="isTopNavigationActive('/bucket/list') ? mdiPiggyBank : mdiPiggyBankOutline" size="24" />
                                        <v-tooltip activator="parent">{{ tt('Buckets') }}</v-tooltip>
                                    </v-btn>
                                </router-link>

                                <router-link to="/invoice/list"
                                             :aria-current="isInvoicePageActive ? 'page' : undefined">
                                    <v-btn class="top-navigation-button ms-1" density="comfortable" variant="text"
                                           :aria-label="tt('Invoices')" :icon="true"
                                           :active="isInvoicePageActive"
                                           :color="isInvoicePageActive ? 'primary' : 'default'">
                                        <v-icon :icon="isInvoicePageActive ? mdiFileDocument : mdiFileDocumentOutline" size="24" />
                                        <v-tooltip activator="parent">{{ tt('Invoices') }}</v-tooltip>
                                    </v-btn>
                                </router-link>
```

Add `mdiWallet, mdiWalletOutline, mdiPiggyBank, mdiPiggyBankOutline, mdiFileDocument, mdiFileDocumentOutline` to the `@mdi/js` import. `isTopNavigationActive` (line ~280) compares `route.path` for equality, so add next to it:

```ts
const isInvoicePageActive = computed<boolean>(() => route.path === '/invoice/list' || route.path === '/invoice/view');
```

- [ ] **Step 5: Update `src/views/desktop/settings/SettingsPageLayout.vue`**

Delete the whole "Planning" block (the `nav-section-title` "Planning" `<li>` and the two `/budget/list` and `/bucket/list` `<li class="nav-link">` items). Remove `mdiWalletOutline` and `mdiPiggyBankOutline` from the script's icon import if nothing else uses them. After the `/settings/sms_capture` nav item (search for `sms_capture`), add:

```vue
            <li class="nav-link">
                <router-link to="/settings/invoice_profile">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentEditOutline"/>
                    <span class="nav-item-title">{{ tt('Invoice Profile') }}</span>
                </router-link>
            </li>
```

and add `mdiFileDocumentEditOutline` to its `@mdi/js` import.

- [ ] **Step 6: Create `src/views/desktop/settings/InvoiceProfileSettingsPage.vue`**

```vue
<template>
    <v-row>
        <v-col cols="12">
            <v-card :class="{ 'disabled': loading }">
                <template #title>
                    <span>{{ tt('Invoice Profile') }}</span>
                </template>

                <v-card-text class="pt-2">
                    <span class="text-body-large">{{ tt('These details are printed on every invoice.') }}</span>
                </v-card-text>

                <v-card-text>
                    <v-row>
                        <v-col cols="12" md="6">
                            <v-text-field variant="outlined" density="compact" :label="tt('Business Name')" v-model="profile.businessName" />
                        </v-col>
                        <v-col cols="12" md="3">
                            <v-text-field variant="outlined" density="compact" :label="tt('Invoice Number Prefix')" v-model="profile.numberPrefix" />
                        </v-col>
                        <v-col cols="12" md="3">
                            <v-text-field variant="outlined" density="compact" type="number" min="0" max="365"
                                          :label="tt('Payment Terms (days)')" v-model.number="profile.defaultTermsDays" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-textarea variant="outlined" density="compact" rows="4" :label="tt('Address')" v-model="profile.addressLines" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field variant="outlined" density="compact" :label="tt('Phone')" v-model="profile.phone" />
                            <v-text-field variant="outlined" density="compact" :label="tt('Website')" v-model="profile.website" />
                        </v-col>
                        <v-col cols="12">
                            <v-textarea variant="outlined" density="compact" rows="7" :label="tt('Default Notes / Terms')" v-model="profile.defaultNotes" />
                        </v-col>
                        <v-col cols="12" class="d-flex align-center gap-4">
                            <img class="invoice-profile-logo" :src="profile.logoDataUrl" alt="" v-if="profile.logoDataUrl" />
                            <v-btn variant="tonal" @click="logoInput?.click()">{{ tt('Choose Logo') }}</v-btn>
                            <v-btn variant="text" color="secondary" @click="profile.logoDataUrl = ''" v-if="profile.logoDataUrl">{{ tt('Remove Logo') }}</v-btn>
                            <input ref="logoInput" type="file" accept="image/png,image/jpeg" class="d-none" @change="onLogoChosen" />
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text>
                    <v-btn color="primary" :loading="saving" :disabled="saving || !profile.numberPrefix" @click="save">{{ tt('Save Changes') }}</v-btn>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, onMounted, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import type { InvoiceProfile } from '@/models/invoice.ts';

import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

// matches the backend's max length for logoDataUrl (base64 grows the file by about a third)
const MAX_LOGO_BYTES = 250 * 1024;

const { tt } = useI18n();
const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const logoInput = useTemplateRef<HTMLInputElement>('logoInput');

const loading = ref<boolean>(true);
const saving = ref<boolean>(false);
const profile = ref<InvoiceProfile>({
    businessName: '', addressLines: '', phone: '', website: '', logoDataUrl: '', defaultNotes: '', numberPrefix: 'INV', defaultTermsDays: 7
});

function showError(error: { processed?: boolean }): void {
    if (!error.processed) {
        snackbar.value?.showError(error);
    }
}

function onLogoChosen(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';

    if (!file) {
        return;
    }

    if (file.size > MAX_LOGO_BYTES) {
        snackbar.value?.showMessage('Logo must be smaller than 250 KB');
        return;
    }

    const reader = new FileReader();
    reader.onload = () => {
        profile.value.logoDataUrl = reader.result as string;
    };
    reader.readAsDataURL(file);
}

function save(): void {
    saving.value = true;

    invoicesStore.updateInvoiceProfile(profile.value).then(saved => {
        profile.value = saved;
        snackbar.value?.showMessage('Invoice profile saved');
    }).catch(showError).finally(() => {
        saving.value = false;
    });
}

onMounted(() => {
    invoicesStore.getInvoiceProfile().then(loaded => {
        profile.value = loaded;
    }).catch(showError).finally(() => {
        loading.value = false;
    });
});
</script>

<style scoped>
.invoice-profile-logo {
    max-height: 48px;
    max-width: 240px;
}
</style>
```

- [ ] **Step 7: Type-check and check in the browser**

Run: `npx vue-tsc --noEmit && npx eslint src/router/desktop.ts src/components/desktop/MainPageLayout.vue src/views/desktop/settings src/views/desktop/common`
Expected: clean.

Run the dev server (`npm run serve`, plus the Go backend) and check:
- The header shows Budgets, Buckets and Invoices buttons after Accounts. Clicking Budgets opens the budget list with **no** settings sidebar, and the Budgets button is highlighted.
- Settings sidebar no longer has the Planning section; it has "Invoice Profile".
- Invoice Profile: fill in the sample's details (Rishabh Jain, F-452, Phase-8B / Industrial Area, Sector 91 / Mohali, Punjab 160055 / India, Mobile 7009600910, rishabhjain.dev, prefix `CA`, 7 days, the bank notes block, the Codeation logo PNG), save, reload: values persist.

- [ ] **Step 8: Commit**

```bash
git add src/views/desktop/common/NoNavbarPageLayout.vue src/views/desktop/invoices src/views/desktop/settings src/router/desktop.ts src/components/desktop/MainPageLayout.vue
git commit --no-verify -m "invoices: header navigation and invoice profile settings"
```

---

### Task 8: Invoice list page, edit dialog, customer dialog

**Files:**
- Create (replace placeholder): `src/views/desktop/invoices/ListPage.vue`
- Create: `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`, `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`, `src/views/desktop/invoices/InvoiceStatusChip.vue`

**Interfaces:**
- Consumes: `useInvoicesStore()` (Task 6), helpers from `src/lib/invoice.ts` (Task 6).
- Produces:
  - `InvoiceEditDialog` props `show: boolean`, `invoice: Invoice | null` (an existing invoice with `lines` to edit, `null` for new), `duplicateFrom: Invoice | null` (copy lines/customer/notes into a new invoice); emits `update:show`, `saved(invoice: Invoice)`.
  - `CustomerEditDialog` props `show: boolean`, `customer: Customer | null`; emits `update:show`, `saved(customer: Customer)`.

- [ ] **Step 1: Create `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`**

```vue
<template>
    <v-dialog v-model="showDialog" max-width="560" persistent>
        <v-card>
            <v-card-title>{{ customer ? tt('Edit Customer') : tt('Add Customer') }}</v-card-title>
            <v-card-text>
                <v-text-field variant="outlined" density="compact" :label="tt('Customer Name')" v-model="form.name" autofocus />
                <v-text-field variant="outlined" density="compact" :label="tt('Contact Person')" v-model="form.contactName" />
                <v-textarea variant="outlined" density="compact" rows="3" :label="tt('Address')" v-model="form.addressLines" />
                <v-text-field variant="outlined" density="compact" type="email" :label="tt('Email')" v-model="form.email" />
            </v-card-text>
            <v-card-actions>
                <v-spacer />
                <v-btn variant="text" :disabled="saving" @click="showDialog = false">{{ tt('Cancel') }}</v-btn>
                <v-btn color="primary" variant="tonal" :loading="saving" :disabled="saving || !form.name.trim()" @click="save">{{ tt('Save') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, watch, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import type { Customer, CustomerSaveRequest } from '@/models/invoice.ts';

import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

const props = defineProps<{
    show: boolean;
    customer: Customer | null;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved', customer: Customer): void;
}>();

const { tt } = useI18n();
const invoicesStore = useInvoicesStore();
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showDialog = computed({
    get: () => props.show,
    set: value => emit('update:show', value)
});

const saving = ref<boolean>(false);
const form = ref<CustomerSaveRequest>({ name: '', contactName: '', addressLines: '', email: '' });

watch(() => props.show, show => {
    if (show) {
        form.value = props.customer ? { ...props.customer } : { name: '', contactName: '', addressLines: '', email: '' };
    }
});

function save(): void {
    saving.value = true;

    invoicesStore.saveCustomer(form.value).then(saved => {
        emit('saved', saved);
        showDialog.value = false;
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    }).finally(() => {
        saving.value = false;
    });
}
</script>
```

- [ ] **Step 2: Create `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`**

```vue
<template>
    <v-dialog v-model="showDialog" max-width="960" persistent>
        <v-card>
            <v-card-title>{{ invoice ? tt('Edit Invoice') : tt('New Invoice') }}</v-card-title>

            <v-card-text>
                <v-row>
                    <v-col cols="12" md="6" class="d-flex align-start gap-2">
                        <v-select variant="outlined" density="compact" :label="tt('Customer')"
                                  :items="customers" item-title="name" item-value="id" v-model="form.customerId" />
                        <v-btn variant="tonal" :icon="true" size="small" class="mt-1" @click="showCustomerDialog = true">
                            <v-icon :icon="mdiPlus" />
                            <v-tooltip activator="parent">{{ tt('Add Customer') }}</v-tooltip>
                        </v-btn>
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field variant="outlined" density="compact" :label="tt('Invoice Number')" v-model="form.number" />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field variant="outlined" density="compact" type="date" :label="tt('Invoice Date')"
                                      v-model="form.invoiceDate" @update:model-value="onInvoiceDateChanged" />
                    </v-col>
                    <v-col cols="12" md="6">
                        <v-text-field variant="outlined" density="compact" type="date" :label="tt('Payment Due')"
                                      :min="form.invoiceDate" v-model="form.dueDate" />
                    </v-col>
                </v-row>

                <div class="text-title-small mb-2">{{ tt('Services') }}</div>
                <v-row dense v-for="(line, index) in lines" :key="index" class="align-start">
                    <v-col cols="12" md="3">
                        <v-text-field variant="outlined" density="compact" :label="tt('Item')" v-model="line.itemName" />
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-textarea variant="outlined" density="compact" rows="1" auto-grow :label="tt('Description')" v-model="line.description" />
                    </v-col>
                    <v-col cols="4" md="1">
                        <v-text-field variant="outlined" density="compact" type="number" min="1" :label="tt('Qty')" v-model.number="line.quantity" />
                    </v-col>
                    <v-col cols="4" md="2">
                        <v-text-field variant="outlined" density="compact" type="number" min="0" step="0.01" :label="tt('Rate')" prefix="₹" v-model.number="line.rateRupees" />
                    </v-col>
                    <v-col cols="3" md="1" class="text-right pt-4">{{ formatInvoiceAmount(rupeesToPaise(line.rateRupees || 0) * (line.quantity || 0)) }}</v-col>
                    <v-col cols="1" md="1" class="text-right">
                        <v-btn variant="text" :icon="true" size="small" :disabled="lines.length <= 1" @click="lines.splice(index, 1)">
                            <v-icon :icon="mdiDeleteOutline" />
                        </v-btn>
                    </v-col>
                </v-row>
                <v-btn variant="text" size="small" @click="addLine">
                    <v-icon :icon="mdiPlus" start />{{ tt('Add Line') }}
                </v-btn>

                <div class="d-flex justify-end text-title-medium my-4">
                    <span class="me-4">{{ tt('Total') }}:</span>
                    <span>{{ formatInvoiceAmount(totals.total) }}</span>
                </div>

                <v-textarea variant="outlined" density="compact" rows="6" :label="tt('Notes / Terms')" v-model="form.notes" />
            </v-card-text>

            <v-card-actions>
                <v-spacer />
                <v-btn variant="text" :disabled="saving" @click="showDialog = false">{{ tt('Cancel') }}</v-btn>
                <v-btn color="primary" variant="tonal" :loading="saving" :disabled="saving || !isValid" @click="save">{{ tt('Save') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <customer-edit-dialog v-model:show="showCustomerDialog" :customer="null" @saved="onCustomerAdded" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, watch, useTemplateRef } from 'vue';

import { mdiPlus, mdiDeleteOutline } from '@mdi/js';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import type { Customer, Invoice, InvoiceProfile, InvoiceSaveRequest } from '@/models/invoice.ts';
import { computeInvoiceTotals, formatInvoiceAmount, addDaysToDate, getTodayDateString, rupeesToPaise, paiseToRupees } from '@/lib/invoice.ts';

import CustomerEditDialog from './CustomerEditDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

interface EditableLine {
    itemName: string;
    description: string;
    quantity: number;
    rateRupees: number;
}

const props = defineProps<{
    show: boolean;
    invoice: Invoice | null;
    duplicateFrom: Invoice | null;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved', invoice: Invoice): void;
}>();

const { tt } = useI18n();
const invoicesStore = useInvoicesStore();
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showDialog = computed({
    get: () => props.show,
    set: value => emit('update:show', value)
});

const saving = ref<boolean>(false);
const showCustomerDialog = ref<boolean>(false);
const customers = ref<Customer[]>([]);
const profile = ref<InvoiceProfile | null>(null);
const form = ref<Omit<InvoiceSaveRequest, 'lines'>>({ number: '', customerId: '', invoiceDate: '', dueDate: '', notes: '' });
const lines = ref<EditableLine[]>([]);

const totals = computed(() => computeInvoiceTotals(lines.value.map(line => ({ quantity: line.quantity || 0, rate: rupeesToPaise(line.rateRupees || 0) }))));

const isValid = computed(() => !!form.value.number.trim() && !!form.value.customerId && !!form.value.invoiceDate
    && form.value.dueDate >= form.value.invoiceDate
    && lines.value.length > 0 && lines.value.every(line => line.itemName.trim() && line.quantity >= 1 && line.rateRupees > 0));

function showError(error: { processed?: boolean }): void {
    if (!error.processed) {
        snackbar.value?.showError(error);
    }
}

function addLine(): void {
    lines.value.push({ itemName: '', description: '', quantity: 1, rateRupees: 0 });
}

function suggestNumber(): void {
    invoicesStore.getNextInvoiceNumber(form.value.invoiceDate).then(number => {
        form.value.number = number;
    }).catch(showError);
}

function onInvoiceDateChanged(): void {
    if (!form.value.invoiceDate) {
        return;
    }

    form.value.dueDate = addDaysToDate(form.value.invoiceDate, profile.value?.defaultTermsDays ?? 7);

    if (!props.invoice) {
        suggestNumber();
    }
}

function onCustomerAdded(customer: Customer): void {
    customers.value.push(customer);
    form.value.customerId = customer.id;
}

watch(() => props.show, show => {
    if (!show) {
        return;
    }

    invoicesStore.getAllCustomers().then(loaded => {
        customers.value = loaded;
    }).catch(showError);

    invoicesStore.getInvoiceProfile().then(loaded => {
        profile.value = loaded;
        const source = props.invoice ?? props.duplicateFrom;

        if (props.invoice) {
            form.value = {
                id: props.invoice.id, number: props.invoice.number, customerId: props.invoice.customerId,
                invoiceDate: props.invoice.invoiceDate, dueDate: props.invoice.dueDate, notes: props.invoice.notes
            };
        } else {
            const today = getTodayDateString();
            form.value = {
                number: '', customerId: source?.customerId ?? '', invoiceDate: today,
                dueDate: addDaysToDate(today, loaded.defaultTermsDays), notes: source?.notes ?? loaded.defaultNotes
            };
            suggestNumber();
        }

        lines.value = source?.lines?.length
            ? source.lines.map(line => ({ itemName: line.itemName, description: line.description, quantity: line.quantity, rateRupees: paiseToRupees(line.rate) }))
            : [{ itemName: '', description: '', quantity: 1, rateRupees: 0 }];
    }).catch(showError);
});

function save(): void {
    saving.value = true;

    invoicesStore.saveInvoice({
        ...form.value,
        lines: lines.value.map(line => ({ itemName: line.itemName.trim(), description: line.description, quantity: line.quantity, rate: rupeesToPaise(line.rateRupees) }))
    }).then(saved => {
        emit('saved', saved);
        showDialog.value = false;
    }).catch(showError).finally(() => {
        saving.value = false;
    });
}
</script>
```

- [ ] **Step 3: Replace `src/views/desktop/invoices/ListPage.vue`**

```vue
<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Invoices') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24" class="ms-2" :icon="true" :loading="loading" @click="reload">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer />
                        <v-btn variant="text" size="small" class="me-2" to="/settings/invoice_profile">{{ tt('Invoice Profile') }}</v-btn>
                        <v-btn color="primary" variant="tonal" size="small" :disabled="loading" @click="addInvoice">
                            <v-icon :icon="mdiPlus" size="20" start />{{ tt('New Invoice') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="d-flex flex-wrap gap-6">
                    <div>
                        <div class="text-body-small">{{ tt('Outstanding') }}</div>
                        <div class="text-title-large">{{ formatInvoiceAmount(outstandingTotal) }}</div>
                    </div>
                    <div>
                        <div class="text-body-small">{{ tt('Overdue') }}</div>
                        <div class="text-title-large" :class="{ 'text-error': overdueTotal > 0 }">{{ formatInvoiceAmount(overdueTotal) }}</div>
                    </div>
                </v-card-text>

                <v-card-text class="d-flex flex-wrap align-center gap-4">
                    <v-btn-toggle v-model="filter" density="compact" variant="outlined" mandatory>
                        <v-btn value="all">{{ tt('All') }}</v-btn>
                        <v-btn value="outstanding">{{ tt('Outstanding') }}</v-btn>
                        <v-btn value="paid">{{ tt('Paid') }}</v-btn>
                    </v-btn-toggle>
                    <v-select class="invoice-customer-filter" variant="outlined" density="compact" hide-details clearable
                              :label="tt('Customer')" :items="customerOptions" v-model="customerFilter" />
                </v-card-text>

                <v-table hover>
                    <thead>
                    <tr>
                        <th>{{ tt('Number') }}</th>
                        <th>{{ tt('Customer') }}</th>
                        <th>{{ tt('Invoice Date') }}</th>
                        <th>{{ tt('Payment Due') }}</th>
                        <th class="text-right">{{ tt('Total') }}</th>
                        <th class="text-right">{{ tt('Received') }}</th>
                        <th class="text-right">{{ tt('Balance Due') }}</th>
                        <th>{{ tt('Status') }}</th>
                    </tr>
                    </thead>
                    <tbody>
                    <tr class="cursor-pointer" :key="invoice.id" v-for="invoice in filteredInvoices" @click="openInvoice(invoice)">
                        <td>{{ invoice.number }}</td>
                        <td>{{ invoice.billToName }}</td>
                        <td>{{ formatInvoiceDate(invoice.invoiceDate) }}</td>
                        <td>{{ formatInvoiceDate(invoice.dueDate) }}</td>
                        <td class="text-right">{{ formatInvoiceAmount(invoice.total) }}</td>
                        <td class="text-right">{{ formatInvoiceAmount(invoice.received) }}</td>
                        <td class="text-right">{{ formatInvoiceAmount(getInvoiceBalanceDue(invoice)) }}</td>
                        <td><invoice-status-chip :invoice="invoice" :today="today" /></td>
                    </tr>
                    <tr v-if="!loading && !filteredInvoices.length">
                        <td colspan="8" class="text-center text-medium-emphasis py-6">{{ tt('No invoices') }}</td>
                    </tr>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>
    </v-row>

    <invoice-edit-dialog v-model:show="showEditDialog" :invoice="null" :duplicate-from="null" @saved="onInvoiceSaved" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, useTemplateRef } from 'vue';
import { useRouter } from 'vue-router';

import { mdiRefresh, mdiPlus } from '@mdi/js';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import type { Invoice } from '@/models/invoice.ts';
import { formatInvoiceAmount, formatInvoiceDate, getInvoiceBalanceDue, getInvoiceStatus, isInvoiceOverdue, getTodayDateString } from '@/lib/invoice.ts';

import InvoiceEditDialog from './dialogs/InvoiceEditDialog.vue';
import InvoiceStatusChip from './InvoiceStatusChip.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const router = useRouter();
const invoicesStore = useInvoicesStore();
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const loading = ref<boolean>(false);
const showEditDialog = ref<boolean>(false);
const invoices = ref<Invoice[]>([]);
const filter = ref<'all' | 'outstanding' | 'paid'>('all');
const customerFilter = ref<string | null>(null);
const today = getTodayDateString();

const customerOptions = computed(() => Array.from(new Set(invoices.value.map(invoice => invoice.billToName))).sort());

const filteredInvoices = computed(() => invoices.value.filter(invoice => {
    const paid = getInvoiceStatus(invoice) === 'paid';
    return (filter.value === 'all' || (filter.value === 'paid') === paid)
        && (!customerFilter.value || invoice.billToName === customerFilter.value);
}));

const outstandingTotal = computed(() => invoices.value
    .filter(invoice => getInvoiceStatus(invoice) !== 'paid')
    .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

const overdueTotal = computed(() => invoices.value
    .filter(invoice => isInvoiceOverdue(invoice, today))
    .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

function reload(): void {
    loading.value = true;

    invoicesStore.getAllInvoices().then(loaded => {
        invoices.value = loaded;
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    }).finally(() => {
        loading.value = false;
    });
}

function addInvoice(): void {
    showEditDialog.value = true;
}

function openInvoice(invoice: Invoice): void {
    router.push(`/invoice/view?id=${invoice.id}`);
}

function onInvoiceSaved(invoice: Invoice): void {
    openInvoice(invoice);
}

onMounted(reload);
</script>

<style scoped>
.invoice-customer-filter {
    max-width: 280px;
}
</style>
```

- [ ] **Step 4: Create `src/views/desktop/invoices/InvoiceStatusChip.vue`** (used by the list and the view page)

```vue
<template>
    <v-chip size="small" variant="tonal" :color="color">{{ tt(label) }}</v-chip>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import type { Invoice } from '@/models/invoice.ts';
import { getInvoiceStatus, isInvoiceOverdue } from '@/lib/invoice.ts';

const props = defineProps<{
    invoice: Invoice;
    today: string;
}>();

const { tt } = useI18n();

const label = computed<string>(() => {
    if (isInvoiceOverdue(props.invoice, props.today)) {
        return 'Overdue';
    }

    return { unpaid: 'Unpaid', partial: 'Partially Paid', paid: 'Paid' }[getInvoiceStatus(props.invoice)];
});

const color = computed<string>(() => ({ Overdue: 'error', Unpaid: 'warning', 'Partially Paid': 'info', Paid: 'success' } as Record<string, string>)[label.value]!);
</script>
```

- [ ] **Step 5: Type-check and check in the browser**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices`
Expected: clean.

In the browser: Invoices → New Invoice. Add customer "Quber Tech" (Kanika Batra, the London address, kanika.batra@qubertech.com) from the + button; it gets selected. Set invoice date 2026-04-23: number becomes `CA/2026-27/01`, due date 2026-04-30, notes prefilled from the profile. One line: Consulting / "Software Consulting\nQuber Tech" / 1 / 370000. Total shows ₹370,000.00. Save: you land on `/invoice/view?id=…` (blank until Task 9). Back on the list: the row shows ₹370,000.00 total, ₹0.00 received, and Overdue (since its due date is in the past). Outstanding and Overdue both show ₹370,000.00. The Paid tab shows nothing.

- [ ] **Step 6: Commit**

```bash
git add src/views/desktop/invoices
git commit --no-verify -m "invoices: list page and edit dialogs"
```

---

### Task 9: Invoice view page, print document, payments panel

**Files:**
- Create: `src/components/desktop/InvoiceDocument.vue`
- Create (replace placeholder): `src/views/desktop/invoices/ViewPage.vue`

**Interfaces:**
- Consumes: Task 6 store and helpers; Task 8 `InvoiceEditDialog`, `InvoiceStatusChip`; existing `useAccountsStore()` (`loadAllAccounts({ force: false })`, `allAccountsMap: Record<string, Account>` with `.name`).
- Produces: `InvoiceDocument` props `invoice: Invoice` (with `lines`), `profile: InvoiceProfile`. Route prop `id: string` on `ViewPage`.

- [ ] **Step 1: Create `src/components/desktop/InvoiceDocument.vue`**

A pure component, so a server-side renderer for emailed invoices can later mirror its structure.

```vue
<template>
    <div class="invoice-document">
        <div class="invoice-header">
            <img class="invoice-logo" :src="profile.logoDataUrl" alt="" v-if="profile.logoDataUrl" />
            <div v-else></div>
            <div class="invoice-business">
                <div class="invoice-strong">{{ profile.businessName }}</div>
                <div class="invoice-pre">{{ profile.addressLines }}</div>
                <div class="invoice-gap" v-if="profile.phone || profile.website"></div>
                <div v-if="profile.phone">Mobile: {{ profile.phone }}</div>
                <div v-if="profile.website">{{ profile.website }}</div>
            </div>
        </div>

        <div class="invoice-banner">
            <div class="invoice-rule"></div>
            <div class="invoice-title">Invoice</div>
            <div class="invoice-rule"></div>
        </div>

        <div class="invoice-parties">
            <div>
                <div class="invoice-muted">BILL TO</div>
                <div class="invoice-strong">{{ invoice.billToName }}</div>
                <div v-if="invoice.billToContactName">{{ invoice.billToContactName }}</div>
                <div class="invoice-pre">{{ invoice.billToAddressLines }}</div>
                <div class="invoice-gap" v-if="invoice.billToEmail"></div>
                <div v-if="invoice.billToEmail">{{ invoice.billToEmail }}</div>
            </div>
            <table class="invoice-meta">
                <tbody>
                <tr><th>Invoice Number:</th><td>{{ invoice.number }}</td></tr>
                <tr><th>Invoice Date:</th><td>{{ formatInvoiceDate(invoice.invoiceDate) }}</td></tr>
                <tr><th>Payment Due:</th><td>{{ formatInvoiceDate(invoice.dueDate) }}</td></tr>
                <tr class="invoice-meta-highlight"><th>Amount Due ({{ invoice.currency }}):</th><td class="invoice-strong">{{ formatInvoiceAmount(balanceDue) }}</td></tr>
                </tbody>
            </table>
        </div>

        <table class="invoice-lines">
            <thead>
            <tr>
                <th class="text-left">Services</th>
                <th class="text-right" v-if="showQuantity">Quantity</th>
                <th class="text-right">Rate</th>
                <th class="text-right">Amount</th>
            </tr>
            </thead>
            <tbody>
            <tr :key="index" v-for="(line, index) in invoice.lines">
                <td>
                    <div class="invoice-strong">{{ line.itemName }}</div>
                    <div class="invoice-muted invoice-pre">{{ line.description }}</div>
                </td>
                <td class="text-right" v-if="showQuantity">{{ line.quantity }}</td>
                <td class="text-right">{{ formatInvoiceAmount(line.rate) }}</td>
                <td class="text-right">{{ formatInvoiceAmount(line.amount) }}</td>
            </tr>
            </tbody>
        </table>

        <table class="invoice-totals">
            <tbody>
            <tr><th>Total:</th><td>{{ formatInvoiceAmount(invoice.total) }}</td></tr>
            <tr v-if="invoice.received > 0"><th>Payments received:</th><td>−{{ formatInvoiceAmount(Math.min(invoice.received, invoice.total)) }}</td></tr>
            <tr class="invoice-totals-due"><th>Amount Due ({{ invoice.currency }}):</th><td>{{ formatInvoiceAmount(balanceDue) }}</td></tr>
            </tbody>
        </table>

        <div class="invoice-notes" v-if="invoice.notes">
            <div class="invoice-muted invoice-strong">Notes / Terms</div>
            <div class="invoice-muted invoice-pre">{{ invoice.notes }}</div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import type { Invoice, InvoiceProfile } from '@/models/invoice.ts';
import { formatInvoiceAmount, formatInvoiceDate, getInvoiceBalanceDue } from '@/lib/invoice.ts';

const props = defineProps<{
    invoice: Invoice;
    profile: InvoiceProfile;
}>();

const balanceDue = computed<number>(() => getInvoiceBalanceDue(props.invoice));
const showQuantity = computed<boolean>(() => !!props.invoice.lines?.some(line => line.quantity !== 1));
</script>

<style scoped>
/* Printed in English regardless of app language: it is a document sent to the customer. */
.invoice-document {
    --invoice-accent: #3d86f5;
    background: #fff;
    color: #000;
    font-family: Georgia, 'Noto Serif', 'Times New Roman', serif; /* these fonts carry the ₹ glyph */
    font-size: 13px;
    line-height: 1.35;
    max-width: 800px;
    margin: 0 auto;
    padding: 32px;
}

.invoice-header, .invoice-parties {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
}

.invoice-logo {
    max-height: 48px;
    max-width: 320px;
}

.invoice-business {
    text-align: right;
}

.invoice-strong {
    font-weight: 700;
}

.invoice-muted {
    color: #6b7280;
}

.invoice-pre {
    white-space: pre-line;
}

.invoice-gap {
    height: 12px;
}

.invoice-banner {
    display: flex;
    align-items: center;
    gap: 16px;
    margin: 40px 0 56px;
}

.invoice-rule {
    flex: 1;
    border-top: 2px solid var(--invoice-accent);
    border-bottom: 2px solid var(--invoice-accent);
    height: 6px;
}

.invoice-title {
    color: var(--invoice-accent);
    font-size: 30px;
    border: 2px solid var(--invoice-accent);
    border-radius: 6px;
    outline: 2px solid var(--invoice-accent);
    outline-offset: 3px;
    padding: 6px 34px;
}

.invoice-meta th {
    text-align: right;
    padding: 2px 12px 2px 16px;
}

.invoice-meta td {
    padding: 2px 16px 2px 0;
}

.invoice-meta-highlight {
    background: #f3f4f6;
}

.invoice-lines {
    width: 100%;
    border-collapse: collapse;
    margin-top: 40px;
}

.invoice-lines th {
    padding: 8px 0;
    border-bottom: 1px dotted #000;
}

.invoice-lines td {
    padding: 12px 0;
    border-bottom: 1px dotted #000;
    vertical-align: top;
}

.invoice-totals {
    margin: 16px 0 0 auto;
    border-collapse: collapse;
}

.invoice-totals th {
    text-align: right;
    padding: 8px 48px 8px 0;
}

.invoice-totals td {
    text-align: right;
    padding: 8px 0;
}

.invoice-totals-due {
    border-top: 3px solid #e5e7eb;
    font-weight: 700;
}

.invoice-notes {
    margin-top: 56px;
}
</style>
```

- [ ] **Step 2: Replace `src/views/desktop/invoices/ViewPage.vue`**

```vue
<template>
    <div class="invoice-view-toolbar d-flex flex-wrap align-center gap-2 mb-4">
        <v-btn variant="text" :prepend-icon="mdiArrowLeft" to="/invoice/list">{{ tt('Invoices') }}</v-btn>
        <invoice-status-chip :invoice="invoice" :today="today" v-if="invoice" />
        <v-spacer />
        <v-btn variant="tonal" :disabled="!invoice" @click="showEditDialog = true">{{ tt('Edit') }}</v-btn>
        <v-btn variant="tonal" :disabled="!invoice" @click="showDuplicateDialog = true">{{ tt('Duplicate') }}</v-btn>
        <v-btn variant="tonal" :disabled="!invoice || busy" @click="toggleClosed" v-if="invoice">
            {{ invoice.closedManually ? tt('Reopen') : tt('Mark as Paid') }}
        </v-btn>
        <v-btn color="error" variant="text" :disabled="!invoice || busy" @click="remove">{{ tt('Delete') }}</v-btn>
        <v-btn color="primary" :prepend-icon="mdiPrinterOutline" :disabled="!invoice || !profile" @click="print">{{ tt('Print / Save PDF') }}</v-btn>
    </div>

    <v-row v-if="invoice && profile">
        <v-col cols="12" lg="8">
            <v-card class="invoice-print-area">
                <invoice-document :invoice="invoice" :profile="profile" />
            </v-card>
        </v-col>

        <v-col cols="12" lg="4" class="invoice-view-payments">
            <v-card>
                <v-card-title>{{ tt('Payments') }}</v-card-title>
                <v-card-text>
                    <div class="d-flex justify-space-between"><span>{{ tt('Total') }}</span><span>{{ formatInvoiceAmount(invoice.total) }}</span></div>
                    <div class="d-flex justify-space-between"><span>{{ tt('Received') }}</span><span>{{ formatInvoiceAmount(invoice.received) }}</span></div>
                    <div class="d-flex justify-space-between" v-if="bankCharges > 0"><span>{{ tt('Bank Charges') }}</span><span>{{ formatInvoiceAmount(bankCharges) }}</span></div>
                    <div class="d-flex justify-space-between text-title-medium mt-1" v-else>
                        <span>{{ tt('Balance Due') }}</span><span>{{ formatInvoiceAmount(getInvoiceBalanceDue(invoice)) }}</span>
                    </div>
                </v-card-text>

                <v-list density="compact" v-if="invoice.payments?.length">
                    <v-list-item :key="payment.transactionId" v-for="payment in invoice.payments"
                                 :title="formatInvoiceAmount(payment.amount)" :subtitle="paymentSubtitle(payment)">
                        <template #append>
                            <v-btn variant="text" size="small" :icon="true" :disabled="busy" @click="unlink(payment)">
                                <v-icon :icon="mdiLinkOff" />
                                <v-tooltip activator="parent">{{ tt('Unlink') }}</v-tooltip>
                            </v-btn>
                        </template>
                    </v-list-item>
                </v-list>

                <template v-if="suggestions.length">
                    <v-divider />
                    <v-card-subtitle class="mt-3">{{ tt('Suggested Matches') }}</v-card-subtitle>
                    <v-list density="compact">
                        <v-list-item :key="suggestion.transactionId" v-for="suggestion in suggestions"
                                     :title="formatInvoiceAmount(suggestion.amount)" :subtitle="paymentSubtitle(suggestion)">
                            <template #append>
                                <v-btn variant="tonal" size="small" :disabled="busy" @click="link(suggestion)">{{ tt('Link') }}</v-btn>
                            </template>
                        </v-list-item>
                    </v-list>
                </template>
                <v-card-text class="text-medium-emphasis" v-else-if="getInvoiceStatus(invoice) !== 'paid'">
                    {{ tt('No matching income transactions found yet.') }}
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <invoice-edit-dialog v-model:show="showEditDialog" :invoice="invoice" :duplicate-from="null" @saved="load" />
    <invoice-edit-dialog v-model:show="showDuplicateDialog" :invoice="null" :duplicate-from="invoice" @saved="openInvoice" />
    <confirm-dialog ref="confirmDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount, useTemplateRef } from 'vue';
import { useRouter } from 'vue-router';

import { mdiArrowLeft, mdiPrinterOutline, mdiLinkOff } from '@mdi/js';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';
import type { Invoice, InvoicePayment, InvoiceProfile } from '@/models/invoice.ts';
import {
    formatInvoiceAmount,
    getInvoiceBalanceDue,
    getInvoiceBankCharges,
    getInvoiceStatus,
    getInvoicePdfFileName,
    getTodayDateString
} from '@/lib/invoice.ts';

import InvoiceDocument from '@/components/desktop/InvoiceDocument.vue';
import InvoiceEditDialog from './dialogs/InvoiceEditDialog.vue';
import InvoiceStatusChip from './InvoiceStatusChip.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const props = defineProps<{
    id?: string;
}>();

const { tt } = useI18n();
const router = useRouter();
const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const invoice = ref<Invoice | null>(null);
const profile = ref<InvoiceProfile | null>(null);
const suggestions = ref<InvoicePayment[]>([]);
const busy = ref<boolean>(false);
const showEditDialog = ref<boolean>(false);
const showDuplicateDialog = ref<boolean>(false);
const today = getTodayDateString();
const originalTitle = document.title;

const bankCharges = computed<number>(() => invoice.value ? getInvoiceBankCharges(invoice.value) : 0);

function showError(error: { processed?: boolean }): void {
    if (!error.processed) {
        snackbar.value?.showError(error);
    }
}

function paymentSubtitle(payment: InvoicePayment): string {
    const date = new Date(payment.transactionTime * 1000).toLocaleDateString();
    const account = accountsStore.allAccountsMap[payment.accountId]?.name ?? '';
    return [date, account, payment.comment].filter(part => !!part).join(' · ');
}

function load(): void {
    if (!props.id) {
        return;
    }

    Promise.all([
        invoicesStore.getInvoice(props.id),
        invoicesStore.getInvoiceProfile(),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;

        return getInvoiceStatus(loadedInvoice) === 'paid' ? [] : invoicesStore.getInvoicePaymentSuggestions(loadedInvoice.id);
    }).then(loadedSuggestions => {
        suggestions.value = loadedSuggestions;
    }).catch(showError);
}

function runAndReload(action: () => Promise<boolean>): void {
    busy.value = true;
    action().then(load).catch(showError).finally(() => {
        busy.value = false;
    });
}

function link(suggestion: InvoicePayment): void {
    runAndReload(() => invoicesStore.linkInvoicePayment(invoice.value!.id, suggestion.transactionId));
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment(invoice.value!.id, payment.transactionId));
}

function toggleClosed(): void {
    const id = invoice.value!.id;
    runAndReload(() => invoice.value!.closedManually ? invoicesStore.reopenInvoice(id) : invoicesStore.closeInvoice(id));
}

function remove(): void {
    confirmDialog.value?.open('Are you sure you want to delete this invoice?').then(() => {
        busy.value = true;
        invoicesStore.deleteInvoice(invoice.value!.id).then(() => {
            router.push('/invoice/list');
        }).catch(showError).finally(() => {
            busy.value = false;
        });
    });
}

function openInvoice(saved: Invoice): void {
    router.push(`/invoice/view?id=${saved.id}`);
}

function print(): void {
    // the browser uses document.title as the suggested PDF file name
    document.title = getInvoicePdfFileName(invoice.value!.number, invoice.value!.dueDate);
    window.addEventListener('afterprint', () => {
        document.title = originalTitle;
    }, { once: true });
    window.print();
}

watch(() => props.id, load, { immediate: true });

onBeforeUnmount(() => {
    document.title = originalTitle;
});
</script>

<style>
/* Not scoped: printing has to hide the app chrome outside this component. */
@media print {
    body * {
        visibility: hidden;
    }

    .invoice-print-area, .invoice-print-area * {
        visibility: visible;
    }

    .invoice-print-area {
        position: absolute;
        inset: 0;
        box-shadow: none !important;
    }

    @page {
        margin: 12mm;
    }
}
</style>
```

- [ ] **Step 3: Type-check and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices src/components/desktop/InvoiceDocument.vue`
Expected: clean.

- [ ] **Step 4: Check against the sample in the browser**

Open the `CA/2026-27/01` invoice created in Task 8.
- The document matches the Wave sample (`~/Desktop/Invoice_CA_2026-27_01_2026-04-30.pdf`): logo top-left, business block top-right, double-rule "Invoice" banner, bill-to left, meta box right with grey "Amount Due (INR): ₹370,000.00" row, Services/Rate/Amount table with dotted rules (no Quantity column), totals, notes. The ₹ glyph renders (Wave's PDF shows ☐).
- Print / Save PDF: the dialog shows only the invoice (no header, no payments panel) and the suggested file name is `Invoice_CA_2026-27_01_2026-04-30`. Save it and compare side by side with the Wave PDF.
- Payments: add an income transaction of ₹3,68,500 to the PNB account dated 2026-04-29 (Transactions page). Reload the invoice: it appears under Suggested Matches. Link: status becomes Paid, panel shows Bank Charges ₹1,500.00, the document's Amount Due shows ₹1,500.00 and "Payments received" appears. Unlink: back to Overdue.
- Partial: link a ₹2,00,000 income → Partially Paid with ₹1,70,000 balance due; the suggestions now only offer amounts ≤ ₹1,70,000.
- Mark as Paid with the partial payment → Paid, Bank Charges ₹1,70,000; Reopen → Partially Paid.
- Duplicate → new-invoice dialog with the same customer, lines and notes, the next number, today's date.
- Delete the transaction you linked (Transactions page) → the invoice's received drops back.

- [ ] **Step 5: Commit**

```bash
git add src/components/desktop/InvoiceDocument.vue src/views/desktop/invoices/ViewPage.vue
git commit --no-verify -m "invoices: view page, print layout and payments panel"
```

---

### Task 10: Locale strings and full verification

**Files:**
- Modify: `src/locales/en.json`

- [ ] **Step 1: Add UI strings** as top-level keys (next to `"Buckets"`, ~line 2845):

```json
    "Invoices": "Invoices",
    "Invoice Profile": "Invoice Profile",
    "These details are printed on every invoice.": "These details are printed on every invoice.",
    "Business Name": "Business Name",
    "Invoice Number Prefix": "Invoice Number Prefix",
    "Payment Terms (days)": "Payment Terms (days)",
    "Address": "Address",
    "Phone": "Phone",
    "Website": "Website",
    "Default Notes / Terms": "Default Notes / Terms",
    "Choose Logo": "Choose Logo",
    "Remove Logo": "Remove Logo",
    "Logo must be smaller than 250 KB": "Logo must be smaller than 250 KB",
    "Invoice profile saved": "Invoice profile saved",
    "Add Customer": "Add Customer",
    "Edit Customer": "Edit Customer",
    "Customer": "Customer",
    "Customer Name": "Customer Name",
    "Contact Person": "Contact Person",
    "New Invoice": "New Invoice",
    "Edit Invoice": "Edit Invoice",
    "Invoice Number": "Invoice Number",
    "Invoice Date": "Invoice Date",
    "Payment Due": "Payment Due",
    "Services": "Services",
    "Item": "Item",
    "Qty": "Qty",
    "Rate": "Rate",
    "Add Line": "Add Line",
    "Notes / Terms": "Notes / Terms",
    "Outstanding": "Outstanding",
    "Overdue": "Overdue",
    "Paid": "Paid",
    "Unpaid": "Unpaid",
    "Partially Paid": "Partially Paid",
    "Number": "Number",
    "Received": "Received",
    "Balance Due": "Balance Due",
    "Bank Charges": "Bank Charges",
    "No invoices": "No invoices",
    "Payments": "Payments",
    "Suggested Matches": "Suggested Matches",
    "No matching income transactions found yet.": "No matching income transactions found yet.",
    "Link": "Link",
    "Unlink": "Unlink",
    "Mark as Paid": "Mark as Paid",
    "Reopen": "Reopen",
    "Print / Save PDF": "Print / Save PDF",
    "Are you sure you want to delete this invoice?": "Are you sure you want to delete this invoice?",
    "Unable to process invoice request": "Unable to process invoice request",
```

Before adding each key, search `en.json` for it (`Total`, `Description`, `Email`, `Status`, `Duplicate`, `Edit`, `Delete`, `All`, `Save`, `Cancel`, `Refresh`, `Save Changes`, `Budgets`, `Buckets` already exist; several above, such as `Address`, `Customer` or `Paid`, may exist too). Keys must stay unique; skip any that are already present.

- [ ] **Step 2: Add error strings** inside the `"error"` object:

```json
        "invoice id is invalid": "Invoice ID is invalid",
        "invoice not found": "Invoice not found",
        "invoice number already exists": "Invoice number already exists",
        "invoice date is invalid": "Invoice date is invalid",
        "due date cannot be before invoice date": "Due date cannot be before the invoice date",
        "customer id is invalid": "Customer ID is invalid",
        "customer not found": "Customer not found",
        "customer is used by invoices": "This customer is used by invoices and cannot be deleted",
        "transaction must be an income in the invoice currency": "Only income transactions in the invoice currency can be linked",
        "transaction is already linked to an invoice": "This transaction is already linked to an invoice",
        "payment exceeds the invoice balance due": "This payment is more than the invoice's balance due",
        "invoice payment not found": "Invoice payment not found",
        "invoice logo must be a png or jpeg image": "The logo must be a PNG or JPEG image",
```

- [ ] **Step 3: Validate JSON and run everything**

Run:

```bash
python3 -c "import json; json.load(open('src/locales/en.json'))" \
  && go build ./... && go vet ./pkg/... ./cmd/... \
  && go test ./pkg/services ./pkg/api -count=1 \
  && npx vue-tsc --noEmit && npx eslint src && npx vitest run
```

Expected: every command succeeds. (`go test ./pkg/exchangerates` hits live bank APIs and may time out; it is unrelated and not part of this run.)

- [ ] **Step 4: Error-path check in the browser**

- Create a second invoice with number `CA/2026-27/01` → snackbar "Invoice number already exists".
- Set the due date before the invoice date by typing it → Save stays disabled (the server also refuses it with "Due date cannot be before the invoice date").
- There is no delete-customer UI in this phase; check the guard with curl: `POST /api/v1/customers/delete.json {"id":"<Quber Tech id>"}` → `"customer is used by invoices"`.

- [ ] **Step 5: Commit**

```bash
git add src/locales/en.json
git commit --no-verify -m "invoices: locale strings"
```

---

## Spec coverage (self-check)

| Spec item | Task |
|---|---|
| Profile, customer, invoice, line, payment tables; snapshot; currency/kind columns | 1, 2, 3 |
| Number uniqueness, FY numbering, suggested next number | 3 |
| Totals computed in one place | 3 (Go), 6 (TS) |
| Status (unpaid/partial/paid/overdue), 3% tolerance, bank charges, manual close | 6, 8, 9 |
| Multiple payments, link/unlink, deleted transaction drops out | 4, 9 |
| Suggestions (window, currency, unlinked, ranking, top 10) | 4, 9 |
| Validation at the API boundary; ownership | 1 (binding tags), 3, 4, 5 |
| Print-to-PDF layout matching the sample, file name | 9 |
| Header nav for Budgets/Buckets/Invoices; Planning removed from settings | 7 |
| Invoice profile settings incl. logo | 7 |
| List page with filters, Outstanding/Overdue totals | 8 |
| Edit dialog, add customer inline, duplicate | 8, 9 |
| Locale strings | 10 |
| Future extensions | not built (by design); seams: `Kind`, `Currency`, `computeInvoiceTotal`, `CreateInvoice`, pure `InvoiceDocument` |
