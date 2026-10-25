# Invoicing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create customer invoices in ezBookkeeping on desktop and mobile, print them to PDF in the Wave layout, and track each one until it is paid by linking existing income transactions.

**Architecture:** Five new tables (profile, customer, invoice, invoice line, invoice payment) behind a new `InvoiceService` and `InvoicesApi`, following the buckets backend pattern. The frontend follows the project's existing three-layer UI structure exactly: model classes (`src/models/`), a caching Pinia store (`src/stores/`), shared page logic composables (`src/views/base/invoices/`), then a Vuetify desktop UI (`src/views/desktop/invoices/`) and a Framework7 mobile UI (`src/views/mobile/invoices/`) that both consume the same composables. The only genuinely new component is the printable `InvoiceDocument`, because nothing in the app prints today.

**Tech Stack:** Go + gin + xorm (sqlite/mysql/postgres), Vue 3 + TypeScript + Pinia, Vuetify 3 (desktop), Framework7 (mobile), vitest, testify.

**Spec:** `docs/superpowers/specs/2026-10-25-invoicing-design.md`

## Global Constraints

- INR only, no GST. `Invoice.Currency` is always `"INR"` and `Invoice.Kind` is always `"invoice"` in this phase.
- Amounts are integer paise (₹1 = 100) everywhere: database, API, models, `AmountInput` / `NumberPadSheet` v-models. Never divide by 100 in app UI; format with `formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), 'INR')`. Only the printed `InvoiceDocument` uses its own fixed English formatter (it is a document sent to a UK customer, independent of app language).
- Every table and query is scoped by `uid`; invoicing never creates or modifies transactions; matching is suggested, never automatic.
- Paid tolerance `0.03`: an invoice is Paid when `closedManually` or `total - received <= total * 0.03`.
- Suggestions: income, same-currency account, not linked, not deleted, invoice date to due date + 60 days, amount ≤ balance due; ranked exact, `[97%, 100%)`, smaller; top 10.
- A transaction links to at most one invoice (unique index).
- Number suggestion `{NumberPrefix}/{FY}/{seq:02}`, FY April–March.
- No new npm or Go dependencies.
- **UI conventions (from the user):** reuse existing structure, components and logic; build new UI only where nothing fits. Concretely, mirror these reference files and never the branch's Budgets/Buckets pages (they do not follow conventions):
  - shared logic: `src/views/base/accounts/AccountEditPageBase.ts`, `src/views/base/categories/CategoryEditPageBase.ts`, `src/views/base/users/UserProfilePageBase.ts` (state + computed + English message keys; no snackbar/toast inside base)
  - desktop list: `src/views/desktop/templates/ListPage.vue` (card title toolbar, `v-table`, skeleton rows, hover row actions) inside a `main-page-layout` with a sidebar like `src/views/desktop/accounts/ListPage.vue`
  - desktop dialogs: `src/views/desktop/categories/list/dialogs/EditDialog.vue` (`v-dialog` + `one-column-dialog-layout`, `open()` returning a Promise, `persistent` when modified, footer save button with problem-message tooltip, `defineExpose({ open })`)
  - desktop settings page: `src/views/desktop/users/UserBasicSettingPage.vue` (`setting-items` card, avatar menu with hidden file input, `setting-item` rows, Save Changes + Reset)
  - mobile list: `src/views/mobile/templates/ListPage.vue` (`f7-page :ptr`, navbar back/ellipsis/plus, skeleton list, swipeout Edit/Delete, delete action sheet, `onPageAfterIn` reload)
  - mobile edit: `src/views/mobile/accounts/EditPage.vue` (checkmark save, `f7-list form strong inset dividers`, `NumberPadSheet` for amounts, `DateSelectionSheet` for dates, `ListItemSelectionPopup` for pickers, per-child `f7-list` with group title + red delete button, `showAlert(problemMessage)`, `router.back()`)
  - mobile settings form: `src/views/mobile/users/UserProfilePage.vue`; mobile image picker: hidden input + `image-box` as in `src/views/mobile/transactions/EditPage.vue:397-426, 518`
  - i18n: `useI18n` from `@/locales/helpers.ts` (`tt`, `te`, `formatAmountToLocalizedNumeralsWithCurrency`, `formatGregorianTextualYearMonthDayToLongDate`, `formatDateTimeToLongDate`); mobile UI helpers from `@/lib/ui/mobile.ts` (`useI18nUIComponents`, `showLoading`, `hideLoading`, `onSwipeoutDeleted`)
- Strings: English keys passed to `tt`/`showMessage`/`showToast`; every new key added to `src/locales/en.json` as a flat top-level key; backend errors under `"error"`.
- Commit with `git commit --no-verify` (repo convention).

## Review Focus

1. **Short payment exactly at the 3% boundary** (₹3,70,000 invoice, ₹3,58,900 received) must show Paid with ₹11,100 bank charges; ₹3,58,899 must show Partially Paid. Pinned in Task 6 vitest.
2. **A linked transaction is later deleted**: received drops and the invoice goes back to Unpaid/Partially Paid without errors. Pinned in Task 4 Go test `TestInvoiceReceivedIgnoresDeletedTransaction`; the store never caches the invoice list (Task 6), so the next visit shows it.
3. **Linking another user's transaction ID** is refused as not found. Pinned in Task 4 Go test `TestLinkPaymentRejectsOtherUsersTransaction`.
4. **March vs April invoice dates** get `2025-26` vs `2026-27`, and a numbering gap (01, 03) suggests 04. Pinned in Task 3 Go tests.
5. **Editing a fully paid invoice's total below what was received** shows Paid with no negative balance or bank charges. Pinned in Task 6 vitest (`balance due never negative`).

---

## File map

Backend (Tasks 1–5):
- Create `pkg/models/invoice.go`, `pkg/errs/invoice.go`, `pkg/services/invoices.go`, `pkg/services/invoices_test.go`, `pkg/api/invoices.go`, `pkg/api/invoices_test.go`
- Modify `pkg/errs/error.go`, `pkg/uuid/uuid_type.go`, `cmd/database.go`, `cmd/webserver.go`, `pkg/services/alerts_testutil_test.go`

Frontend shared (Tasks 6–8):
- Create `src/models/invoice.ts`, `src/lib/invoice.ts`, `src/lib/__tests__/invoice.test.ts`, `src/stores/invoice.ts`
- Create `src/views/base/invoices/InvoiceListPageBase.ts`, `InvoiceEditPageBase.ts`, `InvoiceDetailPageBase.ts`, `CustomerEditPageBase.ts`, `InvoiceProfilePageBase.ts`
- Create `src/components/common/InvoiceDocument.vue` (new: the only printable component; shared by both platforms)
- Modify `src/lib/services.ts`, `src/stores/index.ts` (reset on logout)

Desktop (Tasks 9–11):
- Create `src/views/desktop/invoices/ListPage.vue`, `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`, `InvoiceDetailDialog.vue`, `CustomerEditDialog.vue`, `src/views/desktop/settings/InvoiceProfileSettingPage.vue`
- Modify `src/router/desktop.ts`, `src/components/desktop/MainPageLayout.vue` (header buttons), `src/views/desktop/settings/SettingsPageLayout.vue` (Invoice Profile item)

Mobile (Tasks 12–13):
- Create `src/views/mobile/invoices/ListPage.vue`, `EditPage.vue`, `DetailPage.vue`, `CustomerListPage.vue`, `CustomerEditPage.vue`, `ProfilePage.vue`
- Modify `src/router/mobile.ts`, `src/views/mobile/SettingsPage.vue`

Strings (Task 14): `src/locales/en.json`

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
- Modify: `src/lib/services.ts` (type imports near the bucket imports ~line 206-213; methods after the bucket methods ~line 1015), `src/stores/index.ts` (import, instantiate in `useRootStore`, call reset in `resetAllStates` lines 61-81)

**Interfaces:**
- Consumes: Task 5 endpoints.
- Produces:
  - `src/models/invoice.ts`: classes `InvoiceProfile`, `Customer`, `Invoice` (each with `static of(response)`, `clone()`, `fillFrom(other)`, request builders), interfaces `InvoiceLine`, `InvoicePayment`, response/request interfaces, `INVOICE_CURRENCY = 'INR'`.
  - `src/lib/invoice.ts`: `INVOICE_PAID_TOLERANCE`, `InvoiceDisplayStatus`, `computeInvoiceTotals`, `getInvoiceBalanceDue`, `getInvoiceStatus`, `isInvoiceOverdue`, `getInvoiceDisplayStatus`, `getInvoiceBankCharges`, `formatInvoiceDocumentAmount`, `formatInvoiceDocumentDate`, `getInvoicePdfFileName`, `addDaysToDate`, `getTodayTextualDate`.
  - `useInvoicesStore()` state `allInvoices`, `allInvoicesMap`, `allCustomers`, `allCustomersMap`, `customerListStateInvalid`, `invoiceProfile`; functions listed in Step 7.

- [ ] **Step 1: Create `src/models/invoice.ts`**

```ts
import type { TextualYearMonthDay } from '@/core/datetime.ts';

export const INVOICE_CURRENCY: string = 'INR';

export interface InvoiceProfileInfoResponse {
    readonly businessName: string;
    readonly addressLines: string;
    readonly phone: string;
    readonly website: string;
    readonly logoDataUrl: string;
    readonly defaultNotes: string;
    readonly numberPrefix: string;
    readonly defaultTermsDays: number;
}

export type InvoiceProfileUpdateRequest = InvoiceProfileInfoResponse;

export interface CustomerInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
}

export interface CustomerCreateRequest {
    readonly name: string;
    readonly contactName: string;
    readonly addressLines: string;
    readonly email: string;
}

export interface CustomerModifyRequest extends CustomerCreateRequest {
    readonly id: string;
}

export interface InvoiceLine {
    itemName: string;
    description: string;
    quantity: number;
    rate: number;
    amount: number;
}

export interface InvoicePayment {
    readonly transactionId: string;
    readonly amount: number;
    readonly transactionTime: number;
    readonly accountId: string;
    readonly comment: string;
}

export interface InvoiceInfoResponse {
    readonly id: string;
    readonly kind: string;
    readonly number: string;
    readonly customerId: string;
    readonly billToName: string;
    readonly billToContactName: string;
    readonly billToAddressLines: string;
    readonly billToEmail: string;
    readonly currency: string;
    readonly invoiceDate: TextualYearMonthDay;
    readonly dueDate: TextualYearMonthDay;
    readonly notes: string;
    readonly total: number;
    readonly received: number;
    readonly closedManually: boolean;
    readonly lines?: InvoiceLine[];
    readonly payments?: InvoicePayment[];
}

export interface InvoiceLineRequest {
    readonly itemName: string;
    readonly description: string;
    readonly quantity: number;
    readonly rate: number;
}

export interface InvoiceCreateRequest {
    readonly number: string;
    readonly customerId: string;
    readonly invoiceDate: string;
    readonly dueDate: string;
    readonly notes: string;
    readonly lines: InvoiceLineRequest[];
}

export interface InvoiceModifyRequest extends InvoiceCreateRequest {
    readonly id: string;
}

export class InvoiceProfile implements InvoiceProfileInfoResponse {
    public businessName: string = '';
    public addressLines: string = '';
    public phone: string = '';
    public website: string = '';
    public logoDataUrl: string = '';
    public defaultNotes: string = '';
    public numberPrefix: string = 'INV';
    public defaultTermsDays: number = 7;

    public fillFrom(other: InvoiceProfileInfoResponse): void {
        this.businessName = other.businessName;
        this.addressLines = other.addressLines;
        this.phone = other.phone;
        this.website = other.website;
        this.logoDataUrl = other.logoDataUrl;
        this.defaultNotes = other.defaultNotes;
        this.numberPrefix = other.numberPrefix;
        this.defaultTermsDays = other.defaultTermsDays;
    }

    public toUpdateRequest(): InvoiceProfileUpdateRequest {
        return {
            businessName: this.businessName,
            addressLines: this.addressLines,
            phone: this.phone,
            website: this.website,
            logoDataUrl: this.logoDataUrl,
            defaultNotes: this.defaultNotes,
            numberPrefix: this.numberPrefix.trim(),
            defaultTermsDays: this.defaultTermsDays
        };
    }

    public clone(): InvoiceProfile {
        return InvoiceProfile.of(this);
    }

    public static of(response: InvoiceProfileInfoResponse): InvoiceProfile {
        const profile = new InvoiceProfile();
        profile.fillFrom(response);
        return profile;
    }

    public static createNewProfile(): InvoiceProfile {
        return new InvoiceProfile();
    }
}

export class Customer implements CustomerInfoResponse {
    public id: string = '';
    public name: string = '';
    public contactName: string = '';
    public addressLines: string = '';
    public email: string = '';

    public fillFrom(other: CustomerInfoResponse): void {
        this.id = other.id;
        this.name = other.name;
        this.contactName = other.contactName;
        this.addressLines = other.addressLines;
        this.email = other.email;
    }

    public toCreateRequest(): CustomerCreateRequest {
        return {
            name: this.name.trim(),
            contactName: this.contactName,
            addressLines: this.addressLines,
            email: this.email
        };
    }

    public toModifyRequest(): CustomerModifyRequest {
        return {
            id: this.id,
            ...this.toCreateRequest()
        };
    }

    public clone(): Customer {
        return Customer.of(this);
    }

    public static of(response: CustomerInfoResponse): Customer {
        const customer = new Customer();
        customer.fillFrom(response);
        return customer;
    }

    public static ofMulti(responses: CustomerInfoResponse[]): Customer[] {
        return responses.map(response => Customer.of(response));
    }

    public static createNewCustomer(): Customer {
        return new Customer();
    }
}

export class Invoice implements InvoiceInfoResponse {
    public id: string = '';
    public kind: string = 'invoice';
    public number: string = '';
    public customerId: string = '';
    public billToName: string = '';
    public billToContactName: string = '';
    public billToAddressLines: string = '';
    public billToEmail: string = '';
    public currency: string = INVOICE_CURRENCY;
    public invoiceDate: TextualYearMonthDay = '0-0-0';
    public dueDate: TextualYearMonthDay = '0-0-0';
    public notes: string = '';
    public total: number = 0;
    public received: number = 0;
    public closedManually: boolean = false;
    public lines: InvoiceLine[] = [];
    public payments: InvoicePayment[] = [];

    public fillFrom(other: InvoiceInfoResponse): void {
        this.id = other.id;
        this.kind = other.kind;
        this.number = other.number;
        this.customerId = other.customerId;
        this.billToName = other.billToName;
        this.billToContactName = other.billToContactName;
        this.billToAddressLines = other.billToAddressLines;
        this.billToEmail = other.billToEmail;
        this.currency = other.currency;
        this.invoiceDate = other.invoiceDate;
        this.dueDate = other.dueDate;
        this.notes = other.notes;
        this.total = other.total;
        this.received = other.received;
        this.closedManually = other.closedManually;
        this.lines = (other.lines ?? []).map(line => ({ ...line }));
        this.payments = (other.payments ?? []).map(payment => ({ ...payment }));
    }

    public toCreateRequest(): InvoiceCreateRequest {
        return {
            number: this.number.trim(),
            customerId: this.customerId,
            invoiceDate: this.invoiceDate,
            dueDate: this.dueDate,
            notes: this.notes,
            lines: this.lines.map(line => ({
                itemName: line.itemName.trim(),
                description: line.description,
                quantity: line.quantity,
                rate: line.rate
            }))
        };
    }

    public toModifyRequest(): InvoiceModifyRequest {
        return {
            id: this.id,
            ...this.toCreateRequest()
        };
    }

    public clone(): Invoice {
        return Invoice.of(this);
    }

    // a new unsaved invoice with this invoice's customer, lines and notes (the "Duplicate" action)
    public duplicate(): Invoice {
        const invoice = Invoice.createNewInvoice();
        invoice.customerId = this.customerId;
        invoice.notes = this.notes;
        invoice.lines = this.lines.map(line => ({ ...line }));
        return invoice;
    }

    public static of(response: InvoiceInfoResponse): Invoice {
        const invoice = new Invoice();
        invoice.fillFrom(response);
        return invoice;
    }

    public static ofMulti(responses: InvoiceInfoResponse[]): Invoice[] {
        return responses.map(response => Invoice.of(response));
    }

    public static createNewInvoice(): Invoice {
        const invoice = new Invoice();
        invoice.lines = [Invoice.createNewLine()];
        return invoice;
    }

    public static createNewLine(): InvoiceLine {
        return { itemName: '', description: '', quantity: 1, rate: 0, amount: 0 };
    }
}
```

