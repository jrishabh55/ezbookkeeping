# CRM Part A (Products, Renewals, Customer View) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a product catalog linked to invoice lines, turn recurring invoices into configurable renewals (every N months; create / draft-ahead / auto-send), and add a customer detail page and a home "Renewals" card.

**Architecture:** Everything extends the existing invoicing module. Renewals are the existing `RecurringInvoice` rows with two new columns (`PeriodMonths`, `Mode`); the existing daily cron job `CreateRecurringInvoices` learns the modes. Products are a new table; invoice lines get an optional `ProductId`. Generated invoices remember their schedule (`Invoice.RecurringId`) so the home card and timeline can find them. Customer page data comes from one new read endpoint; totals reuse existing frontend helpers.

**Tech Stack:** Go (gin, xorm, testify) backend; Vue 3 + Pinia + Vuetify (desktop) + Framework7 (mobile) frontend; vitest.

**Spec:** `docs/superpowers/specs/2026-10-29-crm-part-a-products-renewals-design.md`

## Global Constraints

- Every schema change is a new column or table added by xorm `Sync2` (`cmd/database.go` invoice table loop and the test list in `pkg/services/alerts_testutil_test.go`). No data migration.
- Existing recurring rows keep today's behaviour: `PeriodMonths` default 1, `Mode` default `create`.
- Renewal modes are exactly `create`, `draft_ahead`, `auto_send`.
- `PeriodMonths` and `RenewalMonths` (when renewable) are 1..120. `RenewalDraftDays` is 0..90, default 15.
- Day of month stays 1..28 (`models.RecurringInvoiceMaxDayOfMonth`).
- When products on one invoice differ in mode, the most hands-on wins: `draft_ahead` > `create` > `auto_send`.
- "Link past invoice lines" groups by `strings.ToLower(strings.TrimSpace(ItemName))` and never links without the user confirming.
- A product with linked invoice lines cannot be deleted (archive instead).
- Desktop and mobile both get every screen; shared logic lives in `src/views/base/invoices/`. New UI strings go in `src/locales/en.json` only (other locales fall back).
- Amounts are minor units of the invoice currency.
- Go tests: `go test ./pkg/services/ ./pkg/api/ -run <Name> -count=1`. Frontend tests: `npx vitest run --exclude '.claude/**' <file>`. Typecheck: `npx vue-tsc --noEmit`.
- Commit with `git commit --no-verify` (project convention).

## Review Focus

1. **Auto-send with no customer email or SMTP down** — the invoice must still be created, `NextDate` must still advance (no duplicate next day), and the home card must show "Auto-send failed". Pinned in Task 3.
2. **Draft-ahead job running daily inside the window** — exactly one draft per period, even though the row stays "in window" for N days. Pinned in Task 3 (`NextDate` advances when the draft is created).
3. **Re-sending an invoice that already has a schedule** — the send dialog must not offer, and the server must not create, a second schedule. Pinned in Task 6.
4. **Yearly schedule paused for 2 years then resumed** — catch-up must step by 12 months and land on the anniversary, not next month. Pinned in Task 1.
5. **Deleting a product used on old invoices** — refused with a clear error; archiving hides it from the picker but past lines still show it on the customer page. Pinned in Task 5.

---

## File Structure

Backend:
- Modify `pkg/models/recurring_invoice.go` — PeriodMonths, Mode, mode constants, request/response fields.
- Modify `pkg/models/invoice.go` — `InvoiceProfile.RenewalDraftDays`, `Invoice.RecurringId`, `InvoiceLine.ProductId`, request/response fields, renewal option + renewals-due + customer overview response types.
- Create `pkg/models/product.go` — Product table, requests, response.
- Modify `pkg/errs/invoice.go` — new error codes.
- Modify `pkg/services/recurring_invoices.go` — period stepping, modes in the job, renewals-due query, renewal option, auto-create on send.
- Create `pkg/services/products.go` + `pkg/services/products_test.go` — product CRUD, past-line grouping/linking.
- Create `pkg/services/customer_overview.go` + test — products bought and timeline.
- Modify `pkg/services/invoices.go` — profile default, `prepareNewInvoice` resets `RecurringId`.
- Modify `pkg/mail/mailer_container.go` — `SetMailer` for tests.
- Create `pkg/api/products.go`; modify `pkg/api/invoices.go`, `pkg/api/recurring_invoices.go`.
- Modify `cmd/webserver.go`, `cmd/database.go`, `pkg/services/alerts_testutil_test.go`.

Frontend:
- Modify `src/models/invoice.ts`; create `src/models/product.ts`.
- Create `src/lib/product.ts` + `src/lib/__tests__/product.test.ts` — suggestion filter.
- Modify `src/lib/recurringInvoice.ts` (+ its test) — period-aware next date.
- Modify `src/lib/services.ts`, `src/stores/invoice.ts`.
- Create `src/views/base/invoices/ProductEditPageBase.ts`, `CustomerDetailPageBase.ts`, `RenewalsCardBase.ts`.
- Modify `src/views/base/invoices/RecurringInvoiceEditPageBase.ts`, `InvoiceEditPageBase.ts`, `InvoiceDetailPageBase.ts`, `InvoiceProfilePageBase.ts`.
- Desktop: modify `invoices/ListPage.vue`, `dialogs/RecurringInvoiceEditDialog.vue`, `dialogs/InvoiceEditDialog.vue`, `dialogs/InvoiceDetailDialog.vue`, `settings/InvoiceProfileSettingPage.vue`, `HomePage.vue`; create `dialogs/ProductEditDialog.vue`, `dialogs/ProductLinkDialog.vue`, `dialogs/CustomerDetailDialog.vue`, `invoices/RenewalsCard.vue`.
- Mobile: modify `invoices/ListPage.vue`, `CustomerListPage.vue`, `RecurringInvoiceAddPopup.vue`, `RecurringListPage.vue`, `EditPage.vue`, `SendPage.vue`, `ProfilePage.vue`, `HomePage.vue`, `src/router/mobile.ts`; create `invoices/ProductListPage.vue`, `ProductEditPage.vue`, `ProductLinkPage.vue`, `CustomerDetailPage.vue`, `RenewalsCard.vue`.

---
### Task 1: Recurring invoices repeat every N months with a mode (backend)

**Files:**
- Modify: `pkg/models/recurring_invoice.go`
- Modify: `pkg/errs/invoice.go`
- Modify: `pkg/services/recurring_invoices.go:16-48` (date helpers), `:49-58` (validate), `:96-160` (create/modify), `:268` (call site)
- Modify: `pkg/api/recurring_invoices.go:64,85`
- Test: `pkg/services/recurring_invoices_test.go`

**Interfaces:**
- Produces: constants `models.RECURRING_INVOICE_MODE_CREATE = "create"`, `models.RECURRING_INVOICE_MODE_DRAFT_AHEAD = "draft_ahead"`, `models.RECURRING_INVOICE_MODE_AUTO_SEND = "auto_send"`, `models.RecurringInvoiceMaxPeriodMonths = 120`; fields `RecurringInvoice.PeriodMonths int32`, `RecurringInvoice.Mode string`; JSON `periodMonths`, `mode` on create/modify requests and the response; `nextRecurringDate(date string, dayOfMonth int32, periodMonths int32) (string, error)`; `catchUpRecurringDate(nextDate string, dayOfMonth int32, periodMonths int32, today string) string`; `models.IsValidRecurringInvoiceMode(mode string) bool`; errors `errs.ErrRecurringInvoicePeriodInvalid` (43), `errs.ErrRecurringInvoiceModeInvalid` (44).

- [ ] **Step 1: Write the failing tests**

Replace `TestNextRecurringDateAdvancesOneMonthAcrossYearBoundary` and `TestCatchUpRecurringDate` in `pkg/services/recurring_invoices_test.go`, and add the two new tests below:

```go
func TestNextRecurringDateAdvancesByPeriod(t *testing.T) {
	next, err := nextRecurringDate("2026-12-15", 15, 1)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", next)

	next, err = nextRecurringDate("2026-01-31", 28, 1)
	assert.Nil(t, err)
	assert.Equal(t, "2026-02-28", next)

	next, err = nextRecurringDate("2026-11-10", 10, 3)
	assert.Nil(t, err)
	assert.Equal(t, "2027-02-10", next)

	next, err = nextRecurringDate("2026-03-28", 28, 12)
	assert.Nil(t, err)
	assert.Equal(t, "2027-03-28", next)

	_, err = nextRecurringDate("2026-13-01", 1, 1)
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)
}

func TestCatchUpRecurringDate(t *testing.T) {
	// monthly: a stale next date moves to the next occurrence of the day of month on or after today
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-01"))
	assert.Equal(t, "2026-10-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-15"))
	assert.Equal(t, "2026-11-15", catchUpRecurringDate("2026-07-15", 15, 1, "2026-10-16"))
	assert.Equal(t, "2027-01-05", catchUpRecurringDate("2026-07-05", 5, 1, "2026-12-31"))
	// a future next date is left alone
	assert.Equal(t, "2026-12-15", catchUpRecurringDate("2026-12-15", 15, 1, "2026-10-01"))
	// yearly: steps by whole years so the anniversary is kept
	assert.Equal(t, "2027-03-10", catchUpRecurringDate("2024-03-10", 10, 12, "2026-10-29"))
	// quarterly
	assert.Equal(t, "2026-11-01", catchUpRecurringDate("2026-02-01", 1, 3, "2026-10-29"))
}

func TestRecurringInvoicePeriodAndModeDefaultAndValidate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)

	// zero values mean today's behaviour
	recurring := createTestRecurringInvoice(t, uid, source.InvoiceId, 15, "2026-12-15")
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, int32(1), reloaded.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_CREATE, reloaded.Mode)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2026-12-15", PeriodMonths: 121, Enabled: true})
	assert.Equal(t, errs.ErrRecurringInvoicePeriodInvalid, err)

	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2026-12-15", Mode: "weekly", Enabled: true})
	assert.Equal(t, errs.ErrRecurringInvoiceModeInvalid, err)

	reloaded.PeriodMonths, reloaded.Mode = 12, models.RECURRING_INVOICE_MODE_AUTO_SEND
	assert.Nil(t, Invoices.ModifyRecurringInvoice(ctx, reloaded))
	reloaded, err = Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, int32(12), reloaded.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_AUTO_SEND, reloaded.Mode)
}

func TestCreateRecurringInvoicesAdvancesByPeriod(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: source.InvoiceId, DayOfMonth: 15, NextDate: "2026-12-15", PeriodMonths: 12, Enabled: true}
	assert.Nil(t, Invoices.CreateRecurringInvoice(ctx, recurring))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-12-15", reloaded.NextDate)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestNextRecurringDate|TestCatchUpRecurringDate|TestRecurringInvoicePeriodAndMode|TestCreateRecurringInvoicesAdvancesByPeriod' -count=1`
Expected: compile FAIL (`too many arguments in call to nextRecurringDate`, `undefined: models.RECURRING_INVOICE_MODE_CREATE`).

- [ ] **Step 3: Add model fields, constants and errors**

In `pkg/models/recurring_invoice.go`, below `RecurringInvoiceMaxDayOfMonth`:

```go
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
```

Change the struct comment to `// RecurringInvoice repeats a source invoice every PeriodMonths months; the cron job creates the next invoice once NextDate (minus the draft days for draft_ahead) is reached` and add after `LastRunUnixTime`:

```go
	PeriodMonths    int32  `xorm:"NOT NULL DEFAULT 1"`
	Mode            string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'create'"`
```

Add to both `RecurringInvoiceCreateRequest` and `RecurringInvoiceModifyRequest`:

```go
	PeriodMonths int32  `json:"periodMonths" binding:"min=0,max=120"` // 0 means 1 (clients before renewals)
	Mode         string `json:"mode" binding:"max=16"`                 // empty means create
```

Add to `RecurringInvoiceResponse`:

```go
	PeriodMonths        int32  `json:"periodMonths"`
	Mode                string `json:"mode"`
```

and set `PeriodMonths: r.PeriodMonths, Mode: r.Mode,` in `ToRecurringInvoiceResponse`.

In `pkg/errs/invoice.go` after code 42:

```go
	ErrRecurringInvoicePeriodInvalid     = NewNormalError(NormalSubcategoryInvoice, 43, http.StatusBadRequest, "recurring invoice period must be between 1 and 120 months")
	ErrRecurringInvoiceModeInvalid       = NewNormalError(NormalSubcategoryInvoice, 44, http.StatusBadRequest, "recurring invoice mode is invalid")
```

- [ ] **Step 4: Implement period stepping and validation**

In `pkg/services/recurring_invoices.go` replace `nextRecurringDate` and `catchUpRecurringDate`:

```go
func nextRecurringDate(date string, dayOfMonth int32, periodMonths int32) (string, error) {
	t, err := time.Parse(invoiceDateLayout, date)

	if err != nil {
		return "", errs.ErrInvoiceDateInvalid
	}

	return time.Date(t.Year(), t.Month()+time.Month(periodMonths), int(dayOfMonth), 0, 0, 0, 0, time.UTC).Format(invoiceDateLayout), nil
}

// catchUpRecurringDate steps a next date that is already in the past forward by whole periods until it is on or after today,
// so re-enabling a paused row does not back-fill one invoice per missed period and a yearly row keeps its anniversary
func catchUpRecurringDate(nextDate string, dayOfMonth int32, periodMonths int32, today string) string {
	candidate := nextDate

	for candidate < today {
		next, err := nextRecurringDate(candidate, dayOfMonth, periodMonths)

		if err != nil {
			return nextDate
		}

		candidate = next
	}

	return candidate
}
```

Replace `validateRecurringInvoice` (it now also fills the defaults):

```go
// validateRecurringInvoice fills the period and mode defaults (1 month, create) and checks every field
func validateRecurringInvoice(recurring *models.RecurringInvoice) error {
	if recurring.DayOfMonth < 1 || recurring.DayOfMonth > models.RecurringInvoiceMaxDayOfMonth {
		return errs.ErrRecurringInvoiceDayOfMonthInvalid
	}

	if _, err := time.Parse(invoiceDateLayout, recurring.NextDate); err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	if recurring.PeriodMonths == 0 {
		recurring.PeriodMonths = 1
	}

	if recurring.PeriodMonths < 1 || recurring.PeriodMonths > models.RecurringInvoiceMaxPeriodMonths {
		return errs.ErrRecurringInvoicePeriodInvalid
	}

	if recurring.Mode == "" {
		recurring.Mode = models.RECURRING_INVOICE_MODE_CREATE
	}

	if !models.IsValidRecurringInvoiceMode(recurring.Mode) {
		return errs.ErrRecurringInvoiceModeInvalid
	}

	return nil
}
```

In `ModifyRecurringInvoice` change the catch-up call to `catchUpRecurringDate(recurring.NextDate, recurring.DayOfMonth, recurring.PeriodMonths, time.Now().Format(invoiceDateLayout))` and the updated columns to `sess.Cols("day_of_month", "next_date", "enabled", "period_months", "mode", "updated_unix_time")`.

In `createRecurringInvoice` change `nextRecurringDate(recurring.NextDate, recurring.DayOfMonth)` to `nextRecurringDate(recurring.NextDate, recurring.DayOfMonth, max(recurring.PeriodMonths, 1))` (rows read before the column existed have 0 in memory only if Sync2 has not run; `max` keeps them monthly).

- [ ] **Step 5: Pass the fields through the API**

In `pkg/api/recurring_invoices.go`:
- create handler: `recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: req.SourceInvoiceId, DayOfMonth: req.DayOfMonth, NextDate: req.NextDate, PeriodMonths: req.PeriodMonths, Mode: req.Mode, Enabled: true}`
- modify handler: `recurring := &models.RecurringInvoice{RecurringId: req.Id, Uid: uid, DayOfMonth: req.DayOfMonth, NextDate: req.NextDate, PeriodMonths: req.PeriodMonths, Mode: req.Mode, Enabled: req.Enabled}`

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ -run 'Recurring' -count=1`
Expected: PASS (all recurring tests, old and new).

- [ ] **Step 7: Commit**

```bash
git add pkg/models/recurring_invoice.go pkg/errs/invoice.go pkg/services/recurring_invoices.go pkg/services/recurring_invoices_test.go pkg/api/recurring_invoices.go
git commit --no-verify -m "recurring invoices: repeat every N months with a mode (create, draft ahead, auto send)"
```

---

### Task 2: Renewal draft days on the invoice profile (backend)

**Files:**
- Modify: `pkg/models/invoice.go` (InvoiceProfile, InvoiceProfileUpdateRequest, InvoiceProfileResponse, ToInvoiceProfileResponse)
- Modify: `pkg/services/invoices.go:96-98` (`defaultInvoiceProfile`)
- Modify: `pkg/api/invoices.go` (`InvoiceProfileUpdateHandler`)
- Test: `pkg/services/invoices_test.go`

**Interfaces:**
- Produces: `InvoiceProfile.RenewalDraftDays int32` (DB default 15), JSON `renewalDraftDays` on profile get/update; `models.InvoiceRenewalDraftDaysDefault = 15`, `models.InvoiceRenewalDraftDaysMax = 90`.

- [ ] **Step 1: Write the failing test**

Append to `pkg/services/invoices_test.go`:

```go
func TestInvoiceProfileRenewalDraftDaysDefaultsAndPersists(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, int32(models.InvoiceRenewalDraftDaysDefault), profile.RenewalDraftDays)

	profile.RenewalDraftDays = 30
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, profile))

	reloaded, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, int32(30), reloaded.RenewalDraftDays)
	assert.Equal(t, int32(30), reloaded.ToInvoiceProfileResponse().RenewalDraftDays)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/services/ -run TestInvoiceProfileRenewalDraftDays -count=1`
Expected: compile FAIL (`profile.RenewalDraftDays undefined`).

- [ ] **Step 3: Implement**

In `pkg/models/invoice.go` near the other invoice constants:

```go
// InvoiceRenewalDraftDaysDefault is how many days before the renewal date a draft_ahead invoice is created, unless the profile says otherwise
const InvoiceRenewalDraftDaysDefault = 15

// InvoiceRenewalDraftDaysMax is the largest allowed renewal draft lead time
const InvoiceRenewalDraftDaysMax = 90
```

Add to `InvoiceProfile` after `DefaultCurrency`:

```go
	RenewalDraftDays     int32  `xorm:"NOT NULL DEFAULT 15"`
```

Add to `InvoiceProfileUpdateRequest`: ``RenewalDraftDays int32 `json:"renewalDraftDays" binding:"min=0,max=90"` ``.
Add to `InvoiceProfileResponse`: ``RenewalDraftDays int32 `json:"renewalDraftDays"` `` and set `RenewalDraftDays: p.RenewalDraftDays,` in `ToInvoiceProfileResponse`.

In `pkg/services/invoices.go`:

```go
func defaultInvoiceProfile(uid int64) *models.InvoiceProfile {
	return &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", DefaultTermsDays: 7, RenewalDraftDays: models.InvoiceRenewalDraftDaysDefault}
}
```

In `InvoiceProfileUpdateHandler` add `RenewalDraftDays: req.RenewalDraftDays,` to the profile literal.

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/services/ ./pkg/api/ -run 'InvoiceProfile' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/invoice.go pkg/services/invoices.go pkg/services/invoices_test.go pkg/api/invoices.go
git commit --no-verify -m "invoices: renewal draft days on the invoice profile (default 15)"
```

---
### Task 3: The daily job creates drafts ahead and auto-sends (backend)

**Files:**
- Modify: `pkg/models/invoice.go` (`Invoice.RecurringId`)
- Modify: `pkg/services/invoices.go` (`prepareNewInvoice` resets `RecurringId`)
- Modify: `pkg/services/recurring_invoices.go` (`CreateRecurringInvoices`, `createRecurringInvoice`, new `autoSendRecurringInvoice`)
- Modify: `pkg/mail/mailer_container.go` (`SetMailer`)
- Test: `pkg/services/recurring_invoices_test.go`

**Interfaces:**
- Consumes (Task 1): `RecurringInvoice.PeriodMonths`, `RecurringInvoice.Mode`, mode constants. (Task 2): `InvoiceProfile.RenewalDraftDays`, `models.InvoiceRenewalDraftDaysMax`.
- Produces: `Invoice.RecurringId int64` (the schedule that generated the invoice, 0 otherwise); `createRecurringInvoice(c, recurring) (*models.Invoice, error)` (nil invoice when the source is deleted); `mail.Container.SetMailer(mailer Mailer) Mailer`; test helper `withCapturingMailer(t) *capturingMailer`.

- [ ] **Step 1: Add the test mailer hook**

In `pkg/mail/mailer_container.go`:

```go
// SetMailer replaces the current mailer and returns the previous one; tests use it to capture sent mail
func (m *MailerContainer) SetMailer(mailer Mailer) Mailer {
	previous := m.current
	m.current = mailer
	return previous
}
```

- [ ] **Step 2: Write the failing tests**

Append to `pkg/services/recurring_invoices_test.go` (add `"github.com/mayswind/ezbookkeeping/pkg/mail"` to the imports):

```go
type capturingMailer struct {
	sent []*mail.MailMessage
}

func (m *capturingMailer) SendMail(message *mail.MailMessage) error {
	m.sent = append(m.sent, message)
	return nil
}

// withCapturingMailer enables SMTP for one test and records every sent mail instead of sending it
func withCapturingMailer(t *testing.T) *capturingMailer {
	t.Helper()
	withSmtpEnabled(t)
	mailer := &capturingMailer{}
	previous := mail.Container.SetMailer(mailer)
	t.Cleanup(func() { mail.Container.SetMailer(previous) })
	return mailer
}

func createTestRenewal(t *testing.T, uid int64, sourceInvoiceId int64, nextDate string, periodMonths int32, mode string) *models.RecurringInvoice {
	t.Helper()
	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: sourceInvoiceId, DayOfMonth: 15, NextDate: nextDate, PeriodMonths: periodMonths, Mode: mode, Enabled: true}
	assert.Nil(t, Invoices.CreateRecurringInvoice(core.NewNullContext(), recurring))
	return recurring
}

func TestDraftAheadCreatesOneDraftInsideTheWindow(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	// 16 days before: outside the default 15-day window
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-11-29"))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))

	// exactly 15 days before: the draft is created, dated on the renewal date, and not sent
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-11-30"))
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-12-15", reloaded.NextDate)

	draft, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "2026-12-15", draft.InvoiceDate)
	assert.Equal(t, int64(0), draft.SentUnixTime)
	assert.Equal(t, recurring.RecurringId, draft.RecurringId)

	// the next days of the window do not create another one
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-01"))
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))
	all, err = Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
}

func TestDraftAheadUsesTheProfileDraftDays(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	profile.RenewalDraftDays = 3
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, profile))
	customer := createTestCustomer(t, uid)
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 1, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-11"))
	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-12"))
	all, err = Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
}

func TestAutoSendEmailsTheCustomerOnTheDate(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	customer.Email = "accounts@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 6, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-14"))
	assert.Equal(t, 0, len(mailer.sent))

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))
	assert.Equal(t, 1, len(mailer.sent))
	assert.Equal(t, "accounts@qubertech.com", mailer.sent[0].To)

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-06-15", reloaded.NextDate)
	sent, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.True(t, sent.SentUnixTime > 0)
}

func TestAutoSendFailureKeepsTheUnsentInvoiceAndAdvances(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withCapturingMailer(t)
	customer := createTestCustomer(t, uid) // no email address
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	recurring := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 1, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, recurring.RecurringId)
	assert.Nil(t, err)
	assert.Equal(t, "2027-01-15", reloaded.NextDate)
	unsent, _, err := Invoices.GetInvoice(ctx, uid, reloaded.LastInvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), unsent.SentUnixTime)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestDraftAhead|TestAutoSend' -count=1`
Expected: compile FAIL (`draft.RecurringId undefined`).

- [ ] **Step 4: Add `Invoice.RecurringId`**

In `pkg/models/invoice.go` `Invoice` struct after `SentUnixTime`:

```go
	// RecurringId is the recurring invoice that created this invoice; 0 for invoices made by hand
	RecurringId int64 `xorm:"INDEX(IDX_invoice_uid_recurring_id) NOT NULL DEFAULT 0"`
```

In `prepareNewInvoice` (`pkg/services/invoices.go`), next to `invoice.SentUnixTime = 0`, add `invoice.RecurringId = 0`.

- [ ] **Step 5: Implement the modes in the job**

In `pkg/services/recurring_invoices.go` replace `CreateRecurringInvoices`:

```go
// CreateRecurringInvoices creates the next invoice of every enabled recurring invoice that is due: on the next date for create
// and auto_send rows, and up to the profile's renewal draft days earlier for draft_ahead rows. auto_send invoices are emailed
// to the customer right after they are created. A failing row is logged and left for the next run; it never fails the cron job.
func (s *InvoiceService) CreateRecurringInvoices(c core.Context, today string) error {
	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return errs.ErrInvoiceDateInvalid
	}

	// draft_ahead rows are read up to the longest lead time any user may set, then filtered by their own profile
	latestDraftDate := todayTime.AddDate(0, 0, models.InvoiceRenewalDraftDaysMax).Format(invoiceDateLayout)

	for i := 0; i < s.UserDataDBCount(); i++ {
		var recurrings []*models.RecurringInvoice
		err := s.UserDataDBByIndex(i).NewSession(c).
			Where("deleted=? AND enabled=? AND (next_date<=? OR (mode=? AND next_date<=?))", false, true, today, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD, latestDraftDate).
			Find(&recurrings)

		if err != nil {
			return err
		}

		draftDaysByUid := make(map[int64]int32)

		for _, recurring := range recurrings {
			if recurring.NextDate > today {
				draftDays, loaded := draftDaysByUid[recurring.Uid]

				if !loaded {
					profile, err := s.GetProfile(c, recurring.Uid)

					if err != nil {
						log.Errorf(c, "[recurring_invoices.CreateRecurringInvoices] failed to get invoice profile of user \"uid:%d\", because %s", recurring.Uid, err.Error())
						continue
					}

					draftDays = profile.RenewalDraftDays
					draftDaysByUid[recurring.Uid] = draftDays
				}

				if recurring.NextDate > todayTime.AddDate(0, 0, int(draftDays)).Format(invoiceDateLayout) {
					continue
				}
			}

			invoice, err := s.createRecurringInvoice(c, recurring)

			if err != nil {
				log.Errorf(c, "[recurring_invoices.CreateRecurringInvoices] failed to create invoice for recurring invoice \"id:%d\" of user \"uid:%d\", because %s", recurring.RecurringId, recurring.Uid, err.Error())
				continue
			}

			if invoice != nil && recurring.Mode == models.RECURRING_INVOICE_MODE_AUTO_SEND {
				s.autoSendRecurringInvoice(c, invoice)
			}
		}
	}

	return nil
}

// autoSendRecurringInvoice emails a just-created auto_send invoice to its bill-to address with the saved template.
// A failure only logs: the invoice stays unsent and the home card offers to send it by hand.
func (s *InvoiceService) autoSendRecurringInvoice(c core.Context, invoice *models.Invoice) {
	if invoice.BillToEmail == "" {
		log.Warnf(c, "[recurring_invoices.autoSendRecurringInvoice] invoice \"id:%d\" of user \"uid:%d\" was not sent, because the customer has no email", invoice.InvoiceId, invoice.Uid)
		return
	}

	if err := s.SendInvoiceWithContent(c, invoice.Uid, invoice.InvoiceId, invoice.BillToEmail, "", nil, nil); err != nil {
		log.Warnf(c, "[recurring_invoices.autoSendRecurringInvoice] failed to send invoice \"id:%d\" of user \"uid:%d\", because %s", invoice.InvoiceId, invoice.Uid, err.Error())
	}
}
```

Change `createRecurringInvoice` to return the created invoice:
- signature `func (s *InvoiceService) createRecurringInvoice(c core.Context, recurring *models.RecurringInvoice) (*models.Invoice, error)`
- deleted source: `return nil, nil`; every other `return err` becomes `return nil, err`
- after `if err := s.prepareNewInvoice(c, &invoice, lines); err != nil { return nil, err }` add `invoice.RecurringId = recurring.RecurringId`
- final line `return &invoice, nil`

- [ ] **Step 6: Run tests**

Run: `go test ./pkg/services/ -run 'Recurring|TestDraftAhead|TestAutoSend|TestSendInvoice' -count=1`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/mail/mailer_container.go pkg/models/invoice.go pkg/services/invoices.go pkg/services/recurring_invoices.go pkg/services/recurring_invoices_test.go
git commit --no-verify -m "recurring invoices: create drafts ahead of the date and auto-send on the date"
```

---

### Task 4: Renewals-due list (backend)

**Files:**
- Modify: `pkg/models/recurring_invoice.go` (renewal state constants, `InvoiceRenewal`, `InvoiceRenewalResponse`)
- Modify: `pkg/services/recurring_invoices.go` (`GetInvoiceRenewals`)
- Modify: `pkg/api/recurring_invoices.go` (`InvoiceRenewalListHandler`)
- Modify: `cmd/webserver.go` (route)
- Test: `pkg/services/recurring_invoices_test.go`

**Interfaces:**
- Consumes (Task 3): `Invoice.RecurringId`, `withCapturingMailer`, `createTestRenewal`.
- Produces: `GET /api/v1/invoices/renewals.json` → `[]InvoiceRenewalResponse{ state: "due"|"send_failed"|"auto_sent", recurringId: string, invoice: InvoiceInfoResponse }`; `Invoices.GetInvoiceRenewals(c, uid int64, nowUnixTime int64) ([]*models.InvoiceRenewal, error)`.

- [ ] **Step 1: Write the failing test**

```go
func TestGetInvoiceRenewalsClassifiesGeneratedInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid) // no email: auto-send fails
	source := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)

	draftAhead := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	autoSend := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)
	createOnly := createTestRenewal(t, uid, source.InvoiceId, "2026-12-15", 12, models.RECURRING_INVOICE_MODE_CREATE)
	assert.Nil(t, Invoices.CreateRecurringInvoices(ctx, "2026-12-15"))

	now := time.Now().Unix()
	renewals, err := Invoices.GetInvoiceRenewals(ctx, uid, now)
	assert.Nil(t, err)
	states := map[int64]string{}
	for _, renewal := range renewals {
		states[renewal.RecurringId] = renewal.State
	}
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_DUE, states[draftAhead.RecurringId])
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_SEND_FAILED, states[autoSend.RecurringId])
	_, listed := states[createOnly.RecurringId]
	assert.False(t, listed)

	// once the failed one is sent it shows as auto sent for 7 days, then disappears
	reloaded, err := Invoices.GetRecurringInvoice(ctx, uid, autoSend.RecurringId)
	assert.Nil(t, err)
	_, err = Invoices.UserDataDB(uid).NewSession(ctx).Cols("sent_unix_time").Where("invoice_id=?", reloaded.LastInvoiceId).Update(&models.Invoice{SentUnixTime: now - 3600})
	assert.Nil(t, err)
	renewals, err = Invoices.GetInvoiceRenewals(ctx, uid, now)
	assert.Nil(t, err)
	states = map[int64]string{}
	for _, renewal := range renewals {
		states[renewal.RecurringId] = renewal.State
	}
	assert.Equal(t, models.INVOICE_RENEWAL_STATE_AUTO_SENT, states[autoSend.RecurringId])

	renewals, err = Invoices.GetInvoiceRenewals(ctx, uid, now+8*24*3600)
	assert.Nil(t, err)
	for _, renewal := range renewals {
		assert.NotEqual(t, autoSend.RecurringId, renewal.RecurringId)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/services/ -run TestGetInvoiceRenewals -count=1`
Expected: compile FAIL (`Invoices.GetInvoiceRenewals undefined`).

- [ ] **Step 3: Implement**

In `pkg/models/recurring_invoice.go`:

```go
// Renewal states shown on the home card
const (
	INVOICE_RENEWAL_STATE_DUE         = "due"         // a draft_ahead invoice waiting to be sent
	INVOICE_RENEWAL_STATE_SEND_FAILED = "send_failed" // an auto_send invoice that could not be emailed
	INVOICE_RENEWAL_STATE_AUTO_SENT   = "auto_sent"   // an auto_send invoice emailed in the last few days
)

// InvoiceRenewalAutoSentVisibleSeconds is how long an auto-sent renewal stays on the home card (7 days)
const InvoiceRenewalAutoSentVisibleSeconds = 7 * 24 * 3600

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
```

In `pkg/services/recurring_invoices.go`:

```go
// GetInvoiceRenewals returns the generated invoices the home card lists, soonest invoice date first: unsent draft_ahead
// invoices (due), unsent auto_send invoices (send failed) and auto_send invoices sent in the last 7 days (auto sent).
// The schedule's current mode decides the state; create-mode invoices are never listed.
func (s *InvoiceService) GetInvoiceRenewals(c core.Context, uid int64, nowUnixTime int64) ([]*models.InvoiceRenewal, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).
		Where("uid=? AND deleted=? AND recurring_id>? AND closed_manually=? AND (sent_unix_time=? OR sent_unix_time>=?)", uid, false, 0, false, 0, nowUnixTime-models.InvoiceRenewalAutoSentVisibleSeconds).
		OrderBy("invoice_date asc, number asc").Find(&invoices)

	if err != nil || len(invoices) < 1 {
		return nil, err
	}

	recurringIds := make([]int64, 0, len(invoices))

	for _, invoice := range invoices {
		recurringIds = append(recurringIds, invoice.RecurringId)
	}

	var recurrings []*models.RecurringInvoice
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("recurring_id", recurringIds).Find(&recurrings)

	if err != nil {
		return nil, err
	}

	modes := make(map[int64]string, len(recurrings))

	for _, recurring := range recurrings {
		modes[recurring.RecurringId] = recurring.Mode
	}

	renewals := make([]*models.InvoiceRenewal, 0, len(invoices))

	for _, invoice := range invoices {
		state := ""

		switch mode := modes[invoice.RecurringId]; {
		case mode == models.RECURRING_INVOICE_MODE_DRAFT_AHEAD && invoice.SentUnixTime == 0:
			state = models.INVOICE_RENEWAL_STATE_DUE
		case mode == models.RECURRING_INVOICE_MODE_AUTO_SEND && invoice.SentUnixTime == 0:
			state = models.INVOICE_RENEWAL_STATE_SEND_FAILED
		case mode == models.RECURRING_INVOICE_MODE_AUTO_SEND:
			state = models.INVOICE_RENEWAL_STATE_AUTO_SENT
		}

		if state != "" {
			renewals = append(renewals, &models.InvoiceRenewal{State: state, RecurringId: invoice.RecurringId, Invoice: invoice})
		}
	}

	return renewals, nil
}
```

In `pkg/api/recurring_invoices.go` (add `"time"` import):

```go
// InvoiceRenewalListHandler returns the renewals the home card lists for current user
func (a *InvoicesApi) InvoiceRenewalListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	renewals, err := a.invoices.GetInvoiceRenewals(c, uid, time.Now().Unix())

	if err != nil {
		log.Errorf(c, "[recurring_invoices.InvoiceRenewalListHandler] failed to get renewals for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	invoiceIds := make([]int64, len(renewals))

	for idx, renewal := range renewals {
		invoiceIds[idx] = renewal.Invoice.InvoiceId
	}

	payments, err := a.invoices.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		log.Errorf(c, "[recurring_invoices.InvoiceRenewalListHandler] failed to get payments for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.InvoiceRenewalResponse, len(renewals))

	for idx, renewal := range renewals {
		resps[idx] = &models.InvoiceRenewalResponse{
			State:       renewal.State,
			RecurringId: utils.Int64ToString(renewal.RecurringId),
			Invoice:     renewal.Invoice.ToInvoiceInfoResponse(nil, payments[renewal.Invoice.InvoiceId]),
		}
	}

	return resps, nil
}
```

(add `"github.com/mayswind/ezbookkeeping/pkg/utils"` to the imports if missing.)

In `cmd/webserver.go` next to the recurring routes:

```go
			apiV1Route.GET("/invoices/renewals.json", bindApi(api.Invoices.InvoiceRenewalListHandler, config))
```

- [ ] **Step 4: Run tests and build**

Run: `go test ./pkg/services/ -run 'TestGetInvoiceRenewals|Recurring' -count=1 && go build ./...`
Expected: PASS, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/recurring_invoice.go pkg/services/recurring_invoices.go pkg/services/recurring_invoices_test.go pkg/api/recurring_invoices.go cmd/webserver.go
git commit --no-verify -m "invoices: renewals list for the home card (due, send failed, auto sent)"
```

---
### Task 5: Product catalog and product-linked invoice lines (backend)

**Files:**
- Create: `pkg/models/product.go`
- Modify: `pkg/models/invoice.go` (`InvoiceLine.ProductId`, `InvoiceLineRequest.ProductId`, `InvoiceLineResponse.ProductId`, both line loops in `ToInvoiceInfoResponse` / `ToPublicInvoiceResponse` — public response leaves it empty)
- Modify: `pkg/errs/invoice.go` (codes 50-53)
- Create: `pkg/services/products.go`
- Modify: `pkg/services/invoices.go` (`prepareInvoice` checks product ownership)
- Create: `pkg/api/products.go`
- Modify: `pkg/api/invoices.go` (`invoiceLinesFromRequest`)
- Modify: `cmd/webserver.go`, `cmd/database.go:240`, `pkg/services/alerts_testutil_test.go:61`
- Test: `pkg/services/products_test.go`

**Interfaces:**
- Consumes (Task 1): `models.IsValidRecurringInvoiceMode`, mode constants, `models.RecurringInvoiceMaxPeriodMonths`.
- Produces:
  - `models.Product{ProductId, Uid, Deleted, Name, Description, Sac, DefaultRate, Currency, RenewalMonths, DefaultRenewalMode, Archived, ...}`; `ProductResponse` JSON `{id, name, description, sac, defaultRate, currency, renewalMonths, defaultRenewalMode, archived}`.
  - `InvoiceLine.ProductId int64`; line request/response JSON `productId` (string; `""`/`"0"` = none).
  - Services: `GetAllProducts(c, uid) ([]*models.Product, error)`, `GetProductsByIds(c, uid, ids []int64) (map[int64]*models.Product, error)`, `CreateProduct(c, *models.Product) error`, `ModifyProduct(c, *models.Product) error`, `DeleteProduct(c, uid, productId) error`, `GetUnlinkedInvoiceLineGroups(c, uid) ([]*models.InvoiceLineGroupResponse, error)`, `LinkInvoiceLinesToProduct(c, uid, itemName string, productId int64) (int64, error)`, helper `invoiceLineGroupKey(itemName string) string`.
  - Routes: `GET /v1/products/list.json`, `POST /v1/products/add.json`, `POST /v1/products/modify.json`, `POST /v1/products/delete.json`, `GET /v1/products/unlinked_lines.json`, `POST /v1/products/link_lines.json`.
  - Errors: `ErrProductIdInvalid` (50), `ErrProductNotFound` (51), `ErrProductInUse` (52), `ErrInvoiceLineProductInvalid` (53).

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/products_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func createTestProduct(t *testing.T, uid int64, name string, renewalMonths int32, mode string) *models.Product {
	t.Helper()
	product := &models.Product{Uid: uid, Name: name, DefaultRate: 150000, RenewalMonths: renewalMonths, DefaultRenewalMode: mode}
	assert.Nil(t, Invoices.CreateProduct(core.NewNullContext(), product))
	return product
}

func createTestInvoiceWithLines(t *testing.T, uid int64, customerId int64, number string, lines []*models.InvoiceLine) *models.Invoice {
	t.Helper()
	invoice := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateInvoice(core.NewNullContext(), invoice, lines))
	return invoice
}

func TestProductCrudIsScopedToUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)

	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	createTestProduct(t, uid, "Audit", 0, "")

	all, err := Invoices.GetAllProducts(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
	assert.Equal(t, "Audit", all[0].Name) // ordered by name

	others, err := Invoices.GetAllProducts(ctx, otherUid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(others))

	domain.Uid = otherUid
	assert.Equal(t, errs.ErrProductNotFound, Invoices.ModifyProduct(ctx, domain))

	domain.Uid = uid
	domain.Archived = true
	assert.Nil(t, Invoices.ModifyProduct(ctx, domain))
	byId, err := Invoices.GetProductsByIds(ctx, uid, []int64{domain.ProductId})
	assert.Nil(t, err)
	assert.True(t, byId[domain.ProductId].Archived)
}