- [ ] **Step 2: Write the failing vitest** `src/lib/__tests__/invoice.test.ts`

```ts
import { describe, expect, test } from 'vitest';

import {
    computeInvoiceTotals,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    isInvoiceOverdue,
    getInvoiceDisplayStatus,
    getInvoiceBankCharges,
    formatInvoiceDocumentAmount,
    formatInvoiceDocumentDate,
    getInvoicePdfFileName,
    addDaysToDate
} from '@/lib/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const invoice = (total: number, received: number, closedManually = false, dueDate: TextualYearMonthDay = '2026-04-30') =>
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

describe('overdue', () => {
    test('only unpaid or partial invoices past the due date', () => {
        expect(isInvoiceOverdue(invoice(100, 0), '2026-05-01')).toBe(true);
        expect(isInvoiceOverdue(invoice(100, 0), '2026-04-30')).toBe(false);
        expect(isInvoiceOverdue(invoice(100, 100), '2026-05-01')).toBe(false);
        expect(getInvoiceDisplayStatus(invoice(100, 50), '2026-05-01')).toBe('overdue');
        expect(getInvoiceDisplayStatus(invoice(100, 50), '2026-04-01')).toBe('partial');
    });
});

describe('document formatting', () => {
    test('amount like the Wave sample', () => {
        expect(formatInvoiceDocumentAmount(37000000)).toBe('₹370,000.00');
    });

    test('date like the Wave sample', () => {
        expect(formatInvoiceDocumentDate('2026-04-23')).toBe('April 23, 2026');
    });

    test('pdf file name like the Wave sample', () => {
        expect(getInvoicePdfFileName('CA/2026-27/01', '2026-04-30')).toBe('Invoice_CA_2026-27_01_2026-04-30');
    });

    test('add days across month end', () => {
        expect(addDaysToDate('2026-04-23', 7)).toBe('2026-04-30');
        expect(addDaysToDate('2026-04-28', 7)).toBe('2026-05-05');
    });
});
```

- [ ] **Step 3: Run to verify it fails**

Run: `npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: FAIL, cannot resolve `@/lib/invoice.ts`.

- [ ] **Step 4: Create `src/lib/invoice.ts`**

```ts
import type { TextualYearMonthDay } from '@/core/datetime.ts';

// mirrors models.InvoicePaidTolerance in pkg/models/invoice.go
export const INVOICE_PAID_TOLERANCE = 0.03;

export type InvoiceStatus = 'unpaid' | 'partial' | 'paid';
export type InvoiceDisplayStatus = InvoiceStatus | 'overdue';

interface InvoiceAmounts {
    readonly total: number;
    readonly received: number;
    readonly closedManually: boolean;
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

export function isInvoiceOverdue(invoice: InvoiceAmounts & { readonly dueDate: string }, today: string): boolean {
    return getInvoiceStatus(invoice) !== 'paid' && invoice.dueDate < today;
}

export function getInvoiceDisplayStatus(invoice: InvoiceAmounts & { readonly dueDate: string }, today: string): InvoiceDisplayStatus {
    return isInvoiceOverdue(invoice, today) ? 'overdue' : getInvoiceStatus(invoice);
}

// the shortfall written off on a paid invoice (bank / forex charges, or a manual close)
export function getInvoiceBankCharges(invoice: InvoiceAmounts): number {
    return getInvoiceStatus(invoice) === 'paid' ? getInvoiceBalanceDue(invoice) : 0;
}

// ponytail: the printed document is always English / en-US style like the Wave original; app screens use the i18n helpers instead
const invoiceDocumentAmountFormat = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'INR' });

export function formatInvoiceDocumentAmount(paise: number): string {
    return invoiceDocumentAmountFormat.format(paise / 100);
}

export function formatInvoiceDocumentDate(date: string): string {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day!)).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' });
}

export function getInvoicePdfFileName(number: string, dueDate: string): string {
    return `Invoice_${number.replace(/[\\/:*?"<>|\s]+/g, '_')}_${dueDate}`;
}

export function addDaysToDate(date: string, days: number): TextualYearMonthDay {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day! + days)).toISOString().substring(0, 10) as TextualYearMonthDay;
}

export function getTodayTextualDate(): TextualYearMonthDay {
    const now = new Date();
    return new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())).toISOString().substring(0, 10) as TextualYearMonthDay;
}
```

- [ ] **Step 5: Run the vitest**

Run: `npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: all PASS.

- [ ] **Step 6: Add API calls to `src/lib/services.ts`**

Imports, next to the bucket model imports:

```ts
import type {
    InvoiceProfileInfoResponse,
    InvoiceProfileUpdateRequest,
    CustomerInfoResponse,
    CustomerCreateRequest,
    CustomerModifyRequest,
    InvoiceInfoResponse,
    InvoiceCreateRequest,
    InvoiceModifyRequest,
    InvoicePayment
} from '@/models/invoice.ts';
```

Methods, after the bucket methods in the same object literal:

```ts
    getInvoiceProfile: (): ApiResponsePromise<InvoiceProfileInfoResponse> => {
        return axios.get<ApiResponse<InvoiceProfileInfoResponse>>('v1/invoice_profile/get.json');
    },
    updateInvoiceProfile: (req: InvoiceProfileUpdateRequest): ApiResponsePromise<InvoiceProfileInfoResponse> => {
        return axios.post<ApiResponse<InvoiceProfileInfoResponse>>('v1/invoice_profile/update.json', req);
    },
    getAllCustomers: (): ApiResponsePromise<CustomerInfoResponse[]> => {
        return axios.get<ApiResponse<CustomerInfoResponse[]>>('v1/customers/list.json');
    },
    addCustomer: (req: CustomerCreateRequest): ApiResponsePromise<CustomerInfoResponse> => {
        return axios.post<ApiResponse<CustomerInfoResponse>>('v1/customers/add.json', req);
    },
    modifyCustomer: (req: CustomerModifyRequest): ApiResponsePromise<CustomerInfoResponse> => {
        return axios.post<ApiResponse<CustomerInfoResponse>>('v1/customers/modify.json', req);
    },
    deleteCustomer: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/customers/delete.json', { id });
    },
    getAllInvoices: (): ApiResponsePromise<InvoiceInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceInfoResponse[]>>('v1/invoices/list.json');
    },
    getInvoice: ({ id }: { id: string }): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.get<ApiResponse<InvoiceInfoResponse>>('v1/invoices/get.json?id=' + id);
    },
    addInvoice: (req: InvoiceCreateRequest): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.post<ApiResponse<InvoiceInfoResponse>>('v1/invoices/add.json', req);
    },
    modifyInvoice: (req: InvoiceModifyRequest): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.post<ApiResponse<InvoiceInfoResponse>>('v1/invoices/modify.json', req);
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
        return axios.get<ApiResponse<string>>('v1/invoices/next_number.json?invoice_date=' + invoiceDate);
    },
    getInvoicePaymentSuggestions: ({ id }: { id: string }): ApiResponsePromise<InvoicePayment[]> => {
        return axios.get<ApiResponse<InvoicePayment[]>>('v1/invoices/payment/suggestions.json?id=' + id);
    },
    linkInvoicePayment: (req: { invoiceId: string, transactionId: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/payment/link.json', req);
    },
    unlinkInvoicePayment: (req: { invoiceId: string, transactionId: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/invoices/payment/unlink.json', req);
    },
```

- [ ] **Step 7: Create `src/stores/invoice.ts`** (state + mutators + load/get/save/delete like `src/stores/transactionTemplate.ts`)

```ts
import { ref } from 'vue';
import { defineStore } from 'pinia';

import {
    type InvoicePayment,
    InvoiceProfile,
    Customer,
    Invoice
} from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';
import services, { type ApiResponsePromise } from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useInvoicesStore = defineStore('invoices', () => {
    const allInvoices = ref<Invoice[]>([]);
    const allInvoicesMap = ref<Record<string, Invoice>>({});
    const allCustomers = ref<Customer[]>([]);
    const allCustomersMap = ref<Record<string, Customer>>({});
    const customerListStateInvalid = ref<boolean>(true);
    const invoiceProfile = ref<InvoiceProfile | null>(null);

    // unwraps the API envelope and normalises errors exactly like the other stores' inline handlers
    function request<T>(apiRequest: ApiResponsePromise<T>, failedMessage: string): Promise<T> {
        return new Promise((resolve, reject) => {
            apiRequest.then(response => {
                const data = response.data;

                if (!data || !data.success || data.result === undefined || data.result === null) {
                    reject({ message: failedMessage });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error(failedMessage, error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: failedMessage });
                } else {
                    reject(error);
                }
            });
        });
    }

    function loadInvoiceList(invoices: Invoice[]): void {
        allInvoices.value = invoices;
        allInvoicesMap.value = {};

        for (const invoice of invoices) {
            allInvoicesMap.value[invoice.id] = invoice;
        }
    }

    function loadCustomerList(customers: Customer[]): void {
        allCustomers.value = customers;
        allCustomersMap.value = {};

        for (const customer of customers) {
            allCustomersMap.value[customer.id] = customer;
        }
    }

    function removeInvoiceFromList(invoiceId: string): void {
        allInvoices.value = allInvoices.value.filter(invoice => invoice.id !== invoiceId);
        delete allInvoicesMap.value[invoiceId];
    }

    function removeCustomerFromList(customerId: string): void {
        allCustomers.value = allCustomers.value.filter(customer => customer.id !== customerId);
        delete allCustomersMap.value[customerId];
    }

    function resetInvoices(): void {
        allInvoices.value = [];
        allInvoicesMap.value = {};
        allCustomers.value = [];
        allCustomersMap.value = {};
        customerListStateInvalid.value = true;
        invoiceProfile.value = null;
    }

    // ponytail: the invoice list is never served from cache, because "received" depends on transactions edited on other pages
    function loadAllInvoices({ force }: { force?: boolean }): Promise<Invoice[]> {
        return request(services.getAllInvoices(), 'Unable to retrieve invoice list').then(result => {
            const invoices = Invoice.ofMulti(result);

            if (force && isEquals(allInvoices.value, invoices)) {
                return Promise.reject({ message: 'Invoice list is up to date', isUpToDate: true });
            }

            loadInvoiceList(invoices);
            return invoices;
        });
    }

    function getInvoice({ invoiceId }: { invoiceId: string }): Promise<Invoice> {
        return request(services.getInvoice({ id: invoiceId }), 'Unable to retrieve invoice').then(result => Invoice.of(result));
    }

    function saveInvoice({ invoice, isEdit }: { invoice: Invoice, isEdit: boolean }): Promise<Invoice> {
        const apiRequest = isEdit ? services.modifyInvoice(invoice.toModifyRequest()) : services.addInvoice(invoice.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save invoice' : 'Unable to add invoice').then(result => Invoice.of(result));
    }

    function deleteInvoice({ invoice, beforeResolve }: { invoice: Invoice, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteInvoice({ id: invoice.id }), 'Unable to delete this invoice').then(result => {
            if (beforeResolve) {
                beforeResolve(() => removeInvoiceFromList(invoice.id));
            } else {
                removeInvoiceFromList(invoice.id);
            }

            return result;
        });
    }

    function setInvoiceClosed({ invoice, closed }: { invoice: Invoice, closed: boolean }): Promise<boolean> {
        const apiRequest = closed ? services.closeInvoice({ id: invoice.id }) : services.reopenInvoice({ id: invoice.id });
        return request(apiRequest, 'Unable to save invoice');
    }

    function getNextInvoiceNumber({ invoiceDate }: { invoiceDate: string }): Promise<string> {
        return request(services.getNextInvoiceNumber({ invoiceDate }), 'Unable to get next invoice number');
    }

    function getInvoicePaymentSuggestions({ invoiceId }: { invoiceId: string }): Promise<InvoicePayment[]> {
        return request(services.getInvoicePaymentSuggestions({ id: invoiceId }), 'Unable to retrieve matching transactions');
    }

    function linkInvoicePayment({ invoiceId, transactionId }: { invoiceId: string, transactionId: string }): Promise<boolean> {
        return request(services.linkInvoicePayment({ invoiceId, transactionId }), 'Unable to link this transaction');
    }

    function unlinkInvoicePayment({ invoiceId, transactionId }: { invoiceId: string, transactionId: string }): Promise<boolean> {
        return request(services.unlinkInvoicePayment({ invoiceId, transactionId }), 'Unable to unlink this transaction');
    }

    function loadAllCustomers({ force }: { force?: boolean }): Promise<Customer[]> {
        if (!force && !customerListStateInvalid.value) {
            return Promise.resolve(allCustomers.value);
        }

        return request(services.getAllCustomers(), 'Unable to retrieve customer list').then(result => {
            const customers = Customer.ofMulti(result);
            customerListStateInvalid.value = false;

            if (force && isEquals(allCustomers.value, customers)) {
                return Promise.reject({ message: 'Customer list is up to date', isUpToDate: true });
            }

            loadCustomerList(customers);
            return customers;
        });
    }

    function saveCustomer({ customer, isEdit }: { customer: Customer, isEdit: boolean }): Promise<Customer> {
        const apiRequest = isEdit ? services.modifyCustomer(customer.toModifyRequest()) : services.addCustomer(customer.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save customer' : 'Unable to add customer').then(result => {
            const saved = Customer.of(result);

            if (isEdit) {
                allCustomers.value = allCustomers.value.map(item => item.id === saved.id ? saved : item);
            } else {
                allCustomers.value = [...allCustomers.value, saved].sort((a, b) => a.name.localeCompare(b.name));
            }

            allCustomersMap.value[saved.id] = saved;
            return saved;
        });
    }

    function deleteCustomer({ customer, beforeResolve }: { customer: Customer, beforeResolve?: (done: () => void) => void }): Promise<boolean> {
        return request(services.deleteCustomer({ id: customer.id }), 'Unable to delete this customer').then(result => {
            if (beforeResolve) {
                beforeResolve(() => removeCustomerFromList(customer.id));
            } else {
                removeCustomerFromList(customer.id);
            }

            return result;
        });
    }

    function loadInvoiceProfile({ force }: { force?: boolean }): Promise<InvoiceProfile> {
        if (!force && invoiceProfile.value) {
            return Promise.resolve(invoiceProfile.value);
        }

        return request(services.getInvoiceProfile(), 'Unable to retrieve invoice profile').then(result => {
            invoiceProfile.value = InvoiceProfile.of(result);
            return invoiceProfile.value;
        });
    }

    function updateInvoiceProfile({ profile }: { profile: InvoiceProfile }): Promise<InvoiceProfile> {
        return request(services.updateInvoiceProfile(profile.toUpdateRequest()), 'Unable to save invoice profile').then(result => {
            invoiceProfile.value = InvoiceProfile.of(result);
            return invoiceProfile.value;
        });
    }

    return {
        // states
        allInvoices,
        allInvoicesMap,
        allCustomers,
        allCustomersMap,
        customerListStateInvalid,
        invoiceProfile,
        // functions
        resetInvoices,
        loadAllInvoices,
        getInvoice,
        saveInvoice,
        deleteInvoice,
        setInvoiceClosed,
        getNextInvoiceNumber,
        getInvoicePaymentSuggestions,
        linkInvoicePayment,
        unlinkInvoicePayment,
        loadAllCustomers,
        saveCustomer,
        deleteCustomer,
        loadInvoiceProfile,
        updateInvoiceProfile
    };
});
```