func TestProductValidatesRenewal(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	err := Invoices.CreateProduct(ctx, &models.Product{Uid: uid, Name: "Hosting", RenewalMonths: 121, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_CREATE})
	assert.Equal(t, errs.ErrRecurringInvoicePeriodInvalid, err)

	err = Invoices.CreateProduct(ctx, &models.Product{Uid: uid, Name: "Hosting", RenewalMonths: 12, DefaultRenewalMode: "weekly"})
	assert.Equal(t, errs.ErrRecurringInvoiceModeInvalid, err)

	// a one-off product never carries a mode
	oneOff := &models.Product{Uid: uid, Name: "Audit", DefaultRenewalMode: "weekly"}
	assert.Nil(t, Invoices.CreateProduct(ctx, oneOff))
	assert.Equal(t, "", oneOff.DefaultRenewalMode)
}

func TestInvoiceLinesKeepProductAndRefuseOtherUsersProduct(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	foreign := createTestProduct(t, otherUid, "Foreign", 0, "")

	invoice := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
	})
	_, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, domain.ProductId, lines[0].ProductId)

	bad := &models.Invoice{Uid: uid, Number: "INV/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	err = Invoices.CreateInvoice(ctx, bad, []*models.InvoiceLine{{ItemName: "x", ProductId: foreign.ProductId, Quantity: 100, Rate: 1}})
	assert.Equal(t, errs.ErrInvoiceLineProductInvalid, err)
}

func TestDeleteProductRefusedWhileLinked(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	invoice := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
	})

	assert.Equal(t, errs.ErrProductInUse, Invoices.DeleteProduct(ctx, uid, domain.ProductId))

	// a deleted invoice no longer holds the product
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	assert.Nil(t, Invoices.DeleteProduct(ctx, uid, domain.ProductId))
	assert.Equal(t, errs.ErrProductNotFound, Invoices.DeleteProduct(ctx, uid, domain.ProductId))
}

func TestLinkPastInvoiceLinesByItemName(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "Domain Renewal", Quantity: 100, Rate: 1},
		{ItemName: "Support", Quantity: 100, Rate: 1},
	})
	createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "  domain renewal ", Quantity: 100, Rate: 1},
	})

	groups, err := Invoices.GetUnlinkedInvoiceLineGroups(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(groups))
	assert.Equal(t, "Domain Renewal", groups[0].ItemName) // most used first
	assert.Equal(t, 2, groups[0].Count)

	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	linked, err := Invoices.LinkInvoiceLinesToProduct(ctx, uid, "DOMAIN RENEWAL", domain.ProductId)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), linked)

	groups, err = Invoices.GetUnlinkedInvoiceLineGroups(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(groups))
	assert.Equal(t, "Support", groups[0].ItemName)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'Product|TestLinkPastInvoiceLines|TestInvoiceLinesKeepProduct' -count=1`
Expected: compile FAIL (`undefined: models.Product`).

- [ ] **Step 3: Add the model, errors and table registration**

Create `pkg/models/product.go`:

```go
package models

import "github.com/mayswind/ezbookkeeping/pkg/utils"

// Product is an item of the user's catalog; RenewalMonths 0 means a one-off product, which never carries a renewal mode
type Product struct {
	ProductId          int64  `xorm:"PK"`
	Uid                int64  `xorm:"INDEX(IDX_product_uid_deleted) NOT NULL"`
	Deleted            bool   `xorm:"INDEX(IDX_product_uid_deleted) NOT NULL"`
	Name               string `xorm:"VARCHAR(128) NOT NULL"`
	Description        string `xorm:"VARCHAR(512) NOT NULL"`
	Sac                string `xorm:"VARCHAR(8) NOT NULL DEFAULT ''"`
	DefaultRate        int64  `xorm:"NOT NULL"`
	Currency           string `xorm:"VARCHAR(3) NOT NULL DEFAULT ''"` // empty means the profile's default currency
	RenewalMonths      int32  `xorm:"NOT NULL DEFAULT 0"`
	DefaultRenewalMode string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	Archived           bool   `xorm:"NOT NULL DEFAULT false"`
	CreatedUnixTime    int64
	UpdatedUnixTime    int64
	DeletedUnixTime    int64
}

// ProductCreateRequest represents all parameters of product creation request
type ProductCreateRequest struct {
	Name               string `json:"name" binding:"required,notBlank,max=128"`
	Description        string `json:"description" binding:"max=512"`
	Sac                string `json:"sac" binding:"max=8"`
	DefaultRate        int64  `json:"defaultRate" binding:"min=0,max=10000000000"`
	Currency           string `json:"currency" binding:"max=3"`
	RenewalMonths      int32  `json:"renewalMonths" binding:"min=0,max=120"`
	DefaultRenewalMode string `json:"defaultRenewalMode" binding:"max=16"`
	Archived           bool   `json:"archived"`
}

// ProductModifyRequest represents all parameters of product modification request
type ProductModifyRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
	ProductCreateRequest
}

// ProductResponse represents a view-object of product
type ProductResponse struct {
	Id                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Sac                string `json:"sac"`
	DefaultRate        int64  `json:"defaultRate"`
	Currency           string `json:"currency"`
	RenewalMonths      int32  `json:"renewalMonths"`
	DefaultRenewalMode string `json:"defaultRenewalMode"`
	Archived           bool   `json:"archived"`
}

// ProductLinkLinesRequest links every unlinked invoice line whose item name matches (trimmed, case-insensitive) to a product
type ProductLinkLinesRequest struct {
	ProductId int64  `json:"productId,string" binding:"required,min=1"`
	ItemName  string `json:"itemName" binding:"required,notBlank,max=128"`
}

// InvoiceLineGroupResponse is a group of unlinked invoice lines sharing an item name
type InvoiceLineGroupResponse struct {
	ItemName string `json:"itemName"`
	Count    int    `json:"count"`
}

// ToProductResponse returns a view-object according to database model
func (p *Product) ToProductResponse() *ProductResponse {
	return &ProductResponse{
		Id:                 utils.Int64ToString(p.ProductId),
		Name:               p.Name,
		Description:        p.Description,
		Sac:                p.Sac,
		DefaultRate:        p.DefaultRate,
		Currency:           p.Currency,
		RenewalMonths:      p.RenewalMonths,
		DefaultRenewalMode: p.DefaultRenewalMode,
		Archived:           p.Archived,
	}
}
```

In `pkg/errs/invoice.go`:

```go
	// products start at 50
	ErrProductIdInvalid          = NewNormalError(NormalSubcategoryInvoice, 50, http.StatusBadRequest, "product id is invalid")
	ErrProductNotFound           = NewNormalError(NormalSubcategoryInvoice, 51, http.StatusBadRequest, "product not found")
	ErrProductInUse              = NewNormalError(NormalSubcategoryInvoice, 52, http.StatusBadRequest, "product is used by invoices, archive it instead")
	ErrInvoiceLineProductInvalid = NewNormalError(NormalSubcategoryInvoice, 53, http.StatusBadRequest, "invoice line product is invalid")
```

Add `new(models.Product)` to the invoice table list in `cmd/database.go:240` and to the list in `pkg/services/alerts_testutil_test.go:61`.

In `pkg/models/invoice.go`:
- `InvoiceLine`: ``ProductId int64 `xorm:"INDEX(IDX_invoice_line_uid_product_id) NOT NULL DEFAULT 0"` `` (and change `Uid` tag to `xorm:"INDEX(IDX_invoice_line_uid_product_id) NOT NULL"`).
- `InvoiceLineRequest`: ``ProductId int64 `json:"productId,string" binding:"min=0"` ``.
- `InvoiceLineResponse`: ``ProductId string `json:"productId,omitempty"` ``.
- In `ToInvoiceInfoResponse` line loop, after building each line response:

```go
			if line.ProductId > 0 {
				resp.Lines[idx].ProductId = utils.Int64ToString(line.ProductId)
			}
```

(the public response never sets it.)

- [ ] **Step 4: Implement the service**

Create `pkg/services/products.go`:

```go
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
```

In `prepareInvoice` (`pkg/services/invoices.go`), right after `validateInvoiceDates`:

```go
	if err := s.checkInvoiceLineProducts(c, invoice.Uid, lines); err != nil {
		return err
	}
```

- [ ] **Step 5: Implement the API and routes**

In `pkg/api/invoices.go` `invoiceLinesFromRequest`, add `ProductId: lineReq.ProductId,`.

Create `pkg/api/products.go`:

```go
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
```

In `cmd/webserver.go` after the customer routes:

```go
			apiV1Route.GET("/products/list.json", bindApi(api.Invoices.ProductListHandler, config))
			apiV1Route.POST("/products/add.json", bindApi(api.Invoices.ProductCreateHandler, config))
			apiV1Route.POST("/products/modify.json", bindApi(api.Invoices.ProductModifyHandler, config))
			apiV1Route.POST("/products/delete.json", bindApi(api.Invoices.ProductDeleteHandler, config))
			apiV1Route.GET("/products/unlinked_lines.json", bindApi(api.Invoices.ProductUnlinkedLinesHandler, config))
			apiV1Route.POST("/products/link_lines.json", bindApi(api.Invoices.ProductLinkLinesHandler, config))
```

- [ ] **Step 6: Run tests and build**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS (whole package, so the new prepareInvoice check breaks nothing), build succeeds.

- [ ] **Step 7: Commit**

```bash
git add pkg/models/product.go pkg/models/invoice.go pkg/errs/invoice.go pkg/services/products.go pkg/services/products_test.go pkg/services/invoices.go pkg/services/alerts_testutil_test.go pkg/api/products.go pkg/api/invoices.go cmd/webserver.go cmd/database.go
git commit --no-verify -m "products: catalog with renewal defaults, invoice lines linked to products, linking past lines by item name"
```

---
### Task 6: Offer and create a renewal when an invoice with renewable products is sent (backend)

**Files:**
- Modify: `pkg/models/recurring_invoice.go` (`InvoiceRenewalOption`)
- Modify: `pkg/models/invoice.go` (`InvoiceInfoResponse.RenewalOption`, `InvoiceSendRequest.CreateRenewal`)
- Modify: `pkg/services/recurring_invoices.go` (`renewalOptionForLines`, `GetInvoiceRenewalOption`, `CreateRenewalForInvoice`)
- Modify: `pkg/api/invoices.go` (`getInvoiceResponse`, `InvoiceSendHandler`)
- Test: `pkg/services/recurring_invoices_test.go`

**Interfaces:**
- Consumes (Task 5): `Product`, `GetProductsByIds`, `createTestProduct`, `createTestInvoiceWithLines`. (Task 3): `Invoice.RecurringId`.
- Produces: `models.InvoiceRenewalOption{PeriodMonths int32 "periodMonths", Mode string "mode", MixedPeriods bool "mixedPeriods", Scheduled bool "scheduled"}`; detail JSON `renewalOption` (absent when the invoice has no renewable product); send request JSON `createRenewal` (bool); `Invoices.GetInvoiceRenewalOption(c, uid, invoiceId) (*models.InvoiceRenewalOption, error)`; `Invoices.CreateRenewalForInvoice(c, uid, invoiceId) (*models.RecurringInvoice, error)` (nil, nil when nothing to create).

- [ ] **Step 1: Write the failing tests**

```go
func TestRenewalOptionForLines(t *testing.T) {
	products := map[int64]*models.Product{
		1: {ProductId: 1, RenewalMonths: 12, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_AUTO_SEND},
		2: {ProductId: 2, RenewalMonths: 12, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_DRAFT_AHEAD},
		3: {ProductId: 3, RenewalMonths: 6, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_CREATE},
		4: {ProductId: 4},
	}

	assert.Nil(t, renewalOptionForLines([]*models.InvoiceLine{{ProductId: 4}, {ProductId: 0}}, products))

	option := renewalOptionForLines([]*models.InvoiceLine{{ProductId: 1}, {ProductId: 2}, {ProductId: 4}}, products)
	assert.Equal(t, int32(12), option.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD, option.Mode) // the most hands-on mode wins
	assert.False(t, option.MixedPeriods)

	option = renewalOptionForLines([]*models.InvoiceLine{{ProductId: 1}, {ProductId: 3}}, products)
	assert.True(t, option.MixedPeriods)
}

func TestCreateRenewalForInvoiceOnce(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_AUTO_SEND)
	invoice := &models.Invoice{Uid: uid, Number: "INV/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-10-31", DueDate: "2026-11-07"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, []*models.InvoiceLine{{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000}}))

	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.False(t, option.Scheduled)

	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int32(12), recurring.PeriodMonths)
	assert.Equal(t, models.RECURRING_INVOICE_MODE_AUTO_SEND, recurring.Mode)
	assert.Equal(t, int32(28), recurring.DayOfMonth)
	assert.Equal(t, "2027-10-28", recurring.NextDate)

	option, err = Invoices.GetInvoiceRenewalOption(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.True(t, option.Scheduled)

	again, err := Invoices.CreateRenewalForInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, again)
}