- [ ] **Step 8: Reset on logout** in `src/stores/index.ts`

Add `import { useInvoicesStore } from '@/stores/invoice.ts';` with the other store imports, `const invoicesStore = useInvoicesStore();` after `const userCustomIconsStore = useUserCustomIconsStore();`, and `invoicesStore.resetInvoices();` as the first line after `setNotificationContent(null);` in `resetAllStates`.

- [ ] **Step 9: Type-check, lint, test**

Run: `npx vue-tsc --noEmit && npx eslint src/models/invoice.ts src/lib/invoice.ts src/stores/invoice.ts src/stores/index.ts src/lib/services.ts && npx vitest run src/lib/__tests__/invoice.test.ts`
Expected: clean; tests PASS.

- [ ] **Step 10: Commit**

```bash
git add src/models/invoice.ts src/lib/invoice.ts src/lib/__tests__/invoice.test.ts src/lib/services.ts src/stores/invoice.ts src/stores/index.ts
git commit --no-verify -m "invoices: frontend models, helpers, API calls and store"
```

---

### Task 7: Shared page logic (`src/views/base/invoices/`)

**Files:**
- Create: `src/views/base/invoices/InvoiceListPageBase.ts`, `InvoiceEditPageBase.ts`, `InvoiceDetailPageBase.ts`, `CustomerEditPageBase.ts`, `InvoiceProfilePageBase.ts`

**Interfaces:**
- Consumes: Task 6 models, helpers, store; `useI18n` from `@/locales/helpers.ts`; `useAccountsStore` (`allAccountsMap`).
- Produces (exact names; desktop and mobile pages destructure these):
  - `useInvoiceListPageBase()` → `{ loading, showPaid, today, allInvoices, displayInvoices, noAvailableInvoice, outstandingTotal, overdueTotal, formatAmount, formatDate, getDisplayStatus, getDisplayStatusName }`
  - `useInvoiceEditPageBase()` → `{ editInvoiceId, loading, submitting, invoice, allCustomers, title, saveButtonTitle, inputEmptyProblemMessage, inputIsEmpty, totals, addLine, removeLine, setInvoice, prepareNewInvoice, setInvoiceDate, formatAmount, formatDate }`
  - `useInvoiceDetailPageBase()` → `{ loading, updating, invoice, profile, suggestions, displayStatus, displayStatusName, balanceDue, bankCharges, formatAmount, formatDate, formatPaymentDescription, printInvoice }`
  - `useCustomerEditPageBase()` → `{ editCustomerId, loading, submitting, customer, title, saveButtonTitle, inputEmptyProblemMessage, inputIsEmpty }`
  - `useInvoiceProfilePageBase()` → `{ loading, saving, newProfile, oldProfile, inputIsNotChanged, inputInvalidProblemMessage, inputIsInvalid, setProfile, reset, readLogoFile }`
  - `getInvoiceDisplayStatusName(status: InvoiceDisplayStatus): string` exported from `InvoiceListPageBase.ts` (English key).

- [ ] **Step 1: Create `src/views/base/invoices/InvoiceListPageBase.ts`**

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    type InvoiceDisplayStatus,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    getInvoiceDisplayStatus,
    isInvoiceOverdue,
    getTodayTextualDate
} from '@/lib/invoice.ts';

export function getInvoiceDisplayStatusName(status: InvoiceDisplayStatus): string {
    const names: Record<InvoiceDisplayStatus, string> = {
        unpaid: 'Unpaid',
        partial: 'Partially Paid',
        paid: 'Paid',
        overdue: 'Overdue'
    };

    return names[status];
}

export function useInvoiceListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const showPaid = ref<boolean>(true);
    const today = getTodayTextualDate();

    const allInvoices = computed<Invoice[]>(() => invoicesStore.allInvoices);
    const displayInvoices = computed<Invoice[]>(() => showPaid.value ? allInvoices.value : allInvoices.value.filter(invoice => getInvoiceStatus(invoice) !== 'paid'));
    const noAvailableInvoice = computed<boolean>(() => displayInvoices.value.length < 1);

    const outstandingTotal = computed<number>(() => allInvoices.value
        .filter(invoice => getInvoiceStatus(invoice) !== 'paid')
        .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

    const overdueTotal = computed<number>(() => allInvoices.value
        .filter(invoice => isInvoiceOverdue(invoice, today))
        .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    function getDisplayStatus(invoice: Invoice): InvoiceDisplayStatus {
        return getInvoiceDisplayStatus(invoice, today);
    }

    function getDisplayStatusName(invoice: Invoice): string {
        return getInvoiceDisplayStatusName(getDisplayStatus(invoice));
    }

    return {
        // states
        loading,
        showPaid,
        today,
        // computed states
        allInvoices,
        displayInvoices,
        noAvailableInvoice,
        outstandingTotal,
        overdueTotal,
        // functions
        formatAmount,
        formatDate,
        getDisplayStatus,
        getDisplayStatusName
    };
}
```

- [ ] **Step 2: Create `src/views/base/invoices/InvoiceEditPageBase.ts`**

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Customer, type InvoiceProfile, Invoice, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { computeInvoiceTotals, addDaysToDate, getTodayTextualDate } from '@/lib/invoice.ts';

export function useInvoiceEditPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const editInvoiceId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const invoice = ref<Invoice>(Invoice.createNewInvoice());

    const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

    const title = computed<string>(() => editInvoiceId.value ? 'Edit Invoice' : 'Add Invoice');
    const saveButtonTitle = computed<string>(() => editInvoiceId.value ? 'Save' : 'Add');

    const totals = computed<{ subtotal: number, total: number }>(() => computeInvoiceTotals(invoice.value.lines));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!invoice.value.customerId) {
            return 'Customer cannot be blank';
        } else if (!invoice.value.number.trim()) {
            return 'Invoice number cannot be blank';
        } else if (invoice.value.dueDate < invoice.value.invoiceDate) {
            return 'Due date cannot be before the invoice date';
        } else if (invoice.value.lines.length < 1) {
            return 'Invoice must have at least one line';
        }

        for (const line of invoice.value.lines) {
            if (!line.itemName.trim()) {
                return 'Item name cannot be blank';
            } else if (line.quantity < 1 || line.rate <= 0) {
                return 'Quantity and rate must be greater than zero';
            }
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function addLine(): void {
        invoice.value.lines.push(Invoice.createNewLine());
    }

    function removeLine(index: number): void {
        invoice.value.lines.splice(index, 1);
    }

    function setInvoice(source: Invoice): void {
        invoice.value = source.clone();
    }

    // a new invoice dated today with the profile's terms and notes, optionally copying another invoice
    function prepareNewInvoice(profile: InvoiceProfile, duplicateFrom?: Invoice | null): void {
        const newInvoice = duplicateFrom ? duplicateFrom.duplicate() : Invoice.createNewInvoice();
        const today = getTodayTextualDate();

        newInvoice.invoiceDate = today;
        newInvoice.dueDate = addDaysToDate(today, profile.defaultTermsDays);

        if (!duplicateFrom) {
            newInvoice.notes = profile.defaultNotes;
        }

        invoice.value = newInvoice;
    }

    function setInvoiceDate(date: TextualYearMonthDay, termsDays: number): void {
        invoice.value.invoiceDate = date;
        invoice.value.dueDate = addDaysToDate(date, termsDays);
    }

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        editInvoiceId,
        loading,
        submitting,
        invoice,
        // computed states
        allCustomers,
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        totals,
        // functions
        addLine,
        removeLine,
        setInvoice,
        prepareNewInvoice,
        setInvoiceDate,
        formatAmount,
        formatDate
    };
}
```

- [ ] **Step 3: Create `src/views/base/invoices/InvoiceDetailPageBase.ts`**

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, type InvoicePayment, type InvoiceProfile, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';
import {
    type InvoiceDisplayStatus,
    getInvoiceBalanceDue,
    getInvoiceBankCharges,
    getInvoiceDisplayStatus,
    getInvoicePdfFileName,
    getTodayTextualDate
} from '@/lib/invoice.ts';

import { getInvoiceDisplayStatusName } from './InvoiceListPageBase.ts';

export function useInvoiceDetailPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDate } = useI18n();

    const accountsStore = useAccountsStore();

    const loading = ref<boolean>(true);
    const updating = ref<boolean>(false);
    const invoice = ref<Invoice | null>(null);
    const profile = ref<InvoiceProfile | null>(null);
    const suggestions = ref<InvoicePayment[]>([]);

    const displayStatus = computed<InvoiceDisplayStatus | null>(() => invoice.value ? getInvoiceDisplayStatus(invoice.value, getTodayTextualDate()) : null);
    const displayStatusName = computed<string>(() => displayStatus.value ? getInvoiceDisplayStatusName(displayStatus.value) : '');
    const balanceDue = computed<number>(() => invoice.value ? getInvoiceBalanceDue(invoice.value) : 0);
    const bankCharges = computed<number>(() => invoice.value ? getInvoiceBankCharges(invoice.value) : 0);

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    // "April 29, 2026 · PNB Savings · NEFT-QUBER TECH"
    function formatPaymentDescription(payment: InvoicePayment): string {
        const date = formatDateTimeToLongDate(parseDateTimeFromUnixTimeWithTimezoneOffset(payment.transactionTime, getTimezoneOffsetMinutes(payment.transactionTime)));
        const accountName = accountsStore.allAccountsMap[payment.accountId]?.name ?? '';
        return [date, accountName, payment.comment].filter(part => !!part).join(' · ');
    }

    // the browser suggests document.title as the PDF file name
    function printInvoice(): void {
        if (!invoice.value) {
            return;
        }

        const originalTitle = document.title;
        document.title = getInvoicePdfFileName(invoice.value.number, invoice.value.dueDate);
        window.addEventListener('afterprint', () => {
            document.title = originalTitle;
        }, { once: true });
        window.print();
    }

    return {
        // states
        loading,
        updating,
        invoice,
        profile,
        suggestions,
        // computed states
        displayStatus,
        displayStatusName,
        balanceDue,
        bankCharges,
        // functions
        formatAmount,
        formatDate,
        formatPaymentDescription,
        printInvoice
    };
}
```

- [ ] **Step 4: Create `src/views/base/invoices/CustomerEditPageBase.ts`**

```ts
import { ref, computed } from 'vue';

import { Customer } from '@/models/invoice.ts';

export function useCustomerEditPageBase() {
    const editCustomerId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const customer = ref<Customer>(Customer.createNewCustomer());

    const title = computed<string>(() => editCustomerId.value ? 'Edit Customer' : 'Add Customer');
    const saveButtonTitle = computed<string>(() => editCustomerId.value ? 'Save' : 'Add');

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!customer.value.name.trim()) {
            return 'Customer name cannot be blank';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    return {
        // states
        editCustomerId,
        loading,
        submitting,
        customer,
        // computed states
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty
    };
}
```

- [ ] **Step 5: Create `src/views/base/invoices/InvoiceProfilePageBase.ts`**

```ts
import { ref, computed } from 'vue';

import { InvoiceProfile } from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';

// matches the backend's 350,000-character limit on logoDataUrl (base64 grows a file by about a third)
const MAX_INVOICE_LOGO_FILE_SIZE = 250 * 1024;

export function useInvoiceProfilePageBase() {
    const loading = ref<boolean>(true);
    const saving = ref<boolean>(false);
    const newProfile = ref<InvoiceProfile>(InvoiceProfile.createNewProfile());
    const oldProfile = ref<InvoiceProfile>(InvoiceProfile.createNewProfile());

    const inputIsNotChanged = computed<boolean>(() => isEquals(newProfile.value.toUpdateRequest(), oldProfile.value.toUpdateRequest()));

    const inputInvalidProblemMessage = computed<string | null>(() => {
        if (!newProfile.value.numberPrefix.trim()) {
            return 'Invoice number prefix cannot be blank';
        } else if (newProfile.value.defaultTermsDays < 0 || newProfile.value.defaultTermsDays > 365) {
            return 'Payment terms must be between 0 and 365 days';
        }

        return null;
    });

    const inputIsInvalid = computed<boolean>(() => !!inputInvalidProblemMessage.value);

    function setProfile(profile: InvoiceProfile): void {
        newProfile.value = profile.clone();
        oldProfile.value = profile.clone();
    }

    function reset(): void {
        newProfile.value = oldProfile.value.clone();
    }

    // reads a png / jpeg file into a data url; rejects with an English message key
    function readLogoFile(file: File): Promise<string> {
        return new Promise((resolve, reject) => {
            if (file.type !== 'image/png' && file.type !== 'image/jpeg') {
                reject({ message: 'The logo must be a PNG or JPEG image' });
                return;
            }

            if (file.size > MAX_INVOICE_LOGO_FILE_SIZE) {
                reject({ message: 'Logo must be smaller than 250 KB' });
                return;
            }

            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.onerror = () => reject({ message: 'Unable to read this file' });
            reader.readAsDataURL(file);
        });
    }

    return {
        // states
        loading,
        saving,
        newProfile,
        oldProfile,
        // computed states
        inputIsNotChanged,
        inputInvalidProblemMessage,
        inputIsInvalid,
        // functions
        setProfile,
        reset,
        readLogoFile
    };
}
```

- [ ] **Step 6: Type-check and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/base/invoices`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add src/views/base/invoices
git commit --no-verify -m "invoices: shared page logic"
```

---

### Task 8: Printable invoice document (the one new component)

**Files:**
- Create: `src/components/common/InvoiceDocument.vue`

**Why new:** nothing in the app renders or prints a document (no `@media print` anywhere in `src`). It is shared by the desktop detail dialog and the mobile detail page, so it lives outside `components/desktop` and `components/mobile`. It is a plain-HTML component with no Vuetify or Framework7 tags so both apps can render it, and it is pure (props in, markup out) so a server-side renderer for emailed invoices can later mirror it.

**Interfaces:**
- Consumes: Task 6 `Invoice`, `InvoiceProfile`, `formatInvoiceDocumentAmount`, `formatInvoiceDocumentDate`, `getInvoiceBalanceDue`.
- Produces: `<invoice-document :invoice="Invoice" :profile="InvoiceProfile" />`. Wrapping element class `invoice-document`; while printing, only `.invoice-document` is visible.

- [ ] **Step 1: Create `src/components/common/InvoiceDocument.vue`**

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
                <tr><th>Invoice Date:</th><td>{{ formatInvoiceDocumentDate(invoice.invoiceDate) }}</td></tr>
                <tr><th>Payment Due:</th><td>{{ formatInvoiceDocumentDate(invoice.dueDate) }}</td></tr>
                <tr class="invoice-meta-highlight"><th>Amount Due ({{ invoice.currency }}):</th><td class="invoice-strong">{{ formatInvoiceDocumentAmount(balanceDue) }}</td></tr>
                </tbody>
            </table>
        </div>

        <table class="invoice-lines">
            <thead>
            <tr>
                <th class="invoice-left">Services</th>
                <th class="invoice-right" v-if="showQuantity">Quantity</th>
                <th class="invoice-right">Rate</th>
                <th class="invoice-right">Amount</th>
            </tr>
            </thead>
            <tbody>
            <tr :key="index" v-for="(line, index) in invoice.lines">
                <td>
                    <div class="invoice-strong">{{ line.itemName }}</div>
                    <div class="invoice-muted invoice-pre">{{ line.description }}</div>
                </td>
                <td class="invoice-right" v-if="showQuantity">{{ line.quantity }}</td>
                <td class="invoice-right">{{ formatInvoiceDocumentAmount(line.rate) }}</td>
                <td class="invoice-right">{{ formatInvoiceDocumentAmount(line.amount) }}</td>
            </tr>
            </tbody>
        </table>

        <table class="invoice-totals">
            <tbody>
            <tr><th>Total:</th><td>{{ formatInvoiceDocumentAmount(invoice.total) }}</td></tr>
            <tr v-if="invoice.received > 0"><th>Payments received:</th><td>−{{ formatInvoiceDocumentAmount(Math.min(invoice.received, invoice.total)) }}</td></tr>
            <tr class="invoice-totals-due"><th>Amount Due ({{ invoice.currency }}):</th><td>{{ formatInvoiceDocumentAmount(balanceDue) }}</td></tr>
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
import { formatInvoiceDocumentAmount, formatInvoiceDocumentDate, getInvoiceBalanceDue } from '@/lib/invoice.ts';

const props = defineProps<{
    invoice: Invoice;
    profile: InvoiceProfile;
}>();

const balanceDue = computed<number>(() => getInvoiceBalanceDue(props.invoice));
const showQuantity = computed<boolean>(() => props.invoice.lines.some(line => line.quantity !== 1));
</script>

<style scoped>
/* A document for the customer: always white paper and black text, whatever the app theme or language. */
.invoice-document {
    --invoice-accent: #3d86f5;
    background: #fff;
    color: #000;
    font-family: Georgia, 'Noto Serif', 'Times New Roman', serif; /* these fonts carry the ₹ glyph */
    font-size: 13px;
    line-height: 1.35;
    max-width: 800px;
    min-width: 640px;
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

.invoice-left {
    text-align: left;
}

.invoice-right {
    text-align: right;
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

<style>
/* Not scoped: when printing, hide the whole app (desktop dialog overlay or mobile page) and show only the document. */
@media print {
    body * {
        visibility: hidden;
    }

    .invoice-document, .invoice-document * {
        visibility: visible;
    }

    .invoice-document {
        position: fixed;
        inset: 0;
        min-width: 0;
        max-width: none;
    }

    @page {
        margin: 12mm;
    }
}
</style>
```

- [ ] **Step 2: Type-check and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/components/common`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add src/components/common/InvoiceDocument.vue
git commit --no-verify -m "invoices: printable invoice document"
```

---

### Task 9: Desktop navigation and Invoice Profile settings page

**Files:**
- Create: `src/views/desktop/settings/InvoiceProfileSettingPage.vue`
- Modify: `src/router/desktop.ts`, `src/components/desktop/MainPageLayout.vue` (after the Accounts `router-link` ~line 89; `@mdi/js` import ~lines 203-225), `src/views/desktop/settings/SettingsPageLayout.vue` (after the `/settings/sms_capture` item ~line 122; its `@mdi/js` import)

**Interfaces:**
- Consumes: Task 6 store (`loadInvoiceProfile`, `updateInvoiceProfile`), Task 7 `useInvoiceProfilePageBase`.
- Produces: route `/settings/invoice_profile` (child of `SettingsPageLayout`); header buttons. The `/invoice/list` route is added in Task 10.

Navigation decision: Budgets and Buckets keep their existing routes and Settings sidebar entries; the header buttons simply link to them (no route moves, no new layout). Invoices gets its own top-level page in Task 10, built like the Accounts page.

- [ ] **Step 1: Header buttons** in `src/components/desktop/MainPageLayout.vue`, after the Accounts `</router-link>` and before the Add Transaction `router-link`, copying the existing button markup exactly:

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
                                             :aria-current="isTopNavigationActive('/invoice/list') ? 'page' : undefined">
                                    <v-btn class="top-navigation-button ms-1" density="comfortable" variant="text"
                                           :aria-label="tt('Invoices')" :icon="true"
                                           :active="isTopNavigationActive('/invoice/list')"
                                           :color="isTopNavigationActive('/invoice/list') ? 'primary' : 'default'">
                                        <v-icon :icon="isTopNavigationActive('/invoice/list') ? mdiFileDocument : mdiFileDocumentOutline" size="24" />
                                        <v-tooltip activator="parent">{{ tt('Invoices') }}</v-tooltip>
                                    </v-btn>
                                </router-link>
```

Add `mdiWallet, mdiWalletOutline, mdiPiggyBank, mdiPiggyBankOutline, mdiFileDocument, mdiFileDocumentOutline` to the file's `@mdi/js` import.

- [ ] **Step 2: Settings sidebar item** in `src/views/desktop/settings/SettingsPageLayout.vue`, right after the `/settings/sms_capture` `<li>`:

```vue
            <li class="nav-link">
                <router-link to="/settings/invoice_profile">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentEditOutline"/>
                    <span class="nav-item-title">{{ tt('Invoice Profile') }}</span>
                </router-link>
            </li>
```

Add `mdiFileDocumentEditOutline` to its `@mdi/js` import.

- [ ] **Step 3: Create `src/views/desktop/settings/InvoiceProfileSettingPage.vue`** (structure copied from `src/views/desktop/users/UserBasicSettingPage.vue`)

```vue
<template>
    <v-row>
        <v-col cols="12">
            <v-card class="setting-items" :class="{ 'disabled': loading || saving }">
                <template #title>
                    <span>{{ tt('Invoice Profile') }}</span>
                    <v-progress-circular indeterminate size="20" class="ms-3" v-if="loading"></v-progress-circular>
                </template>

                <v-card-text class="d-flex mt-2">
                    <v-avatar rounded="lg" variant="tonal" size="100" class="me-4 cursor-pointer">
                        <v-img :src="newProfile.logoDataUrl" v-if="newProfile.logoDataUrl"></v-img>
                        <v-icon size="48" :icon="mdiImageOutline" v-else />
                        <v-menu activator="parent" width="200" location="bottom" offset="14px">
                            <v-list>
                                <v-list-item :disabled="saving" :title="tt('Update Logo')" @click="logoInput?.click()"></v-list-item>
                                <v-list-item :disabled="!newProfile.logoDataUrl || saving" :title="tt('Remove Logo')" @click="newProfile.logoDataUrl = ''"></v-list-item>
                            </v-list>
                        </v-menu>
                    </v-avatar>
                    <div class="d-flex flex-column justify-center">
                        <span class="text-body-large">{{ tt('These details are printed on every invoice.') }}</span>
                    </div>
                </v-card-text>

                <v-divider />

                <v-form>
                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Business Name') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Business Name')"
                                          v-model="newProfile.businessName" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Address') }}</span>
                            <v-spacer/>
                            <v-textarea class="ms-4" density="compact" rows="4" autocomplete="off"
                                        persistent-placeholder max-width="400px"
                                        :disabled="loading || saving" :placeholder="tt('Address')"
                                        v-model="newProfile.addressLines" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Phone') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Phone')"
                                          v-model="newProfile.phone" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Website') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Website')"
                                          v-model="newProfile.website" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Invoice Number Prefix') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Invoice Number Prefix')"
                                          v-model="newProfile.numberPrefix" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Payment Terms (days)') }}</span>
                            <v-spacer/>
                            <number-input class="ms-4" density="compact" max-width="400px"
                                          :disabled="loading || saving"
                                          :min-value="0" :max-value="365" :max-decimal-count="0"
                                          v-model="newProfile.defaultTermsDays" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Default Notes / Terms') }}</span>
                            <v-spacer/>
                            <v-textarea class="ms-4" density="compact" rows="7" autocomplete="off"
                                        persistent-placeholder max-width="400px"
                                        :disabled="loading || saving" :placeholder="tt('Default Notes / Terms')"
                                        v-model="newProfile.defaultNotes" />
                        </div>
                    </v-card-text>

                    <v-card-text class="d-flex flex-wrap gap-4">
                        <v-tooltip :disabled="!inputIsInvalid" :text="inputInvalidProblemMessage ? tt(inputInvalidProblemMessage) : ''">
                            <template v-slot:activator="{ props }">
                                <div v-bind="props" class="d-inline-block">
                                    <v-btn :disabled="inputIsNotChanged || inputIsInvalid || saving" @click="save">
                                        {{ tt('Save Changes') }}
                                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"></v-progress-circular>
                                    </v-btn>
                                </div>
                            </template>
                        </v-tooltip>
                        <v-btn color="default" variant="tonal" :disabled="inputIsNotChanged || saving" @click="reset">{{ tt('Reset') }}</v-btn>
                    </v-card-text>
                </v-form>
            </v-card>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar" />
    <input ref="logoInput" type="file" style="display: none" accept="image/png,image/jpeg" @change="onLogoChosen($event)" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import { mdiImageOutline } from '@mdi/js';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    loading,
    saving,
    newProfile,
    inputIsNotChanged,
    inputInvalidProblemMessage,
    inputIsInvalid,
    setProfile,
    reset,
    readLogoFile
} = useInvoiceProfilePageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const logoInput = useTemplateRef<HTMLInputElement>('logoInput');