func TestCreateRenewalForInvoiceSkipsMixedPeriodsAndGeneratedInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	amc := createTestProduct(t, uid, "AMC", 6, models.RECURRING_INVOICE_MODE_AUTO_SEND)

	mixed := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
		{ItemName: "AMC", ProductId: amc.ProductId, Quantity: 100, Rate: 1},
	})
	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, mixed.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)

	// an invoice the scheduler generated is renewed by its own schedule
	single := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
	})
	_, err = Invoices.UserDataDB(uid).NewSession(ctx).Cols("recurring_id").Where("invoice_id=?", single.InvoiceId).Update(&models.Invoice{RecurringId: 42})
	assert.Nil(t, err)
	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, single.InvoiceId)
	assert.Nil(t, err)
	assert.True(t, option.Scheduled)
	recurring, err = Invoices.CreateRenewalForInvoice(ctx, uid, single.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestRenewalOption|TestCreateRenewalForInvoice' -count=1`
Expected: compile FAIL (`undefined: renewalOptionForLines`).

- [ ] **Step 3: Implement**

In `pkg/models/recurring_invoice.go`:

```go
// InvoiceRenewalOption is the renewal schedule sending an invoice can start, from the renewable products on its lines
type InvoiceRenewalOption struct {
	PeriodMonths int32  `json:"periodMonths"`
	Mode         string `json:"mode"`
	MixedPeriods bool   `json:"mixedPeriods"` // renewable products with different periods: split the invoice to track them
	Scheduled    bool   `json:"scheduled"`    // a schedule already renews this invoice
}
```

In `pkg/models/invoice.go`: `InvoiceInfoResponse` gets ``RenewalOption *InvoiceRenewalOption `json:"renewalOption,omitempty"` ``; `InvoiceSendRequest` gets ``CreateRenewal bool `json:"createRenewal"` ``.

In `pkg/services/recurring_invoices.go`:

```go
// renewalModeRank orders modes from least to most hands-on; when products disagree, the most hands-on one wins
var renewalModeRank = map[string]int{
	models.RECURRING_INVOICE_MODE_AUTO_SEND:   1,
	models.RECURRING_INVOICE_MODE_CREATE:      2,
	models.RECURRING_INVOICE_MODE_DRAFT_AHEAD: 3,
}

// renewalOptionForLines returns the renewal the lines' renewable products ask for, or nil when none is renewable
func renewalOptionForLines(lines []*models.InvoiceLine, products map[int64]*models.Product) *models.InvoiceRenewalOption {
	var option *models.InvoiceRenewalOption

	for _, line := range lines {
		product := products[line.ProductId]

		if product == nil || product.RenewalMonths < 1 {
			continue
		}

		if option == nil {
			option = &models.InvoiceRenewalOption{PeriodMonths: product.RenewalMonths, Mode: product.DefaultRenewalMode}
			continue
		}

		if product.RenewalMonths != option.PeriodMonths {
			option.MixedPeriods = true
		}

		if renewalModeRank[product.DefaultRenewalMode] > renewalModeRank[option.Mode] {
			option.Mode = product.DefaultRenewalMode
		}
	}

	return option
}

// GetInvoiceRenewalOption returns the renewal sending this invoice can start, or nil when it has no renewable product
func (s *InvoiceService) GetInvoiceRenewalOption(c core.Context, uid int64, invoiceId int64) (*models.InvoiceRenewalOption, error) {
	invoice, lines, err := s.GetInvoice(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	productIds := make([]int64, 0, len(lines))

	for _, line := range lines {
		if line.ProductId > 0 {
			productIds = append(productIds, line.ProductId)
		}
	}

	products, err := s.GetProductsByIds(c, uid, productIds)

	if err != nil {
		return nil, err
	}

	option := renewalOptionForLines(lines, products)

	if option == nil {
		return nil, nil
	}

	scheduled := invoice.RecurringId > 0

	if !scheduled {
		scheduled, err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND source_invoice_id=?", uid, false, invoiceId).Exist(&models.RecurringInvoice{})

		if err != nil {
			return nil, err
		}
	}

	option.Scheduled = scheduled
	return option, nil
}

// CreateRenewalForInvoice starts the renewal schedule of a sent invoice: every period months on its day of month (capped at 28),
// first one period after the invoice date. It returns nil when there is nothing to create (no renewable product, mixed periods, already scheduled).
func (s *InvoiceService) CreateRenewalForInvoice(c core.Context, uid int64, invoiceId int64) (*models.RecurringInvoice, error) {
	option, err := s.GetInvoiceRenewalOption(c, uid, invoiceId)

	if err != nil || option == nil || option.MixedPeriods || option.Scheduled {
		return nil, err
	}

	invoice, err := s.getInvoiceHeader(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}

	invoiceDate, err := time.Parse(invoiceDateLayout, invoice.InvoiceDate)

	if err != nil {
		return nil, errs.ErrInvoiceDateInvalid
	}

	dayOfMonth := int32(min(invoiceDate.Day(), models.RecurringInvoiceMaxDayOfMonth))
	nextDate, err := nextRecurringDate(invoice.InvoiceDate, dayOfMonth, option.PeriodMonths)

	if err != nil {
		return nil, err
	}

	recurring := &models.RecurringInvoice{Uid: uid, SourceInvoiceId: invoiceId, DayOfMonth: dayOfMonth, NextDate: nextDate, PeriodMonths: option.PeriodMonths, Mode: option.Mode, Enabled: true}

	if err := s.CreateRecurringInvoice(c, recurring); err != nil {
		return nil, err
	}

	return recurring, nil
}
```

In `pkg/api/invoices.go` `getInvoiceResponse`, before `return resp, nil`:

```go
	resp.RenewalOption, err = a.invoices.GetInvoiceRenewalOption(c, uid, invoiceId)

	if err != nil {
		return nil, err
	}
```

In `InvoiceSendHandler`, right after a successful `SendInvoiceWithContent`:

```go
	// the email is out; a failure to start the renewal is logged, never reported as a failed send
	if req.CreateRenewal {
		if _, err := a.invoices.CreateRenewalForInvoice(c, uid, req.Id); err != nil {
			log.Errorf(c, "[invoices.InvoiceSendHandler] failed to create renewal for invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		}
	}
```

- [ ] **Step 4: Run tests and build**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/recurring_invoice.go pkg/models/invoice.go pkg/services/recurring_invoices.go pkg/services/recurring_invoices_test.go pkg/api/invoices.go
git commit --no-verify -m "invoices: sending an invoice with renewable products can start its renewal schedule"
```

---

### Task 7: Customer overview: products bought and timeline (backend)

**Files:**
- Modify: `pkg/models/invoice.go` (overview response types), `pkg/models/recurring_invoice.go` (`RecurringInvoiceResponse.CustomerId`)
- Create: `pkg/services/customer_overview.go`, `pkg/services/customer_overview_test.go`
- Modify: `pkg/api/invoices.go` (`CustomerOverviewHandler`), `cmd/webserver.go`

**Interfaces:**
- Consumes (Tasks 3, 5): `Invoice.RecurringId`, `InvoiceLine.ProductId`, `GetProductsByIds`.
- Produces:
  - `GET /v1/customers/overview.json?id=` → `CustomerOverviewResponse{ products: CustomerProductResponse[], timeline: CustomerTimelineItemResponse[] }`.
  - `CustomerProductResponse{ productId, name, timesBought int, lastBoughtDate string, archived bool }`.
  - `CustomerTimelineItemResponse{ kind: "invoice"|"email"|"payment", unixTime int64, invoiceId, invoiceNumber string, amount int64, currency string, text string, renewal bool }`.
  - `RecurringInvoiceResponse.CustomerId string` (JSON `customerId`, empty when the source is deleted).
  - `Invoices.GetCustomerOverview(c, uid, customerId) (*models.CustomerOverviewResponse, error)`; `models.CustomerTimelineMaxItems = 200`.

- [ ] **Step 1: Write the failing test**

Create `pkg/services/customer_overview_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCustomerOverviewListsProductsAndTimeline(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	otherCustomer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)

	first := &models.Invoice{Uid: uid, Number: "INV/2025-26/01", CustomerId: customer.CustomerId, InvoiceDate: "2025-10-01", DueDate: "2025-10-08"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, first, []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
		{ItemName: "Setup", Quantity: 100, Rate: 500000},
	}))
	second := &models.Invoice{Uid: uid, Number: "INV/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-10-01", DueDate: "2026-10-08"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, second, []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 160000},
	}))
	createTestInvoiceWithLines(t, uid, otherCustomer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 1},
	})

	accountId := createTestAccount(t, ctx, uid, "HDFC", "INR")
	payment := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 650000, "2025-10-05")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, first.InvoiceId, payment.TransactionId, 0))

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)

	assert.Equal(t, 1, len(overview.Products))
	assert.Equal(t, "Domain", overview.Products[0].Name)
	assert.Equal(t, 2, overview.Products[0].TimesBought)
	assert.Equal(t, "2026-10-01", overview.Products[0].LastBoughtDate)

	kinds := map[string]int{}
	for _, item := range overview.Timeline {
		kinds[item.Kind]++
	}
	assert.Equal(t, 2, kinds[models.CUSTOMER_TIMELINE_KIND_INVOICE]) // the other customer's invoice is not here
	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_PAYMENT])

	for i := 1; i < len(overview.Timeline); i++ {
		assert.True(t, overview.Timeline[i-1].UnixTime >= overview.Timeline[i].UnixTime) // newest first
	}

	_, err = Invoices.GetCustomerOverview(ctx, otherUid, customer.CustomerId)
	assert.Equal(t, errs.ErrCustomerNotFound, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/services/ -run TestCustomerOverview -count=1`
Expected: compile FAIL (`Invoices.GetCustomerOverview undefined`).

- [ ] **Step 3: Implement**

In `pkg/models/invoice.go`:

```go
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
```

In `pkg/models/recurring_invoice.go` add ``CustomerId string `json:"customerId"` `` to `RecurringInvoiceResponse` and `resp.CustomerId = utils.Int64ToString(source.CustomerId)` inside `if source != nil`.

Create `pkg/services/customer_overview.go`:

```go
package services

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// GetCustomerOverview returns the products a customer bought and their timeline (invoices, emails, payments), newest first
func (s *InvoiceService) GetCustomerOverview(c core.Context, uid int64, customerId int64) (*models.CustomerOverviewResponse, error) {
	if _, err := s.GetCustomer(c, uid, customerId); err != nil {
		return nil, err
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Find(&invoices)

	if err != nil {
		return nil, err
	}

	overview := &models.CustomerOverviewResponse{Products: []*models.CustomerProductResponse{}, Timeline: []*models.CustomerTimelineItemResponse{}}

	if len(invoices) < 1 {
		return overview, nil
	}

	invoiceIds := make([]int64, len(invoices))
	invoiceMap := make(map[int64]*models.Invoice, len(invoices))

	for idx, invoice := range invoices {
		invoiceIds[idx] = invoice.InvoiceId
		invoiceMap[invoice.InvoiceId] = invoice
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_INVOICE, UnixTime: invoice.CreatedUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Amount: invoice.Total, Currency: invoice.Currency, Renewal: invoice.RecurringId > 0})
	}

	if err := s.addCustomerProducts(c, uid, invoiceIds, invoiceMap, overview); err != nil {
		return nil, err
	}

	var emails []*models.InvoiceEmail
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", invoiceIds).Omit("email_html").Find(&emails)

	if err != nil {
		return nil, err
	}

	for _, email := range emails {
		invoice := invoiceMap[email.InvoiceId]
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_EMAIL, UnixTime: email.SentUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Text: email.Subject})
	}

	var links []*models.InvoicePayment
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", invoiceIds).Find(&links)

	if err != nil {
		return nil, err
	}

	payments, err := s.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		return nil, err
	}

	for _, link := range links {
		invoice := invoiceMap[link.InvoiceId]

		for _, payment := range payments[link.InvoiceId] {
			if payment.Transaction.TransactionId == link.TransactionId {
				overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_PAYMENT, UnixTime: link.LinkedUnixTime,
					InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Amount: payment.ReceivedAmount(), Currency: invoice.Currency})
			}
		}
	}

	sort.SliceStable(overview.Timeline, func(i, j int) bool { return overview.Timeline[i].UnixTime > overview.Timeline[j].UnixTime })

	if len(overview.Timeline) > models.CustomerTimelineMaxItems {
		overview.Timeline = overview.Timeline[:models.CustomerTimelineMaxItems]
	}

	return overview, nil
}

// addCustomerProducts fills the products bought from the product-linked lines of the customer's invoices, most recently bought first
func (s *InvoiceService) addCustomerProducts(c core.Context, uid int64, invoiceIds []int64, invoiceMap map[int64]*models.Invoice, overview *models.CustomerOverviewResponse) error {
	var lines []*models.InvoiceLine
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND product_id>?", uid, 0).In("invoice_id", invoiceIds).Find(&lines)

	if err != nil {
		return err
	}

	invoicesByProduct := make(map[int64]map[int64]bool)
	productIds := make([]int64, 0)

	for _, line := range lines {
		if invoicesByProduct[line.ProductId] == nil {
			invoicesByProduct[line.ProductId] = make(map[int64]bool)
			productIds = append(productIds, line.ProductId)
		}

		invoicesByProduct[line.ProductId][line.InvoiceId] = true
	}

	products, err := s.GetProductsByIds(c, uid, productIds)

	if err != nil {
		return err
	}

	for _, productId := range productIds {
		product := products[productId]

		if product == nil {
			continue
		}

		item := &models.CustomerProductResponse{ProductId: utils.Int64ToString(productId), Name: product.Name, Archived: product.Archived}

		for invoiceId := range invoicesByProduct[productId] {
			item.TimesBought++

			if date := invoiceMap[invoiceId].InvoiceDate; date > item.LastBoughtDate {
				item.LastBoughtDate = date
			}
		}

		overview.Products = append(overview.Products, item)
	}

	sort.SliceStable(overview.Products, func(i, j int) bool { return overview.Products[i].LastBoughtDate > overview.Products[j].LastBoughtDate })

	return nil
}
```

In `pkg/api/invoices.go`:

```go
// CustomerOverviewHandler returns the products bought and the timeline of a customer of current user
func (a *InvoicesApi) CustomerOverviewHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.CustomerOverviewHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	overview, err := a.invoices.GetCustomerOverview(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[invoices.CustomerOverviewHandler] failed to get overview of customer \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return overview, nil
}
```

Route in `cmd/webserver.go`: `apiV1Route.GET("/customers/overview.json", bindApi(api.Invoices.CustomerOverviewHandler, config))`.

- [ ] **Step 4: Run tests and build**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/models/invoice.go pkg/models/recurring_invoice.go pkg/services/customer_overview.go pkg/services/customer_overview_test.go pkg/api/invoices.go cmd/webserver.go
git commit --no-verify -m "customers: overview with products bought and a timeline of invoices, emails and payments"
```

---
### Task 8: Frontend data layer: models, API calls, store, product helpers

**Files:**
- Modify: `src/models/invoice.ts`
- Create: `src/models/product.ts`
- Create: `src/lib/product.ts`, `src/lib/__tests__/product.test.ts`
- Modify: `src/lib/services.ts` (next to the customer and recurring calls, ~line 1066-1130)
- Modify: `src/stores/invoice.ts`

**Interfaces:**
- Consumes (Tasks 1-7): JSON fields `periodMonths`, `mode`, `customerId` on recurring invoices; `renewalDraftDays` on the profile; `productId` on lines; `renewalOption` on invoice detail; `createRenewal` on send; routes `/v1/products/*`, `/v1/invoices/renewals.json`, `/v1/customers/overview.json`.
- Produces:
  - `type RecurringInvoiceMode = 'create' | 'draft_ahead' | 'auto_send'`, `RECURRING_INVOICE_MODES: { mode, name }[]`, `RECURRING_INVOICE_PERIODS: { months, name }[]` in `src/models/invoice.ts`.
  - `InvoiceLine.productId: string` (`''` = none); `Invoice.renewalOption: InvoiceRenewalOption | null`; `InvoiceSendRequest.createRenewal?: boolean`; `RecurringInvoice.periodMonths/mode/customerId`; `InvoiceProfile.renewalDraftDays`.
  - Types `InvoiceRenewalOption`, `InvoiceRenewal { state, recurringId, invoice: Invoice }`, `CustomerOverview`, `CustomerProduct`, `CustomerTimelineItem`.
  - `class Product` and `InvoiceLineGroup` in `src/models/product.ts`.
  - `filterProductSuggestions(products, query, limit?)`, `applyProductToLine(line, product)` in `src/lib/product.ts`.
  - Store: `allProducts`, `allProductsMap`, `loadAllProducts({ force })`, `saveProduct({ product, isEdit })`, `deleteProduct({ product })`, `getUnlinkedInvoiceLineGroups()`, `linkInvoiceLinesToProduct({ itemName, productId })`, `loadInvoiceRenewals()`, `getCustomerOverview({ customerId })`.

- [ ] **Step 1: Write the failing tests**

Create `src/lib/__tests__/product.test.ts`:

```ts
import { describe, expect, it } from 'vitest';

import { Product } from '@/models/product.ts';
import { Invoice } from '@/models/invoice.ts';
import { filterProductSuggestions, applyProductToLine } from '@/lib/product.ts';

function product(id: string, name: string, archived = false): Product {
    const result = Product.createNewProduct();
    result.id = id;
    result.name = name;
    result.archived = archived;
    return result;
}

describe('filterProductSuggestions', () => {
    const products = [product('1', 'Website Hosting'), product('2', 'Domain'), product('3', 'Old Domain', true), product('4', 'AMC Domain Support')];

    it('hides archived products and matches anywhere in the name, case-insensitively', () => {
        expect(filterProductSuggestions(products, 'domain').map(p => p.id)).toEqual(['2', '4']);
    });

    it('lists names starting with the query first', () => {
        expect(filterProductSuggestions(products, 'do').map(p => p.id)).toEqual(['2', '4']);
        expect(filterProductSuggestions(products, 'host').map(p => p.id)).toEqual(['1']);
    });

    it('lists every active product for an empty query, up to the limit', () => {
        expect(filterProductSuggestions(products, '  ').map(p => p.id)).toEqual(['4', '2', '1']);
        expect(filterProductSuggestions(products, '', 2).length).toBe(2);
    });
});

describe('applyProductToLine', () => {
    it('copies the product into the line and keeps the quantity', () => {
        const line = Invoice.createNewLine('998314');
        line.quantity = 300;
        const domain = product('2', 'Domain');
        domain.description = 'codeation.io, 1 year';
        domain.defaultRate = 150000;

        applyProductToLine(line, domain);

        expect(line).toEqual({ itemName: 'Domain', description: 'codeation.io, 1 year', quantity: 300, rate: 150000, amount: 0, sac: '998314', productId: '2' });
    });

    it('uses the product SAC when it has one', () => {
        const line = Invoice.createNewLine('998314');
        const hosting = product('1', 'Website Hosting');
        hosting.sac = '998315';

        applyProductToLine(line, hosting);

        expect(line.sac).toBe('998315');
    });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/product.test.ts`
Expected: FAIL (`Failed to resolve import "@/models/product.ts"`).

- [ ] **Step 3: Extend `src/models/invoice.ts`**

After `INVOICE_TAX_TYPES` add:

```ts
// mirrors models.RECURRING_INVOICE_MODE_* in pkg/models/recurring_invoice.go
export type RecurringInvoiceMode = 'create' | 'draft_ahead' | 'auto_send';

// the renewal modes with their locale keys, in menu order
export const RECURRING_INVOICE_MODES: { readonly mode: RecurringInvoiceMode, readonly name: string }[] = [
    { mode: 'draft_ahead', name: 'Draft ahead, I send it' },
    { mode: 'auto_send', name: 'Create and send automatically' },
    { mode: 'create', name: 'Create only' }
];

// the period choices offered in menus; any 1..120 is accepted by the server
export const RECURRING_INVOICE_PERIODS: { readonly months: number, readonly name: string }[] = [
    { months: 1, name: 'Every month' },
    { months: 3, name: 'Every 3 months' },
    { months: 6, name: 'Every 6 months' },
    { months: 12, name: 'Every year' },
    { months: 24, name: 'Every 2 years' }
];

// what sending an invoice can start (absent when it has no renewable product)
export interface InvoiceRenewalOption {
    readonly periodMonths: number;
    readonly mode: RecurringInvoiceMode;
    readonly mixedPeriods: boolean;
    readonly scheduled: boolean;
}
```

Changes to existing types:
- `InvoiceProfileInfoResponse`: add `readonly renewalDraftDays: number;`. `InvoiceProfile`: add `public renewalDraftDays: number = 15;`, `this.renewalDraftDays = other.renewalDraftDays ?? 15;` in `fillFrom`, `renewalDraftDays: this.renewalDraftDays,` in `toUpdateRequest`.
- `InvoiceLine`: add `productId: string; // '' = typed by hand`. `InvoiceLineRequest`: add `readonly productId: string;`.
- `InvoiceInfoResponse`: add `readonly renewalOption?: InvoiceRenewalOption;`. `Invoice`: add `public renewalOption: InvoiceRenewalOption | null = null;` and in `fillFrom` `this.renewalOption = other.renewalOption ?? null;`; change the lines copy to `this.lines = (other.lines ?? []).map(line => ({ ...line, productId: line.productId ?? '' }));`.
- `Invoice.toCreateRequest` line mapping: add `productId: line.productId || '0'`.
- `Invoice.createNewLine`: `return { itemName: '', description: '', quantity: 100, rate: 0, amount: 0, sac, productId: '' };`.
- `InvoiceSendRequest`: add `readonly createRenewal?: boolean;`.
- `RecurringInvoiceInfoResponse`: add `readonly customerId: string; readonly periodMonths: number; readonly mode: RecurringInvoiceMode;`. `RecurringInvoiceCreateRequest` and `RecurringInvoiceModifyRequest`: add `readonly periodMonths: number; readonly mode: RecurringInvoiceMode;`.
- `RecurringInvoice`: fields `public customerId: string = ''; public periodMonths: number = 1; public mode: RecurringInvoiceMode = 'create';`; `fillFrom` copies them (`other.periodMonths || 1`, `other.mode || 'create'`, `other.customerId ?? ''`); `toCreateRequest` and `toModifyRequest` add `periodMonths: this.periodMonths, mode: this.mode`.

Append to the file:

```ts
export interface InvoiceRenewalInfoResponse {
    readonly state: 'due' | 'send_failed' | 'auto_sent';
    readonly recurringId: string;
    readonly invoice: InvoiceInfoResponse;
}

export interface InvoiceRenewal {
    readonly state: 'due' | 'send_failed' | 'auto_sent';
    readonly recurringId: string;
    readonly invoice: Invoice;
}

export interface CustomerProduct {
    readonly productId: string;
    readonly name: string;
    readonly timesBought: number;
    readonly lastBoughtDate: TextualYearMonthDay;
    readonly archived: boolean;
}

export interface CustomerTimelineItem {
    readonly kind: 'invoice' | 'email' | 'payment';
    readonly unixTime: number;
    readonly invoiceId: string;
    readonly invoiceNumber: string;
    readonly amount: number;
    readonly currency: string;
    readonly text: string;
    readonly renewal: boolean;
}

export interface CustomerOverview {
    readonly products: CustomerProduct[];
    readonly timeline: CustomerTimelineItem[];
}
```

- [ ] **Step 4: Create `src/models/product.ts`**

```ts
import type { RecurringInvoiceMode } from '@/models/invoice.ts';

export interface ProductInfoResponse {
    readonly id: string;
    readonly name: string;
    readonly description: string;
    readonly sac: string;
    readonly defaultRate: number;
    readonly currency: string; // '' = the profile's default currency
    readonly renewalMonths: number; // 0 = one-off
    readonly defaultRenewalMode: RecurringInvoiceMode | '';
    readonly archived: boolean;
}

export type ProductCreateRequest = Omit<ProductInfoResponse, 'id'>;

export interface ProductModifyRequest extends ProductCreateRequest {
    readonly id: string;
}

// unlinked invoice lines sharing an item name (trimmed, case-insensitive)
export interface InvoiceLineGroup {
    readonly itemName: string;
    readonly count: number;
}

export class Product implements ProductInfoResponse {
    public id: string = '';
    public name: string = '';
    public description: string = '';
    public sac: string = '';
    public defaultRate: number = 0;
    public currency: string = '';
    public renewalMonths: number = 0;
    public defaultRenewalMode: RecurringInvoiceMode | '' = '';
    public archived: boolean = false;

    public get renewable(): boolean {
        return this.renewalMonths > 0;
    }

    public fillFrom(other: ProductInfoResponse): void {
        this.id = other.id;
        this.name = other.name;
        this.description = other.description;
        this.sac = other.sac;
        this.defaultRate = other.defaultRate;
        this.currency = other.currency;
        this.renewalMonths = other.renewalMonths;
        this.defaultRenewalMode = other.defaultRenewalMode;
        this.archived = other.archived;
    }

    public toCreateRequest(): ProductCreateRequest {
        return {
            name: this.name.trim(),
            description: this.description,
            sac: this.sac.trim(),
            defaultRate: this.defaultRate,
            currency: this.currency,
            renewalMonths: this.renewalMonths,
            defaultRenewalMode: this.renewalMonths > 0 ? (this.defaultRenewalMode || 'draft_ahead') : '',
            archived: this.archived
        };
    }

    public toModifyRequest(): ProductModifyRequest {
        return { id: this.id, ...this.toCreateRequest() };
    }

    public clone(): Product {
        return Product.of(this);
    }

    public static of(response: ProductInfoResponse): Product {
        const product = new Product();
        product.fillFrom(response);
        return product;
    }

    public static ofMulti(responses: ProductInfoResponse[]): Product[] {
        return responses.map(response => Product.of(response));
    }

    public static createNewProduct(): Product {
        return new Product();
    }
}
```

- [ ] **Step 5: Create `src/lib/product.ts`**

```ts
import type { InvoiceLine } from '@/models/invoice.ts';
import type { Product } from '@/models/product.ts';

// active products whose name contains the query (case-insensitive), names starting with it first, then alphabetical
export function filterProductSuggestions(products: readonly Product[], query: string, limit: number = 8): Product[] {
    const needle = query.trim().toLowerCase();

    return products
        .filter(product => !product.archived && (!needle || product.name.toLowerCase().includes(needle)))
        .sort((a, b) => {
            const aStarts = needle && a.name.toLowerCase().startsWith(needle) ? 0 : 1;
            const bStarts = needle && b.name.toLowerCase().startsWith(needle) ? 0 : 1;
            return aStarts - bStarts || a.name.localeCompare(b.name);
        })
        .slice(0, limit);
}

// fills a line from a product; the quantity stays, the SAC only changes when the product has one, every field stays editable
export function applyProductToLine(line: InvoiceLine, product: Product): void {
    line.productId = product.id;
    line.itemName = product.name;
    line.description = product.description;
    line.rate = product.defaultRate;

    if (product.sac) {
        line.sac = product.sac;
    }
}
```

- [ ] **Step 6: API calls in `src/lib/services.ts`**

Import `ProductInfoResponse, ProductCreateRequest, ProductModifyRequest, InvoiceLineGroup` from `@/models/product.ts` and `InvoiceRenewalInfoResponse, CustomerOverview` from `@/models/invoice.ts`, then add next to the customer calls:

```ts
    getCustomerOverview: ({ id }: { id: string }): ApiResponsePromise<CustomerOverview> => {
        return axios.get<ApiResponse<CustomerOverview>>('v1/customers/overview.json?id=' + id);
    },
    getAllProducts: (): ApiResponsePromise<ProductInfoResponse[]> => {
        return axios.get<ApiResponse<ProductInfoResponse[]>>('v1/products/list.json');
    },
    addProduct: (req: ProductCreateRequest): ApiResponsePromise<ProductInfoResponse> => {
        return axios.post<ApiResponse<ProductInfoResponse>>('v1/products/add.json', req);
    },
    modifyProduct: (req: ProductModifyRequest): ApiResponsePromise<ProductInfoResponse> => {
        return axios.post<ApiResponse<ProductInfoResponse>>('v1/products/modify.json', req);
    },
    deleteProduct: ({ id }: { id: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>('v1/products/delete.json', { id });
    },
    getUnlinkedInvoiceLineGroups: (): ApiResponsePromise<InvoiceLineGroup[]> => {
        return axios.get<ApiResponse<InvoiceLineGroup[]>>('v1/products/unlinked_lines.json');
    },
    linkInvoiceLinesToProduct: (req: { productId: string, itemName: string }): ApiResponsePromise<number> => {
        return axios.post<ApiResponse<number>>('v1/products/link_lines.json', req);
    },
    getInvoiceRenewals: (): ApiResponsePromise<InvoiceRenewalInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceRenewalInfoResponse[]>>('v1/invoices/renewals.json');
    },
```


- [ ] **Step 7: Store in `src/stores/invoice.ts`**

Imports: `import { type InvoiceLineGroup, Product } from '@/models/product.ts';` and add `type InvoiceRenewal, type CustomerOverview` to the invoice model import. State:

```ts
    const allProducts = ref<Product[]>([]);
    const allProductsMap = ref<Record<string, Product>>({});
```

Functions:

```ts
    function loadProductList(products: Product[]): void {
        allProducts.value = products;
        allProductsMap.value = {};

        for (const product of products) {
            allProductsMap.value[product.id] = product;
        }
    }

    // ponytail: never cached, the list is small and also changes from other devices
    function loadAllProducts({ force }: { force?: boolean }): Promise<Product[]> {
        return request(services.getAllProducts(), 'Unable to retrieve product list').then(result => {
            const products = Product.ofMulti(result);

            if (force && isEquals(allProducts.value, products)) {
                return Promise.reject({ message: 'Product list is up to date', isUpToDate: true });
            }

            loadProductList(products);
            return products;
        });
    }

    function saveProduct({ product, isEdit }: { product: Product, isEdit: boolean }): Promise<Product> {
        const apiRequest = isEdit ? services.modifyProduct(product.toModifyRequest()) : services.addProduct(product.toCreateRequest());

        return request(apiRequest, isEdit ? 'Unable to save product' : 'Unable to add product').then(result => {
            const saved = Product.of(result);
            const others = allProducts.value.filter(item => item.id !== saved.id);
            loadProductList([...others, saved].sort((a, b) => a.name.localeCompare(b.name)));
            return saved;
        });
    }

    function deleteProduct({ product }: { product: Product }): Promise<boolean> {
        return request(services.deleteProduct({ id: product.id }), 'Unable to delete this product').then(result => {
            loadProductList(allProducts.value.filter(item => item.id !== product.id));
            return result;
        });
    }

    function getUnlinkedInvoiceLineGroups(): Promise<InvoiceLineGroup[]> {
        return request(services.getUnlinkedInvoiceLineGroups(), 'Unable to retrieve invoice lines');
    }

    function linkInvoiceLinesToProduct({ itemName, productId }: { itemName: string, productId: string }): Promise<number> {
        return request(services.linkInvoiceLinesToProduct({ itemName, productId }), 'Unable to link invoice lines');
    }

    function loadInvoiceRenewals(): Promise<InvoiceRenewal[]> {
        return request(services.getInvoiceRenewals(), 'Unable to retrieve renewals')
            .then(result => result.map(item => ({ state: item.state, recurringId: item.recurringId, invoice: Invoice.of(item.invoice) })));
    }

    function getCustomerOverview({ customerId }: { customerId: string }): Promise<CustomerOverview> {
        return request(services.getCustomerOverview({ id: customerId }), 'Unable to retrieve customer');
    }
```

In `resetInvoices` add `allProducts.value = []; allProductsMap.value = {};`. Export the two states and seven functions in the returned object.

- [ ] **Step 8: Run tests and typecheck**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/product.test.ts src/lib/__tests__/invoice.test.ts src/lib/__tests__/recurringInvoice.test.ts && npx vue-tsc --noEmit`
Expected: PASS; typecheck clean. If `invoice.test.ts` compares whole line objects, add `productId: ''` to its expected lines (lines now always carry it).

- [ ] **Step 9: Commit**

```bash
git add src/models/invoice.ts src/models/product.ts src/lib/product.ts src/lib/__tests__/product.test.ts src/lib/services.ts src/stores/invoice.ts src/lib/__tests__/invoice.test.ts
git commit --no-verify -m "frontend: products, renewal modes, renewals and customer overview in models, API and store"
```

---
### Task 9: Recurring invoice UI: period, mode and editing; renewal draft days in the profile

**Files:**
- Modify: `src/views/base/invoices/RecurringInvoiceEditPageBase.ts`, `src/views/base/invoices/RecurringInvoiceListPageBase.ts`
- Modify: `src/views/desktop/invoices/dialogs/RecurringInvoiceEditDialog.vue`, `src/views/desktop/invoices/ListPage.vue` (recurring table)
- Modify: `src/views/mobile/invoices/RecurringInvoiceAddPopup.vue`, `src/views/mobile/invoices/RecurringListPage.vue`
- Modify: `src/views/desktop/settings/InvoiceProfileSettingPage.vue`, `src/views/mobile/invoices/ProfilePage.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Task 8): `RECURRING_INVOICE_MODES`, `RECURRING_INVOICE_PERIODS`, `RecurringInvoice.periodMonths/mode`, `InvoiceProfile.renewalDraftDays`, store `modifyRecurringInvoice`.
- Produces: `useRecurringInvoiceEditPageBase()` gains `editRecurringId`, `title`, `saveButtonTitle`, `allModes`, `allPeriods`, `initFromRecurring(recurring)`; `useRecurringInvoiceListPageBase()` gains `getScheduleText(recurring): string`; desktop dialog `open({ invoice })` (add) or `open({ recurring })` (edit); mobile popup prop `recurring?: RecurringInvoice | null` for editing. Later tasks reuse `getScheduleText`.

- [ ] **Step 1: Locale strings**

Add to `src/locales/en.json` next to `"Repeat Monthly"`:

```json
    "Repeat": "Repeat",
    "Edit Recurring Invoice": "Edit Recurring Invoice",
    "Repeat Every": "Repeat Every",
    "Every month": "Every month",
    "Every 3 months": "Every 3 months",
    "Every 6 months": "Every 6 months",
    "Every year": "Every year",
    "Every 2 years": "Every 2 years",
    "When Due": "When Due",
    "Draft ahead, I send it": "Draft ahead, I send it",
    "Create and send automatically": "Create and send automatically",
    "Create only": "Create only",
    "Period must be between 1 and 120 months": "Period must be between 1 and 120 months",
    "Renewal Drafts (days before)": "Renewal Drafts (days before)",
    "Schedule": "Schedule",
    "You have saved this recurring invoice": "You have saved this recurring invoice",
    "Draft ahead: the invoice is created this many days early (set in Invoice Profile) and waits for you to send it.": "Draft ahead: the invoice is created this many days early (set in Invoice Profile) and waits for you to send it.",
    "Send automatically: the invoice is emailed to the customer on the date.": "Send automatically: the invoice is emailed to the customer on the date.",
    "Create only: the invoice is created on the date and not sent.": "Create only: the invoice is created on the date and not sent.",
```

and inside `"format": { "misc": { ... } }` add `"everyNMonths": "Every {count} months"`.

- [ ] **Step 2: Extend the edit base**

Replace `src/views/base/invoices/RecurringInvoiceEditPageBase.ts` with:

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, type RecurringInvoiceMode, RecurringInvoice, RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS } from '@/models/invoice.ts';

import { getTodayTextualDate } from '@/lib/invoice.ts';
import { RECURRING_INVOICE_MAX_DAY_OF_MONTH, getDefaultRecurringDayOfMonth, getNextRecurringDate } from '@/lib/recurringInvoice.ts';

// the "Repeat" form: day of month, first date, period and mode; adds a schedule for an invoice or edits an existing one
export function useRecurringInvoiceEditPageBase() {
    const { tt, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const submitting = ref<boolean>(false);
    const editRecurringId = ref<string | null>(null);
    const recurring = ref<RecurringInvoice>(new RecurringInvoice());

    const title = computed<string>(() => editRecurringId.value ? 'Edit Recurring Invoice' : 'Repeat');
    const saveButtonTitle = computed<string>(() => editRecurringId.value ? 'Save' : 'Add');

    const allModes = computed<{ mode: RecurringInvoiceMode, displayName: string }[]>(() => RECURRING_INVOICE_MODES.map(item => ({ mode: item.mode, displayName: tt(item.name) })));
    const allPeriods = computed<{ months: number, displayName: string }[]>(() => RECURRING_INVOICE_PERIODS.map(item => ({ months: item.months, displayName: tt(item.name) })));
    const selectedModeName = computed<string>(() => allModes.value.find(item => item.mode === recurring.value.mode)?.displayName ?? '');
    const selectedPeriodName = computed<string>(() => allPeriods.value.find(item => item.months === recurring.value.periodMonths)?.displayName
        ?? tt('format.misc.everyNMonths', { count: recurring.value.periodMonths }));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!recurring.value.dayOfMonth || recurring.value.dayOfMonth < 1 || recurring.value.dayOfMonth > RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            return 'Day of month must be between 1 and 28';
        }

        if (!recurring.value.periodMonths || recurring.value.periodMonths < 1 || recurring.value.periodMonths > 120) {
            return 'Period must be between 1 and 120 months';
        }

        if (!recurring.value.nextDate || recurring.value.nextDate === '0-0-0') {
            return 'First invoice date cannot be blank';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function initFromInvoice(invoice: Invoice): void {
        const dayOfMonth = getDefaultRecurringDayOfMonth(invoice.invoiceDate);
        editRecurringId.value = null;
        recurring.value = RecurringInvoice.createNewRecurringInvoice(invoice.id, dayOfMonth, getNextRecurringDate(dayOfMonth, getTodayTextualDate()));
    }

    function initFromRecurring(existing: RecurringInvoice): void {
        editRecurringId.value = existing.id;
        recurring.value = existing.clone();
    }

    // changing the day moves a new schedule's first date to that day's next occurrence; an existing schedule keeps its date
    function setDayOfMonth(dayOfMonth: number): void {
        recurring.value.dayOfMonth = dayOfMonth;

        if (!editRecurringId.value && dayOfMonth >= 1 && dayOfMonth <= RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            recurring.value.nextDate = getNextRecurringDate(dayOfMonth, getTodayTextualDate());
        }
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        submitting,
        editRecurringId,
        recurring,
        // computed states
        title,
        saveButtonTitle,
        allModes,
        allPeriods,
        selectedModeName,
        selectedPeriodName,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        initFromInvoice,
        initFromRecurring,
        setDayOfMonth,
        formatDate
    };
}
```

In `RecurringInvoiceListPageBase.ts` add (and return) a schedule text helper:

```ts
    // "Every year · Draft ahead, I send it"
    function getScheduleText(recurring: RecurringInvoice): string {
        const period = RECURRING_INVOICE_PERIODS.find(item => item.months === recurring.periodMonths);
        const mode = RECURRING_INVOICE_MODES.find(item => item.mode === recurring.mode);
        const periodText = period ? tt(period.name) : tt('format.misc.everyNMonths', { count: recurring.periodMonths });
        return mode ? `${periodText} · ${tt(mode.name)}` : periodText;
    }
```

(import `RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS` from `@/models/invoice.ts`.)

- [ ] **Step 3: Desktop dialog adds and edits**

In `RecurringInvoiceEditDialog.vue`:
- title: `:title="tt(title)"`; footer button text `{{ tt(saveButtonTitle) }}`.
- Insert, before the help text column:

```vue
                        <v-col cols="12" md="6">
                            <v-combobox persistent-placeholder :disabled="submitting"
                                        :label="tt('Repeat Every')"
                                        :items="allPeriods" item-title="displayName" item-value="months" :return-object="false"
                                        :model-value="recurring.periodMonths"
                                        @update:model-value="recurring.periodMonths = Number($event) || 0" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-select persistent-placeholder :disabled="submitting"
                                      :label="tt('When Due')"
                                      :items="allModes" item-title="displayName" item-value="mode"
                                      v-model="recurring.mode" />
                        </v-col>
```

- Replace the help text with `{{ tt(modeHelpText) }}` where `const modeHelpText = computed(() => recurring.value.mode === 'draft_ahead' ? 'Draft ahead: the invoice is created this many days early (set in Invoice Profile) and waits for you to send it.' : recurring.value.mode === 'auto_send' ? 'Send automatically: the invoice is emailed to the customer on the date.' : 'Create only: the invoice is created on the date and not sent.');`
- `open` signature and save:

```ts
function open(options: { invoice?: Invoice, recurring?: RecurringInvoice }): Promise<RecurringInvoiceEditResponse> {
    showState.value = true;
    submitting.value = false;

    if (options.recurring) {
        initFromRecurring(options.recurring);
    } else if (options.invoice) {
        initFromInvoice(options.invoice);
    }

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

    const isEdit = !!editRecurringId.value;
    submitting.value = true;

    const saving = isEdit ? invoicesStore.modifyRecurringInvoice({ recurring: recurring.value }) : invoicesStore.addRecurringInvoice({ recurring: recurring.value });

    saving.then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: isEdit ? 'You have saved this recurring invoice' : 'You have added a recurring invoice', recurring: saved });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}