function init(): void {
    loading.value = true;

    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function onLogoChosen(event: Event): void {
    const el = event.target as HTMLInputElement;

    if (!el.files || !el.files.length || !el.files[0]) {
        return;
    }

    const logoFile = el.files[0] as File;
    el.value = '';

    readLogoFile(logoFile).then(dataUrl => {
        newProfile.value.logoDataUrl = dataUrl;
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

function save(): void {
    saving.value = true;

    invoicesStore.updateInvoiceProfile({ profile: newProfile.value }).then(profile => {
        saving.value = false;
        setProfile(profile);
        snackbar.value?.showMessage('Invoice profile has been saved');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

init();
</script>
```

- [ ] **Step 4: Route** in `src/router/desktop.ts`: import `InvoiceProfileSettingPage from '@/views/desktop/settings/InvoiceProfileSettingPage.vue'` and add to the `SettingsPageLayout` children after `/settings/sms_capture`:

```ts
                {
                    path: '/settings/invoice_profile',
                    component: InvoiceProfileSettingPage,
                    beforeEnter: checkLogin
                },
```

- [ ] **Step 5: Type-check, lint, and check in the browser**

Run: `npx vue-tsc --noEmit && npx eslint src/router/desktop.ts src/components/desktop/MainPageLayout.vue src/views/desktop/settings`
Expected: clean. (The Invoices header button points at `/invoice/list`, which 404s to the catch-all until Task 10.)

With `npm run serve` and the Go backend running:
- Header shows Budgets, Buckets, Invoices after Accounts; Budgets/Buckets open their existing pages and highlight.
- Settings sidebar shows "Invoice Profile". Fill in the sample's details (Rishabh Jain; F-452, Phase-8B / Industrial Area, Sector 91 / Mohali, Punjab 160055 / India; 7009600910; rishabhjain.dev; prefix `CA`; 7 days; the bank-details notes block; the Codeation logo PNG via the avatar menu). Save Changes, reload: values persist; Reset restores unsaved edits; Save is disabled until something changes.

- [ ] **Step 6: Commit**

```bash
git add src/views/desktop/settings src/router/desktop.ts src/components/desktop/MainPageLayout.vue
git commit --no-verify -m "invoices: desktop header navigation and invoice profile settings"
```

---

### Task 10: Desktop invoices page, customer dialog, invoice edit dialog

**Files:**
- Create: `src/views/desktop/invoices/ListPage.vue`, `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`, `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`
- Create (placeholder, replaced in Task 11): `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue` with:

```vue
<template><div/></template>
<script setup lang="ts">
function open(): Promise<{ changed: boolean }> { return Promise.resolve({ changed: false }); }
defineExpose({ open });
</script>
```
- Modify: `src/router/desktop.ts` (top-level route next to `/account/list`)

**Interfaces:**
- Consumes: Task 6 store/models, Task 7 `useInvoiceListPageBase`, `useInvoiceEditPageBase`, `useCustomerEditPageBase`.
- Produces:
  - `CustomerEditDialog.open({ id?: string, currentCustomer?: Customer }): Promise<{ message: string, customer: Customer }>`; cancel rejects with `undefined`.
  - `InvoiceEditDialog.open({ id?: string, currentInvoice?: Invoice, duplicateFrom?: Invoice }): Promise<{ message: string, invoice: Invoice }>`; cancel rejects with `undefined`.
  - `InvoiceDetailDialog.open({ id: string }): Promise<{ changed: boolean }>` (built in Task 11).

- [ ] **Step 1: Create `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`** (pattern: `src/views/desktop/categories/list/dialogs/EditDialog.vue`)

```vue
<template>
    <v-dialog width="640" :persistent="isCustomerModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Customer Name')" :placeholder="tt('Customer Name')"
                                          v-model="customer.name" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Contact Person')" :placeholder="tt('Contact Person (optional)')"
                                          v-model="customer.contactName" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="3"
                                        :disabled="loading || submitting"
                                        :label="tt('Address')" :placeholder="tt('Address')"
                                        v-model="customer.addressLines" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-text-field type="email" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Email')" :placeholder="tt('Email (optional)')"
                                          v-model="customer.email" />
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCustomerEditPageBase } from '@/views/base/invoices/CustomerEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { Customer } from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

interface CustomerEditResponse {
    message: string;
    customer: Customer;
}

const { tt } = useI18n();
const {
    editCustomerId,
    loading,
    submitting,
    customer,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty
} = useCustomerEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: CustomerEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const initCustomer = ref<Customer>(Customer.createNewCustomer());

const isCustomerModified = computed<boolean>(() => !isEquals(customer.value.toCreateRequest(), initCustomer.value.toCreateRequest()));

function open(options: { id?: string, currentCustomer?: Customer }): Promise<CustomerEditResponse> {
    showState.value = true;
    loading.value = false;
    submitting.value = false;
    editCustomerId.value = options.id ?? null;
    initCustomer.value = options.currentCustomer ? options.currentCustomer.clone() : Customer.createNewCustomer();
    customer.value = initCustomer.value.clone();

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    invoicesStore.saveCustomer({ customer: customer.value, isEdit: !!editCustomerId.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: editCustomerId.value ? 'You have saved this customer' : 'You have added a new customer', customer: saved });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
```

- [ ] **Step 2: Create `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`**

```vue
<template>
    <v-dialog width="1000" :persistent="isInvoiceModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="6">
                            <v-autocomplete item-title="name" item-value="id" persistent-placeholder
                                            :disabled="loading || submitting"
                                            :label="tt('Customer')" :placeholder="tt('Customer')"
                                            :items="allCustomers" :no-data-text="tt('No results')"
                                            v-model="invoice.customerId">
                                <template #append>
                                    <v-btn density="compact" color="default" variant="text" :icon="true"
                                           :aria-label="tt('Add Customer')" :disabled="loading || submitting" @click="addCustomer">
                                        <v-icon :icon="mdiAccountPlusOutline" />
                                        <v-tooltip activator="parent">{{ tt('Add Customer') }}</v-tooltip>
                                    </v-btn>
                                </template>
                            </v-autocomplete>
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Invoice Number')" :placeholder="tt('Invoice Number')"
                                          v-model="invoice.number" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <date-select :disabled="loading || submitting"
                                         :label="tt('Invoice Date')"
                                         :model-value="invoice.invoiceDate"
                                         @update:model-value="onInvoiceDateChanged" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <date-select :disabled="loading || submitting"
                                         :label="tt('Payment Due')"
                                         v-model="invoice.dueDate" />
                        </v-col>
                    </v-row>

                    <v-row class="mt-2" :key="index" v-for="(line, index) in invoice.lines">
                        <v-col cols="12" md="3">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Item')" :placeholder="tt('Item')"
                                          v-model="line.itemName" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="1" auto-grow
                                        :disabled="loading || submitting"
                                        :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                        v-model="line.description" />
                        </v-col>
                        <v-col cols="4" md="1">
                            <number-input :disabled="loading || submitting" :persistent-placeholder="true"
                                          :label="tt('Qty')" :min-value="1" :max-value="10000" :max-decimal-count="0"
                                          v-model="line.quantity" />
                        </v-col>
                        <v-col cols="6" md="3">
                            <amount-input :disabled="loading || submitting" :persistent-placeholder="true"
                                          :currency="invoice.currency" :show-currency="true"
                                          :label="tt('Rate')" :placeholder="tt('Rate')"
                                          v-model="line.rate" />
                        </v-col>
                        <v-col cols="2" md="1" class="d-flex align-center">
                            <v-btn color="error" variant="text" :icon="mdiDeleteOutline" :aria-label="tt('Remove')"
                                   :disabled="loading || submitting || invoice.lines.length <= 1"
                                   @click="removeLine(index)"></v-btn>
                        </v-col>
                    </v-row>

                    <div class="d-flex align-center mt-2">
                        <v-btn color="primary" variant="text" density="comfortable" :prepend-icon="mdiPlus"
                               :disabled="loading || submitting" @click="addLine">{{ tt('Add Line') }}</v-btn>
                        <v-spacer />
                        <span class="text-body-large me-4">{{ tt('Total') }}</span>
                        <span class="text-title-medium">{{ formatAmount(totals.total) }}</span>
                    </div>

                    <v-row class="mt-2">
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="6"
                                        :disabled="loading || submitting"
                                        :label="tt('Notes / Terms')" :placeholder="tt('Notes / Terms')"
                                        v-model="invoice.notes" />
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <customer-edit-dialog ref="customerEditDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import { mdiPlus, mdiDeleteOutline, mdiAccountPlusOutline } from '@mdi/js';

import SnackBar from '@/components/desktop/SnackBar.vue';
import CustomerEditDialog from './CustomerEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceEditPageBase } from '@/views/base/invoices/InvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type InvoiceProfile, Invoice } from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';

type SnackBarType = InstanceType<typeof SnackBar>;
type CustomerEditDialogType = InstanceType<typeof CustomerEditDialog>;

interface InvoiceEditResponse {
    message: string;
    invoice: Invoice;
}

const { tt } = useI18n();
const {
    editInvoiceId,
    loading,
    submitting,
    invoice,
    allCustomers,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    totals,
    addLine,
    removeLine,
    setInvoice,
    prepareNewInvoice,
    setInvoiceDate,
    formatAmount
} = useInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const customerEditDialog = useTemplateRef<CustomerEditDialogType>('customerEditDialog');

let resolveFunc: ((value: InvoiceEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const profile = ref<InvoiceProfile | null>(null);
const initInvoice = ref<Invoice>(Invoice.createNewInvoice());

const isInvoiceModified = computed<boolean>(() => !isEquals(invoice.value.toCreateRequest(), initInvoice.value.toCreateRequest()));

function suggestNumber(): void {
    invoicesStore.getNextInvoiceNumber({ invoiceDate: invoice.value.invoiceDate }).then(number => {
        invoice.value.number = number;
        initInvoice.value.number = number;
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function open(options: { id?: string, currentInvoice?: Invoice, duplicateFrom?: Invoice }): Promise<InvoiceEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    editInvoiceId.value = options.id ?? null;

    if (options.currentInvoice) {
        setInvoice(options.currentInvoice);
    }

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        options.id ? invoicesStore.getInvoice({ invoiceId: options.id }) : Promise.resolve(null)
    ]).then(([, loadedProfile, loadedInvoice]) => {
        profile.value = loadedProfile;

        if (loadedInvoice) {
            setInvoice(loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, options.duplicateFrom);
            suggestNumber();
        }

        initInvoice.value = invoice.value.clone();
        loading.value = false;
    }).catch(error => {
        loading.value = false;
        showState.value = false;

        if (!error.processed) {
            rejectFunc?.(error);
        }
    });

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function onInvoiceDateChanged(date: TextualYearMonthDay | ''): void {
    if (!date) {
        return;
    }

    setInvoiceDate(date, profile.value?.defaultTermsDays ?? 7);

    if (!editInvoiceId.value) {
        suggestNumber();
    }
}

function addCustomer(): void {
    customerEditDialog.value?.open({}).then(result => {
        invoice.value.customerId = result.customer.id;
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    invoicesStore.saveInvoice({ invoice: invoice.value, isEdit: !!editInvoiceId.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice', invoice: saved });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
```

- [ ] **Step 3: Create `src/views/desktop/invoices/ListPage.vue`** (layout: `accounts/ListPage.vue` sidebar; card and table: `templates/ListPage.vue`)

```vue
<template>
    <main-page-layout nav-items-class="my-3">
        <template #nav-items>
            <div class="mx-2 mb-2">
                <span class="text-body-medium">{{ tt('Outstanding') }}</span>
                <div class="text-body-large text-truncate mt-1 mb-2">
                    <span v-if="!loading || allInvoices.length > 0">{{ formatAmount(outstandingTotal) }}</span>
                    <v-skeleton-loader class="skeleton-no-margin mt-1 mb-2" type="text" :loading="true" v-else></v-skeleton-loader>
                </div>
                <span class="text-body-medium">{{ tt('Overdue') }}</span>
                <div class="text-body-large text-expense text-truncate mt-1">
                    <span v-if="!loading || allInvoices.length > 0">{{ formatAmount(overdueTotal) }}</span>
                    <v-skeleton-loader class="skeleton-no-margin mt-1 mb-2" type="text" :loading="true" v-else></v-skeleton-loader>
                </div>
            </div>

            <v-divider class="my-2" />

            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'invoices', 'disabled': loading }"
                   @click="activeView = 'invoices'">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentOutline"/>
                    <span class="nav-item-title">{{ tt('Invoices') }}</span>
                </a>
            </li>
            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'customers', 'disabled': loading }"
                   @click="activeView = 'customers'">
                    <v-icon class="nav-item-icon" :icon="mdiAccountMultipleOutline"/>
                    <span class="nav-item-title">{{ tt('Customers') }}</span>
                </a>
            </li>
        </template>

        <template #content>
            <v-row class="match-height">
                <v-col cols="12">
                    <v-card>
                        <template #title>
                            <div class="title-and-toolbar d-flex align-center">
                                <span>{{ activeView === 'invoices' ? tt('Invoices') : tt('Customers') }}</span>
                                <v-btn class="ms-3" color="default" variant="outlined"
                                       :disabled="loading || updating" @click="add">{{ tt('Add') }}</v-btn>
                                <v-btn density="compact" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('Refresh')" :icon="true" :disabled="loading || updating"
                                       :loading="loading" @click="reload(true)">
                                    <template #loader>
                                        <v-progress-circular indeterminate size="20"/>
                                    </template>
                                    <v-icon :icon="mdiRefresh" size="24" />
                                    <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                                </v-btn>
                                <v-spacer/>
                                <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('More')" :disabled="loading || updating" :icon="true">
                                    <v-icon :icon="mdiDotsVertical" />
                                    <v-menu activator="parent">
                                        <v-list>
                                            <v-list-item :prepend-icon="mdiEyeOutline" :title="tt('Show Paid Invoices')"
                                                         v-if="activeView === 'invoices' && !showPaid" @click="showPaid = true"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEyeOffOutline" :title="tt('Hide Paid Invoices')"
                                                         v-if="activeView === 'invoices' && showPaid" @click="showPaid = false"></v-list-item>
                                            <v-list-item :prepend-icon="mdiFileDocumentEditOutline" :title="tt('Invoice Profile')"
                                                         to="/settings/invoice_profile"></v-list-item>
                                        </v-list>
                                    </v-menu>
                                </v-btn>
                            </div>
                        </template>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'invoices'">
                            <thead>
                            <tr>
                                <th>{{ tt('Invoice Number') }}</th>
                                <th>{{ tt('Customer') }}</th>
                                <th>{{ tt('Invoice Date') }}</th>
                                <th>{{ tt('Payment Due') }}</th>
                                <th class="text-right">{{ tt('Total') }}</th>
                                <th class="text-right">{{ tt('Balance Due') }}</th>
                                <th>{{ tt('Status') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="loading && noAvailableInvoice">
                            <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                                <td class="px-0" colspan="7"><v-skeleton-loader type="text" :loading="true"></v-skeleton-loader></td>
                            </tr>
                            </tbody>

                            <tbody v-if="!loading && noAvailableInvoice">
                            <tr><td colspan="7">{{ tt('No available invoice') }}</td></tr>
                            </tbody>

                            <tbody v-if="!noAvailableInvoice">
                            <tr class="cursor-pointer" :key="invoice.id" v-for="invoice in displayInvoices" @click="view(invoice)">
                                <td>{{ invoice.number }}</td>
                                <td>{{ invoice.billToName }}</td>
                                <td>{{ formatDate(invoice.invoiceDate) }}</td>
                                <td>{{ formatDate(invoice.dueDate) }}</td>
                                <td class="text-right">{{ formatAmount(invoice.total) }}</td>
                                <td class="text-right">{{ formatAmount(getInvoiceBalanceDue(invoice)) }}</td>
                                <td>
                                    <v-chip size="small" variant="tonal" :color="statusColors[getDisplayStatus(invoice)]">{{ tt(getDisplayStatusName(invoice)) }}</v-chip>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'customers'">
                            <thead>
                            <tr>
                                <th>
                                    <div class="d-flex align-center">
                                        <span>{{ tt('Customer Name') }}</span>
                                        <v-spacer/>
                                        <span>{{ tt('Operation') }}</span>
                                    </div>
                                </th>
                            </tr>
                            </thead>

                            <tbody v-if="!loading && !allCustomers.length">
                            <tr><td>{{ tt('No available customer') }}</td></tr>
                            </tbody>

                            <tbody>
                            <tr :key="customer.id" v-for="customer in allCustomers"
                                @mouseover="hoveredCustomerId = customer.id" @mouseleave="hoveredCustomerId = ''">
                                <td>
                                    <div class="d-flex align-center">
                                        <span>{{ customer.name }}</span>
                                        <small class="text-medium-emphasis ms-2" v-if="customer.contactName">{{ customer.contactName }}</small>
                                        <v-spacer/>
                                        <template v-if="hoveredCustomerId === customer.id && !loading">
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                                   @click="editCustomer(customer)">{{ tt('Edit') }}</v-btn>
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiDeleteOutline" :disabled="loading || updating"
                                                   @click="removeCustomer(customer)">{{ tt('Delete') }}</v-btn>
                                        </template>
                                    </div>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>
                    </v-card>
                </v-col>
            </v-row>
        </template>
    </main-page-layout>

    <invoice-edit-dialog ref="invoiceEditDialog" />
    <invoice-detail-dialog ref="invoiceDetailDialog" />
    <customer-edit-dialog ref="customerEditDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import {
    mdiRefresh,
    mdiDotsVertical,
    mdiEyeOutline,
    mdiEyeOffOutline,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiFileDocumentOutline,
    mdiFileDocumentEditOutline,
    mdiAccountMultipleOutline
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceEditDialog from './dialogs/InvoiceEditDialog.vue';
import InvoiceDetailDialog from './dialogs/InvoiceDetailDialog.vue';
import CustomerEditDialog from './dialogs/CustomerEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceListPageBase } from '@/views/base/invoices/InvoiceListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Customer, Invoice } from '@/models/invoice.ts';

import { type InvoiceDisplayStatus, getInvoiceBalanceDue } from '@/lib/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;
type InvoiceDetailDialogType = InstanceType<typeof InvoiceDetailDialog>;
type CustomerEditDialogType = InstanceType<typeof CustomerEditDialog>;

const { tt } = useI18n();
const {
    loading,
    showPaid,
    allInvoices,
    displayInvoices,
    noAvailableInvoice,
    outstandingTotal,
    overdueTotal,
    formatAmount,
    formatDate,
    getDisplayStatus,
    getDisplayStatusName
} = useInvoiceListPageBase();

const invoicesStore = useInvoicesStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');
const invoiceDetailDialog = useTemplateRef<InvoiceDetailDialogType>('invoiceDetailDialog');
const customerEditDialog = useTemplateRef<CustomerEditDialogType>('customerEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'info', paid: 'success', overdue: 'error' };

const activeView = ref<'invoices' | 'customers'>('invoices');
const updating = ref<boolean>(false);
const hoveredCustomerId = ref<string>('');

const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

function showError(error: { processed?: boolean, isUpToDate?: boolean }): void {
    if (error && !error.processed && !error.isUpToDate) {
        snackbar.value?.showError(error);
    }
}

function reload(force: boolean): void {
    loading.value = true;

    Promise.all([
        invoicesStore.loadAllInvoices({ force: force }),
        invoicesStore.loadAllCustomers({ force: force })
    ]).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Invoice list has been updated');
        }
    }).catch(error => {
        loading.value = false;

        if (error.isUpToDate) {
            snackbar.value?.showMessage('Invoice list is up to date');
        } else {
            showError(error);
        }
    });
}

function add(): void {
    if (activeView.value === 'customers') {
        customerEditDialog.value?.open({}).then(result => {
            snackbar.value?.showMessage(result.message);
        }).catch(showError);
        return;
    }

    invoiceEditDialog.value?.open({}).then(result => {
        snackbar.value?.showMessage(result.message);
        reload(false);
        view(result.invoice);
    }).catch(showError);
}

function view(invoice: Invoice): void {
    invoiceDetailDialog.value?.open({ id: invoice.id }).then(result => {
        if (result.changed) {
            reload(false);
        }
    }).catch(showError);
}

function editCustomer(customer: Customer): void {
    customerEditDialog.value?.open({ id: customer.id, currentCustomer: customer }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(showError);
}

function removeCustomer(customer: Customer): void {
    confirmDialog.value?.open('Are you sure you want to delete this customer?').then(() => {
        updating.value = true;

        invoicesStore.deleteCustomer({ customer }).then(() => {
            updating.value = false;
        }).catch(error => {
            updating.value = false;
            showError(error);
        });
    });
}

reload(false);
</script>
```

- [ ] **Step 4: Route** in `src/router/desktop.ts`: import `InvoiceListPage from '@/views/desktop/invoices/ListPage.vue'` and add right after the `/account/list` route:

```ts
        {
            path: '/invoice/list',
            component: InvoiceListPage,
            beforeEnter: checkLogin
        },
```

- [ ] **Step 5: Type-check, lint, and check in the browser**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices src/router/desktop.ts`
Expected: clean.

In the browser: header → Invoices. The sidebar shows Outstanding / Overdue (₹0.00) and Invoices / Customers items, like the Accounts sidebar. Customers → Add → "Quber Tech" / Kanika Batra / the London address / kanika.batra@qubertech.com → saved; hover shows Edit/Delete. Invoices → Add: dialog has the customer autocomplete (with an add-customer button), number `CA/2026-27/…`, today's date and due date +7 days, notes from the profile. Change the invoice date to 2026-04-23: due date becomes 2026-04-30 and number `CA/2026-27/01`. Line: Consulting / "Software Consulting\nQuber Tech" / 1 / ₹3,70,000 (typed in the amount field). Total shows ₹3,70,000.00 formatted per the user's settings. Save: snackbar, list row appears with an Overdue chip; sidebar Outstanding and Overdue update. Closing the dialog with edits asks nothing (persistent only blocks outside-click), Cancel discards.

- [ ] **Step 6: Commit**

```bash
git add src/views/desktop/invoices src/router/desktop.ts
git commit --no-verify -m "invoices: desktop invoices page and edit dialogs"
```

---

### Task 11: Desktop invoice detail dialog (document, payments, actions, print)

**Files:**
- Create (replace placeholder): `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`

**Interfaces:**
- Consumes: Task 6 store, Task 7 `useInvoiceDetailPageBase`, Task 8 `InvoiceDocument`, Task 10 `InvoiceEditDialog.open`, `useAccountsStore().loadAllAccounts`.
- Produces: `open({ id: string }): Promise<{ changed: boolean }>`, resolved when the dialog is closed (it is a view dialog; closing is not a cancel).

- [ ] **Step 1: Replace `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`**

```vue
<template>
    <v-dialog width="1200" v-model="showState">
        <one-column-dialog-layout :disabled="loading || updating" :loading="loading"
                                  :title="invoice ? invoice.number : tt('Invoice')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #after-title>
                <v-chip class="ms-3" size="small" variant="tonal" :color="displayStatus ? statusColors[displayStatus] : 'default'"
                        v-if="invoice">{{ tt(displayStatusName) }}</v-chip>
            </template>

            <template #toolbar>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Edit')" :disabled="!invoice || loading || updating" @click="edit">
                    <v-icon :icon="mdiPencilOutline" />
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Duplicate')" :disabled="!invoice || loading || updating" @click="duplicate">
                    <v-icon :icon="mdiContentCopy" />
                    <v-tooltip activator="parent">{{ tt('Duplicate') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid')"
                       :disabled="!invoice || loading || updating" @click="toggleClosed">
                    <v-icon :icon="invoice?.closedManually ? mdiLockOpenVariantOutline : mdiCheckDecagramOutline" />
                    <v-tooltip activator="parent">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Delete')" :disabled="!invoice || loading || updating" @click="remove">
                    <v-icon :icon="mdiDeleteOutline" />
                    <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Print / Save PDF')" :disabled="!invoice || !profile || loading" @click="printInvoice">
                    <v-icon :icon="mdiPrinterOutline" />
                    <v-tooltip activator="parent">{{ tt('Print / Save PDF') }}</v-tooltip>
                </v-btn>
            </template>

            <template #content>
                <v-row v-if="invoice && profile">
                    <v-col cols="12" lg="8" class="overflow-x-auto">
                        <invoice-document :invoice="invoice" :profile="profile" />
                    </v-col>

                    <v-col cols="12" lg="4">
                        <v-list density="compact">
                            <v-list-item :title="tt('Total')">
                                <template #append><span>{{ formatAmount(invoice.total) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Received')">
                                <template #append><span>{{ formatAmount(invoice.received) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Bank Charges')" v-if="bankCharges > 0">
                                <template #append><span>{{ formatAmount(bankCharges) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Balance Due')" v-else>
                                <template #append><span class="font-weight-bold">{{ formatAmount(balanceDue) }}</span></template>
                            </v-list-item>
                        </v-list>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Payments') }}</div>
                        <v-list density="compact" v-if="invoice.payments.length">
                            <v-list-item :key="payment.transactionId" v-for="payment in invoice.payments"
                                         :title="formatAmount(payment.amount)" :subtitle="formatPaymentDescription(payment)">
                                <template #append>
                                    <v-btn density="comfortable" color="default" variant="text" :icon="true"
                                           :aria-label="tt('Unlink')" :disabled="updating" @click="unlink(payment)">
                                        <v-icon :icon="mdiLinkOff" />
                                        <v-tooltip activator="parent">{{ tt('Unlink') }}</v-tooltip>
                                    </v-btn>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No payments linked yet.') }}</div>

                        <template v-if="displayStatus !== 'paid'">
                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Suggested Matches') }}</div>
                            <v-list density="compact" v-if="suggestions.length">
                                <v-list-item :key="suggestion.transactionId" v-for="suggestion in suggestions"
                                             :title="formatAmount(suggestion.amount)" :subtitle="formatPaymentDescription(suggestion)">
                                    <template #append>
                                        <v-btn color="primary" variant="tonal" size="small" :disabled="updating" @click="link(suggestion)">{{ tt('Link') }}</v-btn>
                                    </template>
                                </v-list-item>
                            </v-list>
                            <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No matching income transactions found yet.') }}</div>
                        </template>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <invoice-edit-dialog ref="invoiceEditDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';

import {
    mdiPencilOutline,
    mdiContentCopy,
    mdiCheckDecagramOutline,
    mdiLockOpenVariantOutline,
    mdiDeleteOutline,
    mdiPrinterOutline,
    mdiLinkOff
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceDocument from '@/components/common/InvoiceDocument.vue';
import InvoiceEditDialog from './InvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { InvoicePayment } from '@/models/invoice.ts';

import type { InvoiceDisplayStatus } from '@/lib/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;

interface InvoiceDetailResponse {
    changed: boolean;
}

const { tt } = useI18n();
const {
    loading,
    updating,
    invoice,
    profile,
    suggestions,
    displayStatus,
    displayStatusName,
    balanceDue,
    bankCharges,
    formatAmount,
    formatPaymentDescription,
    printInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'info', paid: 'success', overdue: 'error' };

let resolveFunc: ((value: InvoiceDetailResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const invoiceId = ref<string>('');
const changed = ref<boolean>(false);

function showError(error: { processed?: boolean }): void {
    if (error && !error.processed) {
        snackbar.value?.showError(error);
    }
}

function load(): void {
    loading.value = true;

    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        loading.value = false;

        if (displayStatus.value === 'paid') {
            suggestions.value = [];
            return;
        }

        invoicesStore.getInvoicePaymentSuggestions({ invoiceId: loadedInvoice.id }).then(result => {
            suggestions.value = result;
        }).catch(showError);
    }).catch(error => {
        loading.value = false;
        showState.value = false;

        if (!error.processed) {
            rejectFunc?.(error);
        }
    });
}

function open(options: { id: string }): Promise<InvoiceDetailResponse> {
    showState.value = true;
    invoiceId.value = options.id;
    invoice.value = null;
    suggestions.value = [];
    changed.value = false;
    load();

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function runAndReload(action: () => Promise<unknown>): void {
    updating.value = true;

    action().then(() => {
        updating.value = false;
        changed.value = true;
        load();
    }).catch(error => {
        updating.value = false;
        showError(error);
    });
}

function link(suggestion: InvoicePayment): void {
    runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment({ invoiceId: invoiceId.value, transactionId: payment.transactionId }));
}

function toggleClosed(): void {
    if (invoice.value) {
        const current = invoice.value;
        runAndReload(() => invoicesStore.setInvoiceClosed({ invoice: current, closed: !current.closedManually }));
    }
}

function edit(): void {
    invoiceEditDialog.value?.open({ id: invoiceId.value, currentInvoice: invoice.value ?? undefined }).then(result => {
        snackbar.value?.showMessage(result.message);
        changed.value = true;
        load();
    }).catch(showError);
}

function duplicate(): void {
    invoiceEditDialog.value?.open({ duplicateFrom: invoice.value ?? undefined }).then(result => {
        snackbar.value?.showMessage(result.message);
        changed.value = true;
        invoiceId.value = result.invoice.id;
        load();
    }).catch(showError);
}

function remove(): void {
    confirmDialog.value?.open('Are you sure you want to delete this invoice?').then(() => {
        if (!invoice.value) {
            return;
        }

        updating.value = true;

        invoicesStore.deleteInvoice({ invoice: invoice.value }).then(() => {
            updating.value = false;
            changed.value = true;
            close();
        }).catch(error => {
            updating.value = false;
            showError(error);
        });
    });
}

function close(): void {
    resolveFunc?.({ changed: changed.value });
    showState.value = false;
}

defineExpose({
    open
});
</script>
```

- [ ] **Step 2: Type-check and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices`
Expected: clean.

- [ ] **Step 3: Check against the sample in the browser**

Click the `CA/2026-27/01` row:
- Title `CA/2026-27/01` with an Overdue chip; toolbar icons Edit, Duplicate, Mark as Paid, Delete, Print.
- The document matches the Wave sample (`~/Desktop/Invoice_CA_2026-27_01_2026-04-30.pdf`): logo top-left, business block top-right, double-rule "Invoice" banner, bill-to left, meta box right with grey "Amount Due (INR): ₹370,000.00" row, Services/Rate/Amount with dotted rules (no Quantity column), totals, notes. The ₹ glyph renders (Wave shows ☐). It stays white in dark theme.
- Print: the print preview shows only the document; suggested file name `Invoice_CA_2026-27_01_2026-04-30`. Save as PDF and compare side by side with the Wave PDF.
- Add an income of ₹3,68,500 to the PNB account dated 2026-04-29 (Transactions page), reopen the invoice: it is under Suggested Matches with "date · PNB … · comment". Link: chip turns Paid, Bank Charges ₹1,500.00, document's Amount Due ₹1,500.00 with a "Payments received" row. Unlink: back to Overdue. Close the dialog: the list row and sidebar totals refresh.
- Link a ₹2,00,000 income instead: Partially Paid (or Overdue), Balance Due ₹1,70,000; suggestions now only offer amounts ≤ ₹1,70,000. Mark as Paid: Paid with Bank Charges ₹1,70,000; Reopen restores.
- Duplicate: edit dialog prefilled with the same customer, lines and notes, today's date and the next number; saving switches the detail dialog to the new invoice.
- Delete the linked transaction on the Transactions page, reopen the invoice: received has dropped.

- [ ] **Step 4: Commit**

```bash
git add src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue
git commit --no-verify -m "invoices: desktop invoice detail dialog with payments and print"
```

---

### Task 12: Mobile routes, settings link, invoice profile, customers

**Files:**
- Create: `src/views/mobile/invoices/ProfilePage.vue`, `src/views/mobile/invoices/CustomerListPage.vue`, `src/views/mobile/invoices/CustomerEditPage.vue`
- Create (placeholders, replaced in Task 13): `src/views/mobile/invoices/ListPage.vue`, `EditPage.vue`, `DetailPage.vue`, each:

```vue
<template>
    <f7-page><f7-navbar :back-link="tt('Back')"></f7-navbar></f7-page>
</template>
<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';
const { tt } = useI18n();
</script>
```
- Modify: `src/router/mobile.ts` (imports at the top; routes before the `(.*)` catch-all at ~line 390), `src/views/mobile/SettingsPage.vue` (after the Buckets item, line 72)

**Interfaces:**
- Consumes: Task 6 store, Task 7 `useInvoiceProfilePageBase`, `useCustomerEditPageBase`.
- Produces mobile routes: `/invoice/list`, `/invoice/add` (optional `?duplicateFrom=<id>`), `/invoice/edit?id=`, `/invoice/detail?id=`, `/invoice/customer/list`, `/invoice/customer/add`, `/invoice/customer/edit?id=`, `/invoice/profile`.

- [ ] **Step 1: Routes** in `src/router/mobile.ts`. Imports with the other page imports:

```ts
import InvoiceListPage from '@/views/mobile/invoices/ListPage.vue';
import InvoiceEditPage from '@/views/mobile/invoices/EditPage.vue';
import InvoiceDetailPage from '@/views/mobile/invoices/DetailPage.vue';
import InvoiceCustomerListPage from '@/views/mobile/invoices/CustomerListPage.vue';
import InvoiceCustomerEditPage from '@/views/mobile/invoices/CustomerEditPage.vue';
import InvoiceProfilePage from '@/views/mobile/invoices/ProfilePage.vue';
```

Routes, after the `/bucket/list` route and before the `(.*)` catch-all:

```ts
    {
        path: '/invoice/list',
        async: asyncResolve(InvoiceListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/add',
        async: asyncResolve(InvoiceEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/edit',
        async: asyncResolve(InvoiceEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/detail',
        async: asyncResolve(InvoiceDetailPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/customer/list',
        async: asyncResolve(InvoiceCustomerListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/customer/add',
        async: asyncResolve(InvoiceCustomerEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/customer/edit',
        async: asyncResolve(InvoiceCustomerEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/profile',
        async: asyncResolve(InvoiceProfilePage),
        beforeEnter: [checkLogin]
    },
```

- [ ] **Step 2: Settings link** in `src/views/mobile/SettingsPage.vue`, after `<f7-list-item :title="tt('Buckets')" link="/bucket/list"></f7-list-item>`:

```vue
            <f7-list-item :title="tt('Invoices')" link="/invoice/list"></f7-list-item>
```

- [ ] **Step 3: Create `src/views/mobile/invoices/ProfilePage.vue`** (pattern: `src/views/mobile/users/UserProfilePage.vue`; image picker: `src/views/mobile/transactions/EditPage.vue:397-426, 518`)

```vue
<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Invoice Profile')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsNotChanged || inputIsInvalid || saving }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title" header="Business Name" title="Business Name"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Address" title="Address"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Number Prefix" title="INV"></f7-list-item>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else>
            <f7-list-item :header="tt('Logo')">
                <template #footer>
                    <div class="display-flex align-items-center padding-top-half">
                        <image-box style="height: 48px; width: auto" alt="logo" :src="newProfile.logoDataUrl" v-if="newProfile.logoDataUrl"></image-box>
                        <f7-button class="margin-left-half" :text="tt('Update Logo')" @click="logoInput?.click()"></f7-button>
                        <f7-button class="margin-left-half" color="red" :text="tt('Remove Logo')" @click="newProfile.logoDataUrl = ''" v-if="newProfile.logoDataUrl"></f7-button>
                    </div>
                </template>
            </f7-list-item>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Business Name')" :placeholder="tt('Business Name')"
                           v-model:value="newProfile.businessName"></f7-list-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Address')" :placeholder="tt('Address')"
                           v-textarea-auto-size v-model:value="newProfile.addressLines"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Phone')" :placeholder="tt('Phone')"
                           v-model:value="newProfile.phone"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Website')" :placeholder="tt('Website')"
                           v-model:value="newProfile.website"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Invoice Number Prefix')" :placeholder="tt('Invoice Number Prefix')"
                           v-model:value="newProfile.numberPrefix"></f7-list-input>
            <list-number-input :label="tt('Payment Terms (days)')" :placeholder="tt('Payment Terms (days)')"
                               :min-value="0" :max-value="365" :max-decimal-count="0"
                               v-model="newProfile.defaultTermsDays"></list-number-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Default Notes / Terms')" :placeholder="tt('Default Notes / Terms')"
                           v-textarea-auto-size v-model:value="newProfile.defaultNotes"></f7-list-input>
        </f7-list>

        <input ref="logoInput" type="file" style="display: none" accept="image/png,image/jpeg" @change="onLogoChosen($event)" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    saving,
    newProfile,
    inputIsNotChanged,
    inputInvalidProblemMessage,
    inputIsInvalid,
    setProfile,
    readLogoFile
} = useInvoiceProfilePageBase();

const invoicesStore = useInvoicesStore();

const logoInput = useTemplateRef<HTMLInputElement>('logoInput');
const loadingError = ref<unknown | null>(null);

function init(): void {
    loading.value = true;

    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function onLogoChosen(event: Event): void {
    const el = event.target as HTMLInputElement;

    if (!el.files || !el.files.length || !el.files[0]) {
        return;
    }

    const logoFile = el.files[0] as File;
    el.value = '';

    readLogoFile(logoFile).then(dataUrl => {
        newProfile.value.logoDataUrl = dataUrl;
    }).catch(error => {
        showAlert(error.message);
    });
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputInvalidProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    saving.value = true;
    showLoading(() => saving.value);

    invoicesStore.updateInvoiceProfile({ profile: newProfile.value }).then(profile => {
        saving.value = false;
        hideLoading();
        setProfile(profile);
        showToast('Invoice profile has been saved');
        router.back();
    }).catch(error => {
        saving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
```

- [ ] **Step 4: Create `src/views/mobile/invoices/CustomerListPage.vue`** (pattern: `src/views/mobile/templates/ListPage.vue`)

```vue
<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Customers')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/customer/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Customer Name" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !customers.length">
            <f7-list-item :title="tt('No available customer')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && customers.length">
            <f7-list-item swipeout
                          :id="getCustomerDomId(customer)"
                          :title="customer.name"
                          :footer="customer.contactName"
                          :key="customer.id"
                          v-for="customer in customers">
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(customer)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(customer, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this customer?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(customerToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Customer } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const invoicesStore = useInvoicesStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const customerToDelete = ref<Customer | null>(null);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());
const customers = computed<Customer[]>(() => invoicesStore.allCustomers);

function getCustomerDomId(customer: Customer): string {
    return 'customer_' + customer.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllCustomers({ force: false }).then(() => {
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function reload(done?: () => void): void {
    const force = !!done;

    invoicesStore.loadAllCustomers({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Customer list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Customer list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(customer: Customer): void {
    props.f7router.navigate(`/invoice/customer/edit?id=${customer.id}`);
}

function remove(customer: Customer | null, confirm: boolean): void {
    if (!customer) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        customerToDelete.value = customer;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    customerToDelete.value = null;
    showLoading();

    invoicesStore.deleteCustomer({
        customer: customer,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getCustomerDomId(customer), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
```

- [ ] **Step 5: Create `src/views/mobile/invoices/CustomerEditPage.vue`** (pattern: `src/views/mobile/categories/EditPage.vue` / `accounts/EditPage.vue`)

```vue
<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list form strong inset dividers class="margin-vertical">
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Customer Name')" :placeholder="tt('Customer Name')"
                           v-model:value="customer.name"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Contact Person')" :placeholder="tt('Contact Person (optional)')"
                           v-model:value="customer.contactName"></f7-list-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Address')" :placeholder="tt('Address')"
                           v-textarea-auto-size v-model:value="customer.addressLines"></f7-list-input>
            <f7-list-input type="email" autocomplete="off" clear-button
                           :label="tt('Email')" :placeholder="tt('Email (optional)')"
                           v-model:value="customer.email"></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useCustomerEditPageBase } from '@/views/base/invoices/CustomerEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editCustomerId,
    loading,
    submitting,
    customer,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty
} = useCustomerEditPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);

function init(): void {
    const customerId = props.f7route.query['id'];

    if (!customerId) {
        return;
    }

    loading.value = true;
    editCustomerId.value = customerId;

    invoicesStore.loadAllCustomers({ force: false }).then(() => {
        const existing = invoicesStore.allCustomersMap[customerId];

        if (!existing) {
            loadingError.value = 'Unable to retrieve customer';
            showToast('Unable to retrieve customer');
            return;
        }

        customer.value = existing.clone();
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    invoicesStore.saveCustomer({ customer: customer.value, isEdit: !!editCustomerId.value }).then(() => {
        submitting.value = false;
        hideLoading();
        showToast(editCustomerId.value ? 'You have saved this customer' : 'You have added a new customer');
        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
```

- [ ] **Step 6: Type-check, lint, and check on a phone-sized browser**

Run: `npx vue-tsc --noEmit && npx eslint src/views/mobile/invoices src/router/mobile.ts src/views/mobile/SettingsPage.vue`
Expected: clean.

Open the mobile UI (`/mobile` entry, phone-size viewport): Settings shows "Invoices" after Buckets (it opens the placeholder page for now). Navigate to `#!/invoice/profile` directly: profile loads with the values saved on desktop; change the website, tap the checkmark, toast, back. `#!/invoice/customer/list`: Quber Tech listed; swipe left → Edit opens the edit page with values; Add (plus) creates a second customer; swipe → Delete asks via action sheet; deleting Quber Tech fails with "This customer is used by invoices…" if an invoice exists.

- [ ] **Step 7: Commit**

```bash
git add src/views/mobile/invoices src/router/mobile.ts src/views/mobile/SettingsPage.vue
git commit --no-verify -m "invoices: mobile routes, invoice profile and customers"
```

---

### Task 13: Mobile invoice list, edit and detail pages

**Files:**
- Create (replace placeholders): `src/views/mobile/invoices/ListPage.vue`, `EditPage.vue`, `DetailPage.vue`

**Interfaces:**
- Consumes: Task 6 store, Task 7 `useInvoiceListPageBase`, `useInvoiceEditPageBase`, `useInvoiceDetailPageBase`, `getInvoiceDisplayStatusName`; Task 8 `InvoiceDocument`; global mobile components `list-item-selection-popup`, `date-selection-sheet`, `number-pad-sheet`, `list-number-input`.
- Produces: the three routes from Task 12.

- [ ] **Step 1: Replace `src/views/mobile/invoices/ListPage.vue`** (pattern: `src/views/mobile/templates/ListPage.vue`; amount rows like `accounts/ListPage.vue:69-79`)

```vue
<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Invoices')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half" :class="{ 'skeleton-text': loading }">
            <f7-list-item :title="tt('Outstanding')" :after="loading ? '₹0.00' : formatAmount(outstandingTotal)"></f7-list-item>
            <f7-list-item :title="tt('Overdue')" :after="loading ? '₹0.00' : formatAmount(overdueTotal)"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="CA/2026-27/01" after="₹000,000.00" footer="Customer · Unpaid" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableInvoice">
            <f7-list-item :title="tt('No available invoice')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !noAvailableInvoice">
            <f7-list-item swipeout
                          :id="getInvoiceDomId(invoice)"
                          :title="invoice.number"
                          :after="formatAmount(getInvoiceBalanceDue(invoice))"
                          :link="'/invoice/detail?id=' + invoice.id"
                          :key="invoice.id"
                          v-for="invoice in displayInvoices">
                <template #footer>
                    <div>{{ invoice.billToName }} · {{ tt(getDisplayStatusName(invoice)) }} · {{ formatDate(invoice.dueDate) }}</div>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(invoice)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(invoice, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showPaid" @click="showPaid = true">{{ tt('Show Paid Invoices') }}</f7-actions-button>
                <f7-actions-button v-if="showPaid" @click="showPaid = false">{{ tt('Hide Paid Invoices') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/customer/list')">{{ tt('Customers') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/profile')">{{ tt('Invoice Profile') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(invoiceToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';
import { useInvoiceListPageBase } from '@/views/base/invoices/InvoiceListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Invoice } from '@/models/invoice.ts';

import { getInvoiceBalanceDue } from '@/lib/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showPaid,
    displayInvoices,
    noAvailableInvoice,
    outstandingTotal,
    overdueTotal,
    formatAmount,
    formatDate,
    getDisplayStatusName
} = useInvoiceListPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const invoiceToDelete = ref<Invoice | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getInvoiceDomId(invoice: Invoice): string {
    return 'invoice_' + invoice.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllInvoices({ force: false }).then(() => {
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function reload(done?: () => void): void {
    const force = !!done;

    invoicesStore.loadAllInvoices({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Invoice list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Invoice list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(invoice: Invoice): void {
    props.f7router.navigate(`/invoice/edit?id=${invoice.id}`);
}

function remove(invoice: Invoice | null, confirm: boolean): void {
    if (!invoice) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        invoiceToDelete.value = invoice;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    invoiceToDelete.value = null;
    showLoading();

    invoicesStore.deleteInvoice({
        invoice: invoice,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getInvoiceDomId(invoice), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        reload();
    }

    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
```

- [ ] **Step 2: Replace `src/views/mobile/invoices/EditPage.vue`** (pattern: `src/views/mobile/accounts/EditPage.vue`, including per-child lists like its sub-accounts at lines 447-466)

```vue
<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title" header="Customer" title="Customer Name"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Number" title="CA/2026-27/01"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Date" title="April 23, 2026"></f7-list-item>
        </f7-list>

        <template v-else>
            <f7-list form strong inset dividers class="margin-vertical">
                <f7-list-item class="list-item-with-header-and-title" link="#"
                              :header="tt('Customer')"
                              :title="selectedCustomerName"
                              @click="showCustomerPopup = true">
                    <list-item-selection-popup value-type="item"
                                               key-field="id" value-field="id"
                                               title-field="name"
                                               :title="tt('Customer')"
                                               :enable-filter="true"
                                               :filter-placeholder="tt('Customer')"
                                               :filter-no-items-text="tt('No results')"
                                               :items="allCustomers"
                                               v-model:show="showCustomerPopup"
                                               v-model="invoice.customerId">
                    </list-item-selection-popup>
                </f7-list-item>
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('Invoice Number')" :placeholder="tt('Invoice Number')"
                               v-model:value="invoice.number"></f7-list-input>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Invoice Date')"
                              :title="formatDate(invoice.invoiceDate)"
                              @click="showInvoiceDateSheet = true">
                    <date-selection-sheet v-model:show="showInvoiceDateSheet"
                                          :model-value="invoice.invoiceDate"
                                          @update:model-value="onInvoiceDateChanged">
                    </date-selection-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Payment Due')"
                              :title="formatDate(invoice.dueDate)"
                              @click="showDueDateSheet = true">
                    <date-selection-sheet v-model:show="showDueDateSheet"
                                          v-model="invoice.dueDate">
                    </date-selection-sheet>
                </f7-list-item>
            </f7-list>

            <f7-block class="no-padding no-margin">
                <f7-list strong inset dividers class="margin-vertical"
                         :key="index" v-for="(line, index) in invoice.lines">
                    <f7-list-item group-title>
                        <small>{{ tt('Line') + ' #' + (index + 1) }}</small>
                        <f7-button rasied fill color="red" icon-f7="trash" icon-size="16px" :aria-label="tt('Remove')"
                                   :class="{ 'disabled': invoice.lines.length <= 1 }"
                                   @click="removeLine(index)"></f7-button>
                    </f7-list-item>
                    <f7-list-input type="text" autocomplete="off" clear-button
                                   :label="tt('Item')" :placeholder="tt('Item')"
                                   v-model:value="line.itemName"></f7-list-input>
                    <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                                   :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                   v-textarea-auto-size v-model:value="line.description"></f7-list-input>
                    <list-number-input :label="tt('Qty')" :placeholder="tt('Qty')"
                                       :min-value="1" :max-value="10000" :max-decimal-count="0"
                                       v-model="line.quantity"></list-number-input>
                    <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                                  :header="tt('Rate')"
                                  :title="formatAmount(line.rate)"
                                  @click="rateSheetLineIndex = index">
                        <number-pad-sheet :min-value="0"
                                          :max-value="TRANSACTION_MAX_AMOUNT"
                                          :currency="invoice.currency"
                                          :show="rateSheetLineIndex === index"
                                          @update:show="rateSheetLineIndex = $event ? index : -1"
                                          v-model="line.rate"></number-pad-sheet>
                    </f7-list-item>
                </f7-list>
            </f7-block>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item :title="tt('Total')" :after="formatAmount(totals.total)"></f7-list-item>
                <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                               :label="tt('Notes / Terms')" :placeholder="tt('Notes / Terms')"
                               v-textarea-auto-size v-model:value="invoice.notes"></f7-list-input>
            </f7-list>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="addLine">{{ tt('Add Line') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/customer/add')">{{ tt('Add Customer') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceEditPageBase } from '@/views/base/invoices/InvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
import type { InvoiceProfile } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editInvoiceId,
    loading,
    submitting,
    invoice,
    allCustomers,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    totals,
    addLine,
    removeLine,
    setInvoice,
    prepareNewInvoice,
    setInvoiceDate,
    formatAmount,
    formatDate
} = useInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const profile = ref<InvoiceProfile | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showCustomerPopup = ref<boolean>(false);
const showInvoiceDateSheet = ref<boolean>(false);
const showDueDateSheet = ref<boolean>(false);
const rateSheetLineIndex = ref<number>(-1);

const selectedCustomerName = computed<string>(() => invoicesStore.allCustomersMap[invoice.value.customerId]?.name ?? tt('None'));

function suggestNumber(): void {
    invoicesStore.getNextInvoiceNumber({ invoiceDate: invoice.value.invoiceDate }).then(number => {
        invoice.value.number = number;
    }).catch(error => {
        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function init(): void {
    const query = props.f7route.query;
    loading.value = true;
    editInvoiceId.value = query['id'] || null;

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        query['id'] ? invoicesStore.getInvoice({ invoiceId: query['id'] }) : Promise.resolve(null),
        query['duplicateFrom'] ? invoicesStore.getInvoice({ invoiceId: query['duplicateFrom'] }) : Promise.resolve(null)
    ]).then(([, loadedProfile, loadedInvoice, duplicateFrom]) => {
        profile.value = loadedProfile;

        if (loadedInvoice) {
            setInvoice(loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, duplicateFrom);
            suggestNumber();
        }

        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function onInvoiceDateChanged(date: TextualYearMonthDay): void {
    setInvoiceDate(date, profile.value?.defaultTermsDays ?? 7);

    if (!editInvoiceId.value) {
        suggestNumber();
    }
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    invoicesStore.saveInvoice({ invoice: invoice.value, isEdit: !!editInvoiceId.value }).then(() => {
        submitting.value = false;
        hideLoading();
        showToast(editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice');
        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
```

- [ ] **Step 3: Replace `src/views/mobile/invoices/DetailPage.vue`**

```vue
<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="invoice ? invoice.number : tt('Invoice')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading || updating }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item title="Status" after="Unpaid"></f7-list-item>
            <f7-list-item title="Total" after="₹000,000.00"></f7-list-item>
            <f7-list-item title="Balance Due" after="₹000,000.00"></f7-list-item>
        </f7-list>

        <template v-if="!loading && invoice && profile">
            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item :title="tt('Status')" :after="tt(displayStatusName)"></f7-list-item>
                <f7-list-item :title="tt('Customer')" :after="invoice.billToName"></f7-list-item>
                <f7-list-item :title="tt('Payment Due')" :after="formatDate(invoice.dueDate)"></f7-list-item>
                <f7-list-item :title="tt('Total')" :after="formatAmount(invoice.total)"></f7-list-item>
                <f7-list-item :title="tt('Received')" :after="formatAmount(invoice.received)"></f7-list-item>
                <f7-list-item :title="tt('Bank Charges')" :after="formatAmount(bankCharges)" v-if="bankCharges > 0"></f7-list-item>
                <f7-list-item :title="tt('Balance Due')" :after="formatAmount(balanceDue)" v-else></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item group-title>
                    <small>{{ tt('Payments') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No payments linked yet.')" v-if="!invoice.payments.length"></f7-list-item>
                <f7-list-item swipeout
                              :title="formatAmount(payment.amount)"
                              :footer="formatPaymentDescription(payment)"
                              :key="payment.transactionId"
                              v-for="payment in invoice.payments">
                    <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                         :right="textDirection === TextDirection.LTR">
                        <f7-swipeout-button color="red" close :text="tt('Unlink')" @click="unlink(payment)"></f7-swipeout-button>
                    </f7-swipeout-actions>
                </f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical" v-if="displayStatus !== 'paid'">
                <f7-list-item group-title>
                    <small>{{ tt('Suggested Matches') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No matching income transactions found yet.')" v-if="!suggestions.length"></f7-list-item>
                <f7-list-item :title="formatAmount(suggestion.amount)"
                              :footer="formatPaymentDescription(suggestion)"
                              :key="suggestion.transactionId"
                              v-for="suggestion in suggestions">
                    <template #after>
                        <f7-button small fill :class="{ 'disabled': updating }" :text="tt('Link')" @click="link(suggestion)"></f7-button>
                    </template>
                </f7-list-item>
            </f7-list>

            <f7-block class="invoice-document-preview">
                <invoice-document :invoice="invoice" :profile="profile" />
            </f7-block>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/add?duplicateFrom=${invoiceId}`)">{{ tt('Duplicate') }}</f7-actions-button>
                <f7-actions-button @click="toggleClosed">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</f7-actions-button>
                <f7-actions-button @click="printInvoice">{{ tt('Print / Save PDF') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button color="red" @click="showDeleteActionSheet = true">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { TextDirection } from '@/core/text.ts';
import type { InvoicePayment } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    updating,
    invoice,
    profile,
    suggestions,
    displayStatus,
    displayStatusName,
    balanceDue,
    bankCharges,
    formatAmount,
    formatDate,
    formatPaymentDescription,
    printInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const loadingError = ref<unknown | null>(null);
const invoiceId = ref<string>(props.f7route.query['id'] || '');
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function load(): void {
    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        loading.value = false;

        if (displayStatus.value === 'paid') {
            suggestions.value = [];
            return;
        }

        invoicesStore.getInvoicePaymentSuggestions({ invoiceId: loadedInvoice.id }).then(result => {
            suggestions.value = result;
        }).catch(error => {
            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function runAndReload(action: () => Promise<unknown>): void {
    updating.value = true;
    showLoading(() => updating.value);

    action().then(() => {
        updating.value = false;
        hideLoading();
        load();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function link(suggestion: InvoicePayment): void {
    runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment({ invoiceId: invoiceId.value, transactionId: payment.transactionId }));
}

function toggleClosed(): void {
    if (invoice.value) {
        const current = invoice.value;
        runAndReload(() => invoicesStore.setInvoiceClosed({ invoice: current, closed: !current.closedManually }));
    }
}

function remove(): void {
    if (!invoice.value) {
        return;
    }

    const router = props.f7router;
    showDeleteActionSheet.value = false;
    updating.value = true;
    showLoading(() => updating.value);

    invoicesStore.deleteInvoice({ invoice: invoice.value }).then(() => {
        updating.value = false;
        hideLoading();
        router.back();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        load(); // back from the edit page
    }

    routeBackOnError(props.f7router, loadingError);
}

load();
</script>

<style>
.invoice-document-preview {
    overflow-x: auto;
}
</style>
```

- [ ] **Step 4: Type-check, lint, and check on a phone-sized browser**

Run: `npx vue-tsc --noEmit && npx eslint src/views/mobile/invoices`
Expected: clean.

In the mobile UI (phone viewport):
- Settings → Invoices: Outstanding/Overdue summary, then `CA/2026-27/01` with balance due and "Quber Tech · Overdue · April 30, 2026". Pull to refresh shows a toast. Ellipsis: Show/Hide Paid Invoices, Customers, Invoice Profile.
- Plus → Add Invoice: customer popup with filter, number pre-suggested, invoice date sheet (changing it moves the due date by the profile's terms and re-suggests the number), Line #1 with item, description, Qty, and Rate via the number pad; ellipsis → Add Line adds Line #2, its trash button removes it; Total updates; checkmark saves, toast, back to list showing the new invoice.
- Tap an invoice → detail: status, totals, Payments (empty), Suggested Matches with a Link button, and the invoice document (scrolls horizontally on a narrow screen). Link a suggestion → Paid / Bank Charges as on desktop; swipe the payment → Unlink. Ellipsis → Edit (edit page, back returns to an updated detail), Duplicate (add page prefilled), Mark as Paid / Reopen, Print / Save PDF (iOS Safari: share sheet → Save to Files gives a PDF of only the document), Delete (confirm, back to list).
- Swipe an invoice in the list → Edit / Delete work.

- [ ] **Step 5: Commit**

```bash
git add src/views/mobile/invoices
git commit --no-verify -m "invoices: mobile invoice list, edit and detail pages"
```

---

### Task 14: Locale strings and full verification

**Files:**
- Modify: `src/locales/en.json`

- [ ] **Step 1: Collect every new key.** Run:

```bash
grep -rhoE "(tt|showMessage|showToast|showAlert|confirmDialog\.value\?\.open)\('[^']+'" \
  src/views/desktop/invoices src/views/mobile/invoices src/views/desktop/settings/InvoiceProfileSettingPage.vue \
  src/components/desktop/MainPageLayout.vue src/views/desktop/settings/SettingsPageLayout.vue src/views/mobile/SettingsPage.vue \
  | sed -E "s/^[^']*'//; s/'$//" | sort -u > /tmp/invoice-keys.txt
grep -rhoE "return '[^']+'" src/views/base/invoices src/stores/invoice.ts | sed -E "s/^return '//; s/'$//" | sort -u >> /tmp/invoice-keys.txt
grep -rhoE "message: '[^']+'" src/views/base/invoices src/stores/invoice.ts | sed -E "s/^message: '//; s/'$//" | sort -u >> /tmp/invoice-keys.txt
grep -rhoE "'(Unpaid|Partially Paid|Paid|Overdue)'" src/views/base/invoices | tr -d "'" >> /tmp/invoice-keys.txt
python3 - <<'PY'
import json
d = json.load(open('src/locales/en.json'))
keys = sorted(set(l.strip() for l in open('/tmp/invoice-keys.txt') if l.strip()))
missing = [k for k in keys if k not in d]
print('\n'.join(missing))
PY
```

Expected: the list of keys not yet in `en.json`.

- [ ] **Step 2: Add the missing keys** as flat top-level entries next to `"Buckets"` (~line 2845), each `"Key": "Key"`. They include at least: `Invoices`, `Invoice`, `Invoice Profile`, `These details are printed on every invoice.`, `Business Name`, `Address`, `Phone`, `Website`, `Logo`, `Update Logo`, `Remove Logo`, `Invoice Number Prefix`, `Payment Terms (days)`, `Default Notes / Terms`, `Customers`, `Customer`, `Customer Name`, `Contact Person`, `Contact Person (optional)`, `Email (optional)`, `Add Customer`, `Edit Customer`, `Add Invoice`, `Edit Invoice`, `Invoice Number`, `Invoice Date`, `Payment Due`, `Item`, `Description (optional)`, `Qty`, `Rate`, `Line`, `Add Line`, `Notes / Terms`, `Outstanding`, `Overdue`, `Paid`, `Unpaid`, `Partially Paid`, `Received`, `Balance Due`, `Bank Charges`, `Payments`, `Suggested Matches`, `No payments linked yet.`, `No matching income transactions found yet.`, `Link`, `Unlink`, `Mark as Paid`, `Reopen`, `Print / Save PDF`, `Show Paid Invoices`, `Hide Paid Invoices`, `No available invoice`, `No available customer`, `Are you sure you want to delete this invoice?`, `Are you sure you want to delete this customer?`, `You have added a new invoice`, `You have saved this invoice`, `You have added a new customer`, `You have saved this customer`, `Invoice profile has been saved`, `Invoice list has been updated`, `Invoice list is up to date`, `Customer list has been updated`, `Customer list is up to date`, `Customer cannot be blank`, `Customer name cannot be blank`, `Invoice number cannot be blank`, `Due date cannot be before the invoice date`, `Invoice must have at least one line`, `Item name cannot be blank`, `Quantity and rate must be greater than zero`, `Invoice number prefix cannot be blank`, `Payment terms must be between 0 and 365 days`, `The logo must be a PNG or JPEG image`, `Logo must be smaller than 250 KB`, `Unable to read this file`, and every `Unable to …` message from `src/stores/invoice.ts`. Skip any key already present (keys must stay unique; `Total`, `Edit`, `Delete`, `Duplicate`, `Status`, `Email`, `Description`, `Add`, `Save`, `Cancel`, `Close`, `Refresh`, `More`, `Operation`, `Budgets`, `Buckets`, `None`, `No results`, `Save Changes`, `Reset` already exist).

- [ ] **Step 3: Add backend error strings** inside the `"error"` object:

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

- [ ] **Step 4: Re-run Step 1** — expected: no output (no missing keys).

- [ ] **Step 5: Full verification**

```bash
python3 -c "import json; json.load(open('src/locales/en.json'))" \
  && go build ./... && go vet ./pkg/... ./cmd/... \
  && go test ./pkg/services ./pkg/api -count=1 \
  && npx vue-tsc --noEmit && npx eslint src && npx vitest run
```

Expected: every command succeeds. (`go test ./pkg/exchangerates` calls live bank APIs and may time out; it is unrelated and not part of this run.)

- [ ] **Step 6: Error paths in the browser**

- Desktop: create a second invoice numbered `CA/2026-27/01` → snackbar "Invoice number already exists".
- Desktop: Customers → delete Quber Tech while an invoice uses it → "This customer is used by invoices and cannot be deleted".
- Desktop edit dialog with an empty item name: Save is disabled and its tooltip reads "Item name cannot be blank"; on mobile the checkmark is disabled.

- [ ] **Step 7: Commit**

```bash
git add src/locales/en.json
git commit --no-verify -m "invoices: locale strings"
```

---

## Spec coverage (self-check)

| Spec item | Task |
|---|---|
| Tables, snapshot, `currency`/`kind` seams | 1, 2, 3 |
| Number uniqueness, FY numbering, next number | 3; UI 10, 13 |
| Totals in one place | 3 (Go), 6 (TS) |
| Status, 3% tolerance, bank charges, manual close, overdue | 6; UI 10, 11, 13 |
| Multiple payments, link/unlink, deleted transaction drops out | 4; UI 11, 13 |
| Suggestions | 4; UI 11, 13 |
| Validation and ownership | 1, 3, 4, 5; client-side problem messages 7 |
| Print-to-PDF matching the sample, file name | 8; actions 11 (desktop), 13 (mobile) |
| Header buttons for Budgets/Buckets/Invoices | 9 |
| Invoice profile with logo | 9 (desktop), 12 (mobile) |
| List with Outstanding/Overdue, show/hide paid, customers | 10 (desktop), 12–13 (mobile) |
| Edit with inline add customer, duplicate | 10, 11 (desktop), 13 (mobile) |
| Existing UI structure on desktop and mobile, shared logic in `views/base` | 6–13 |
| Locale strings | 14 |
| Future extensions | not built; seams: `Kind`, `Currency`, `computeInvoiceTotal(s)`, `CreateInvoice`, pure `InvoiceDocument` |