```

(destructure `editRecurringId, title, saveButtonTitle, allModes, allPeriods, initFromRecurring` from the base; import `computed`.)

In `InvoiceDetailDialog.vue` and `src/views/mobile/invoices/DetailPage.vue` change the `'Repeat Monthly'` labels to `'Repeat'`.

- [ ] **Step 4: Desktop recurring table shows and edits the schedule**

In `src/views/desktop/invoices/ListPage.vue` recurring table:
- header: add `<th>{{ tt('Schedule') }}</th>` after "Source Invoice"; change empty-row `colspan` to 6.
- row: add `<td>{{ getScheduleText(recurring) }}</td>` after the source cell.
- operation cell: before Delete add

```vue
                                    <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                           :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                           v-if="hoveredRecurringId === recurring.id && !loading"
                                           @click="editRecurring(recurring)">{{ tt('Edit') }}</v-btn>
```

- script: destructure `getScheduleText` from `useRecurringInvoiceListPageBase()`, import `RecurringInvoiceEditDialog from './dialogs/RecurringInvoiceEditDialog.vue'`, add `<recurring-invoice-edit-dialog ref="recurringInvoiceEditDialog" />` beside the other dialogs, and:

```ts
type RecurringInvoiceEditDialogType = InstanceType<typeof RecurringInvoiceEditDialog>;
const recurringInvoiceEditDialog = useTemplateRef<RecurringInvoiceEditDialogType>('recurringInvoiceEditDialog');

function editRecurring(recurring: RecurringInvoice): void {
    recurringInvoiceEditDialog.value?.open({ recurring }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}
```

- [ ] **Step 5: Mobile popup adds and edits; mobile list shows the schedule**

In `RecurringInvoiceAddPopup.vue`:
- props: `invoice: Invoice | null; recurring?: RecurringInvoice | null; show: boolean;`
- navbar title `:title="tt(title)"`, check button `:aria-label="tt(saveButtonTitle)"`.
- After the date item inside `#list` add:

```vue
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('Repeat Every')" :title="selectedPeriodName"
                                  @click="showPeriodSheet = true">
                        <list-item-selection-sheet value-type="item"
                                                   key-field="months" value-field="months" title-field="displayName"
                                                   :items="allPeriods"
                                                   v-model:show="showPeriodSheet"
                                                   v-model="recurring.periodMonths">
                        </list-item-selection-sheet>
                    </f7-list-item>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('When Due')" :title="selectedModeName"
                                  @click="showModeSheet = true">
                        <list-item-selection-sheet value-type="item"
                                                   key-field="mode" value-field="mode" title-field="displayName"
                                                   :items="allModes"
                                                   v-model:show="showModeSheet"
                                                   v-model="recurring.mode">
                        </list-item-selection-sheet>
                    </f7-list-item>
```

- help block: same `modeHelpText` computed as the desktop dialog.
- script: `const showPeriodSheet = ref(false); const showModeSheet = ref(false);`; `onPopupOpen` becomes:

```ts
function onPopupOpen(): void {
    submitting.value = false;

    if (props.recurring) {
        initFromRecurring(props.recurring);
    } else if (props.invoice) {
        initFromInvoice(props.invoice);
    }
}
```

- `save` uses `modifyRecurringInvoice` when `editRecurringId.value` is set (same branching and messages as the desktop dialog).

In `RecurringListPage.vue`:
- footer: `<div>{{ getSourceNumber(recurring) }} · {{ getScheduleText(recurring) }} · {{ tt('Next Date') }}: {{ formatDate(recurring.nextDate) }}</div>`
- swipeout: before the delete button add `<f7-swipeout-button color="orange" close :text="tt('Edit')" @click="recurringToEdit = recurring; showEditPopup = true"></f7-swipeout-button>`
- page bottom: `<recurring-invoice-add-popup :invoice="null" :recurring="recurringToEdit" v-model:show="showEditPopup" />`
- script: import `RecurringInvoiceAddPopup from './RecurringInvoiceAddPopup.vue'`; `const recurringToEdit = ref<RecurringInvoice | null>(null); const showEditPopup = ref<boolean>(false);`; destructure `getScheduleText`.

- [ ] **Step 6: Renewal draft days in the profile**

Desktop `InvoiceProfileSettingPage.vue`, after the "Payment Terms (days)" setting item:

```vue
                        <div class="setting-item">
                            <span>{{ tt('Renewal Drafts (days before)') }}</span>
                            <v-spacer/>
                            <number-input class="ms-4" density="compact" max-width="400px"
                                          :disabled="loading || saving"
                                          :min-value="0" :max-value="90" :max-decimal-count="0"
                                          v-model="newProfile.renewalDraftDays" />
                        </div>
```

Mobile `ProfilePage.vue`, after the payment terms input:

```vue
                <list-number-input :label="tt('Renewal Drafts (days before)')" :placeholder="tt('Renewal Drafts (days before)')"
                                   :min-value="0" :max-value="90" :max-decimal-count="0"
                                   v-model="newProfile.renewalDraftDays"></list-number-input>
```

- [ ] **Step 7: Typecheck, lint and look at it**

Run: `npx vue-tsc --noEmit && npx eslint src/views src/models src/lib src/stores`
Expected: clean.
Then run the app (`run` skill or `npm run serve` + the Go server) and check on desktop and mobile widths: add a recurring invoice from an invoice's Repeat action with "Every year / Draft ahead"; the recurring list shows "Every year · Draft ahead, I send it"; edit it to "Every 6 months"; the profile page saves 30 renewal draft days.

- [ ] **Step 8: Commit**

```bash
git add src/views src/locales/en.json
git commit --no-verify -m "recurring invoices UI: repeat every N months with a mode, edit schedules, renewal draft days in the profile"
```

---
### Task 10: Product catalog screens and linking past invoice lines (desktop + mobile)

**Files:**
- Create: `src/views/base/invoices/ProductEditPageBase.ts`
- Create: `src/views/desktop/invoices/dialogs/ProductEditDialog.vue`, `src/views/desktop/invoices/dialogs/ProductLinkDialog.vue`
- Modify: `src/views/desktop/invoices/ListPage.vue` (fourth view "Products")
- Create: `src/views/mobile/invoices/ProductListPage.vue`, `ProductEditPage.vue`, `ProductLinkPage.vue`
- Modify: `src/views/mobile/invoices/ListPage.vue` (More menu), `src/router/mobile.ts`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Task 8): `Product`, `InvoiceLineGroup`, store `allProducts`, `loadAllProducts`, `saveProduct`, `deleteProduct`, `getUnlinkedInvoiceLineGroups`, `linkInvoiceLinesToProduct`. (Task 9): `allModes` pattern, locale keys for periods and modes.
- Produces: `useProductEditPageBase()` → `{ editProductId, loading, submitting, product, title, saveButtonTitle, allModes, allPeriods, selectedModeName, selectedPeriodName, inputEmptyProblemMessage, inputIsEmpty, setRenewalMonths, getProductSummary, formatRate }`; mobile routes `/invoice/product/list`, `/invoice/product/add`, `/invoice/product/edit?id=`, `/invoice/product/link`.

- [ ] **Step 1: Locale strings**

```json
    "Products": "Products",
    "Product": "Product",
    "Product Name": "Product Name",
    "Add Product": "Add Product",
    "Edit Product": "Edit Product",
    "Default Rate": "Default Rate",
    "Renews": "Renews",
    "One-off": "One-off",
    "Archived": "Archived",
    "Archive": "Archive",
    "Unarchive": "Unarchive",
    "Product name cannot be blank": "Product name cannot be blank",
    "No available product": "No available product",
    "You have added a new product": "You have added a new product",
    "You have saved this product": "You have saved this product",
    "Are you sure you want to delete this product?": "Are you sure you want to delete this product?",
    "Link Past Invoice Lines": "Link Past Invoice Lines",
    "Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.": "Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.",
    "All past invoice lines are linked or skipped.": "All past invoice lines are linked or skipped.",
    "Link": "Link",
    "Lines": "Lines",
    "Unable to retrieve product list": "Unable to retrieve product list",
    "Unable to add product": "Unable to add product",
    "Unable to save product": "Unable to save product",
    "Unable to delete this product": "Unable to delete this product",
    "Unable to retrieve invoice lines": "Unable to retrieve invoice lines",
    "Unable to link invoice lines": "Unable to link invoice lines",
    "Product list has been updated": "Product list has been updated",
    "Product list is up to date": "Product list is up to date",
```

and in `format.misc`: `"linkedLinesCount": "Linked {count} invoice lines"`, `"lineCount": "{count} lines"`.

- [ ] **Step 2: Shared base**

Create `src/views/base/invoices/ProductEditPageBase.ts`:

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { type RecurringInvoiceMode, RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS } from '@/models/invoice.ts';
import { Product } from '@/models/product.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { INVOICE_DEFAULT_CURRENCY } from '@/lib/invoice.ts';

// ponytail: products are priced in the profile's default currency; the currency column stays for a later per-product choice
export function useProductEditPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const invoicesStore = useInvoicesStore();

    const editProductId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const product = ref<Product>(Product.createNewProduct());

    const title = computed<string>(() => editProductId.value ? 'Edit Product' : 'Add Product');
    const saveButtonTitle = computed<string>(() => editProductId.value ? 'Save' : 'Add');
    const currency = computed<string>(() => invoicesStore.invoiceProfile?.defaultCurrency || INVOICE_DEFAULT_CURRENCY);

    const allModes = computed<{ mode: RecurringInvoiceMode, displayName: string }[]>(() => RECURRING_INVOICE_MODES.map(item => ({ mode: item.mode, displayName: tt(item.name) })));
    const allPeriods = computed<{ months: number, displayName: string }[]>(() => [
        { months: 0, displayName: tt('One-off') },
        ...RECURRING_INVOICE_PERIODS.map(item => ({ months: item.months, displayName: tt(item.name) }))
    ]);
    const selectedModeName = computed<string>(() => allModes.value.find(item => item.mode === product.value.defaultRenewalMode)?.displayName ?? '');
    const selectedPeriodName = computed<string>(() => getPeriodName(product.value.renewalMonths));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!product.value.name.trim()) {
            return 'Product name cannot be blank';
        } else if (product.value.renewalMonths < 0 || product.value.renewalMonths > 120) {
            return 'Period must be between 1 and 120 months';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function getPeriodName(months: number): string {
        return allPeriods.value.find(item => item.months === months)?.displayName ?? tt('format.misc.everyNMonths', { count: months });
    }

    // a renewable product needs a mode; draft ahead is the default
    function setRenewalMonths(months: number): void {
        product.value.renewalMonths = months;

        if (months > 0 && !product.value.defaultRenewalMode) {
            product.value.defaultRenewalMode = 'draft_ahead';
        }
    }

    // "Every year · Draft ahead, I send it" or "One-off"
    function getProductSummary(item: Product): string {
        if (!item.renewable) {
            return tt('One-off');
        }

        const mode = RECURRING_INVOICE_MODES.find(entry => entry.mode === item.defaultRenewalMode);
        return mode ? `${getPeriodName(item.renewalMonths)} · ${tt(mode.name)}` : getPeriodName(item.renewalMonths);
    }

    function formatRate(item: Product): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(item.defaultRate), item.currency || currency.value);
    }

    return {
        // states
        editProductId,
        loading,
        submitting,
        product,
        // computed states
        title,
        saveButtonTitle,
        currency,
        allModes,
        allPeriods,
        selectedModeName,
        selectedPeriodName,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        setRenewalMonths,
        getProductSummary,
        formatRate
    };
}
```

- [ ] **Step 3: Desktop product dialog**

Create `src/views/desktop/invoices/dialogs/ProductEditDialog.vue` following `CustomerEditDialog.vue` exactly (same dialog layout, `open({ id?, currentProduct? })`, `isProductModified` with `isEquals(product.toCreateRequest(), initProduct.toCreateRequest())`, save via `invoicesStore.saveProduct`, messages `'You have saved this product'` / `'You have added a new product'`). The form body:

```vue
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="8">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Product Name')" :placeholder="tt('Product Name')"
                                          v-model="product.name" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('SAC')" :placeholder="tt('SAC')"
                                          v-model="product.sac" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="2"
                                        :disabled="loading || submitting"
                                        :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                        v-model="product.description" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <amount-input :disabled="loading || submitting" :persistent-placeholder="true"
                                          :currency="currency" :show-currency="true"
                                          :label="tt('Default Rate')" :placeholder="tt('Default Rate')"
                                          v-model="product.defaultRate" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-select persistent-placeholder :disabled="loading || submitting"
                                      :label="tt('Renews')" :items="allPeriods" item-title="displayName" item-value="months"
                                      :model-value="product.renewalMonths" @update:model-value="setRenewalMonths" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-select persistent-placeholder :disabled="loading || submitting || !product.renewable"
                                      :label="tt('When Due')" :items="allModes" item-title="displayName" item-value="mode"
                                      v-model="product.defaultRenewalMode" />
                        </v-col>
                        <v-col cols="12" md="12" v-if="editProductId">
                            <v-switch density="compact" hide-details color="primary" :disabled="loading || submitting"
                                      :label="tt('Archived')" v-model="product.archived" />
                        </v-col>
                    </v-row>
                </v-form>
```

`open` additionally calls `invoicesStore.loadInvoiceProfile({ force: false })` (ignore failures with `.catch(() => null)`) so the currency is right.

- [ ] **Step 4: Desktop link dialog**

Create `src/views/desktop/invoices/dialogs/ProductLinkDialog.vue`:

```vue
<template>
    <v-dialog width="720" v-model="showState">
        <one-column-dialog-layout :disabled="linkingItemName !== ''" :loading="loading"
                                  :title="tt('Link Past Invoice Lines')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #content>
                <div class="text-body-medium mb-4">{{ tt('Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.') }}</div>
                <div class="text-body-medium" v-if="!loading && !groups.length">{{ tt('All past invoice lines are linked or skipped.') }}</div>
                <v-row class="align-center" :key="group.itemName" v-for="group in groups">
                    <v-col cols="12" md="5">
                        <div>{{ group.itemName }}</div>
                        <small class="text-medium-emphasis">{{ tt('format.misc.lineCount', { count: group.count }) }}</small>
                    </v-col>
                    <v-col cols="8" md="5">
                        <v-select density="compact" hide-details persistent-placeholder
                                  :label="tt('Product')" :items="activeProducts" item-title="name" item-value="id"
                                  v-model="selections[group.itemName]" />
                    </v-col>
                    <v-col cols="4" md="2">
                        <v-btn color="primary" variant="tonal" :disabled="!selections[group.itemName] || linkingItemName !== ''"
                               :loading="linkingItemName === group.itemName" @click="link(group)">{{ tt('Link') }}</v-btn>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceLineGroup, Product } from '@/models/product.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const loading = ref<boolean>(false);
const groups = ref<InvoiceLineGroup[]>([]);
const selections = ref<Record<string, string>>({});
const linkingItemName = ref<string>('');

const activeProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

function open(): void {
    showState.value = true;
    loading.value = true;
    selections.value = {};

    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.getUnlinkedInvoiceLineGroups()]).then(([, loadedGroups]) => {
        groups.value = loadedGroups;
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function link(group: InvoiceLineGroup): void {
    const productId = selections.value[group.itemName];

    if (!productId) {
        return;
    }

    linkingItemName.value = group.itemName;

    invoicesStore.linkInvoiceLinesToProduct({ itemName: group.itemName, productId }).then(count => {
        linkingItemName.value = '';
        groups.value = groups.value.filter(item => item.itemName !== group.itemName);
        snackbar.value?.showMessage(tt('format.misc.linkedLinesCount', { count }));
    }).catch(error => {
        linkingItemName.value = '';

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>
```

- [ ] **Step 5: Desktop "Products" view in the invoice page**

In `src/views/desktop/invoices/ListPage.vue`:
- nav: a fourth `<li class="nav-link">` like Customers, `activeView === 'products'`, icon `mdiPackageVariantClosed`, title `tt('Products')`.
- `activeView` type `'invoices' | 'customers' | 'recurring' | 'products'`; `activeViewTitle` returns `'Products'` for it.
- More menu: add `<v-list-item :prepend-icon="mdiLinkVariant" :title="tt('Link Past Invoice Lines')" v-if="activeView === 'products'" @click="productLinkDialog?.open()"></v-list-item>`.
- table (same pattern as customers):

```vue
                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'products'">
                            <thead>
                            <tr>
                                <th>{{ tt('Product Name') }}</th>
                                <th>{{ tt('Renews') }}</th>
                                <th class="text-right">{{ tt('Default Rate') }}</th>
                                <th class="text-right">{{ tt('Operation') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="!loading && !allProducts.length">
                            <tr><td colspan="4">{{ tt('No available product') }}</td></tr>
                            </tbody>

                            <tbody>
                            <tr :key="product.id" v-for="product in allProducts"
                                @mouseover="hoveredProductId = product.id" @mouseleave="hoveredProductId = ''">
                                <td>
                                    <span :class="{ 'text-medium-emphasis': product.archived }">{{ product.name }}</span>
                                    <v-chip class="ms-2" size="x-small" variant="tonal" v-if="product.archived">{{ tt('Archived') }}</v-chip>
                                </td>
                                <td>{{ getProductSummary(product) }}</td>
                                <td class="text-right">{{ formatRate(product) }}</td>
                                <td class="text-right">
                                    <template v-if="hoveredProductId === product.id && !loading">
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                               @click="editProduct(product)">{{ tt('Edit') }}</v-btn>
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiDeleteOutline" :disabled="loading || updating"
                                               @click="removeProduct(product)">{{ tt('Delete') }}</v-btn>
                                    </template>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>
```

- script: `reload` adds `invoicesStore.loadAllProducts({ force })` and `invoicesStore.loadInvoiceProfile({ force: false })` to its `Promise.all`; `const allProducts = computed(() => invoicesStore.allProducts); const hoveredProductId = ref('');`; destructure `getProductSummary, formatRate` from `useProductEditPageBase()`; `add()` opens `productEditDialog` when `activeView === 'products'`; mount `<product-edit-dialog ref="productEditDialog" />` and `<product-link-dialog ref="productLinkDialog" />`;

```ts
function editProduct(product: Product): void {
    productEditDialog.value?.open({ id: product.id, currentProduct: product }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeProduct(product: Product): void {
    confirmDialog.value?.open('Are you sure you want to delete this product?').then(() => {
        updating.value = true;

        invoicesStore.deleteProduct({ product }).then(() => {
            updating.value = false;
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error); // "product is used by invoices, archive it instead"
            }
        });
    });
}
```

Add the server error text to `src/locales/en.json` error section the same way the existing `"customer is used by invoices"` message is translated (search for that string and add `"product is used by invoices, archive it instead"`, `"product not found"`, `"product id is invalid"`, `"invoice line product is invalid"`, `"recurring invoice period must be between 1 and 120 months"`, `"recurring invoice mode is invalid"` beside it in the same form).

- [ ] **Step 6: Mobile pages and routes**

`src/views/mobile/invoices/ProductListPage.vue`: copy `CustomerListPage.vue`, replacing customers with products: title `tt('Products')`, add link `/invoice/product/add`, items `:title="product.name"`, `:footer="getProductSummary(product) + ' · ' + formatRate(product)"`, swipeout Edit → `f7router.navigate('/invoice/product/edit?id=' + product.id)`, Delete → `invoicesStore.deleteProduct`, delete label `'Are you sure you want to delete this product?'`, loading via `invoicesStore.loadAllProducts` + `loadInvoiceProfile({ force: false })`, and a navbar right link `icon-f7="link"` with `:aria-label="tt('Link Past Invoice Lines')"` and `href="/invoice/product/link"`.

`src/views/mobile/invoices/ProductEditPage.vue`: copy `CustomerEditPage.vue` using `useProductEditPageBase()`; `#list` items:

```vue
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('Product Name')" :placeholder="tt('Product Name')"
                               v-model:value="product.name"></f7-list-input>
                <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                               :label="tt('Description')" :placeholder="tt('Description (optional)')"
                               v-textarea-auto-size v-model:value="product.description"></f7-list-input>
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('SAC')" :placeholder="tt('SAC')"
                               v-model:value="product.sac"></f7-list-input>
                <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                              :header="tt('Default Rate')" :title="formatRate(product)"
                              @click="showRateSheet = true">
                    <number-pad-sheet :min-value="0" :max-value="TRANSACTION_MAX_AMOUNT" :currency="currency"
                                      v-model:show="showRateSheet" v-model="product.defaultRate"></number-pad-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Renews')" :title="selectedPeriodName" @click="showPeriodSheet = true">
                    <list-item-selection-sheet value-type="item" key-field="months" value-field="months" title-field="displayName"
                                               :items="allPeriods" v-model:show="showPeriodSheet"
                                               :model-value="product.renewalMonths" @update:model-value="setRenewalMonths">
                    </list-item-selection-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('When Due')" :title="selectedModeName" @click="showModeSheet = true"
                              v-if="product.renewable">
                    <list-item-selection-sheet value-type="item" key-field="mode" value-field="mode" title-field="displayName"
                                               :items="allModes" v-model:show="showModeSheet" v-model="product.defaultRenewalMode">
                    </list-item-selection-sheet>
                </f7-list-item>
                <f7-list-item :title="tt('Archived')" v-if="editProductId">
                    <template #after>
                        <f7-toggle :checked="product.archived" @toggle:change="product.archived = $event"></f7-toggle>
                    </template>
                </f7-list-item>
```

Import `TRANSACTION_MAX_AMOUNT` the same way mobile `EditPage.vue` does; declare `showRateSheet`, `showPeriodSheet`, `showModeSheet` refs. `init` loads `invoicesStore.loadAllProducts({ force: false })` + `loadInvoiceProfile({ force: false })` and clones `invoicesStore.allProductsMap[id]` when `id` is in the query.

`src/views/mobile/invoices/ProductLinkPage.vue`:

```vue
<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Link Past Invoice Lines')"></f7-nav-title>
        </f7-navbar>

        <f7-block class="margin-vertical"><small>{{ tt('Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.') }}</small></f7-block>

        <f7-list strong inset dividers class="skeleton-text" v-if="loading">
            <f7-list-item title="Item name" footer="3 lines" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers v-if="!loading && !groups.length">
            <f7-list-item :title="tt('All past invoice lines are linked or skipped.')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers v-if="!loading && groups.length">
            <f7-list-item link="#" :key="group.itemName" v-for="group in groups"
                          :title="group.itemName" :footer="tt('format.misc.lineCount', { count: group.count })"
                          @click="groupToLink = group; showProductPopup = true">
            </f7-list-item>
        </f7-list>

        <list-item-selection-popup value-type="item" key-field="id" value-field="id" title-field="name"
                                   :title="tt('Product')" :enable-filter="true"
                                   :filter-placeholder="tt('Product')" :filter-no-items-text="tt('No results')"
                                   :items="activeProducts" v-model:show="showProductPopup"
                                   :model-value="''" @update:model-value="link">
        </list-item-selection-popup>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceLineGroup, Product } from '@/models/product.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();

const invoicesStore = useInvoicesStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const groups = ref<InvoiceLineGroup[]>([]);
const groupToLink = ref<InvoiceLineGroup | null>(null);
const showProductPopup = ref<boolean>(false);

const activeProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

function init(): void {
    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.getUnlinkedInvoiceLineGroups()]).then(([, loadedGroups]) => {
        groups.value = loadedGroups;
        loading.value = false;
    }).catch(error => {
        loadingError.value = error;

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

// picking the product is the confirmation for that one group
function link(productId: string): void {
    const group = groupToLink.value;

    if (!group || !productId) {
        return;
    }

    showLoading();

    invoicesStore.linkInvoiceLinesToProduct({ itemName: group.itemName, productId }).then(count => {
        hideLoading();
        groups.value = groups.value.filter(item => item.itemName !== group.itemName);
        showToast(tt('format.misc.linkedLinesCount', { count }));
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

`src/router/mobile.ts`: import the three pages and add routes beside the customer routes:

```ts
    {
        path: '/invoice/product/list',
        async: asyncResolve(InvoiceProductListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/product/add',
        async: asyncResolve(InvoiceProductEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/product/edit',
        async: asyncResolve(InvoiceProductEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/invoice/product/link',
        async: asyncResolve(InvoiceProductLinkPage),
        beforeEnter: [checkLogin]
    },
```

Mobile invoice `ListPage.vue` More sheet: after Customers add `<f7-actions-button @click="f7router.navigate('/invoice/product/list')">{{ tt('Products') }}</f7-actions-button>`.

- [ ] **Step 7: Typecheck and look at it**

Run: `npx vue-tsc --noEmit && npx eslint src/views src/router`
Expected: clean. In the running app: add "Domain" (Every year, Draft ahead) and "Website build" (One-off); archive and unarchive one; deleting a product used on an invoice shows the "archive it instead" error; Link Past Invoice Lines lists your real item names and linking a group removes it from the list. Check both desktop and a phone-width mobile view.

- [ ] **Step 8: Commit**

```bash
git add src/views src/router/mobile.ts src/locales/en.json
git commit --no-verify -m "products UI: catalog on desktop and mobile, linking past invoice lines by item name"
```

---

### Task 11: Pick a product on an invoice line (desktop + mobile)

**Files:**
- Modify: `src/views/base/invoices/InvoiceEditPageBase.ts`
- Modify: `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue:49-55` (item field) and its `open`
- Modify: `src/views/mobile/invoices/EditPage.vue` (line block, init)

**Interfaces:**
- Consumes (Task 8): `filterProductSuggestions`, `applyProductToLine`, store `allProducts`, `allProductsMap`, `loadAllProducts`.
- Produces: `useInvoiceEditPageBase()` gains `allProducts` (active only), `getProductSuggestions(query: string): Product[]`, `selectProduct(line: InvoiceLine, product: Product): void`, `getLinkedProductName(line: InvoiceLine): string`.

- [ ] **Step 1: Base helpers**

In `InvoiceEditPageBase.ts` (import `Product` type from `@/models/product.ts`, `InvoiceLine` type from `@/models/invoice.ts`, and the two helpers from `@/lib/product.ts`):

```ts
    const allProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

    function getProductSuggestions(query: string): Product[] {
        return filterProductSuggestions(invoicesStore.allProducts, query);
    }

    function selectProduct(line: InvoiceLine, product: Product): void {
        applyProductToLine(line, product);
    }

    // the catalog name of a line's product, also for archived products; '' when typed by hand
    function getLinkedProductName(line: InvoiceLine): string {
        return line.productId ? invoicesStore.allProductsMap[line.productId]?.name ?? '' : '';
    }
```

Return all four.

- [ ] **Step 2: Desktop line item becomes a combobox**

Replace the Item `v-text-field` in `InvoiceEditDialog.vue` with:

```vue
                            <v-combobox autocomplete="off" persistent-placeholder no-filter hide-no-data
                                        :disabled="loading || submitting"
                                        :label="tt('Item')" :placeholder="tt('Item')"
                                        :items="getProductSuggestions(line.itemName)" item-title="name"
                                        :hint="getLinkedProductName(line) ? tt('Product') + ': ' + getLinkedProductName(line) : ''"
                                        :persistent-hint="!!getLinkedProductName(line)"
                                        :model-value="line.itemName"
                                        @update:model-value="onItemChanged(line, $event)" />
```

```ts
// a picked suggestion fills the line from the product; typing keeps any product link (the text may be edited freely)
function onItemChanged(line: InvoiceLine, value: string | Product | null): void {
    if (value && typeof value === 'object') {
        selectProduct(line, value);
    } else {
        line.itemName = value ?? '';
    }
}
```

In `open`, add `invoicesStore.loadAllProducts({ force: false }).catch(() => [])` to the `Promise.all` list (a product list failure must not block editing invoices).

- [ ] **Step 3: Mobile "From product" picker per line**

In `EditPage.vue`, right after each line's group-title item:

```vue
                        <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                                      :header="tt('Product')"
                                      :title="getLinkedProductName(line) || tt('None')"
                                      @click="productPopupLineIndex = index"
                                      v-if="allProducts.length">
                        </f7-list-item>
```

and once, after the lines block:

```vue
        <list-item-selection-popup value-type="item" key-field="id" value-field="id" title-field="name"
                                   :title="tt('Product')" :enable-filter="true"
                                   :filter-placeholder="tt('Product')" :filter-no-items-text="tt('No results')"
                                   :items="allProducts"
                                   :show="productPopupLineIndex >= 0"
                                   @update:show="productPopupLineIndex = $event ? productPopupLineIndex : -1"
                                   :model-value="productPopupLineIndex >= 0 ? invoice.lines[productPopupLineIndex]?.productId ?? '' : ''"
                                   @update:model-value="onProductPicked">
        </list-item-selection-popup>
```

```ts
const productPopupLineIndex = ref<number>(-1);

function onProductPicked(productId: string): void {
    const line = invoice.value.lines[productPopupLineIndex.value];
    const product = invoicesStore.allProductsMap[productId];

    if (line && product) {
        selectProduct(line, product);
    }

    productPopupLineIndex.value = -1;
}
```

`init` adds `invoicesStore.loadAllProducts({ force: false }).catch(() => [])` to its loads.

- [ ] **Step 4: Typecheck and look at it**

Run: `npx vue-tsc --noEmit && npx vitest run --exclude '.claude/**' src/lib/__tests__/product.test.ts`
Expected: clean, PASS. In the app: on desktop type "dom" in an Item field → "Domain" is suggested; picking it fills description, SAC and rate and shows "Product: Domain" under the field; save and reopen keeps the link. On mobile pick a product from a line's Product row. Typing a brand-new item name still works.

- [ ] **Step 5: Commit**

```bash
git add src/views
git commit --no-verify -m "invoices: pick a catalog product on an invoice line"
```

---
### Task 12: "Create renewal" option when sending (desktop + mobile)

**Files:**
- Modify: `src/models/invoice.ts` (`InvoiceSendForm` already derives from `InvoiceSendRequest`; nothing else)
- Modify: `src/views/base/invoices/InvoiceDetailPageBase.ts` (`buildSendRequest`, `sendInvoice`, new `renewalOptionText`)
- Modify: `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue` (send dialog)
- Modify: `src/views/mobile/invoices/SendPage.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 6, 8): `Invoice.renewalOption`, `InvoiceSendRequest.createRenewal`.
- Produces: `useInvoiceDetailPageBase()` gains `renewalOption` (computed: the option when it is offerable or a mixed-period warning, else null) and `renewalOptionText` (computed string).

- [ ] **Step 1: Locale strings**

```json
    "These products renew on different periods. Split them into separate invoices to track their renewals.": "These products renew on different periods. Split them into separate invoices to track their renewals.",
```

and `format.misc`: `"createRenewal": "Create renewal: {period}, {mode}"`.

- [ ] **Step 2: Base**

In `InvoiceDetailPageBase.ts` (import `RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS, type InvoiceRenewalOption`):

```ts
    // shown in the send form only while no schedule renews this invoice yet
    const renewalOption = computed<InvoiceRenewalOption | null>(() => {
        const option = invoice.value?.renewalOption;
        return option && !option.scheduled ? option : null;
    });

    const renewalOptionText = computed<string>(() => {
        const option = renewalOption.value;

        if (!option) {
            return '';
        }

        const period = RECURRING_INVOICE_PERIODS.find(item => item.months === option.periodMonths);
        const mode = RECURRING_INVOICE_MODES.find(item => item.mode === option.mode);

        return tt('format.misc.createRenewal', {
            period: period ? tt(period.name) : tt('format.misc.everyNMonths', { count: option.periodMonths }),
            mode: mode ? tt(mode.name) : option.mode
        });
    });
```

`buildSendRequest` ticks the option by default:

```ts
    function buildSendRequest(): InvoiceSendRequest {
        if (!invoice.value) {
            return { id: '', to: '', subject: '', bodyHtml: '', message: '' };
        }

        const option = renewalOption.value;
        return { ...createInvoiceEmailSendRequest(invoice.value, profile.value), createRenewal: !!option && !option.mixedPeriods };
    }
```

In `sendInvoice`'s `.then`, also copy `invoice.value.renewalOption = sent.renewalOption;` so the option disappears once scheduled. Return `renewalOption` and `renewalOptionText`.

- [ ] **Step 3: Desktop send dialog**

In the send dialog of `InvoiceDetailDialog.vue`, between the To field and the template form:

```vue
                <template v-if="renewalOption">
                    <v-alert class="mb-2" type="warning" variant="tonal" density="compact" v-if="renewalOption.mixedPeriods">
                        {{ tt('These products renew on different periods. Split them into separate invoices to track their renewals.') }}
                    </v-alert>
                    <v-checkbox class="mb-2" density="compact" hide-details :disabled="sending"
                                :label="renewalOptionText" v-model="sendRequest.createRenewal" v-else />
                </template>
```

Destructure `renewalOption, renewalOptionText` from the base. After a successful send the existing `changed.value = true` makes the list page reload its recurring list.

- [ ] **Step 4: Mobile send page**

In `SendPage.vue` after the To list:

```vue
        <f7-block strong inset v-if="!loading && renewalOption && renewalOption.mixedPeriods">
            <small>{{ tt('These products renew on different periods. Split them into separate invoices to track their renewals.') }}</small>
        </f7-block>
        <f7-list strong inset dividers v-if="!loading && renewalOption && !renewalOption.mixedPeriods">
            <f7-list-item checkbox :title="renewalOptionText" :disabled="sending"
                          :checked="!!sendRequest.createRenewal" @change="sendRequest.createRenewal = $event.target.checked"></f7-list-item>
        </f7-list>
```

Destructure `renewalOption, renewalOptionText` from the base; after a successful send also call `invoicesStore.loadAllRecurringInvoices({ force: false }).catch(() => [])` so the recurring list is fresh.

- [ ] **Step 5: Typecheck and look at it**

Run: `npx vue-tsc --noEmit`
Expected: clean. In the app (SMTP configured locally or against a test mailbox): an invoice with the "Domain" product shows "Create renewal: Every year, Draft ahead, I send it", ticked; sending creates a row in Recurring; opening Send again shows no option. An invoice with Domain + a 6-monthly AMC shows the split warning instead.

- [ ] **Step 6: Commit**

```bash
git add src/views src/locales/en.json
git commit --no-verify -m "invoices: create the renewal schedule when sending an invoice with renewable products"
```

---

### Task 13: Customer detail page (desktop dialog + mobile page)

**Files:**
- Create: `src/views/base/invoices/CustomerDetailPageBase.ts`
- Create: `src/views/desktop/invoices/dialogs/CustomerDetailDialog.vue`
- Modify: `src/views/desktop/invoices/ListPage.vue` (customer row opens the dialog)
- Create: `src/views/mobile/invoices/CustomerDetailPage.vue`
- Modify: `src/views/mobile/invoices/CustomerListPage.vue`, `src/router/mobile.ts`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 7-9): store `getCustomerOverview`, `allInvoices`, `allRecurringInvoices` (with `customerId`), `getScheduleText`; `getInvoiceOutstandingByCurrency` from `src/lib/invoice.ts`.
- Produces: `useCustomerDetailPageBase()` → `{ loading, customer, overview, invoices, recurrings, billedTotals, outstandingTotals, load(customerId), formatAmount(amount, currency), formatDate(date), formatTime(unixTime), getTimelineTitle(item) }`; mobile route `/invoice/customer/detail?id=`.

- [ ] **Step 1: Locale strings**

```json
    "Customer Details": "Customer Details",
    "Billed": "Billed",
    "Products Bought": "Products Bought",
    "Renewals": "Renewals",
    "Timeline": "Timeline",
    "No products bought yet.": "No products bought yet.",
    "No renewals yet.": "No renewals yet.",
    "Nothing has happened yet.": "Nothing has happened yet.",
    "Unable to retrieve customer": "Unable to retrieve customer",
```

and `format.misc`: `"timesBought": "Bought {count} times, last on {date}"`, `"timelineInvoice": "Invoice {number}"`, `"timelineRenewal": "Renewal invoice {number}"`, `"timelineEmail": "Emailed {number}: {subject}"`, `"timelinePayment": "Payment received for {number}"`.

- [ ] **Step 2: Base**

Create `src/views/base/invoices/CustomerDetailPageBase.ts`:

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Customer, CustomerOverview, CustomerTimelineItem, Invoice, RecurringInvoice } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { type InvoiceCurrencyAmount, getInvoiceOutstandingByCurrency } from '@/lib/invoice.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';

export function useCustomerDetailPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDateTime } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const customer = ref<Customer | null>(null);
    const overview = ref<CustomerOverview>({ products: [], timeline: [] });

    const invoices = computed<Invoice[]>(() => customer.value ? invoicesStore.allInvoices.filter(invoice => invoice.customerId === customer.value?.id) : []);
    const recurrings = computed<RecurringInvoice[]>(() => customer.value ? invoicesStore.allRecurringInvoices.filter(recurring => recurring.customerId === customer.value?.id) : []);

    // one line per currency the customer was billed in
    const billedTotals = computed<InvoiceCurrencyAmount[]>(() => {
        const totals = new Map<string, number>();

        for (const invoice of invoices.value) {
            totals.set(invoice.currency, (totals.get(invoice.currency) ?? 0) + invoice.total);
        }

        return [...totals.entries()].map(([currency, amount]) => ({ currency, amount }));
    });

    const outstandingTotals = computed<InvoiceCurrencyAmount[]>(() => getInvoiceOutstandingByCurrency(invoices.value).filter(total => total.amount > 0));

    function load(customerId: string): Promise<void> {
        loading.value = true;

        return Promise.all([
            invoicesStore.loadAllCustomers({ force: false }),
            invoicesStore.loadAllInvoices({ force: false }),
            invoicesStore.loadAllRecurringInvoices({ force: false }),
            invoicesStore.getCustomerOverview({ customerId })
        ]).then(([, , , loadedOverview]) => {
            customer.value = invoicesStore.allCustomersMap[customerId] ?? null;
            overview.value = loadedOverview;
            loading.value = false;

            if (!customer.value) {
                throw { message: 'Unable to retrieve customer' };
            }
        });
    }

    function formatAmount(amount: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    function formatTime(unixTime: number): string {
        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTimeWithTimezoneOffset(unixTime, getTimezoneOffsetMinutes(unixTime)));
    }

    function getTimelineTitle(item: CustomerTimelineItem): string {
        switch (item.kind) {
            case 'email':
                return tt('format.misc.timelineEmail', { number: item.invoiceNumber, subject: item.text });
            case 'payment':
                return tt('format.misc.timelinePayment', { number: item.invoiceNumber });
            default:
                return tt(item.renewal ? 'format.misc.timelineRenewal' : 'format.misc.timelineInvoice', { number: item.invoiceNumber });
        }
    }

    return {
        // states
        loading,
        customer,
        overview,
        // computed states
        invoices,
        recurrings,
        billedTotals,
        outstandingTotals,
        // functions
        load,
        formatAmount,
        formatDate,
        formatTime,
        getTimelineTitle
    };
}
```

Copy the datetime imports exactly from `InvoiceDetailPageBase.ts` (it already formats `email.sentUnixTime` the same way).

- [ ] **Step 3: Desktop dialog**

Create `src/views/desktop/invoices/dialogs/CustomerDetailDialog.vue`:

```vue
<template>
    <v-dialog width="1000" v-model="showState">
        <one-column-dialog-layout :loading="loading" :title="customer ? customer.name : tt('Customer Details')"
                                  :cancel-button-title="tt('Close')" @cancel="showState = false">
            <template #content>
                <v-row v-if="customer">
                    <v-col cols="12" md="5">
                        <div class="text-body-medium">
                            <div v-if="customer.contactName">{{ customer.contactName }}</div>
                            <div v-if="customer.email">{{ customer.email }}</div>
                            <div v-if="customer.phone">{{ customer.phone }}</div>
                            <div v-if="customer.gstin">{{ tt('GSTIN') }}: {{ customer.gstin }}</div>
                        </div>
                        <v-list density="compact" class="mt-2">
                            <v-list-item :title="tt('Billed')" :key="'billed-' + total.currency" v-for="total in billedTotals">
                                <template #append>{{ formatAmount(total.amount, total.currency) }}</template>
                            </v-list-item>
                            <v-list-item :title="tt('Outstanding')" :key="'due-' + total.currency" v-for="total in outstandingTotals">
                                <template #append><span class="font-weight-bold">{{ formatAmount(total.amount, total.currency) }}</span></template>
                            </v-list-item>
                        </v-list>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Products Bought') }}</div>
                        <v-list density="compact" v-if="overview.products.length">
                            <v-list-item :key="product.productId" v-for="product in overview.products" :title="product.name"
                                         :subtitle="tt('format.misc.timesBought', { count: product.timesBought, date: formatDate(product.lastBoughtDate) })" />
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No products bought yet.') }}</div>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Renewals') }}</div>
                        <v-list density="compact" v-if="recurrings.length">
                            <v-list-item :key="recurring.id" v-for="recurring in recurrings" link
                                         :title="getScheduleText(recurring)"
                                         :subtitle="`${tt('Next Date')}: ${formatDate(recurring.nextDate)}`"
                                         @click="editRecurring(recurring)">
                                <template #append>
                                    <v-chip size="x-small" variant="tonal" v-if="!recurring.enabled">{{ tt('Paused') }}</v-chip>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No renewals yet.') }}</div>

                        <template v-if="customer.notes">
                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Internal Notes') }}</div>
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line">{{ customer.notes }}</div>
                        </template>
                    </v-col>

                    <v-col cols="12" md="7">
                        <div class="text-title-small px-4 py-2">{{ tt('Timeline') }}</div>
                        <v-list density="compact" v-if="overview.timeline.length">
                            <v-list-item :key="`${item.kind}-${item.invoiceId}-${item.unixTime}`" v-for="item in overview.timeline" link
                                         :title="getTimelineTitle(item)" :subtitle="formatTime(item.unixTime)"
                                         @click="viewInvoice(item.invoiceId)">
                                <template #append v-if="item.kind !== 'email'">
                                    <span :class="{ 'text-income': item.kind === 'payment' }">{{ formatAmount(item.amount, item.currency) }}</span>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('Nothing has happened yet.') }}</div>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <invoice-detail-dialog ref="invoiceDetailDialog" />
    <recurring-invoice-edit-dialog ref="recurringInvoiceEditDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceDetailDialog from './InvoiceDetailDialog.vue';
import RecurringInvoiceEditDialog from './RecurringInvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCustomerDetailPageBase } from '@/views/base/invoices/CustomerDetailPageBase.ts';
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';

import type { RecurringInvoice } from '@/models/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceDetailDialogType = InstanceType<typeof InvoiceDetailDialog>;
type RecurringInvoiceEditDialogType = InstanceType<typeof RecurringInvoiceEditDialog>;

const { tt } = useI18n();
const { loading, customer, overview, recurrings, billedTotals, outstandingTotals, load, formatAmount, formatDate, formatTime, getTimelineTitle } = useCustomerDetailPageBase();
const { getScheduleText } = useRecurringInvoiceListPageBase();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceDetailDialog = useTemplateRef<InvoiceDetailDialogType>('invoiceDetailDialog');
const recurringInvoiceEditDialog = useTemplateRef<RecurringInvoiceEditDialogType>('recurringInvoiceEditDialog');

const showState = ref<boolean>(false);
let customerId = '';

function open(options: { id: string }): void {
    customerId = options.id;
    showState.value = true;
    reload();
}

function reload(): void {
    load(customerId).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function viewInvoice(invoiceId: string): void {
    invoiceDetailDialog.value?.open({ id: invoiceId }).then(result => {
        if (result.changed) {
            reload();
        }
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function editRecurring(recurring: RecurringInvoice): void {
    recurringInvoiceEditDialog.value?.open({ recurring }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

defineExpose({
    open
});
</script>
```

Add `"Paused": "Paused"` to `en.json`. Because `loadAllInvoices` / `loadAllRecurringInvoices` never serve from cache, the dialog always shows fresh numbers.

In `ListPage.vue` customers table, make the name clickable: wrap `<span>{{ customer.name }}</span>` as `<a class="cursor-pointer" @click="viewCustomer(customer)">{{ customer.name }}</a>`; mount `<customer-detail-dialog ref="customerDetailDialog" />`; `function viewCustomer(customer: Customer): void { customerDetailDialog.value?.open({ id: customer.id }); }`.

- [ ] **Step 4: Mobile page**

Create `src/views/mobile/invoices/CustomerDetailPage.vue`:

```vue
<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="customer ? customer.name : tt('Customer Details')"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading || !customer }">
                <f7-link :text="tt('Edit')" @click="f7router.navigate('/invoice/customer/edit?id=' + customerId)"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Billed" after="₹ 0.00" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <template v-if="!loading && customer">
            <f7-list strong inset dividers class="margin-top-half">
                <f7-list-item :title="tt('Billed')" :after="formatAmount(total.amount, total.currency)" :key="'billed-' + total.currency" v-for="total in billedTotals"></f7-list-item>
                <f7-list-item :title="tt('Outstanding')" :after="formatAmount(total.amount, total.currency)" :key="'due-' + total.currency" v-for="total in outstandingTotals"></f7-list-item>
            </f7-list>

            <f7-block-title>{{ tt('Products Bought') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('No products bought yet.')" v-if="!overview.products.length"></f7-list-item>
                <f7-list-item :key="product.productId" v-for="product in overview.products" :title="product.name"
                              :footer="tt('format.misc.timesBought', { count: product.timesBought, date: formatDate(product.lastBoughtDate) })"></f7-list-item>
            </f7-list>

            <f7-block-title>{{ tt('Renewals') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('No renewals yet.')" v-if="!recurrings.length"></f7-list-item>
                <f7-list-item link="#" :key="recurring.id" v-for="recurring in recurrings"
                              :title="getScheduleText(recurring)" :footer="`${tt('Next Date')}: ${formatDate(recurring.nextDate)}`"
                              :after="recurring.enabled ? '' : tt('Paused')"
                              @click="recurringToEdit = recurring; showEditPopup = true"></f7-list-item>
            </f7-list>

            <f7-block-title>{{ tt('Timeline') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('Nothing has happened yet.')" v-if="!overview.timeline.length"></f7-list-item>
                <f7-list-item :key="`${item.kind}-${item.invoiceId}-${item.unixTime}`" v-for="item in overview.timeline"
                              :link="'/invoice/detail?id=' + item.invoiceId"
                              :title="getTimelineTitle(item)" :footer="formatTime(item.unixTime)"
                              :after="item.kind === 'email' ? '' : formatAmount(item.amount, item.currency)"></f7-list-item>
            </f7-list>
        </template>

        <recurring-invoice-add-popup :invoice="null" :recurring="recurringToEdit" v-model:show="showEditPopup" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import RecurringInvoiceAddPopup from './RecurringInvoiceAddPopup.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { useCustomerDetailPageBase } from '@/views/base/invoices/CustomerDetailPageBase.ts';
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';

import type { RecurringInvoice } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();
const { loading, customer, overview, recurrings, billedTotals, outstandingTotals, load, formatAmount, formatDate, formatTime, getTimelineTitle } = useCustomerDetailPageBase();
const { getScheduleText } = useRecurringInvoiceListPageBase();

const customerId = props.f7route.query['id'] ?? '';
const loadingError = ref<unknown | null>(null);
const recurringToEdit = ref<RecurringInvoice | null>(null);
const showEditPopup = ref<boolean>(false);

function reload(done?: () => void): void {
    load(customerId).then(() => done?.()).catch(error => {
        done?.();

        if (!done) {
            loadingError.value = error;
        }

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

reload();
</script>
```

Route in `src/router/mobile.ts` beside the customer routes (import `InvoiceCustomerDetailPage`):

```ts
    {
        path: '/invoice/customer/detail',
        async: asyncResolve(InvoiceCustomerDetailPage),
        beforeEnter: [checkLogin]
    },
```

In `CustomerListPage.vue` add `:link="'/invoice/customer/detail?id=' + customer.id"` to each customer `f7-list-item` (swipeout Edit/Delete stay).

- [ ] **Step 5: Typecheck and look at it**

Run: `npx vue-tsc --noEmit && npx eslint src/views src/router`
Expected: clean. In the app, open a real customer that has invoices: totals match the invoice list's outstanding for that customer; products bought show after Task 10's linking; the timeline lists invoices, emails and payments newest first; clicking a timeline row opens that invoice; a renewal row opens the schedule editor. Check desktop and phone width.

- [ ] **Step 6: Commit**

```bash
git add src/views src/router/mobile.ts src/locales/en.json
git commit --no-verify -m "customers: detail page with totals, products bought, renewals and timeline"
```

---

### Task 14: Home "Renewals" card (desktop + mobile)

**Files:**
- Create: `src/views/base/invoices/RenewalsCardBase.ts`
- Create: `src/views/desktop/invoices/RenewalsCard.vue`, `src/views/mobile/invoices/RenewalsCard.vue`
- Modify: `src/views/desktop/HomePage.vue`, `src/views/mobile/HomePage.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 4, 8): store `loadInvoiceRenewals`, `loadInvoiceProfile`, `sendInvoice`; `createInvoiceEmailSendRequest` from `src/lib/invoiceEmail.ts`.
- Produces: `useRenewalsCardBase()` → `{ renewals, sendingId, load(), send(renewal), canSend(renewal), getStateName(renewal), formatAmount(invoice), formatDate(date) }`.

- [ ] **Step 1: Locale strings**

```json
    "Renewal due": "Renewal due",
    "Auto-send failed": "Auto-send failed",
    "Sent automatically": "Sent automatically",
    "Open": "Open",
    "Invoice has been sent": "Invoice has been sent",
```

(skip any key that already exists — `"Invoice has been sent"` does.)

- [ ] **Step 2: Base**

Create `src/views/base/invoices/RenewalsCardBase.ts`:

```ts
import { ref } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, InvoiceRenewal } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { createInvoiceEmailSendRequest } from '@/lib/invoiceEmail.ts';

const STATE_NAMES: Record<InvoiceRenewal['state'], string> = {
    due: 'Renewal due',
    send_failed: 'Auto-send failed',
    auto_sent: 'Sent automatically'
};

// the home card: drafts waiting to be sent, failed auto-sends, and recent auto-sends; hidden when empty or when loading fails
export function useRenewalsCardBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const renewals = ref<InvoiceRenewal[]>([]);
    const sendingId = ref<string>('');

    function load(): Promise<void> {
        return invoicesStore.loadInvoiceRenewals().then(loaded => {
            renewals.value = loaded;
        }).catch(() => {
            renewals.value = []; // ponytail: the card simply disappears; the invoice page still has everything
        });
    }

    function canSend(renewal: InvoiceRenewal): boolean {
        return renewal.state !== 'auto_sent' && !!renewal.invoice.billToEmail;
    }

    // one click: the saved email template to the customer's address, exactly as the send form would prefill it
    function send(renewal: InvoiceRenewal): Promise<void> {
        sendingId.value = renewal.invoice.id;

        return invoicesStore.loadInvoiceProfile({ force: false })
            .then(profile => invoicesStore.sendInvoice(createInvoiceEmailSendRequest(renewal.invoice, profile)))
            .then(() => load())
            .finally(() => {
                sendingId.value = '';
            });
    }

    function getStateName(renewal: InvoiceRenewal): string {
        return tt(STATE_NAMES[renewal.state]);
    }

    function formatAmount(invoice: Invoice): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(invoice.total), invoice.currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        renewals,
        sendingId,
        // functions
        load,
        canSend,
        send,
        getStateName,
        formatAmount,
        formatDate
    };
}
```

- [ ] **Step 3: Desktop card**

Create `src/views/desktop/invoices/RenewalsCard.vue`:

```vue
<template>
    <v-card class="mb-4" v-if="renewals.length">
        <template #title>{{ tt('Renewals') }}</template>
        <v-list density="compact">
            <v-list-item :key="renewal.invoice.id" v-for="renewal in renewals"
                         :title="`${renewal.invoice.billToName} · ${renewal.invoice.number}`"
                         :subtitle="`${getStateName(renewal)} · ${formatDate(renewal.invoice.invoiceDate)} · ${formatAmount(renewal.invoice)}`">
                <template #append>
                    <v-btn class="me-2" color="primary" variant="tonal" size="small" v-if="canSend(renewal)"
                           :loading="sendingId === renewal.invoice.id" :disabled="sendingId !== ''"
                           @click="sendRenewal(renewal)">{{ tt('Send') }}</v-btn>
                    <v-btn variant="text" size="small" :to="'/invoice/list?invoiceId=' + renewal.invoice.id">{{ tt('Open') }}</v-btn>
                </template>
            </v-list-item>
        </v-list>
    </v-card>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useRenewalsCardBase } from '@/views/base/invoices/RenewalsCardBase.ts';

import type { InvoiceRenewal } from '@/models/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const { renewals, sendingId, load, canSend, send, getStateName, formatAmount, formatDate } = useRenewalsCardBase();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

function sendRenewal(renewal: InvoiceRenewal): void {
    send(renewal).then(() => {
        snackbar.value?.showMessage('Invoice has been sent');
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

load();

defineExpose({
    load
});
</script>
```

In desktop `HomePage.vue`, render `<renewals-card ref="renewalsCard" />` directly above `<overview-dashboard ...>` inside `#content`, import it, and call `renewalsCard.value?.load()` inside the existing `reload` function. (`/invoice/list?invoiceId=` is the route the transaction invoice chip already uses to open an invoice.)

- [ ] **Step 4: Mobile card**

Create `src/views/mobile/invoices/RenewalsCard.vue`:

```vue
<template>
    <template v-if="renewals.length">
        <f7-block-title>{{ tt('Renewals') }}</f7-block-title>
        <f7-list strong inset dividers media-list>
            <f7-list-item swipeout :key="renewal.invoice.id" v-for="renewal in renewals"
                          :link="'/invoice/detail?id=' + renewal.invoice.id"
                          :title="renewal.invoice.billToName"
                          :subtitle="`${renewal.invoice.number} · ${formatAmount(renewal.invoice)}`"
                          :footer="`${getStateName(renewal)} · ${formatDate(renewal.invoice.invoiceDate)}`">
                <f7-swipeout-actions right v-if="canSend(renewal)">
                    <f7-swipeout-button color="blue" close :text="tt('Send')" @click="sendRenewal(renewal)"></f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>
    </template>
</template>

<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useRenewalsCardBase } from '@/views/base/invoices/RenewalsCardBase.ts';

import type { InvoiceRenewal } from '@/models/invoice.ts';

const { tt } = useI18n();
const { showToast } = useI18nUIComponents();
const { renewals, load, canSend, send, getStateName, formatAmount, formatDate } = useRenewalsCardBase();

function sendRenewal(renewal: InvoiceRenewal): void {
    showLoading();

    send(renewal).then(() => {
        hideLoading();
        showToast('Invoice has been sent');
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

load();

defineExpose({
    load
});
</script>
```

In mobile `HomePage.vue`, render `<renewals-card ref="renewalsCard" />` right above `<overview-dashboard ...>`, import it, and call `renewalsCard.value?.load()` inside `reload`.

- [ ] **Step 5: Typecheck and look at it**

Run: `npx vue-tsc --noEmit && npx eslint src/views`
Expected: clean. In the app: with no renewals the home page looks exactly as before. Create a draft-ahead schedule with a next date inside the window and run the job once (`go test` cannot reach the dev DB; instead set the schedule's next date to today + 2 days and restart the server, or call the cron directly from a scratch `go run` that invokes `services.Invoices.CreateRecurringInvoices`). The card shows "Renewal due"; Send emails it and the row disappears; Open opens the invoice. Phone width: the card sits above the dashboard and the Send swipe action works.

- [ ] **Step 6: Commit**

```bash
git add src/views src/locales/en.json
git commit --no-verify -m "home: renewals card with one-click send (desktop and mobile)"
```

---

### Task 15: Full verification and deploy

**Files:** none new.

- [ ] **Step 1: Whole test suites**

Run: `go test ./... -count=1 && npx vitest run --exclude '.claude/**' && npx vue-tsc --noEmit && npx eslint .`
Expected: all PASS / clean. Fix anything red before continuing; do not skip.

- [ ] **Step 2: Manual pass on a copy of real data**

With a local server on a copy of the production database (never the live one):
1. Products: create Domain (yearly, draft ahead) and AMC (6-monthly, auto-send); Link Past Invoice Lines on your real item names.
2. Send an invoice with Domain → renewal created; Send again → no option.
3. Recurring list: existing monthly schedules show "Every month · Create only" and behave as before.
4. Customer page for your largest customer: totals, products, timeline look right.
5. Home card after a draft is generated.
Check every screen at desktop width and at 375px phone width.

- [ ] **Step 3: Deploy**

Follow the project deploy routine (push → GHCR image → Dokploy API redeploy for books.codeation.io). After deploy, open books.codeation.io and confirm: the invoice page has the Products view, an existing recurring invoice still reads "Every month · Create only", and the home page loads.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit --no-verify -m "crm part a: verification fixes"
```
