# Estimates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send clients estimates (quotes) with the invoice editor, products, PDF, public link and email; let the client accept or decline online; turn an accepted estimate into invoices, all at once or milestone by milestone.

**Architecture:** An estimate is a row of the existing `invoice` table with `Kind = "estimate"` plus five new columns (`EstimateStatus`, `RespondedUnixTime`, `RespondedByClient`, `ResponseNote`, `EstimateId`); its `DueDate` is the valid-until date. Payment terms live in a new `estimate_milestone` table saved together with the estimate. `GET /invoices/list.json` keeps returning invoices only and every invoice-only service filters `kind = invoice`, so outstanding, payments, recurring and renewals never see an estimate. Converting is a read endpoint that returns an unsaved invoice draft for the editor, plus optional `estimateId` / `milestoneId` on invoice create; the server rebuilds the milestone amount and marks the milestone invoiced in the same transaction as the insert. The client answers through two unauthenticated endpoints keyed by the existing public token; the owner gets an email.

**Tech Stack:** Go (gin, xorm, testify) backend; Vue 3 + Pinia + Vuetify (desktop) + Framework7 (mobile) frontend; vitest.

**Spec:** `docs/superpowers/specs/2026-10-30-estimates-design.md`

## Global Constraints

- Kinds are exactly `invoice` and `estimate` (`models.INVOICE_KIND_INVOICE`, `models.INVOICE_KIND_ESTIMATE`); existing rows are already `invoice`.
- Stored estimate statuses are exactly `draft`, `sent`, `accepted`, `declined`; invoices store `""`.
- `expired` is never stored: an estimate in `draft` or `sent` whose `DueDate` (valid until) is before today; `invoiced` is a display status for a fully invoiced estimate.
- `EstimateNumberPrefix` VARCHAR(16), default `EST`; `EstimateValidDays` int32, default 30, range 1..365 (an empty prefix or 0 days from an older client means the default).
- Estimate numbers are `{EstimateNumberPrefix}/{financial year}/{sequence}`, a separate sequence; `NextInvoiceNumber` counts only `kind = invoice`; number uniqueness is per kind.
- An estimate has 0 milestones or 1..10; each label 1..128 characters (trimmed); each percent in basis points > 0; together exactly 10000.
- Milestone invoice rate = round half up of `subtotal × percent / 10000`; the last open milestone gets `subtotal − sum of the subtotals of the non-deleted invoices made from the other milestones`. The Payment Terms table uses the estimate total incl. tax with the same rounding, the last row taking the remainder.
- Editable only while `draft` or `sent` and no non-deleted invoice has `EstimateId` = the estimate. Mark accepted / declined from `draft`, `sent` (incl. expired); Reopen only from `accepted` / `declined` back to `sent`, and only while nothing is invoiced. An estimate with invoices made from it cannot be deleted; deleting such an invoice frees its milestone.
- Client answers: only `sent`, `DueDate >= today`, token enabled, kind estimate; the update is conditional on `estimate_status = 'sent'`; the decline note is plain text, at most 500 characters; responses carry no ids.
- `kind = invoice` filters: invoice list (and so outstanding / overdue and the customer page Billed / Paid / Outstanding), payment link and suggestions, Mark as Paid, recurring create, renewals list, renewal option / Create renewal, invoices linked to transactions, products bought on the customer page.
- Schema changes are new columns / tables via xorm `Sync2` only: the table loop in `cmd/database.go` and the list in `pkg/services/alerts_testutil_test.go`. No data migration.
- New error codes are 60..68 in `pkg/errs/invoice.go`; their English texts go under `"error"` in `src/locales/en.json`.
- New UI strings go in `src/locales/en.json` only; `format.misc.*` keys live inside `"format"`. The printed document (PDF, document component) stays English.
- Desktop and mobile both get every screen; shared logic lives in `src/views/base/invoices/` and `src/lib/estimate.ts`.
- Go tests: `go test ./pkg/services/ ./pkg/api/ -count=1` (add `-run <Name>` while iterating; `./pkg/invoicepdf/ ./pkg/invoiceemail/` where a task touches them). Frontend tests: `npx vitest run --exclude '.claude/**' <file>`. Typecheck: `npx vue-tsc --noEmit`. Lint: `npx eslint <paths>`.
- Commit with `git commit --no-verify` (project convention).

## Review Focus

1. **Milestone money that does not add up** — 33.33% / 33.33% / 33.34% of an odd subtotal, invoiced out of order, one invoice edited or deleted in between, or a tampered rate from the client: the invoices must always sum to the estimate subtotal and the server must ignore the client's rate. Pinned in Task 4 (`TestEstimateMilestoneRateRoundsHalfUpAndLastOpenTakesTheRemainder`, `TestCreateInvoiceFromMilestoneRecomputesTheRateAndInvoicesEachMilestoneOnce`).
2. **Double clicks and two tabs** — a second "Create invoice" on the same milestone, a second full conversion, a second Accept / Decline from the public page, or two answers racing: exactly one invoice per milestone, exactly one recorded answer and one owner email. Pinned in Task 4 (`...InvoicesEachMilestoneOnce`, `TestFullConversionHappensOnceWhileItsInvoiceExists`) and Task 5 (`TestClientAcceptsEstimateThroughPublicLinkAndOwnerIsEmailed`, `TestConcurrentPublicAnswersRecordOnlyOne`).
3. **A link that should no longer answer** — an expired estimate (page opened yesterday, clicked after midnight), a never-sent draft, a disabled or reset link, an invoice's link, an over-long note: refused with no change; valid-until today still answers. Pinned in Task 5 (`TestPublicEstimateAnswerGuards`).
4. **Changing an estimate after the client said yes** — editing an accepted or invoiced estimate, deleting an estimate that has invoices, deleting a milestone invoice: edits refused, delete refused, the milestone becomes invoiceable again. Pinned in Task 2 (`TestAnsweredOrInvoicedEstimatesAreLocked`, `TestDeletingEstimatesAndTheInvoicesMadeFromThem`).
5. **Estimates leaking into money** — an estimate showing in outstanding / overdue, consuming an invoice number, getting payment suggestions or a linked payment, being repeated or renewed, or counting as a product bought: none of these may happen. Pinned in Task 1 (`TestCreateEstimateStartsAsDraftAndListsApartFromInvoices`, `TestEstimateNumberingIsSeparateFromInvoices`), Task 3 (`TestEstimatesStayOutOfInvoiceOnlyFeatures`) and Task 7 (`TestCustomerOverviewShowsEstimateEventsButNeverCountsEstimatesAsBought`).

---

## File Structure

Backend:
- Create `pkg/models/estimate.go` — status and limit constants, `EstimateMilestone` table, milestone / respond / draft / public request and response types, `EstimateProgress`.
- Modify `pkg/models/invoice.go` — kind constants, new `Invoice` and `InvoiceProfile` columns, request / response fields, `SetEstimateProgress`, customer timeline kinds.
- Modify `pkg/errs/invoice.go` — codes 60..68.
- Create `pkg/services/estimates.go` — milestone validation, `CreateEstimate`, `GetEstimateMilestones`.
- Create `pkg/services/estimate_lifecycle.go` — `ModifyEstimate`, locking, `SetEstimateResponse`, `ReopenEstimate`, `GetEstimateProgress`.
- Create `pkg/services/estimate_conversion.go` — milestone rate, line text, `GetEstimateInvoiceDraft`, `CreateInvoiceFromEstimate`.
- Create `pkg/services/estimate_public.go` — `RespondToEstimateByPublicToken`, owner notification.
- Create `pkg/services/estimate_attention.go` — `GetEstimatesToInvoice`.
- Modify `pkg/services/invoices.go` — profile defaults, numbering per kind, kind filters, `ModifyInvoice`, `DeleteInvoice`.
- Modify `pkg/services/recurring_invoices.go`, `pkg/services/invoice_email.go`, `pkg/services/customer_overview.go`.
- Modify `pkg/invoicepdf/render.go`, `pkg/invoicepdf/format.go`; modify `pkg/invoiceemail/render.go`.
- Modify `pkg/api/invoices.go`; create `pkg/api/estimates.go`, `pkg/api/estimate_conversion.go`, `pkg/api/estimate_public.go`.
- Modify `cmd/webserver.go`, `cmd/database.go`, `pkg/services/alerts_testutil_test.go`.
- Tests: create `pkg/services/estimates_test.go`, `estimate_lifecycle_test.go`, `estimate_filters_test.go`, `estimate_conversion_test.go`, `estimate_public_test.go`, `estimate_email_test.go`, `estimate_overview_test.go`, `pkg/api/estimates_test.go`; modify `pkg/invoicepdf/invoicepdf_test.go`, `pkg/invoiceemail/render_test.go`, `pkg/services/invoice_email_test.go`.

Frontend:
- Create `src/lib/estimate.ts` + `src/lib/__tests__/estimate.test.ts`.
- Modify `src/models/invoice.ts`, `src/lib/invoice.ts`, `src/lib/invoiceEmail.ts`, `src/lib/services.ts`, `src/stores/invoice.ts`.
- Create `src/views/base/invoices/EstimateListPageBase.ts`; modify `InvoiceListPageBase.ts`, `InvoiceEditPageBase.ts`, `InvoiceDetailPageBase.ts`, `InvoiceProfilePageBase.ts`, `CustomerDetailPageBase.ts`, `RenewalsCardBase.ts`.
- Modify `src/components/common/InvoiceDocumentContent.vue`.
- Desktop: modify `invoices/ListPage.vue`, `invoices/dialogs/InvoiceEditDialog.vue`, `invoices/dialogs/InvoiceDetailDialog.vue`, `invoices/PublicInvoicePage.vue`, `invoices/RenewalsCard.vue`, `settings/InvoiceProfileSettingPage.vue`.
- Mobile: create `invoices/EstimateListPage.vue`; modify `invoices/ListPage.vue`, `EditPage.vue`, `DetailPage.vue`, `SendPage.vue`, `ProfilePage.vue`, `RenewalsCard.vue`, `src/router/mobile.ts`.
- Modify `src/locales/en.json`.

---

### Task 1: Estimate rows, milestones, profile settings and numbering (backend)

**Files:**
- Create: `pkg/models/estimate.go`
- Modify: `pkg/models/invoice.go` (kind constant, `InvoiceProfile`, `Invoice`, profile request / response, create request, `InvoiceInfoResponse`, `ToInvoiceInfoResponse`, `ToInvoiceProfileResponse`, new `InvoiceListRequest`, `InvoiceNextNumberRequest`)
- Modify: `pkg/errs/invoice.go`
- Create: `pkg/services/estimates.go`
- Modify: `pkg/services/invoices.go` (`defaultInvoiceProfile`, `saveProfile`, `GetAllInvoices`, new `GetAllEstimates`, `checkInvoiceNumberUnique`, `prepareNewInvoice`, `NextInvoiceNumber`, new `NextEstimateNumber`)
- Modify: `pkg/api/invoices.go` (`InvoiceProfileUpdateHandler`, `InvoiceListHandler`, `InvoiceCreateHandler`, `InvoiceNextNumberHandler`, new `estimateMilestonesFromRequest`)
- Modify: `cmd/database.go:240`, `pkg/services/alerts_testutil_test.go` (table list)
- Test: create `pkg/services/estimates_test.go`

**Interfaces:**
- Consumes: `computeInvoiceTotals`, `prepareNewInvoice`, `insertInvoice`, `invoiceFinancialYear`, `nextInvoiceSequence` (existing).
- Produces:
  - constants `models.INVOICE_KIND_ESTIMATE = "estimate"`; `models.ESTIMATE_STATUS_DRAFT/SENT/ACCEPTED/DECLINED`; `models.EstimateNumberPrefixDefault = "EST"`, `models.EstimateValidDaysDefault = 30`, `models.EstimateMaxMilestones = 10`, `models.EstimateMilestonePercentTotal = 10000`, `models.EstimateMilestoneLabelMaxLength = 128`, `models.EstimateResponseNoteMaxLength = 500`.
  - `Invoice.EstimateStatus string`, `RespondedUnixTime int64`, `RespondedByClient bool`, `ResponseNote string`, `EstimateId int64`; `InvoiceProfile.EstimateNumberPrefix string`, `EstimateValidDays int32`.
  - table `models.EstimateMilestone{MilestoneId, Uid, EstimateId, DisplayOrder int32, Label string, Percent int32, InvoiceId int64}`; `models.EstimateMilestoneRequest{Label, Percent}`; `models.EstimateMilestoneResponse{Id, Label, Percent, InvoiceId, InvoiceNumber}`.
  - JSON: `kind` and `milestones` on invoice create; `estimateStatus`, `respondedUnixTime`, `respondedByClient`, `responseNote`, `estimateId` on invoice responses; `estimateNumberPrefix`, `estimateValidDays` on profile get / update; `GET /api/v1/invoices/list.json?kind=estimate`; `GET /api/v1/invoices/next_number.json?invoice_date=...&kind=estimate`.
  - `models.InvoiceListRequest{Kind}`; `models.InvoiceNextNumberRequest.Kind`.
  - `errs.ErrEstimateMilestonesInvalid` (60).
  - `(*InvoiceService).CreateEstimate(c, estimate *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) error`, `GetEstimateMilestones(c, uid, estimateId int64) ([]*models.EstimateMilestone, error)`, `GetAllEstimates(c, uid) ([]*models.Invoice, error)`, `NextEstimateNumber(c, uid, date string) (string, error)`; `validateEstimateMilestones(milestones) error`; `(*InvoiceService).prepareEstimateMilestones(estimate, milestones) error`.
  - API helper `estimateMilestonesFromRequest(reqs []*models.EstimateMilestoneRequest) []*models.EstimateMilestone`.
  - Test helpers `testEstimateLines()`, `testEstimateMilestones()`, `createTestEstimate(t, uid, customerId, number, lines, milestones) *models.Invoice`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimates_test.go`:

```go
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// testEstimateLines come to 1,999.99 before tax: 1,000.00 plus three hours of 333.33
func testEstimateLines() []*models.InvoiceLine {
	return []*models.InvoiceLine{
		{ItemName: "Website", Description: "Design and build", Quantity: 100, Rate: 100000, Sac: "998314"},
		{ItemName: "Support", Quantity: 300, Rate: 33333, Sac: "998315"},
	}
}

// testEstimateMilestones are three thirds that need rounding: 33.33%, 33.33%, 33.34%
func testEstimateMilestones() []*models.EstimateMilestone {
	return []*models.EstimateMilestone{
		{Label: "Advance", Percent: 3333},
		{Label: "Design sign-off", Percent: 3333},
		{Label: "Go-live", Percent: 3334},
	}
}

// createTestEstimate saves a draft estimate dated 2026-04-23 and valid until 2099-12-31, so it never expires in a test
func createTestEstimate(t *testing.T, uid int64, customerId int64, number string, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) *models.Invoice {
	t.Helper()
	estimate := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
	assert.Nil(t, Invoices.CreateEstimate(core.NewNullContext(), estimate, lines, milestones))
	return estimate
}

func TestCreateEstimateStartsAsDraftAndListsApartFromInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	reloaded, lines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, int64(199999), reloaded.Subtotal)
	assert.Equal(t, "Quber Tech", reloaded.BillToName)
	assert.Equal(t, 2, len(lines))

	invoices, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(invoices))
	assert.Equal(t, "INV/2026-27/01", invoices[0].Number)

	estimates, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates))
	assert.Equal(t, estimate.InvoiceId, estimates[0].InvoiceId)

	// a plain invoice never carries estimate state, whatever the caller put in
	invoice := &models.Invoice{Uid: uid, Number: "INV/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30",
		EstimateStatus: models.ESTIMATE_STATUS_ACCEPTED, EstimateId: estimate.InvoiceId, ResponseNote: "x"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, testEstimateLines()))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, reloaded.Kind)
	assert.Equal(t, "", reloaded.EstimateStatus)
	assert.Equal(t, int64(0), reloaded.EstimateId)
	assert.Equal(t, "", reloaded.ResponseNote)
}

func TestEstimateNumberingIsSeparateFromInvoices(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/02", 100)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01", next)

	createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	// an estimate typed with an invoice number neither collides with the invoice nor moves the invoice series
	createTestEstimate(t, uid, customer.CustomerId, "INV/2026-27/02", testEstimateLines(), nil)
	createTestEstimate(t, uid, customer.CustomerId, "INV/2026-27/09", testEstimateLines(), nil)

	next, err = Invoices.NextEstimateNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/02", next)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "INV/2026-27/03", next)

	duplicate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, Invoices.CreateEstimate(ctx, duplicate, testEstimateLines(), nil))
}

func TestEstimateProfileSettingsDefaultAndPersist(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, models.EstimateNumberPrefixDefault, profile.EstimateNumberPrefix)
	assert.Equal(t, int32(models.EstimateValidDaysDefault), profile.EstimateValidDays)

	// a client from before estimates sends neither field: the defaults stay
	profile.EstimateNumberPrefix, profile.EstimateValidDays = "", 0
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, profile))
	reloaded, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "EST", reloaded.EstimateNumberPrefix)
	assert.Equal(t, int32(30), reloaded.EstimateValidDays)

	reloaded.EstimateNumberPrefix, reloaded.EstimateValidDays = "QT", 15
	assert.Nil(t, Invoices.SaveBusinessProfile(ctx, reloaded))
	reloaded, err = Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "QT", reloaded.ToInvoiceProfileResponse().EstimateNumberPrefix)
	assert.Equal(t, int32(15), reloaded.ToInvoiceProfileResponse().EstimateValidDays)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2027-01-10")
	assert.Nil(t, err)
	assert.Equal(t, "QT/2026-27/01", next)
}

func TestEstimateMilestonesMustBeOneToTenLabelledAndAddUpTo100Percent(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	eleven := make([]*models.EstimateMilestone, 11)
	for idx := range eleven {
		eleven[idx] = &models.EstimateMilestone{Label: "Part", Percent: 909}
	}
	eleven[10].Percent = 910

	for name, milestones := range map[string][]*models.EstimateMilestone{
		"99.99%":       {{Label: "Advance", Percent: 4999}, {Label: "Delivery", Percent: 5000}},
		"100.01%":      {{Label: "Advance", Percent: 5001}, {Label: "Delivery", Percent: 5000}},
		"blank label":  {{Label: "   ", Percent: 10000}},
		"zero percent": {{Label: "Advance", Percent: 0}, {Label: "Delivery", Percent: 10000}},
		"long label":   {{Label: strings.Repeat("x", 129), Percent: 10000}},
		"eleven":       eleven,
	} {
		estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31"}
		assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.CreateEstimate(ctx, estimate, testEstimateLines(), milestones), name)
	}

	estimates, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, estimates) // nothing was saved by a refused create

	ten := make([]*models.EstimateMilestone, 10)
	for idx := range ten {
		ten[idx] = &models.EstimateMilestone{Label: "Part", Percent: 1000}
	}
	ten[0].Label = "  Advance  "

	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), ten)
	saved, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 10, len(saved))
	assert.Equal(t, "Advance", saved[0].Label)
	assert.Equal(t, int32(1), saved[0].DisplayOrder)
	assert.Equal(t, int32(10), saved[9].DisplayOrder)
	assert.Equal(t, int64(0), saved[9].InvoiceId)
	assert.Equal(t, estimate.InvoiceId, saved[9].EstimateId)

	none := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	saved, err = Invoices.GetEstimateMilestones(ctx, uid, none.InvoiceId)
	assert.Nil(t, err)
	assert.Empty(t, saved)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestCreateEstimate|TestEstimateNumbering|TestEstimateProfile|TestEstimateMilestones' -count=1`
Expected: compile FAIL (`undefined: models.EstimateMilestone`, `Invoices.CreateEstimate undefined`).

- [ ] **Step 3: Models and error**

Create `pkg/models/estimate.go`:

```go
package models

// Estimate statuses stored in Invoice.EstimateStatus. "expired" is never stored: a draft or sent estimate whose
// valid-until date (DueDate) is before today shows as expired, and extending the date revives it.
const (
	ESTIMATE_STATUS_DRAFT    = "draft"
	ESTIMATE_STATUS_SENT     = "sent"
	ESTIMATE_STATUS_ACCEPTED = "accepted"
	ESTIMATE_STATUS_DECLINED = "declined"
)

// EstimateNumberPrefixDefault is the estimate number prefix of a profile that has not set one
const EstimateNumberPrefixDefault = "EST"

// EstimateValidDaysDefault is how long a new estimate stays valid unless the profile says otherwise
const EstimateValidDaysDefault = 30

// EstimateMaxMilestones is the most payment terms one estimate may have
const EstimateMaxMilestones = 10

// EstimateMilestonePercentTotal is what the milestone percents (basis points) of an estimate must add up to: 100%
const EstimateMilestonePercentTotal = 10000

// EstimateMilestoneLabelMaxLength is the longest milestone label in characters
const EstimateMilestoneLabelMaxLength = 128

// EstimateResponseNoteMaxLength is the longest note a client may leave when declining, in characters
const EstimateResponseNoteMaxLength = 500

// EstimateMilestone is one payment term of an estimate: a share of its subtotal that is invoiced on its own
type EstimateMilestone struct {
	MilestoneId  int64  `xorm:"PK"`
	Uid          int64  `xorm:"INDEX(IDX_estimate_milestone_uid_estimate_id) NOT NULL"`
	EstimateId   int64  `xorm:"INDEX(IDX_estimate_milestone_uid_estimate_id) NOT NULL"`
	DisplayOrder int32  `xorm:"NOT NULL"`
	Label        string `xorm:"VARCHAR(128) NOT NULL"`
	Percent      int32  `xorm:"NOT NULL"`           // basis points
	InvoiceId    int64  `xorm:"NOT NULL DEFAULT 0"` // the non-deleted invoice made from this milestone; 0 until invoiced
}

// EstimateMilestoneRequest is one milestone of an estimate creation / modification request
type EstimateMilestoneRequest struct {
	Label   string `json:"label" binding:"required,notBlank,max=128"`
	Percent int32  `json:"percent" binding:"required,min=1,max=10000"`
}

// EstimateMilestoneResponse is a milestone with the invoice made from it, if any
type EstimateMilestoneResponse struct {
	Id            string `json:"id"`
	Label         string `json:"label"`
	Percent       int32  `json:"percent"`
	InvoiceId     string `json:"invoiceId,omitempty"`
	InvoiceNumber string `json:"invoiceNumber,omitempty"`
}
```

In `pkg/models/invoice.go` replace the kind constant:

```go
// Invoice kinds: an invoice, or an estimate (quote) that can be converted into invoices; estimates share the table and the editor
const (
	INVOICE_KIND_INVOICE  = "invoice"
	INVOICE_KIND_ESTIMATE = "estimate"
)
```

Add to `InvoiceProfile` after `RenewalDraftDays`:

```go
	EstimateNumberPrefix string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'EST'"`
	EstimateValidDays    int32  `xorm:"NOT NULL DEFAULT 30"`
```

In `Invoice` add `INDEX(IDX_invoice_uid_estimate_id)` to the `Uid` tag (it becomes ``xorm:"INDEX(IDX_invoice_uid_deleted) INDEX(IDX_invoice_uid_recurring_id) INDEX(IDX_invoice_uid_estimate_id) NOT NULL"``) and append after `RecurringId`:

```go
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
```

Add to `InvoiceProfileUpdateRequest`:

```go
	EstimateNumberPrefix string `json:"estimateNumberPrefix" binding:"max=16"`        // empty means EST
	EstimateValidDays    int32  `json:"estimateValidDays" binding:"min=0,max=365"` // 0 means 30
```

Add to `InvoiceProfileResponse`: ``EstimateNumberPrefix string `json:"estimateNumberPrefix"` `` and ``EstimateValidDays int32 `json:"estimateValidDays"` ``; set `EstimateNumberPrefix: p.EstimateNumberPrefix, EstimateValidDays: p.EstimateValidDays,` in `ToInvoiceProfileResponse`.

Add to `InvoiceCreateRequest`:

```go
	Kind       string                      `json:"kind" binding:"omitempty,oneof=invoice estimate"` // empty means invoice
	Milestones []*EstimateMilestoneRequest `json:"milestones" binding:"omitempty,max=10,dive"`      // estimates only
```

Add the list request next to `InvoiceIdRequest`:

```go
// InvoiceListRequest picks invoices (default) or estimates for the list
type InvoiceListRequest struct {
	Kind string `form:"kind" binding:"omitempty,oneof=invoice estimate"`
}
```

Add to `InvoiceNextNumberRequest`: ``Kind string `form:"kind" binding:"omitempty,oneof=invoice estimate"` ``.

Add to `InvoiceInfoResponse`:

```go
	EstimateStatus    string `json:"estimateStatus,omitempty"`
	RespondedUnixTime int64  `json:"respondedUnixTime,omitempty"`
	RespondedByClient bool   `json:"respondedByClient,omitempty"`
	ResponseNote      string `json:"responseNote,omitempty"`
	EstimateId        string `json:"estimateId,omitempty"` // invoices made from an estimate
```

and in `ToInvoiceInfoResponse` set `EstimateStatus: i.EstimateStatus, RespondedUnixTime: i.RespondedUnixTime, RespondedByClient: i.RespondedByClient, ResponseNote: i.ResponseNote,` in the literal, then before `if lines != nil {`:

```go
	if i.EstimateId > 0 {
		resp.EstimateId = utils.Int64ToString(i.EstimateId)
	}
```

In `pkg/errs/invoice.go` after `ErrInvoiceLineProductInvalid`:

```go
	// estimates start at 60
	ErrEstimateMilestonesInvalid = NewNormalError(NormalSubcategoryInvoice, 60, http.StatusBadRequest, "estimate milestones must be 1 to 10 with labels and add up to 100%")
```

Register the table: in `cmd/database.go:240` and in the `SyncStructs` list of `pkg/services/alerts_testutil_test.go` append `new(models.EstimateMilestone)` after `new(models.Product)`.

- [ ] **Step 4: Services**

Create `pkg/services/estimates.go`:

```go
package services

import (
	"strings"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// validateEstimateMilestones checks an estimate's payment terms: none, or 1 to 10, each with a label (trimmed here) and a
// positive percent, the percents (basis points) adding up to exactly 100%
func validateEstimateMilestones(milestones []*models.EstimateMilestone) error {
	if len(milestones) == 0 {
		return nil
	}

	if len(milestones) > models.EstimateMaxMilestones {
		return errs.ErrEstimateMilestonesInvalid
	}

	total := int32(0)

	for _, milestone := range milestones {
		milestone.Label = strings.TrimSpace(milestone.Label)

		if milestone.Label == "" || utf8.RuneCountInString(milestone.Label) > models.EstimateMilestoneLabelMaxLength ||
			milestone.Percent <= 0 || milestone.Percent > models.EstimateMilestonePercentTotal {
			return errs.ErrEstimateMilestonesInvalid
		}

		total += milestone.Percent
	}

	if total != models.EstimateMilestonePercentTotal {
		return errs.ErrEstimateMilestonesInvalid
	}

	return nil
}

// prepareEstimateMilestones gives the milestones of a saved estimate their ids, owner and order; nothing is invoiced yet
func (s *InvoiceService) prepareEstimateMilestones(estimate *models.Invoice, milestones []*models.EstimateMilestone) error {
	for idx, milestone := range milestones {
		milestone.MilestoneId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

		if milestone.MilestoneId < 1 {
			return errs.ErrSystemIsBusy
		}

		milestone.Uid = estimate.Uid
		milestone.EstimateId = estimate.InvoiceId
		milestone.DisplayOrder = int32(idx + 1)
		milestone.InvoiceId = 0
	}

	return nil
}

// CreateEstimate saves a new draft estimate with its lines and milestones. It is numbered, totalled and snapshotted
// exactly like an invoice (prepareNewInvoice), then marked as an estimate.
func (s *InvoiceService) CreateEstimate(c core.Context, estimate *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) error {
	if err := validateEstimateMilestones(milestones); err != nil {
		return err
	}

	if err := s.prepareNewInvoice(c, estimate, lines); err != nil {
		return err
	}

	estimate.Kind = models.INVOICE_KIND_ESTIMATE
	estimate.EstimateStatus = models.ESTIMATE_STATUS_DRAFT

	if err := s.prepareEstimateMilestones(estimate, milestones); err != nil {
		return err
	}

	return s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := insertInvoice(sess, estimate, lines); err != nil {
			return err
		}

		if len(milestones) == 0 {
			return nil
		}

		_, err := sess.Insert(milestones)
		return err
	})
}

// GetEstimateMilestones returns the milestones of an estimate of user in display order; an invoice has none
func (s *InvoiceService) GetEstimateMilestones(c core.Context, uid int64, estimateId int64) ([]*models.EstimateMilestone, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var milestones []*models.EstimateMilestone
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND estimate_id=?", uid, estimateId).OrderBy("display_order asc").Find(&milestones)

	return milestones, err
}
```

In `pkg/services/invoices.go`:

```go
func defaultInvoiceProfile(uid int64) *models.InvoiceProfile {
	return &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", DefaultTermsDays: 7, RenewalDraftDays: models.InvoiceRenewalDraftDaysDefault,
		EstimateNumberPrefix: models.EstimateNumberPrefixDefault, EstimateValidDays: models.EstimateValidDaysDefault}
}
```

At the top of `saveProfile`, right after the `profile.Uid <= 0` check:

```go
	// a client from before estimates sends neither field
	if strings.TrimSpace(profile.EstimateNumberPrefix) == "" {
		profile.EstimateNumberPrefix = models.EstimateNumberPrefixDefault
	}

	if profile.EstimateValidDays <= 0 {
		profile.EstimateValidDays = models.EstimateValidDaysDefault
	}
```

Replace `GetAllInvoices` and add `GetAllEstimates` below it:

```go
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
```

Replace `checkInvoiceNumberUnique` (numbers are unique per kind, matching the separate series):

```go
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
```

In `prepareNewInvoice`, after `invoice.RecurringId = 0` add:

```go
	invoice.EstimateStatus = ""
	invoice.RespondedUnixTime = 0
	invoice.RespondedByClient = false
	invoice.ResponseNote = ""
	invoice.EstimateId = 0
```

Replace `NextInvoiceNumber`:

```go
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
```

- [ ] **Step 5: API**

In `pkg/api/invoices.go` add below `invoiceLinesFromRequest`:

```go
func estimateMilestonesFromRequest(milestoneReqs []*models.EstimateMilestoneRequest) []*models.EstimateMilestone {
	milestones := make([]*models.EstimateMilestone, len(milestoneReqs))

	for idx, milestoneReq := range milestoneReqs {
		milestones[idx] = &models.EstimateMilestone{Label: strings.TrimSpace(milestoneReq.Label), Percent: milestoneReq.Percent}
	}

	return milestones
}
```

In `InvoiceProfileUpdateHandler` add to the profile literal: `EstimateNumberPrefix: strings.TrimSpace(req.EstimateNumberPrefix), EstimateValidDays: req.EstimateValidDays,`.

Replace `InvoiceListHandler`:

```go
// InvoiceListHandler returns all invoices (or, with kind=estimate, all estimates) of current user with received amounts
func (a *InvoicesApi) InvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceListRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[invoices.InvoiceListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	var invoices []*models.Invoice
	var err error

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		invoices, err = a.invoices.GetAllEstimates(c, uid)
	} else {
		invoices, err = a.invoices.GetAllInvoices(c, uid)
	}

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
```

In `InvoiceCreateHandler` replace the `if err := a.invoices.CreateInvoice(...)` block with:

```go
	lines := invoiceLinesFromRequest(req.Lines)

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		err = a.invoices.CreateEstimate(c, invoice, lines, estimateMilestonesFromRequest(req.Milestones))
	} else if len(req.Milestones) > 0 {
		err = errs.ErrEstimateMilestonesInvalid // only an estimate has payment terms
	} else {
		err = a.invoices.CreateInvoice(c, invoice, lines)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceCreateHandler] failed to create invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
```

In `InvoiceNextNumberHandler` replace the service call with:

```go
	var number string
	var err error

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		number, err = a.invoices.NextEstimateNumber(c, uid, req.InvoiceDate)
	} else {
		number, err = a.invoices.NextInvoiceNumber(c, uid, req.InvoiceDate)
	}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1`
Expected: PASS (new estimate tests and every existing invoice, recurring and product test).

- [ ] **Step 7: Commit**

```bash
git add pkg/models/estimate.go pkg/models/invoice.go pkg/errs/invoice.go pkg/services/estimates.go pkg/services/estimates_test.go pkg/services/invoices.go pkg/services/alerts_testutil_test.go pkg/api/invoices.go cmd/database.go
git commit --no-verify -m "estimates: estimate rows with milestones, profile settings and their own number series"
```

---

### Task 2: Editing, locking, deleting and answering estimates (backend)

**Files:**
- Modify: `pkg/models/estimate.go` (`EstimateProgress`, `EstimateConvertedInvoiceResponse`, `EstimateRespondRequest`)
- Modify: `pkg/models/invoice.go` (`InvoiceModifyRequest`, `InvoiceInfoResponse` fields, `SetEstimateProgress`)
- Modify: `pkg/errs/invoice.go` (61..63)
- Create: `pkg/services/estimate_lifecycle.go`
- Modify: `pkg/services/invoices.go` (`invoiceModifyColumns`, `ModifyInvoice`, `DeleteInvoice`)
- Modify: `pkg/api/invoices.go` (`InvoiceListHandler`, `getInvoiceResponse`, `InvoiceModifyHandler`)
- Create: `pkg/api/estimates.go`
- Modify: `cmd/webserver.go` (two routes after `/invoices/send.json`)
- Test: create `pkg/services/estimate_lifecycle_test.go`

**Interfaces:**
- Consumes (Task 1): `CreateEstimate`, `GetEstimateMilestones`, `validateEstimateMilestones`, `prepareEstimateMilestones`, status constants, `estimateMilestonesFromRequest`, `createTestEstimate`, `testEstimateLines`, `testEstimateMilestones`.
- Produces:
  - `models.EstimateProgress{Milestones []*EstimateMilestone, Invoices []*Invoice, FullyInvoiced bool}`; `models.EstimateConvertedInvoiceResponse{Id, Number}`; `models.EstimateRespondRequest{Id int64, Status string}`.
  - `InvoiceInfoResponse.Milestones []*EstimateMilestoneResponse`, `ConvertedInvoices []*EstimateConvertedInvoiceResponse`, `FullyInvoiced bool`, `EstimateNumber string`; `(*InvoiceInfoResponse).SetEstimateProgress(progress *EstimateProgress)`; `InvoiceModifyRequest.Kind`, `.Milestones`.
  - `errs.ErrEstimateLocked` (61), `errs.ErrEstimateHasInvoices` (62), `errs.ErrEstimateStatusInvalid` (63).
  - `estimateHasInvoices(sess, uid, estimateId) (bool, error)`, `getEstimateInSession(sess, uid, estimateId) (*models.Invoice, error)`, `checkEstimateEditable(sess, estimate) error`.
  - `(*InvoiceService).ModifyEstimate(c, estimate, lines, milestones) error`, `SetEstimateResponse(c, uid, estimateId int64, status string) error`, `ReopenEstimate(c, uid, estimateId int64) error`, `GetEstimateProgress(c, uid, estimateIds []int64) (map[int64]*models.EstimateProgress, error)`.
  - `ModifyInvoice` only touches `kind = invoice`; `DeleteInvoice` refuses an estimate with invoices and frees a milestone.
  - Routes `POST /api/v1/estimates/respond.json {id, status}`, `POST /api/v1/estimates/reopen.json {id}` returning the estimate as `InvoiceInfoResponse`; handlers `EstimateRespondHandler`, `EstimateReopenHandler`.
  - Test helper `linkTestInvoiceToEstimate(t, uid, invoiceId, estimateId)`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_lifecycle_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// linkTestInvoiceToEstimate marks an invoice as made from an estimate, as converting does
func linkTestInvoiceToEstimate(t *testing.T, uid int64, invoiceId int64, estimateId int64) {
	t.Helper()
	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(core.NewNullContext()).Cols("estimate_id").
		Where("uid=? AND invoice_id=?", uid, invoiceId).Update(&models.Invoice{EstimateId: estimateId})
	assert.Nil(t, err)
}

func TestModifyEstimateReplacesLinesAndMilestones(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	estimate.Notes = "Valid for 30 days"
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, []*models.InvoiceLine{{ItemName: "Website", Quantity: 100, Rate: 500000}},
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}}))

	reloaded, lines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, int64(500000), reloaded.Subtotal)
	assert.Equal(t, "Valid for 30 days", reloaded.Notes)
	assert.Equal(t, 1, len(lines))

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(milestones))
	assert.Equal(t, "Delivery", milestones[1].Label)

	assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), []*models.EstimateMilestone{{Label: "Advance", Percent: 5000}}))

	// an estimate is never edited through the invoice path, nor an invoice through the estimate path
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.ModifyInvoice(ctx, reloaded, testEstimateLines()))
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.ModifyEstimate(ctx, invoice, testEstimateLines(), nil))
}

func TestAnsweredOrInvoicedEstimatesAreLocked(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(milestones)) // the refused edit left the payment terms alone

	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), testEstimateMilestones()))

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, estimate.InvoiceId)
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))
}

func TestEstimateResponseTransitions(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_SENT))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId)) // nothing to reopen

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_DECLINED, reloaded.EstimateStatus)
	assert.False(t, reloaded.RespondedByClient)
	assert.Greater(t, reloaded.RespondedUnixTime, int64(0))

	// an answered estimate takes no second answer until it is reopened
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
	assert.Equal(t, int64(0), reloaded.RespondedUnixTime)
	assert.Equal(t, "", reloaded.ResponseNote)

	// an expired estimate (valid-until in the past) can still be marked
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, expired.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	// reopening is refused once anything was invoiced from the estimate
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, expired.InvoiceId)
	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.ReopenEstimate(ctx, uid, expired.InvoiceId))

	// an invoice has no estimate status
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.SetEstimateResponse(ctx, uid, invoice.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
}

func TestDeletingEstimatesAndTheInvoicesMadeFromThem(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, estimate.InvoiceId)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("invoice_id").
		Where("milestone_id=?", milestones[0].MilestoneId).Update(&models.EstimateMilestone{InvoiceId: invoice.InvoiceId})
	assert.Nil(t, err)

	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	milestones, err = Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), milestones[0].InvoiceId)

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))
	_, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
}

func TestEstimateProgressCountsLiveInvoicesOnly(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	whole := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	staged := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(),
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}})

	progress, err := Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId, staged.InvoiceId})
	assert.Nil(t, err)
	assert.False(t, progress[whole.InvoiceId].FullyInvoiced)
	assert.Empty(t, progress[whole.InvoiceId].Invoices)
	assert.Equal(t, 2, len(progress[staged.InvoiceId].Milestones))

	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, whole.InvoiceId)
	advance := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/02", 100)
	linkTestInvoiceToEstimate(t, uid, advance.InvoiceId, staged.InvoiceId)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("invoice_id").
		Where("milestone_id=?", progress[staged.InvoiceId].Milestones[0].MilestoneId).Update(&models.EstimateMilestone{InvoiceId: advance.InvoiceId})
	assert.Nil(t, err)

	progress, err = Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId, staged.InvoiceId})
	assert.Nil(t, err)
	assert.True(t, progress[whole.InvoiceId].FullyInvoiced)
	assert.Equal(t, "INV/2026-27/01", progress[whole.InvoiceId].Invoices[0].Number)
	assert.False(t, progress[staged.InvoiceId].FullyInvoiced) // Delivery is still open

	resp := whole.ToInvoiceInfoResponse(nil, nil)
	resp.SetEstimateProgress(progress[whole.InvoiceId])
	assert.Equal(t, "INV/2026-27/01", resp.ConvertedInvoices[0].Number)
	assert.True(t, resp.FullyInvoiced)

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	progress, err = Invoices.GetEstimateProgress(ctx, uid, []int64{whole.InvoiceId})
	assert.Nil(t, err)
	assert.False(t, progress[whole.InvoiceId].FullyInvoiced)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestModifyEstimate|TestAnsweredOrInvoiced|TestEstimateResponse|TestDeletingEstimates|TestEstimateProgress' -count=1`
Expected: compile FAIL (`Invoices.ModifyEstimate undefined`, `errs.ErrEstimateLocked undefined`).

- [ ] **Step 3: Models and errors**

Append to `pkg/models/estimate.go`:

```go
// EstimateProgress is what has been invoiced from an estimate: its milestones (with their invoice ids) and every
// non-deleted invoice made from it, ordered by number
type EstimateProgress struct {
	Milestones    []*EstimateMilestone
	Invoices      []*Invoice
	FullyInvoiced bool // every milestone is invoiced or, without milestones, an invoice exists
}

// EstimateConvertedInvoiceResponse is an invoice made from an estimate
type EstimateConvertedInvoiceResponse struct {
	Id     string `json:"id"`
	Number string `json:"number"`
}

// EstimateRespondRequest records your own answer for an estimate (Mark accepted / Mark declined)
type EstimateRespondRequest struct {
	Id     int64  `json:"id,string" binding:"required,min=1"`
	Status string `json:"status" binding:"required,oneof=accepted declined"`
}
```

In `pkg/models/invoice.go` add to `InvoiceModifyRequest`:

```go
	Kind       string                      `json:"kind" binding:"omitempty,oneof=invoice estimate"` // empty means invoice
	Milestones []*EstimateMilestoneRequest `json:"milestones" binding:"omitempty,max=10,dive"`      // estimates only; replaces the stored ones
```

Add to `InvoiceInfoResponse`:

```go
	Milestones        []*EstimateMilestoneResponse        `json:"milestones,omitempty"`        // estimates
	ConvertedInvoices []*EstimateConvertedInvoiceResponse `json:"convertedInvoices,omitempty"` // estimates: invoices made from it
	FullyInvoiced     bool                                `json:"fullyInvoiced,omitempty"`     // estimates
	EstimateNumber    string                              `json:"estimateNumber,omitempty"`    // invoices made from an estimate
```

Append to `pkg/models/invoice.go`:

```go
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
```

In `pkg/errs/invoice.go` after code 60:

```go
	ErrEstimateLocked        = NewNormalError(NormalSubcategoryInvoice, 61, http.StatusBadRequest, "estimate is accepted, declined or invoiced and cannot be edited")
	ErrEstimateHasInvoices   = NewNormalError(NormalSubcategoryInvoice, 62, http.StatusBadRequest, "estimate has invoices created from it")
	ErrEstimateStatusInvalid = NewNormalError(NormalSubcategoryInvoice, 63, http.StatusBadRequest, "estimate status does not allow this action")
```

- [ ] **Step 4: Services**

Create `pkg/services/estimate_lifecycle.go`:

```go
package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateHasInvoices reports whether a non-deleted invoice was made from the estimate
func estimateHasInvoices(sess *xorm.Session, uid int64, estimateId int64) (bool, error) {
	return sess.Where("uid=? AND deleted=? AND kind=? AND estimate_id=?", uid, false, models.INVOICE_KIND_INVOICE, estimateId).Exist(&models.Invoice{})
}

// getEstimateInSession reads an owned, non-deleted estimate with the caller's session; an invoice id is not found
func getEstimateInSession(sess *xorm.Session, uid int64, estimateId int64) (*models.Invoice, error) {
	estimate := &models.Invoice{}
	has, err := sess.Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).Get(estimate)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrInvoiceNotFound
	}

	return estimate, nil
}

// checkEstimateEditable returns ErrEstimateLocked once the estimate is accepted or declined or anything was invoiced from it
func checkEstimateEditable(sess *xorm.Session, estimate *models.Invoice) error {
	if estimate.EstimateStatus != models.ESTIMATE_STATUS_DRAFT && estimate.EstimateStatus != models.ESTIMATE_STATUS_SENT {
		return errs.ErrEstimateLocked
	}

	invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

	if err != nil {
		return err
	} else if invoiced {
		return errs.ErrEstimateLocked
	}

	return nil
}

// ModifyEstimate saves an editable estimate and replaces its lines and milestones, like ModifyInvoice does for lines.
// The update is conditional on the status still being draft or sent, so an answer that lands meanwhile wins.
func (s *InvoiceService) ModifyEstimate(c core.Context, estimate *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) error {
	if estimate.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimate.InvoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	if err := validateEstimateMilestones(milestones); err != nil {
		return err
	}

	if err := s.prepareInvoice(c, estimate, lines); err != nil {
		return err
	}

	if err := s.prepareEstimateMilestones(estimate, milestones); err != nil {
		return err
	}

	estimate.Kind = models.INVOICE_KIND_ESTIMATE
	estimate.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		stored, err := getEstimateInSession(sess, estimate.Uid, estimate.InvoiceId)

		if err != nil {
			return err
		}

		if err := checkEstimateEditable(sess, stored); err != nil {
			return err
		}

		if err := checkInvoiceNumberUnique(sess, estimate); err != nil {
			return err
		}

		updatedRows, err := sess.Cols(invoiceModifyColumns...).
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.InvoiceId).
			In("estimate_status", models.ESTIMATE_STATUS_DRAFT, models.ESTIMATE_STATUS_SENT).
			Update(estimate)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateLocked
		}

		if _, err := sess.Where("uid=? AND invoice_id=?", estimate.Uid, estimate.InvoiceId).Delete(&models.InvoiceLine{}); err != nil {
			return err
		}

		if _, err := sess.Insert(lines); err != nil {
			return err
		}

		if _, err := sess.Where("uid=? AND estimate_id=?", estimate.Uid, estimate.InvoiceId).Delete(&models.EstimateMilestone{}); err != nil {
			return err
		}

		if len(milestones) == 0 {
			return nil
		}

		_, err = sess.Insert(milestones)
		return err
	})
}

// SetEstimateResponse records your own answer (accepted or declined) for a draft or sent estimate, expired ones included.
// Nothing is emailed; RespondedByClient stays false.
func (s *InvoiceService) SetEstimateResponse(c core.Context, uid int64, estimateId int64, status string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	if status != models.ESTIMATE_STATUS_ACCEPTED && status != models.ESTIMATE_STATUS_DECLINED {
		return errs.ErrEstimateStatusInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := getEstimateInSession(sess, uid, estimateId); err != nil {
			return err
		}

		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).
			In("estimate_status", models.ESTIMATE_STATUS_DRAFT, models.ESTIMATE_STATUS_SENT).
			Update(&models.Invoice{EstimateStatus: status, RespondedUnixTime: now, RespondedByClient: false, ResponseNote: "", UpdatedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateStatusInvalid
		}

		return nil
	})
}

// ReopenEstimate returns an accepted or declined estimate to sent and clears the answer, while nothing is invoiced from it
func (s *InvoiceService) ReopenEstimate(c core.Context, uid int64, estimateId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := getEstimateInSession(sess, uid, estimateId); err != nil {
			return err
		}

		invoiced, err := estimateHasInvoices(sess, uid, estimateId)

		if err != nil {
			return err
		} else if invoiced {
			return errs.ErrEstimateHasInvoices
		}

		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).
			In("estimate_status", models.ESTIMATE_STATUS_ACCEPTED, models.ESTIMATE_STATUS_DECLINED).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT, RespondedUnixTime: 0, RespondedByClient: false, ResponseNote: "", UpdatedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateStatusInvalid
		}

		return nil
	})
}

// GetEstimateProgress returns, for each estimate id, its milestones and the non-deleted invoices made from it
func (s *InvoiceService) GetEstimateProgress(c core.Context, uid int64, estimateIds []int64) (map[int64]*models.EstimateProgress, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	result := make(map[int64]*models.EstimateProgress, len(estimateIds))

	for _, estimateId := range estimateIds {
		result[estimateId] = &models.EstimateProgress{Milestones: []*models.EstimateMilestone{}, Invoices: []*models.Invoice{}}
	}

	if len(estimateIds) < 1 {
		return result, nil
	}

	var milestones []*models.EstimateMilestone
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("estimate_id", estimateIds).OrderBy("display_order asc").Find(&milestones)

	if err != nil {
		return nil, err
	}

	for _, milestone := range milestones {
		result[milestone.EstimateId].Milestones = append(result[milestone.EstimateId].Milestones, milestone)
	}

	var invoices []*models.Invoice
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=?", uid, false, models.INVOICE_KIND_INVOICE).
		In("estimate_id", estimateIds).Cols("invoice_id", "estimate_id", "number").OrderBy("number asc").Find(&invoices)

	if err != nil {
		return nil, err
	}

	for _, invoice := range invoices {
		result[invoice.EstimateId].Invoices = append(result[invoice.EstimateId].Invoices, invoice)
	}

	for _, progress := range result {
		if len(progress.Milestones) == 0 {
			progress.FullyInvoiced = len(progress.Invoices) > 0
			continue
		}

		progress.FullyInvoiced = true

		for _, milestone := range progress.Milestones {
			if milestone.InvoiceId == 0 {
				progress.FullyInvoiced = false
			}
		}
	}

	return result, nil
}
```

In `pkg/services/invoices.go` add above `ModifyInvoice`:

```go
// invoiceModifyColumns are what editing rewrites; kind, status, answers, links and sent state never change on edit
var invoiceModifyColumns = []string{"number", "customer_id", "bill_to_name", "bill_to_contact_name", "bill_to_address_lines", "bill_to_email", "bill_to_phone", "bill_to_pan", "bill_to_gstin",
	"invoice_date", "due_date", "notes", "total", "tax_type", "tax_rate", "subtotal", "tax_amount", "updated_unix_time"}
```

Replace `ModifyInvoice`:

```go
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
```

Replace `DeleteInvoice`:

```go
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
		invoice := &models.Invoice{}
		has, err := sess.Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).Get(invoice)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrInvoiceNotFound
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
```

- [ ] **Step 5: API and routes**

In `pkg/api/invoices.go`:

- `InvoiceListHandler`: after the payments are loaded and before building `resps`, add

```go
	var progress map[int64]*models.EstimateProgress

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		progress, err = a.invoices.GetEstimateProgress(c, uid, invoiceIds)

		if err != nil {
			log.Errorf(c, "[invoices.InvoiceListHandler] failed to get estimate progress for user \"uid:%d\", because %s", uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}
```

and inside the loop after `resps[idx] = ...` add `resps[idx].SetEstimateProgress(progress[invoice.InvoiceId])` (a nil map yields nil, which changes nothing).

- `getInvoiceResponse`: before the final `return resp, nil` add

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		progress, err := a.invoices.GetEstimateProgress(c, uid, []int64{invoiceId})

		if err != nil {
			return nil, err
		}

		resp.SetEstimateProgress(progress[invoiceId])
	} else if invoice.EstimateId > 0 {
		// an estimate cannot be deleted while this invoice exists, so it is always there
		estimate, _, err := a.invoices.GetInvoice(c, uid, invoice.EstimateId)

		if err != nil {
			return nil, err
		}

		resp.EstimateNumber = estimate.Number
	}
```

- `InvoiceModifyHandler`: replace the `if err := a.invoices.ModifyInvoice(...)` block with

```go
	lines := invoiceLinesFromRequest(req.Lines)

	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		err = a.invoices.ModifyEstimate(c, invoice, lines, estimateMilestonesFromRequest(req.Milestones))
	} else if len(req.Milestones) > 0 {
		err = errs.ErrEstimateMilestonesInvalid
	} else {
		err = a.invoices.ModifyInvoice(c, invoice, lines)
	}

	if err != nil {
		log.Errorf(c, "[invoices.InvoiceModifyHandler] failed to modify invoice \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}
```

Create `pkg/api/estimates.go`:

```go
package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateResponse returns the estimate as the detail view shows it after an action
func (a *InvoicesApi) estimateResponse(c *core.WebContext, name string, uid int64, estimateId int64) (any, *errs.Error) {
	resp, err := a.getInvoiceResponse(c, uid, estimateId)

	if err != nil {
		log.Errorf(c, "[estimates.%s] failed to get estimate \"id:%d\" for user \"uid:%d\", because %s", name, estimateId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return resp, nil
}

// EstimateRespondHandler records your own accept / decline of an estimate of current user (nothing is emailed)
func (a *InvoicesApi) EstimateRespondHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.EstimateRespondRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateRespondHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.SetEstimateResponse(c, uid, req.Id, req.Status); err != nil {
		log.Errorf(c, "[estimates.EstimateRespondHandler] failed to mark estimate \"id:%d\" %s for user \"uid:%d\", because %s", req.Id, req.Status, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.estimateResponse(c, "EstimateRespondHandler", uid, req.Id)
}

// EstimateReopenHandler returns an answered estimate of current user to sent
func (a *InvoicesApi) EstimateReopenHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateReopenHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.invoices.ReopenEstimate(c, uid, req.Id); err != nil {
		log.Errorf(c, "[estimates.EstimateReopenHandler] failed to reopen estimate \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.estimateResponse(c, "EstimateReopenHandler", uid, req.Id)
}
```

In `cmd/webserver.go` after the `/invoices/send.json` route:

```go
			apiV1Route.POST("/estimates/respond.json", bindApi(api.Invoices.EstimateRespondHandler, config))
			apiV1Route.POST("/estimates/reopen.json", bindApi(api.Invoices.EstimateReopenHandler, config))
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 7: Commit**

```bash
git add pkg/models/estimate.go pkg/models/invoice.go pkg/errs/invoice.go pkg/services/estimate_lifecycle.go pkg/services/estimate_lifecycle_test.go pkg/services/invoices.go pkg/api/invoices.go pkg/api/estimates.go cmd/webserver.go
git commit --no-verify -m "estimates: edit, lock, delete rules, mark accepted / declined and reopen"
```

---
### Task 3: Estimates stay out of invoice-only features (backend)

**Files:**
- Modify: `pkg/errs/invoice.go` (64)
- Modify: `pkg/services/invoices.go` (`SetInvoiceClosed`, `GetLinkedInvoicesByTransactionIds`, `GetPaymentSuggestions`, `LinkPayment`)
- Modify: `pkg/services/recurring_invoices.go` (`CreateRecurringInvoice`, `GetInvoiceRenewalOption`, `GetInvoiceRenewals`)
- Test: create `pkg/services/estimate_filters_test.go`

**Interfaces:**
- Consumes (Task 1): `INVOICE_KIND_ESTIMATE`, `createTestEstimate`, `testEstimateLines`; existing `createTestProduct`, `insertTestTransaction`, `accountIdByName`.
- Produces: `errs.ErrNotAvailableForEstimate` (64). `LinkPayment`, `GetPaymentSuggestions` and `CreateRecurringInvoice` refuse an estimate with it; `GetInvoiceRenewalOption` returns `nil` (so `CreateRenewalForInvoice` creates nothing); `SetInvoiceClosed`, `GetInvoiceRenewals` and `GetLinkedInvoicesByTransactionIds` only see `kind = invoice`.

- [ ] **Step 1: Write the failing test**

Create `pkg/services/estimate_filters_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

func TestEstimatesStayOutOfInvoiceOnlyFeatures(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01",
		[]*models.InvoiceLine{{ItemName: "Domain", Quantity: 100, Rate: 150000, ProductId: domain.ProductId}}, nil)
	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 150000, "2026-04-25")

	// no payments or suggestions on a quote
	assert.Equal(t, errs.ErrNotAvailableForEstimate, Invoices.LinkPayment(ctx, uid, estimate.InvoiceId, income.TransactionId, 0))
	_, err := Invoices.GetPaymentSuggestions(ctx, uid, estimate.InvoiceId)
	assert.Equal(t, errs.ErrNotAvailableForEstimate, err)

	// no Mark as Paid
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.SetInvoiceClosed(ctx, uid, estimate.InvoiceId, true))

	// never repeated or renewed, even with a renewable product on it
	err = Invoices.CreateRecurringInvoice(ctx, &models.RecurringInvoice{Uid: uid, SourceInvoiceId: estimate.InvoiceId, DayOfMonth: 15, NextDate: "2099-01-15", Enabled: true})
	assert.Equal(t, errs.ErrNotAvailableForEstimate, err)
	option, err := Invoices.GetInvoiceRenewalOption(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, option)
	recurring, err := Invoices.CreateRenewalForInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, recurring)

	// a payment row pointing at an estimate (only possible by hand) never shows an invoice chip on the transaction
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Insert(&models.InvoicePayment{
		PaymentId: uuid.Container.GenerateUuid(uuid.UUID_TYPE_INVOICE), InvoiceId: estimate.InvoiceId, Uid: uid, TransactionId: income.TransactionId})
	assert.Nil(t, err)
	linked, err := Invoices.GetLinkedInvoicesByTransactionIds(ctx, uid, []int64{income.TransactionId})
	assert.Nil(t, err)
	assert.Empty(t, linked)

	// and it is not an invoice in the invoice list (outstanding and overdue are computed from it)
	invoices, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, invoices)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/services/ -run TestEstimatesStayOutOfInvoiceOnlyFeatures -count=1`
Expected: compile FAIL (`undefined: errs.ErrNotAvailableForEstimate`).

- [ ] **Step 3: Implement**

In `pkg/errs/invoice.go` after code 63:

```go
	ErrNotAvailableForEstimate = NewNormalError(NormalSubcategoryInvoice, 64, http.StatusBadRequest, "this action is not available for estimates")
```

In `pkg/services/invoices.go`:
- `SetInvoiceClosed`: change the condition to `Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_INVOICE, invoiceId)`.
- `GetLinkedInvoicesByTransactionIds`: change the condition to `Where("invoice_payment.uid=? AND invoice.uid=? AND invoice.deleted=? AND invoice.kind=?", uid, uid, false, models.INVOICE_KIND_INVOICE)`.
- `GetPaymentSuggestions`: right after `invoice, _, err := s.GetInvoice(c, uid, invoiceId)` and its error check add

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return nil, errs.ErrNotAvailableForEstimate
	}
```

- `LinkPayment`: right after `invoice, _, err := s.GetInvoice(c, uid, invoiceId)` and its error check add

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return errs.ErrNotAvailableForEstimate
	}
```

In `pkg/services/recurring_invoices.go`:
- `CreateRecurringInvoice`: replace `if _, _, err := s.GetInvoice(c, recurring.Uid, recurring.SourceInvoiceId); err != nil { return err }` with

```go
	source, _, err := s.GetInvoice(c, recurring.Uid, recurring.SourceInvoiceId)

	if err != nil {
		return err
	}

	// an estimate is a quote, not a bill: it is never repeated
	if source.Kind == models.INVOICE_KIND_ESTIMATE {
		return errs.ErrNotAvailableForEstimate
	}
```

- `GetInvoiceRenewalOption`: right after `invoice, lines, err := s.GetInvoice(c, uid, invoiceId)` and its error check add

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return nil, nil // estimates are never renewed
	}
```

- `GetInvoiceRenewals`: change the condition to `Where("uid=? AND deleted=? AND kind=? AND recurring_id>? AND closed_manually=? AND (sent_unix_time=? OR sent_unix_time>=?)", uid, false, models.INVOICE_KIND_INVOICE, 0, false, 0, nowUnixTime-models.InvoiceRenewalAutoSentVisibleSeconds)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1`
Expected: PASS (the new test and every recurring / renewal / payment test).

- [ ] **Step 5: Commit**

```bash
git add pkg/errs/invoice.go pkg/services/invoices.go pkg/services/recurring_invoices.go pkg/services/estimate_filters_test.go
git commit --no-verify -m "estimates: keep estimates out of payments, mark as paid, recurring and renewals"
```

---

### Task 4: Converting an estimate into invoices (backend)

**Files:**
- Modify: `pkg/models/estimate.go` (`EstimateInvoiceDraftRequest`)
- Modify: `pkg/models/invoice.go` (`InvoiceCreateRequest.EstimateId`, `.MilestoneId`; `InvoiceInfoResponse.MilestoneId`)
- Modify: `pkg/errs/invoice.go` (65, 66)
- Create: `pkg/services/estimate_conversion.go`
- Modify: `pkg/api/invoices.go` (`InvoiceCreateHandler`)
- Create: `pkg/api/estimate_conversion.go`
- Modify: `cmd/webserver.go`
- Test: create `pkg/services/estimate_conversion_test.go`

**Interfaces:**
- Consumes (Tasks 1–2): `GetEstimateMilestones`, `getEstimateInSession`, `estimateHasInvoices`, `prepareNewInvoice`, `insertInvoice`, `computeInvoiceTotals`, `NextInvoiceNumber`, `SetEstimateResponse`, `DeleteInvoice`, test helpers.
- Produces:
  - `estimateShare(amount int64, basisPoints int32) int64` (half up, overflow-safe).
  - `estimateMilestoneRate(subtotal int64, milestones []*models.EstimateMilestone, milestoneId int64, invoicedSubtotals map[int64]int64) (int64, *models.EstimateMilestone, error)`.
  - `estimateMilestoneLineText(label string, percent int32, estimateNumber string) (itemName string, description string)`.
  - `estimateInvoiceLines(sess, estimate *models.Invoice, estimateLines []*models.InvoiceLine, milestoneId int64) ([]*models.InvoiceLine, error)`.
  - `(*InvoiceService).GetEstimateInvoiceDraft(c, uid, estimateId, milestoneId int64, today string) (*models.Invoice, []*models.InvoiceLine, error)`.
  - `(*InvoiceService).CreateInvoiceFromEstimate(c, invoice *models.Invoice, lines []*models.InvoiceLine, estimateId int64, milestoneId int64) error`.
  - `errs.ErrEstimateAlreadyConverted` (65), `errs.ErrEstimateMilestoneNotFound` (66).
  - `models.EstimateInvoiceDraftRequest{Id, MilestoneId}`; JSON `estimateId`, `milestoneId` on invoice create; `milestoneId` on the draft response.
  - Route `GET /api/v1/estimates/invoice_draft.json?id=<estimate>&milestone_id=<milestone, optional>` → unsaved `InvoiceInfoResponse` with `lines`, `estimateId`, `milestoneId`; handler `EstimateInvoiceDraftHandler`.
  - Test helper `newTestInvoiceDraft(uid, customerId int64, number string) *models.Invoice`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_conversion_test.go`:

```go
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// newTestInvoiceDraft is what the editor sends when saving an invoice made from an estimate
func newTestInvoiceDraft(uid int64, customerId int64, number string) *models.Invoice {
	return &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-10-30", DueDate: "2026-11-06"}
}

func TestEstimateMilestoneRateRoundsHalfUpAndLastOpenTakesTheRemainder(t *testing.T) {
	milestones := []*models.EstimateMilestone{{MilestoneId: 1, Percent: 3333}, {MilestoneId: 2, Percent: 3333}, {MilestoneId: 3, Percent: 3334}}

	rate, milestone, err := estimateMilestoneRate(199999, milestones, 1, nil)
	assert.Nil(t, err)
	assert.Equal(t, int64(66660), rate) // 66,659.67 rounds up
	assert.Equal(t, int64(1), milestone.MilestoneId)

	rate, _, err = estimateMilestoneRate(199999, milestones, 3, nil)
	assert.Nil(t, err)
	assert.Equal(t, int64(66680), rate) // not the last open one: its plain share

	// Go-live and Advance invoiced first: Design sign-off is the last open one and takes exactly what is left
	milestones[0].InvoiceId, milestones[2].InvoiceId = 101, 103
	rate, _, err = estimateMilestoneRate(199999, milestones, 2, map[int64]int64{1: 66660, 3: 66680})
	assert.Nil(t, err)
	assert.Equal(t, int64(66659), rate)
	assert.Equal(t, int64(199999), 66660+rate+66680)

	// an invoice edited after it was made: the remainder follows what was really invoiced
	rate, _, err = estimateMilestoneRate(199999, milestones, 2, map[int64]int64{1: 70000, 3: 66680})
	assert.Nil(t, err)
	assert.Equal(t, int64(63319), rate)

	_, _, err = estimateMilestoneRate(199999, milestones, 1, nil)
	assert.Equal(t, errs.ErrEstimateAlreadyConverted, err)
	_, _, err = estimateMilestoneRate(199999, milestones, 9, nil)
	assert.Equal(t, errs.ErrEstimateMilestoneNotFound, err)

	// no overflow on the largest amounts a line allows
	assert.Equal(t, int64(333300000000), estimateShare(1000000000000, 3333))
}

func TestEstimateMilestoneLineText(t *testing.T) {
	name, description := estimateMilestoneLineText("Advance", 3000, "EST/2026-27/01")
	assert.Equal(t, "Advance: 30% of Estimate EST/2026-27/01", name)
	assert.Equal(t, "", description)

	name, _ = estimateMilestoneLineText("Design sign-off", 3333, "EST/2026-27/01")
	assert.Equal(t, "Design sign-off: 33.33% of Estimate EST/2026-27/01", name)

	// a long label keeps the 128-character item name; the share moves to the description
	long := strings.Repeat("x", 120)
	name, description = estimateMilestoneLineText(long, 5000, "EST/2026-27/01")
	assert.Equal(t, long, name)
	assert.Equal(t, "50% of Estimate EST/2026-27/01", description)
}

func TestEstimateInvoiceDraftCopiesEveryLineForAFullConversion(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	support := createTestProduct(t, uid, "Support", 0, "")
	lines := testEstimateLines()
	lines[1].ProductId = support.ProductId
	estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31",
		Notes: "Valid for 30 days", TaxType: models.INVOICE_TAX_TYPE_IGST, TaxRate: 1800}
	assert.Nil(t, Invoices.CreateEstimate(ctx, estimate, lines, nil))

	draft, draftLines, err := Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, 0, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, draft.Kind)
	assert.Equal(t, estimate.InvoiceId, draft.EstimateId)
	assert.Equal(t, customer.CustomerId, draft.CustomerId)
	assert.Equal(t, "INV/2026-27/01", draft.Number)
	assert.Equal(t, "2026-10-30", draft.InvoiceDate)
	assert.Equal(t, "2026-11-06", draft.DueDate) // the profile's default terms: 7 days
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, draft.TaxType)
	assert.Equal(t, int32(1800), draft.TaxRate)
	assert.Equal(t, int64(235999), draft.Total)
	assert.Equal(t, estimate.Total, draft.Total)
	assert.Equal(t, 2, len(draftLines))
	assert.Equal(t, "Website", draftLines[0].ItemName)
	assert.Equal(t, "Design and build", draftLines[0].Description)
	assert.Equal(t, int64(300), draftLines[1].Quantity)
	assert.Equal(t, int64(33333), draftLines[1].Rate)
	assert.Equal(t, "998315", draftLines[1].Sac)
	assert.Equal(t, support.ProductId, draftLines[1].ProductId)
}

func TestEstimateInvoiceDraftOfAMilestoneIsOneLine(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	draft, draftLines, err := Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, milestones[0].MilestoneId, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(draftLines))
	assert.Equal(t, "Advance: 33.33% of Estimate EST/2026-27/01", draftLines[0].ItemName)
	assert.Equal(t, "998314", draftLines[0].Sac) // the estimate's first line
	assert.Equal(t, int64(100), draftLines[0].Quantity)
	assert.Equal(t, int64(66660), draftLines[0].Rate)
	assert.Equal(t, int64(66660), draft.Subtotal)

	// with milestones, the estimate is invoiced milestone by milestone
	_, _, err = Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, 0, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateMilestoneNotFound, err)

	// a declined estimate is not converted
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	_, _, err = Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, milestones[0].MilestoneId, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateStatusInvalid, err)
}

func TestCreateInvoiceFromMilestoneRecomputesTheRateAndInvoicesEachMilestoneOnce(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	other := &models.Customer{Uid: uid, Name: "Someone Else"}
	assert.Nil(t, Invoices.CreateCustomer(ctx, other))
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	// the client sends a tampered rate, quantity and customer; the server bills the estimate's customer the milestone's share
	advance := newTestInvoiceDraft(uid, other.CustomerId, "INV/2026-27/01")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, advance, []*models.InvoiceLine{{ItemName: "Advance", Quantity: 500, Rate: 1}}, estimate.InvoiceId, milestones[0].MilestoneId))
	saved, lines, err := Invoices.GetInvoice(ctx, uid, advance.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(100), lines[0].Quantity)
	assert.Equal(t, int64(66660), lines[0].Rate)
	assert.Equal(t, int64(66660), saved.Subtotal)
	assert.Equal(t, customer.CustomerId, saved.CustomerId)
	assert.Equal(t, estimate.InvoiceId, saved.EstimateId)

	// a second click on the same milestone is refused
	again := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Equal(t, errs.ErrEstimateAlreadyConverted,
		Invoices.CreateInvoiceFromEstimate(ctx, again, []*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 66660}}, estimate.InvoiceId, milestones[0].MilestoneId))

	// Go-live next; Design sign-off is then the last open one and takes the remainder
	goLive := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, goLive, []*models.InvoiceLine{{ItemName: "Go-live", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[2].MilestoneId))
	signOff := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, signOff, []*models.InvoiceLine{{ItemName: "Sign-off", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[1].MilestoneId))
	assert.Equal(t, int64(66680), goLive.Subtotal)
	assert.Equal(t, int64(66659), signOff.Subtotal)
	assert.Equal(t, estimate.Subtotal, advance.Subtotal+goLive.Subtotal+signOff.Subtotal)

	milestones, err = Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, signOff.InvoiceId, milestones[1].InvoiceId)

	// deleting a milestone invoice frees the milestone, which can be invoiced again for the same remainder
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, signOff.InvoiceId))
	redo := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/04")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, redo, []*models.InvoiceLine{{ItemName: "Sign-off", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[1].MilestoneId))
	assert.Equal(t, int64(66659), redo.Subtotal)

	// a milestone invoice has exactly one line
	twoLines := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/05")
	assert.Equal(t, errs.ErrEstimateMilestonesInvalid, Invoices.CreateInvoiceFromEstimate(ctx, twoLines, testEstimateLines(), estimate.InvoiceId, milestones[0].MilestoneId))
}

func TestFullConversionHappensOnceWhileItsInvoiceExists(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	// the editor may adjust the copied lines before saving
	invoice := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01")
	adjusted := testEstimateLines()
	adjusted[1].Quantity = 200
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, invoice, adjusted, estimate.InvoiceId, 0))
	assert.Equal(t, estimate.InvoiceId, invoice.EstimateId)
	assert.Equal(t, int64(166666), invoice.Subtotal)

	second := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02")
	assert.Equal(t, errs.ErrEstimateAlreadyConverted, Invoices.CreateInvoiceFromEstimate(ctx, second, testEstimateLines(), estimate.InvoiceId, 0))
	assert.Equal(t, errs.ErrEstimateHasInvoices, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, second, testEstimateLines(), estimate.InvoiceId, 0))

	// an invoice id is not an estimate
	third := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03")
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.CreateInvoiceFromEstimate(ctx, third, testEstimateLines(), second.InvoiceId, 0))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestEstimateMilestoneRate|TestEstimateMilestoneLineText|TestEstimateInvoiceDraft|TestCreateInvoiceFromMilestone|TestFullConversion' -count=1`
Expected: compile FAIL (`undefined: estimateMilestoneRate`, `Invoices.GetEstimateInvoiceDraft undefined`).

- [ ] **Step 3: Models and errors**

Append to `pkg/models/estimate.go`:

```go
// EstimateInvoiceDraftRequest asks for the unsaved invoice an estimate (no milestone) or one of its milestones turns into
type EstimateInvoiceDraftRequest struct {
	Id          int64 `form:"id" binding:"required,min=1"`
	MilestoneId int64 `form:"milestone_id" binding:"min=0"`
}
```

In `pkg/models/invoice.go` add to `InvoiceCreateRequest`:

```go
	EstimateId  int64 `json:"estimateId,string" binding:"min=0"`  // the invoice is made from this estimate; 0 = typed by hand
	MilestoneId int64 `json:"milestoneId,string" binding:"min=0"` // and from this milestone of it; 0 = the whole estimate
```

and to `InvoiceInfoResponse`: ``MilestoneId string `json:"milestoneId,omitempty"` // conversion drafts of one milestone``.

In `pkg/errs/invoice.go` after code 64:

```go
	ErrEstimateAlreadyConverted  = NewNormalError(NormalSubcategoryInvoice, 65, http.StatusBadRequest, "estimate or milestone has already been invoiced")
	ErrEstimateMilestoneNotFound = NewNormalError(NormalSubcategoryInvoice, 66, http.StatusBadRequest, "estimate milestone not found")
```

- [ ] **Step 4: Services**

Create `pkg/services/estimate_conversion.go`:

```go
package services

import (
	"strconv"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateMilestoneQuantity is one unit (quantities are hundredths): a milestone invoice bills its share once
const estimateMilestoneQuantity = 100

// estimateShare is amount × basisPoints / 10000 rounded half up, split like the tax so it cannot overflow int64;
// src/lib/estimate.ts mirrors it for the Payment Terms table
func estimateShare(amount int64, basisPoints int32) int64 {
	whole, rest := amount/models.EstimateMilestonePercentTotal, amount%models.EstimateMilestonePercentTotal
	return whole*int64(basisPoints) + (rest*int64(basisPoints)+models.EstimateMilestonePercentTotal/2)/models.EstimateMilestonePercentTotal
}

// estimateMilestoneRate is the rate of the one line a milestone invoice gets: the milestone's share of the estimate subtotal, except
// that the last open milestone takes subtotal minus what the other milestones were really invoiced for (invoicedSubtotals, by
// milestone id), so the invoices always add up to the estimate exactly, in any order and after edits
func estimateMilestoneRate(subtotal int64, milestones []*models.EstimateMilestone, milestoneId int64, invoicedSubtotals map[int64]int64) (int64, *models.EstimateMilestone, error) {
	var target *models.EstimateMilestone
	open := 0

	for _, milestone := range milestones {
		if milestone.MilestoneId == milestoneId {
			target = milestone
		}

		if milestone.InvoiceId == 0 {
			open++
		}
	}

	if target == nil {
		return 0, nil, errs.ErrEstimateMilestoneNotFound
	}

	if target.InvoiceId > 0 {
		return 0, nil, errs.ErrEstimateAlreadyConverted
	}

	if open > 1 {
		return estimateShare(subtotal, target.Percent), target, nil
	}

	remainder := subtotal

	for _, milestone := range milestones {
		if milestone.MilestoneId != milestoneId {
			remainder -= invoicedSubtotals[milestone.MilestoneId]
		}
	}

	return remainder, target, nil
}

// estimateMilestoneLineText is the milestone line "<label>: <pct>% of Estimate <number>"; when that would not fit the
// 128-character item name, the label stays the name and the share goes to the description
func estimateMilestoneLineText(label string, percent int32, estimateNumber string) (string, string) {
	share := strconv.FormatFloat(float64(percent)/100, 'f', -1, 64) + "% of Estimate " + estimateNumber
	name := label + ": " + share

	if utf8.RuneCountInString(name) <= models.EstimateMilestoneLabelMaxLength {
		return name, ""
	}

	return label, share
}

// estimateInvoiceLines returns the lines an invoice made from the estimate gets, checking it can still be made: every line
// copied (milestoneId 0; only for an estimate without milestones and not invoiced yet) or the one line of an open milestone
func estimateInvoiceLines(sess *xorm.Session, estimate *models.Invoice, estimateLines []*models.InvoiceLine, milestoneId int64) ([]*models.InvoiceLine, error) {
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED {
		return nil, errs.ErrEstimateStatusInvalid
	}

	var milestones []*models.EstimateMilestone
	if err := sess.Where("uid=? AND estimate_id=?", estimate.Uid, estimate.InvoiceId).OrderBy("display_order asc").Find(&milestones); err != nil {
		return nil, err
	}

	if milestoneId == 0 {
		if len(milestones) > 0 {
			return nil, errs.ErrEstimateMilestoneNotFound
		}

		invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

		if err != nil {
			return nil, err
		} else if invoiced {
			return nil, errs.ErrEstimateAlreadyConverted
		}

		lines := make([]*models.InvoiceLine, len(estimateLines))

		for idx, estimateLine := range estimateLines {
			line := *estimateLine
			line.LineId = 0
			line.InvoiceId = 0
			lines[idx] = &line
		}

		return lines, nil
	}

	invoicedSubtotals := make(map[int64]int64)
	invoiceIds := make([]int64, 0, len(milestones))

	for _, milestone := range milestones {
		if milestone.InvoiceId > 0 {
			invoiceIds = append(invoiceIds, milestone.InvoiceId)
		}
	}

	if len(invoiceIds) > 0 {
		var invoices []*models.Invoice
		if err := sess.Where("uid=? AND deleted=?", estimate.Uid, false).In("invoice_id", invoiceIds).Cols("invoice_id", "subtotal").Find(&invoices); err != nil {
			return nil, err
		}

		subtotals := make(map[int64]int64, len(invoices))

		for _, invoice := range invoices {
			subtotals[invoice.InvoiceId] = invoice.Subtotal
		}

		for _, milestone := range milestones {
			if milestone.InvoiceId > 0 {
				invoicedSubtotals[milestone.MilestoneId] = subtotals[milestone.InvoiceId]
			}
		}
	}

	rate, milestone, err := estimateMilestoneRate(estimate.Subtotal, milestones, milestoneId, invoicedSubtotals)

	if err != nil {
		return nil, err
	} else if rate <= 0 {
		return nil, errs.ErrEstimateAlreadyConverted // the other invoices already add up to the whole estimate
	}

	sac := ""

	if len(estimateLines) > 0 {
		sac = estimateLines[0].Sac
	}

	itemName, description := estimateMilestoneLineText(milestone.Label, milestone.Percent, estimate.Number)

	return []*models.InvoiceLine{{ItemName: itemName, Description: description, Quantity: estimateMilestoneQuantity, Rate: rate, Sac: sac}}, nil
}

// GetEstimateInvoiceDraft builds, without saving, the invoice an estimate (milestoneId 0) or one of its milestones turns into, for the
// editor: dated today, due by the customer's or the profile's terms, numbered from the invoice series, in the estimate's currency with
// its tax settings, and the profile's default notes
func (s *InvoiceService) GetEstimateInvoiceDraft(c core.Context, uid int64, estimateId int64, milestoneId int64, today string) (*models.Invoice, []*models.InvoiceLine, error) {
	estimate, estimateLines, err := s.GetInvoice(c, uid, estimateId)

	if err != nil {
		return nil, nil, err
	} else if estimate.Kind != models.INVOICE_KIND_ESTIMATE {
		return nil, nil, errs.ErrInvoiceNotFound
	}

	lines, err := estimateInvoiceLines(s.UserDataDB(uid).NewSession(c), estimate, estimateLines, milestoneId)

	if err != nil {
		return nil, nil, err
	}

	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return nil, nil, errs.ErrInvoiceDateInvalid
	}

	customer, err := s.GetCustomer(c, uid, estimate.CustomerId)

	if err != nil {
		return nil, nil, err
	}

	profile, err := s.GetProfile(c, uid)

	if err != nil {
		return nil, nil, err
	}

	termsDays := profile.DefaultTermsDays

	if customer.TermsDays > 0 {
		termsDays = customer.TermsDays
	}

	number, err := s.NextInvoiceNumber(c, uid, today)

	if err != nil {
		return nil, nil, err
	}

	draft := &models.Invoice{Uid: uid, Kind: models.INVOICE_KIND_INVOICE, Number: number, CustomerId: estimate.CustomerId, Currency: estimate.Currency,
		BillToName: estimate.BillToName, InvoiceDate: today, DueDate: todayTime.AddDate(0, 0, int(termsDays)).Format(invoiceDateLayout),
		Notes: profile.DefaultNotes, TaxType: estimate.TaxType, TaxRate: estimate.TaxRate, EstimateId: estimateId}

	totals := computeInvoiceTotals(lines, draft.TaxType, draft.TaxRate)
	draft.Subtotal, draft.TaxAmount, draft.Total = totals.Subtotal, totals.TaxAmount, totals.Total

	return draft, lines, nil
}

// CreateInvoiceFromEstimate saves an invoice made from an estimate (milestoneId 0 = the whole estimate). It bills the estimate's customer
// in the estimate's currency. For a milestone the one line's quantity and rate are rebuilt from the estimate inside the transaction, so a
// client cannot change the amount and the remainder rule sees every other conversion; the milestone is marked invoiced in the same
// transaction, conditional on still being open, so a second click cannot invoice it twice. A whole-estimate conversion keeps the lines as
// edited and is refused while another invoice made from the estimate exists.
func (s *InvoiceService) CreateInvoiceFromEstimate(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine, estimateId int64, milestoneId int64) error {
	if invoice.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	estimate, estimateLines, err := s.GetInvoice(c, invoice.Uid, estimateId)

	if err != nil {
		return err
	} else if estimate.Kind != models.INVOICE_KIND_ESTIMATE {
		return errs.ErrInvoiceNotFound
	}

	if milestoneId > 0 && len(lines) != 1 {
		return errs.ErrEstimateMilestonesInvalid
	}

	invoice.CustomerId = estimate.CustomerId
	invoice.Currency = estimate.Currency

	if err := s.prepareNewInvoice(c, invoice, lines); err != nil {
		return err
	}

	invoice.EstimateId = estimateId

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		// writing the estimate row first makes two conversions of the same estimate wait for each other
		if _, err := sess.Cols("updated_unix_time").Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", invoice.Uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).
			Update(&models.Invoice{UpdatedUnixTime: invoice.CreatedUnixTime}); err != nil {
			return err
		}

		stored, err := getEstimateInSession(sess, invoice.Uid, estimateId)

		if err != nil {
			return err
		}

		expected, err := estimateInvoiceLines(sess, stored, estimateLines, milestoneId)

		if err != nil {
			return err
		}

		if milestoneId > 0 {
			lines[0].Quantity = expected[0].Quantity
			lines[0].Rate = expected[0].Rate
			totals := computeInvoiceTotals(lines, invoice.TaxType, invoice.TaxRate)
			invoice.Subtotal, invoice.TaxAmount, invoice.Total = totals.Subtotal, totals.TaxAmount, totals.Total

			updatedRows, err := sess.Cols("invoice_id").Where("uid=? AND estimate_id=? AND milestone_id=? AND invoice_id=?", invoice.Uid, estimateId, milestoneId, 0).
				Update(&models.EstimateMilestone{InvoiceId: invoice.InvoiceId})

			if err != nil {
				return err
			} else if updatedRows < 1 {
				return errs.ErrEstimateAlreadyConverted
			}
		}

		return insertInvoice(sess, invoice, lines)
	})
}
```

- [ ] **Step 5: API and route**

In `InvoiceCreateHandler` (`pkg/api/invoices.go`) change the kind dispatch from Task 1 to:

```go
	if req.Kind == models.INVOICE_KIND_ESTIMATE {
		err = a.invoices.CreateEstimate(c, invoice, lines, estimateMilestonesFromRequest(req.Milestones))
	} else if len(req.Milestones) > 0 {
		err = errs.ErrEstimateMilestonesInvalid // only an estimate has payment terms
	} else if req.EstimateId > 0 {
		err = a.invoices.CreateInvoiceFromEstimate(c, invoice, lines, req.EstimateId, req.MilestoneId)
	} else {
		err = a.invoices.CreateInvoice(c, invoice, lines)
	}
```

Create `pkg/api/estimate_conversion.go`:

```go
package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// EstimateInvoiceDraftHandler returns, unsaved, the invoice an estimate of current user (or one of its milestones) turns into;
// the editor opens it and saves it through /invoices/add.json with estimateId and milestoneId
func (a *InvoicesApi) EstimateInvoiceDraftHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.EstimateInvoiceDraftRequest

	if err := c.ShouldBindQuery(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateInvoiceDraftHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	draft, lines, err := a.invoices.GetEstimateInvoiceDraft(c, uid, req.Id, req.MilestoneId, time.Now().Format("2006-01-02"))

	if err != nil {
		log.Errorf(c, "[estimates.EstimateInvoiceDraftHandler] failed to build invoice from estimate \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resp := draft.ToInvoiceInfoResponse(lines, nil)

	if req.MilestoneId > 0 {
		resp.MilestoneId = utils.Int64ToString(req.MilestoneId)
	}

	return resp, nil
}
```

In `cmd/webserver.go` after the `/estimates/reopen.json` route:

```go
			apiV1Route.GET("/estimates/invoice_draft.json", bindApi(api.Invoices.EstimateInvoiceDraftHandler, config))
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 7: Commit**

```bash
git add pkg/models/estimate.go pkg/models/invoice.go pkg/errs/invoice.go pkg/services/estimate_conversion.go pkg/services/estimate_conversion_test.go pkg/api/invoices.go pkg/api/estimate_conversion.go cmd/webserver.go
git commit --no-verify -m "estimates: convert to invoices, whole or milestone by milestone, with the remainder on the last"
```

---

### Task 5: Client accepts or declines through the public link, owner is emailed (backend)

**Files:**
- Modify: `pkg/models/estimate.go` (`EstimatePublicRespondRequest`)
- Modify: `pkg/errs/invoice.go` (67, 68)
- Create: `pkg/services/estimate_public.go`
- Create: `pkg/api/estimate_public.go`
- Modify: `cmd/webserver.go` (public routes)
- Test: create `pkg/services/estimate_public_test.go`, `pkg/api/estimates_test.go`

**Interfaces:**
- Consumes (Tasks 1–2): status constants, `EstimateResponseNoteMaxLength`, `SetInvoicePublicToken`, `Users.GetUserById`, `SendMail`, `invoicepdf.FormatAmount`; test helpers `withCapturingMailer`, `withFailingMailer`, `createTestEstimate`.
- Produces:
  - `(*InvoiceService).RespondToEstimateByPublicToken(c, token, status, note, today string) error`; `(*InvoiceService).recordEstimateClientResponse(c, estimate *models.Invoice, token, status, note string) (bool, error)`; `(*InvoiceService).notifyEstimateResponse(c, estimate)`; `estimateResponseEmail(estimate *models.Invoice, rootUrl string) (subject string, body string)`.
  - `errs.ErrEstimateCannotBeAnswered` (67), `errs.ErrEstimateResponseNoteTooLong` (68).
  - `models.EstimatePublicRespondRequest{Token, Note}`.
  - Routes `POST /api/public/estimates/accept.json` and `/decline.json` with `{ token, note? }` returning `true`; handlers `EstimatePublicAcceptHandler`, `EstimatePublicDeclineHandler`.
  - Test helper `shareTestEstimate(t, ctx, uid, estimateId) string`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_public_test.go`:

```go
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// shareTestEstimate marks an estimate sent and gives it a public link, as emailing it does; it returns the token
func shareTestEstimate(t *testing.T, ctx core.Context, uid int64, estimateId int64) string {
	t.Helper()
	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("estimate_status").
		Where("uid=? AND invoice_id=?", uid, estimateId).Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT})
	assert.Nil(t, err)
	token, err := Invoices.SetInvoicePublicToken(ctx, uid, estimateId, true)
	assert.Nil(t, err)
	return token
}

func TestClientAcceptsEstimateThroughPublicLinkAndOwnerIsEmailed(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "a note is only kept on decline", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	assert.True(t, reloaded.RespondedByClient)
	assert.Greater(t, reloaded.RespondedUnixTime, int64(0))
	assert.Equal(t, "", reloaded.ResponseNote)

	user, err := Users.GetUserById(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(mailer.sent))
	assert.Equal(t, user.Email, mailer.sent[0].To)
	assert.Equal(t, "Quber Tech accepted Estimate EST/2026-27/01 (₹1,999.99)", mailer.sent[0].Subject)
	assert.Contains(t, mailer.sent[0].Body, "https://books.example/desktop#/invoice/list?invoiceId="+utils.Int64ToString(estimate.InvoiceId))

	// a second click, or the other button in another tab, records nothing and sends nothing
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))
	assert.Equal(t, 1, len(mailer.sent))
}

func TestConcurrentPublicAnswersRecordOnlyOne(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	// what a second request read just before the first one wrote
	stale, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	recorded, err := Invoices.recordEstimateClientResponse(ctx, stale, token, models.ESTIMATE_STATUS_DECLINED, "too late")
	assert.Nil(t, err)
	assert.False(t, recorded)

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	assert.Equal(t, "", reloaded.ResponseNote)
}

func TestPublicEstimateAnswerGuards(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	// never sent
	draft := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	draftToken, err := Invoices.SetInvoicePublicToken(ctx, uid, draft.InvoiceId, true)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, draftToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	// valid until April 30: refused the day after, accepted on the day itself
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	expiredToken := shareTestEstimate(t, ctx, uid, expired.InvoiceId)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, expiredToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-05-01"))
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, expiredToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-04-30"))

	sent := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/03", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, sent.InvoiceId)
	assert.Equal(t, errs.ErrEstimateResponseNoteTooLong, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, strings.Repeat("é", 501), "2026-10-30"))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_SENT, "", "2026-10-30"))

	// a disabled (or reset) link is a plain not-found
	_, err = Invoices.SetInvoicePublicToken(ctx, uid, sent.InvoiceId, false)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))
	assert.Equal(t, errs.ErrInvoiceNotFound, Invoices.RespondToEstimateByPublicToken(ctx, "short", models.ESTIMATE_STATUS_DECLINED, "", "2026-10-30"))

	// an invoice's link answers nothing
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	invoiceToken, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, invoiceToken, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, sent.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
}

func TestClientDeclineKeepsThePlainTextNote(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	note := "<b>Too expensive</b>\nMaybe next quarter"

	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_DECLINED, "  "+note+"  ", "2026-10-30"))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_DECLINED, reloaded.EstimateStatus)
	assert.Equal(t, note, reloaded.ResponseNote)
	assert.Equal(t, "Quber Tech declined Estimate EST/2026-27/01 (₹1,999.99)", mailer.sent[0].Subject)
	assert.Contains(t, mailer.sent[0].Body, "&lt;b&gt;Too expensive&lt;/b&gt;<br/>Maybe next quarter")
	assert.NotContains(t, mailer.sent[0].Body, "<b>Too")
}

func TestClientAnswerCountsWhenTheOwnerEmailFails(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	// SMTP off
	first := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, shareTestEstimate(t, ctx, uid, first.InvoiceId), models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	// SMTP on, sending fails
	withFailingMailer(t)
	second := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	assert.Nil(t, Invoices.RespondToEstimateByPublicToken(ctx, shareTestEstimate(t, ctx, uid, second.InvoiceId), models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	for _, estimate := range []*models.Invoice{first, second} {
		reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
		assert.Nil(t, err)
		assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)
	}
}
```

Create `pkg/api/estimates_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
)

// newTestJsonWebContext wraps a POST request with a JSON body in a web context, like bindApi does
func newTestJsonWebContext(path string, body string) *core.WebContext {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	ginContext.Request.Header.Set("Content-Type", "application/json")

	return &core.WebContext{Context: ginContext}
}

func TestEstimatePublicAnswerHandlersRefuseBadRequestsWithoutTheDatabase(t *testing.T) {
	// a token of the wrong length never reaches the database, so this needs no store
	result, err := Invoices.EstimatePublicAcceptHandler(newTestJsonWebContext("/api/public/estimates/accept.json", `{"token":"unknown"}`))
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{"token":"unknown","note":"`+strings.Repeat("x", 501)+`"}`))
	assert.NotNil(t, err) // the 500-character limit is checked when the body is bound

	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{}`))
	assert.NotNil(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ ./pkg/api/ -run 'TestClientAccepts|TestConcurrentPublicAnswers|TestPublicEstimateAnswerGuards|TestClientDecline|TestClientAnswerCounts|TestEstimatePublicAnswerHandlers' -count=1`
Expected: compile FAIL (`Invoices.RespondToEstimateByPublicToken undefined`).

- [ ] **Step 3: Models and errors**

Append to `pkg/models/estimate.go`:

```go
// EstimatePublicRespondRequest is the client's answer through a public estimate link: the token is the only key
type EstimatePublicRespondRequest struct {
	Token string `json:"token" binding:"required,max=64"`
	Note  string `json:"note" binding:"max=500"` // decline only; plain text
}
```

In `pkg/errs/invoice.go` after code 66:

```go
	ErrEstimateCannotBeAnswered    = NewNormalError(NormalSubcategoryInvoice, 67, http.StatusBadRequest, "this estimate can no longer be answered")
	ErrEstimateResponseNoteTooLong = NewNormalError(NormalSubcategoryInvoice, 68, http.StatusBadRequest, "response note must be at most 500 characters")
```

- [ ] **Step 4: Service**

Create `pkg/services/estimate_public.go`:

```go
package services

import (
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoicepdf"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// RespondToEstimateByPublicToken records the client's answer from the public link. Only a sent estimate that is still valid
// (today on or before its valid-until date) can be answered; a decline may carry a plain-text note of at most 500 characters.
// An empty, unknown or disabled token is a plain not-found. The owner is emailed afterwards; a mail failure is only logged.
func (s *InvoiceService) RespondToEstimateByPublicToken(c core.Context, token string, status string, note string, today string) error {
	if status != models.ESTIMATE_STATUS_ACCEPTED && status != models.ESTIMATE_STATUS_DECLINED {
		return errs.ErrEstimateStatusInvalid
	}

	note = strings.TrimSpace(note)

	if utf8.RuneCountInString(note) > models.EstimateResponseNoteMaxLength {
		return errs.ErrEstimateResponseNoteTooLong
	}

	if status == models.ESTIMATE_STATUS_ACCEPTED {
		note = ""
	}

	if len(token) != models.InvoicePublicTokenLength {
		return errs.ErrInvoiceNotFound
	}

	// ponytail: a public link carries no uid, so the lookup goes to the first user-data store, as GetInvoiceByPublicToken does
	estimate := &models.Invoice{}
	has, err := s.UserDataDB(0).NewSession(c).Where("public_token=? AND deleted=?", token, false).Get(estimate)

	if err != nil {
		return err
	} else if !has {
		return errs.ErrInvoiceNotFound
	}

	if estimate.Kind != models.INVOICE_KIND_ESTIMATE || estimate.EstimateStatus != models.ESTIMATE_STATUS_SENT || estimate.DueDate < today {
		return errs.ErrEstimateCannotBeAnswered
	}

	recorded, err := s.recordEstimateClientResponse(c, estimate, token, status, note)

	if err != nil {
		return err
	} else if !recorded {
		return errs.ErrEstimateCannotBeAnswered
	}

	s.notifyEstimateResponse(c, estimate)
	return nil
}

// recordEstimateClientResponse writes the answer only while the stored estimate is still sent and its link unchanged, so of two
// concurrent clicks exactly one is recorded; it reports whether this one was. The estimate is updated in memory when recorded.
func (s *InvoiceService) recordEstimateClientResponse(c core.Context, estimate *models.Invoice, token string, status string, note string) (bool, error) {
	now := time.Now().Unix()
	recorded := false

	err := s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=? AND public_token=? AND estimate_status=?",
				estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.InvoiceId, token, models.ESTIMATE_STATUS_SENT).
			Update(&models.Invoice{EstimateStatus: status, RespondedUnixTime: now, RespondedByClient: true, ResponseNote: note, UpdatedUnixTime: now})

		recorded = updatedRows > 0
		return err
	})

	if err != nil || !recorded {
		return false, err
	}

	estimate.EstimateStatus = status
	estimate.RespondedUnixTime = now
	estimate.RespondedByClient = true
	estimate.ResponseNote = note

	return true, nil
}

// estimateResponseEmail is the owner notification: "<customer> accepted Estimate <number> (<total>)" (or declined, with the
// client's note as plain text) and a link that opens the estimate in the app
func estimateResponseEmail(estimate *models.Invoice, rootUrl string) (string, string) {
	verb := "accepted"

	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED {
		verb = "declined"
	}

	customer := strings.NewReplacer("\r", " ", "\n", " ").Replace(estimate.BillToName)
	subject := customer + " " + verb + " Estimate " + estimate.Number + " (" + invoicepdf.FormatAmount(estimate.Total, estimate.Currency) + ")"
	link := rootUrl + "desktop#/invoice/list?invoiceId=" + utils.Int64ToString(estimate.InvoiceId)

	var body strings.Builder
	body.WriteString(`<p>` + html.EscapeString(subject) + `</p>`)

	if estimate.ResponseNote != "" {
		body.WriteString(`<p>Note from the client:<br/>` + strings.ReplaceAll(html.EscapeString(estimate.ResponseNote), "\n", "<br/>") + `</p>`)
	}

	body.WriteString(`<p><a href="` + html.EscapeString(link) + `">Open the estimate</a></p>`)

	return subject, body.String()
}

// notifyEstimateResponse emails the owner's account address about a client's answer; SMTP off or a failed send is only logged
func (s *InvoiceService) notifyEstimateResponse(c core.Context, estimate *models.Invoice) {
	if !s.CurrentConfig().EnableSMTP {
		log.Infof(c, "[estimate_public.notifyEstimateResponse] SMTP is off, the answer to estimate \"id:%d\" of user \"uid:%d\" was not emailed", estimate.InvoiceId, estimate.Uid)
		return
	}

	user, err := Users.GetUserById(c, estimate.Uid)

	if err != nil || user.Email == "" {
		log.Warnf(c, "[estimate_public.notifyEstimateResponse] no account email for user \"uid:%d\", the answer to estimate \"id:%d\" was not emailed", estimate.Uid, estimate.InvoiceId)
		return
	}

	subject, body := estimateResponseEmail(estimate, s.CurrentConfig().RootUrl)

	if err := s.SendMail(&mail.MailMessage{To: user.Email, Subject: subject, Body: body}); err != nil {
		log.Warnf(c, "[estimate_public.notifyEstimateResponse] failed to email the answer to estimate \"id:%d\" of user \"uid:%d\", because %s", estimate.InvoiceId, estimate.Uid, err.Error())
	}
}
```

- [ ] **Step 5: API and routes**

Create `pkg/api/estimate_public.go`:

```go
package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// EstimatePublicAcceptHandler records the client's acceptance through a public link; no login, so only true is returned
func (a *InvoicesApi) EstimatePublicAcceptHandler(c *core.WebContext) (any, *errs.Error) {
	return a.estimatePublicRespond(c, "EstimatePublicAcceptHandler", models.ESTIMATE_STATUS_ACCEPTED)
}

// EstimatePublicDeclineHandler records the client's decline (with an optional note) through a public link
func (a *InvoicesApi) EstimatePublicDeclineHandler(c *core.WebContext) (any, *errs.Error) {
	return a.estimatePublicRespond(c, "EstimatePublicDeclineHandler", models.ESTIMATE_STATUS_DECLINED)
}

func (a *InvoicesApi) estimatePublicRespond(c *core.WebContext, name string, status string) (any, *errs.Error) {
	var req models.EstimatePublicRespondRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.%s] parse request failed, because %s", name, err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if err := a.invoices.RespondToEstimateByPublicToken(c, req.Token, status, req.Note, time.Now().Format("2006-01-02")); err != nil {
		// the token is a secret: never log it
		log.Warnf(c, "[estimates.%s] failed to answer an estimate by public token, because %s", name, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}
```

In `cmd/webserver.go` inside the `publicApiRoute` block, after `/invoices/pdf`:

```go
		publicApiRoute.POST("/estimates/accept.json", bindApi(api.Invoices.EstimatePublicAcceptHandler, config))
		publicApiRoute.POST("/estimates/decline.json", bindApi(api.Invoices.EstimatePublicDeclineHandler, config))
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 7: Commit**

```bash
git add pkg/models/estimate.go pkg/errs/invoice.go pkg/services/estimate_public.go pkg/services/estimate_public_test.go pkg/api/estimate_public.go pkg/api/estimates_test.go cmd/webserver.go
git commit --no-verify -m "estimates: client accepts or declines through the public link and the owner is emailed"
```

---

### Task 6: PDF, public view and email for estimates; sending moves draft to sent (backend)

**Files:**
- Modify: `pkg/invoicepdf/format.go` (`DocumentTitle`, `DocumentFileName`, `milestoneShare`)
- Modify: `pkg/invoicepdf/render.go` (`Render`, new `RenderWithMilestones`, `drawBanner`, `metaRows`, `drawParties`, `totalsRows`, `paymentTermsRows`, `drawPaymentTerms`)
- Modify: `pkg/invoiceemail/render.go` (`document_type` variable, estimate preset, `EstimateTemplates`)
- Modify: `pkg/models/estimate.go` (`PublicEstimateMilestoneResponse`, `ToPublicEstimateMilestoneResponses`)
- Modify: `pkg/models/invoice.go` (`PublicInvoiceResponse`, `ToPublicInvoiceResponse`)
- Modify: `pkg/services/invoices.go` (`invoiceEmailSubject`)
- Modify: `pkg/services/invoice_email.go` (`invoiceEmailValues`, `renderInvoiceEmail`, `SendInvoiceWithContent`)
- Modify: `pkg/api/invoices.go` (`InvoicePublicViewHandler`, `InvoicePdfHandler`, `InvoicePublicPdfHandler`)
- Test: modify `pkg/invoicepdf/invoicepdf_test.go`, `pkg/invoiceemail/render_test.go`, `pkg/services/invoice_email_test.go:71`, `pkg/api/estimates_test.go`; create `pkg/services/estimate_email_test.go`

**Interfaces:**
- Consumes (Tasks 1–5): `GetEstimateMilestones`, status constants, `createTestEstimate`, `testEstimateLines`, `testEstimateMilestones`, `withCapturingMailer`, `SetEstimateResponse`.
- Produces:
  - `invoicepdf.DocumentTitle(invoice) string` ("Estimate" / "Invoice"), `invoicepdf.DocumentFileName(invoice) string`, `invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, verifyUrl) ([]byte, error)`; package-private `metaRow`, `metaRows(invoice, received)`, `paymentTermRow`, `paymentTermsRows(invoice, milestones)`, `milestoneShare(amount, basisPoints)`.
  - `invoiceemail.DefaultEstimateSubjectTemplate`, `invoiceemail.DefaultEstimateBodyTemplate`, `invoiceemail.EstimateTemplates(subject, body) (string, string)`; catalog variable `document_type` ("Document Type").
  - `models.PublicEstimateMilestoneResponse{Label, Percent}`, `models.ToPublicEstimateMilestoneResponses(milestones)`; `PublicInvoiceResponse.Kind` (only `"estimate"`, omitted for invoices), `.EstimateStatus`, `.RespondedUnixTime`, `.Milestones`.
  - `invoiceEmailSubject(kind, number, businessName string) string`.
  - Sending a draft estimate stores `estimate_status = 'sent'`.

- [ ] **Step 1: Write the failing tests**

Append to `pkg/invoicepdf/invoicepdf_test.go`:

```go
func sampleEstimate() (*models.Invoice, []*models.InvoiceLine, []*models.EstimateMilestone) {
	estimate, lines := sampleInvoice()
	estimate.Kind = models.INVOICE_KIND_ESTIMATE
	estimate.Number = "EST/2026-27/01"
	estimate.DueDate = "2026-05-01"
	estimate.Total = 235999

	return estimate, lines, []*models.EstimateMilestone{{Label: "Advance", Percent: 3333}, {Label: "Design sign-off", Percent: 3333}, {Label: "Go-live", Percent: 3334}}
}

func TestEstimateDocumentTitleMetaAndTotals(t *testing.T) {
	estimate, _, _ := sampleEstimate()
	assert.Equal(t, "Estimate", DocumentTitle(estimate))
	assert.Equal(t, "Estimate_EST_2026-27_01_2026-05-01.pdf", DocumentFileName(estimate))
	assert.Equal(t, []metaRow{
		{"Estimate Number:", "EST/2026-27/01", false},
		{"Estimate Date:", "April 1, 2026", false},
		{"Valid Until:", "May 1, 2026", false},
		{"Total (INR):", "₹2,359.99", true},
	}, metaRows(estimate, 0))
	// no Payments received / Amount Due on an estimate
	assert.Equal(t, []totalsRow{{"Total (INR):", "₹2,359.99", true}}, totalsRows(estimate, 0))

	invoice, _ := sampleInvoice()
	assert.Equal(t, "Invoice", DocumentTitle(invoice))
	assert.Equal(t, "Invoice_CA_2026-27_01_2026-04-30.pdf", DocumentFileName(invoice))
	assert.Equal(t, "Payment Due:", metaRows(invoice, 0)[2].label)
	rows := totalsRows(invoice, 10000000)
	assert.Equal(t, "Payments received:", rows[len(rows)-2].label)
}

func TestPaymentTermsRowsAddUpToTheEstimateTotal(t *testing.T) {
	estimate, _, milestones := sampleEstimate()

	assert.Equal(t, []paymentTermRow{
		{"Advance", "33.33%", "₹786.58"},
		{"Design sign-off", "33.33%", "₹786.58"},
		{"Go-live", "33.34%", "₹786.83"}, // 2,359.99 − 786.58 − 786.58
	}, paymentTermsRows(estimate, milestones))
	assert.Empty(t, paymentTermsRows(estimate, nil))
}

func TestRenderEstimateWithPaymentTerms(t *testing.T) {
	estimate, lines, milestones := sampleEstimate()

	pdf, err := RenderWithMilestones(sampleProfile(), estimate, lines, milestones, 0, "https://books.codeation.io/i/abc")
	assert.Nil(t, err)
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF")))

	if dir := os.Getenv("INVOICE_PDF_SAMPLE_DIR"); dir != "" {
		assert.Nil(t, os.WriteFile(filepath.Join(dir, "estimate-sample.pdf"), pdf, 0644))
	}
}
```

In `pkg/invoiceemail/render_test.go`: add `"document_type": "Invoice"` to the map in `testValues()`, add `"document_type"` at the end of the key list in `TestRenderExpandsCatalogAsEscapedNonRecursiveText`, and change `require.Len(t, result.Variables, 15)` to `16`. Append:

```go
// Swapping a template the user wrote, or missing the preset after an editor round trip, breaks this test.
func TestEstimateTemplatesSwapOnlyTheBuiltInPreset(t *testing.T) {
	subject, body := EstimateTemplates(DefaultSubjectTemplate, DefaultBodyTemplate)
	require.Equal(t, "Estimate {{invoice_number}} from {{business_name}}", subject)
	require.Contains(t, body, "sent you an estimate")
	require.Contains(t, body, "Valid until")
	require.Contains(t, body, "View Estimate")
	require.NotContains(t, body, "amount_due")

	bareSubject, bareBody := PresetWithoutBusinessName()
	subject, body = EstimateTemplates(bareSubject, bareBody)
	require.Equal(t, "Estimate {{invoice_number}}", subject)
	require.Contains(t, body, "You have received an estimate")

	sanitized, err := SanitizeTemplate(DefaultBodyTemplate)
	require.NoError(t, err)
	_, body = EstimateTemplates(DefaultSubjectTemplate, sanitized)
	require.Contains(t, body, "sent you an estimate")

	subject, body = EstimateTemplates("Your quote {{invoice_number}}", "<p>Custom</p>")
	require.Equal(t, "Your quote {{invoice_number}}", subject)
	require.Equal(t, "<p>Custom</p>", body)

	rendered, err := Render("{{document_type}} {{invoice_number}}", "<p>x</p>", map[string]string{"document_type": "Estimate", "invoice_number": "EST/2026-27/01"})
	require.NoError(t, err)
	require.Equal(t, "Estimate EST/2026-27/01", rendered.Subject)
}
```

In `pkg/services/invoice_email_test.go:71` change `require.Len(t, first.Variables, 15)` to `16`.

Create `pkg/services/estimate_email_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestSendingAnEstimateMarksItSentWithEstimateWording(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	mailer := withCapturingMailer(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())

	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, estimate.InvoiceId, "accounts@qubertech.com", "", nil, nil))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
	assert.Greater(t, reloaded.SentUnixTime, int64(0))
	assert.NotEqual(t, "", reloaded.PublicToken)
	assert.Equal(t, "Estimate EST/2026-27/01", mailer.sent[0].Subject) // the profile has no business name
	assert.Contains(t, mailer.sent[0].Body, "View Estimate")
	assert.Equal(t, "Estimate_EST_2026-27_01_2099-12-31.pdf", mailer.sent[0].Attachments[0].Name)

	// sending again after the client answered never moves the status back
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, estimate.InvoiceId, "accounts@qubertech.com", "", nil, nil))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)

	// an invoice keeps its own wording
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	assert.Nil(t, Invoices.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "accounts@qubertech.com", "", nil, nil))
	assert.Equal(t, "Invoice INV/2026-27/01", mailer.sent[2].Subject)
}
```

Append to `pkg/api/estimates_test.go`, and make its import block:

```go
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)
```

```go
func TestPublicEstimateResponseCarriesStatusAndPaymentTerms(t *testing.T) {
	estimate := &models.Invoice{InvoiceId: 1234, Kind: models.INVOICE_KIND_ESTIMATE, Number: "EST/2026-27/01", Currency: "INR", Total: 235999,
		EstimateStatus: models.ESTIMATE_STATUS_SENT, ResponseNote: "private"}
	resp := estimate.ToPublicInvoiceResponse(nil, 0, &models.InvoiceProfile{})
	resp.Milestones = models.ToPublicEstimateMilestoneResponses([]*models.EstimateMilestone{{MilestoneId: 7, EstimateId: 1234, Label: "Advance", Percent: 3000, InvoiceId: 9}})

	data, err := json.Marshal(resp)
	assert.Nil(t, err)

	var body map[string]any
	assert.Nil(t, json.Unmarshal(data, &body))
	assert.Equal(t, "estimate", body["kind"])
	assert.Equal(t, "sent", body["estimateStatus"])
	assert.Equal(t, map[string]any{"label": "Advance", "percent": float64(3000)}, body["milestones"].([]any)[0])
	assert.NotContains(t, string(data), "1234")
	assert.NotContains(t, string(data), "private")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/invoicepdf/ ./pkg/invoiceemail/ ./pkg/services/ ./pkg/api/ -count=1`
Expected: compile FAIL (`undefined: DocumentTitle`, `undefined: EstimateTemplates`, `undefined: models.ToPublicEstimateMilestoneResponses`).

- [ ] **Step 3: PDF**

Append to `pkg/invoicepdf/format.go`:

```go
// DocumentTitle is the word a row is printed under: "Estimate" for an estimate, otherwise "Invoice"
func DocumentTitle(invoice *models.Invoice) string {
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return "Estimate"
	}

	return "Invoice"
}

// DocumentFileName is FileName led by the document title, e.g. Estimate_EST_2026-27_01_<valid until>.pdf;
// mirrors getInvoicePdfFileName(number, dueDate, kind) in src/lib/invoice.ts
func DocumentFileName(invoice *models.Invoice) string {
	return DocumentTitle(invoice) + "_" + fileNameUnsafeChars.ReplaceAllString(invoice.Number, "_") + "_" + invoice.DueDate + ".pdf"
}

// milestoneShare is amount × basisPoints / 10000 rounded half up, mirroring getEstimateShare in src/lib/estimate.ts
func milestoneShare(amount int64, basisPoints int32) int64 {
	whole, rest := amount/models.EstimateMilestonePercentTotal, amount%models.EstimateMilestonePercentTotal
	return whole*int64(basisPoints) + (rest*int64(basisPoints)+models.EstimateMilestonePercentTotal/2)/models.EstimateMilestonePercentTotal
}
```

In `pkg/invoicepdf/render.go`:

Replace the `Render` signature line and comment with the wrapper plus the new entry point (the old body becomes the body of `RenderWithMilestones`):

```go
// Render draws an invoice (or an estimate without payment terms) on A4 pages and returns the PDF bytes
func Render(profile *models.InvoiceProfile, invoice *models.Invoice, lines []*models.InvoiceLine, received int64, verifyUrl string) ([]byte, error) {
	return RenderWithMilestones(profile, invoice, lines, nil, received, verifyUrl)
}

// RenderWithMilestones draws the document on A4 pages; an estimate's milestones print as a Payment Terms table under the totals.
// received is the sum of linked payments; verifyUrl, when not empty, is printed in the footer of every page.
func RenderWithMilestones(profile *models.InvoiceProfile, invoice *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone, received int64, verifyUrl string) ([]byte, error) {
```

Inside it:
- replace `if invoiceNeedsFallbackFont(profile, invoice, lines, verifyUrl) {` with

```go
	fontLines := make([]*models.InvoiceLine, 0, len(lines)+len(milestones))
	fontLines = append(fontLines, lines...)

	for _, milestone := range milestones {
		fontLines = append(fontLines, &models.InvoiceLine{ItemName: milestone.Label})
	}

	if invoiceNeedsFallbackFont(profile, invoice, fontLines, verifyUrl) {
```

- footer: `pdf.CellFormat(0, 4, "Verify this "+strings.ToLower(DocumentTitle(invoice))+" at "+verifyUrl, "", 0, "C", false, 0, verifyUrl)`
- `y = r.drawBanner(y + 10)` becomes `y = r.drawBanner(y+10, DocumentTitle(invoice))`
- replace `r.drawNotes(invoice, y+14)` with

```go
	if len(milestones) > 0 {
		y = r.drawPaymentTerms(invoice, milestones, y+10)
	}

	r.drawNotes(invoice, y+14)
```

Change `drawBanner` to take the title: signature `func (r *renderer) drawBanner(y float64, title string) float64`, comment `// drawBanner prints the title ("Invoice" / "Estimate") in a double-bordered box between two double rules; it returns the y below it`, and use `title` in `r.pdf.GetStringWidth(title)` and `r.pdf.CellFormat(boxWidth, boxHeight, title, "", 0, "C", false, 0, "")`.

Add above `drawParties`:

```go
// metaRow is one row of the table beside BILL TO
type metaRow struct {
	label     string
	value     string
	highlight bool
}

// metaRows mirrors the meta table of InvoiceDocumentContent.vue: an estimate shows its valid-until date and total, never an amount due
func metaRows(invoice *models.Invoice, received int64) []metaRow {
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return []metaRow{
			{"Estimate Number:", invoice.Number, false},
			{"Estimate Date:", formatDate(invoice.InvoiceDate), false},
			{"Valid Until:", formatDate(invoice.DueDate), false},
			{"Total (" + invoice.Currency + "):", formatAmount(invoice.Total, invoice.Currency), true},
		}
	}

	return []metaRow{
		{"Invoice Number:", invoice.Number, false},
		{"Invoice Date:", formatDate(invoice.InvoiceDate), false},
		{"Payment Due:", formatDate(invoice.DueDate), false},
		{"Amount Due (" + invoice.Currency + "):", formatAmount(balanceDue(invoice, received), invoice.Currency), true},
	}
}
```

and in `drawParties` replace the whole `rows := []struct{...}{...}` literal with `rows := metaRows(invoice, received)` (the loop below reads the same `label`, `value`, `highlight` fields).

In `totalsRows`, insert before `rows = append(rows, totalsRow{"Total:", ...})`:

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		return append(rows, totalsRow{"Total (" + invoice.Currency + "):", formatAmount(invoice.Total, invoice.Currency), true})
	}
```

Append to `render.go`:

```go
// paymentTermRow is one milestone of an estimate's Payment Terms table
type paymentTermRow struct {
	label   string
	percent string
	amount  string
}

// paymentTermsRows mirrors the Payment Terms table of InvoiceDocumentContent.vue: each milestone's share of the estimate total
// (incl. tax), rounded half up, the last taking what is left so the amounts add up to the total
func paymentTermsRows(invoice *models.Invoice, milestones []*models.EstimateMilestone) []paymentTermRow {
	rows := make([]paymentTermRow, len(milestones))
	allocated := int64(0)

	for idx, milestone := range milestones {
		amount := milestoneShare(invoice.Total, milestone.Percent)

		if idx == len(milestones)-1 {
			amount = invoice.Total - allocated
		}

		allocated += amount
		rows[idx] = paymentTermRow{milestone.Label, formatTaxRate(float64(milestone.Percent)), formatAmount(amount, invoice.Currency)}
	}

	return rows
}

// drawPaymentTerms prints the Payment Terms table (milestone, %, amount) under the totals; it returns the y below it
func (r *renderer) drawPaymentTerms(invoice *models.Invoice, milestones []*models.EstimateMilestone, y float64) float64 {
	const (
		padding      = 2.0 // 8px
		percentWidth = 24.0
		amountWidth  = 40.0
	)

	rows := paymentTermsRows(invoice, milestones)
	labelWidth := r.width - percentWidth - amountWidth

	y = r.ensureSpace(y, 3*(lineHeight+2*padding))
	r.font(true, baseFontSize, colorBlack)
	r.cell(r.left, y, r.width, "Payment Terms", "L")
	y += lineHeight + padding
	r.cell(r.left, y+padding, labelWidth, "Milestone", "L")
	r.cell(r.left+labelWidth, y+padding, percentWidth, "%", "R")
	r.cell(r.left+labelWidth+percentWidth, y+padding, amountWidth, "Amount", "R")
	y += lineHeight + 2*padding
	r.dottedLine(y)

	for _, row := range rows {
		r.font(false, baseFontSize, colorBlack)
		height := float64(len(r.wrap(row.label, labelWidth-3)))*lineHeight + 2*padding
		y = r.ensureSpace(y, height)
		r.column(r.left, y+padding, labelWidth-3, row.label, "L")
		r.cell(r.left+labelWidth, y+padding, percentWidth, row.percent, "R")
		r.cell(r.left+labelWidth+percentWidth, y+padding, amountWidth, row.amount, "R")
		y += height
		r.dottedLine(y)
	}

	return y
}
```

- [ ] **Step 4: Email templates**

In `pkg/invoiceemail/render.go` add after `DefaultBodyTemplate`:

```go
// DefaultEstimateSubjectTemplate and DefaultEstimateBodyTemplate are the built-in preset worded for an estimate
const DefaultEstimateSubjectTemplate = "Estimate {{invoice_number}} from {{business_name}}"

var DefaultEstimateBodyTemplate = strings.NewReplacer(
	"sent you an invoice", "sent you an estimate",
	"Please find your invoice attached.", "Please find your estimate attached. You can accept or decline it online.",
	`Amount due: <span data-invoice-variable="amount_due">{{amount_due}}</span>`, `Estimate total: <span data-invoice-variable="invoice_total">{{invoice_total}}</span>`,
	`<p>Due <span data-invoice-variable="due_date">`, `<p>Valid until <span data-invoice-variable="due_date">`,
	">View Invoice</div>", ">View Estimate</div>",
).Replace(DefaultBodyTemplate)
```

Append `{Key: "document_type", Label: "Document Type"}` as the last entry of `catalog`, and add:

```go
// EstimateTemplates rewords the built-in invoice preset (with or without the business name) for an estimate. Bodies are compared in
// their sanitized form, so the preset still matches after a round trip through the editor; a template the user wrote is returned unchanged.
func EstimateTemplates(subject, body string) (string, string) {
	bareSubject, bareBody := PresetWithoutBusinessName()

	switch strings.TrimSpace(subject) {
	case DefaultSubjectTemplate:
		subject = DefaultEstimateSubjectTemplate
	case bareSubject:
		subject = strings.Replace(DefaultEstimateSubjectTemplate, " from {{business_name}}", "", 1)
	}

	switch canonicalTemplate(body) {
	case canonicalTemplate(DefaultBodyTemplate):
		body = DefaultEstimateBodyTemplate
	case canonicalTemplate(bareBody):
		body = strings.Replace(DefaultEstimateBodyTemplate, `<span data-invoice-variable="business_name">{{business_name}}</span> sent you an estimate`, "You have received an estimate", 1)
	}

	return subject, body
}

// canonicalTemplate is a body as SanitizeTemplate stores it; an unparsable body is compared as it is
func canonicalTemplate(body string) string {
	sanitized, err := SanitizeTemplate(body)

	if err != nil {
		return body
	}

	return sanitized
}
```

- [ ] **Step 5: Public view model**

Append to `pkg/models/estimate.go`:

```go
// PublicEstimateMilestoneResponse is a payment term as the client sees it: no ids, no invoices
type PublicEstimateMilestoneResponse struct {
	Label   string `json:"label"`
	Percent int32  `json:"percent"`
}

// ToPublicEstimateMilestoneResponses returns the milestones served through a public estimate link
func ToPublicEstimateMilestoneResponses(milestones []*EstimateMilestone) []*PublicEstimateMilestoneResponse {
	resps := make([]*PublicEstimateMilestoneResponse, len(milestones))

	for idx, milestone := range milestones {
		resps[idx] = &PublicEstimateMilestoneResponse{Label: milestone.Label, Percent: milestone.Percent}
	}

	return resps
}
```

In `pkg/models/invoice.go` add to `PublicInvoiceResponse`:

```go
	Kind              string                             `json:"kind,omitempty"` // "estimate" only; an invoice's link stays as it was
	EstimateStatus    string                             `json:"estimateStatus,omitempty"`
	RespondedUnixTime int64                              `json:"respondedUnixTime,omitempty"`
	Milestones        []*PublicEstimateMilestoneResponse `json:"milestones,omitempty"`
```

and in `ToPublicInvoiceResponse` before `for idx, line := range lines {`:

```go
	if i.Kind == INVOICE_KIND_ESTIMATE {
		resp.Kind = i.Kind
		resp.EstimateStatus = i.EstimateStatus
		resp.RespondedUnixTime = i.RespondedUnixTime
	}
```

- [ ] **Step 6: Services and API**

In `pkg/services/invoices.go` replace `invoiceEmailSubject`:

```go
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
```

In `pkg/services/invoice_email.go`:
- `invoiceEmailValues`: add `"document_type": invoicepdf.DocumentTitle(invoice),` to the map.
- `renderInvoiceEmail`: right before `rendered, err := invoiceemail.Render(...)` add

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		subjectTemplate, bodyTemplate = invoiceemail.EstimateTemplates(subjectTemplate, bodyTemplate)
	}
```

- `SendInvoiceWithContent`: change `subject = invoiceEmailSubject(invoice.Number, profile.BusinessName)` to `subject = invoiceEmailSubject(invoice.Kind, invoice.Number, profile.BusinessName)`; replace `pdf, err := invoicepdf.Render(profile, invoice, lines, received, publicURL)` with

```go
	var milestones []*models.EstimateMilestone
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		if milestones, err = s.GetEstimateMilestones(c, uid, invoiceID); err != nil {
			return err
		}
	}
	pdf, err := invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, publicURL)
```

change the attachment name to `invoicepdf.DocumentFileName(invoice)`, and replace the final transaction with

```go
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Insert(sent); err != nil {
			return err
		}
		if _, err := sess.Cols("sent_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceID).Update(&models.Invoice{SentUnixTime: now, UpdatedUnixTime: now}); err != nil {
			return err
		}
		// emailing a draft estimate sends it; an answered one keeps its answer
		_, err := sess.Cols("estimate_status").Where("uid=? AND deleted=? AND invoice_id=? AND kind=? AND estimate_status=?", uid, false, invoiceID, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_DRAFT).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT})
		return err
	})
```

In `pkg/api/invoices.go`:
- `InvoicePublicViewHandler`: replace `return invoice.ToPublicInvoiceResponse(lines, received, profile), nil` with

```go
	resp := invoice.ToPublicInvoiceResponse(lines, received, profile)

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		milestones, err := a.invoices.GetEstimateMilestones(c, invoice.Uid, invoice.InvoiceId)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePublicViewHandler] failed to get milestones of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}

		resp.Milestones = models.ToPublicEstimateMilestoneResponses(milestones)
	}

	return resp, nil
```

- `InvoicePdfHandler` and `InvoicePublicPdfHandler`: before the `invoicepdf.Render(...)` call add

```go
	var milestones []*models.EstimateMilestone

	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		milestones, err = a.invoices.GetEstimateMilestones(c, invoice.Uid, invoice.InvoiceId)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePdfHandler] failed to get milestones of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, "", errs.Or(err, errs.ErrOperationFailed)
		}
	}
```

(use `InvoicePublicPdfHandler` in the log prefix of the public one), change the render call to `invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, a.invoices.InvoicePublicUrl(invoice.PublicToken))`, and return `invoicepdf.DocumentFileName(invoice)` as the file name.

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./pkg/invoicepdf/ ./pkg/invoiceemail/ ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS and a clean build. Optionally `INVOICE_PDF_SAMPLE_DIR=$TMPDIR go test ./pkg/invoicepdf/ -run TestRenderEstimate -count=1` and open `estimate-sample.pdf`: banner "Estimate", Valid Until, Total row, Payment Terms table.

- [ ] **Step 8: Commit**

```bash
git add pkg/invoicepdf pkg/invoiceemail pkg/models/estimate.go pkg/models/invoice.go pkg/services/invoices.go pkg/services/invoice_email.go pkg/services/invoice_email_test.go pkg/services/estimate_email_test.go pkg/api/invoices.go pkg/api/estimates_test.go
git commit --no-verify -m "estimates: estimate PDF, public view and email; sending a draft estimate marks it sent"
```

---

### Task 7: Customer timeline events and the accepted-estimates list (backend)

**Files:**
- Modify: `pkg/models/invoice.go` (timeline kinds, `CustomerTimelineItemResponse`)
- Modify: `pkg/services/customer_overview.go` (`GetCustomerOverview`, new `estimateTimelineItems`)
- Create: `pkg/services/estimate_attention.go`
- Modify: `pkg/api/estimates.go` (`EstimateToInvoiceListHandler`)
- Modify: `cmd/webserver.go`
- Test: create `pkg/services/estimate_overview_test.go`

**Interfaces:**
- Consumes (Tasks 1–4): `GetEstimateProgress`, `SetEstimateProgress`, `SetEstimateResponse`, `CreateInvoiceFromEstimate`, `newTestInvoiceDraft`, `createTestEstimate`, `testEstimateLines`.
- Produces:
  - `models.CUSTOMER_TIMELINE_KIND_ESTIMATE = "estimate"`, `CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED = "estimate_accepted"`, `CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED = "estimate_declined"`; `CustomerTimelineItemResponse.EstimateNumber` (`estimateNumber`, on an invoice made from an estimate), `.ByClient` (`byClient`).
  - Products bought on the customer page come from invoices only.
  - `(*InvoiceService).GetEstimatesToInvoice(c, uid) ([]*models.Invoice, map[int64]*models.EstimateProgress, error)` — accepted, not fully invoiced, oldest answer first.
  - Route `GET /api/v1/estimates/to_invoice.json` → `[]InvoiceInfoResponse` with milestones / converted invoices; handler `EstimateToInvoiceListHandler`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_overview_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCustomerOverviewShowsEstimateEventsButNeverCountsEstimatesAsBought(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	hosting := createTestProduct(t, uid, "Hosting", 0, "")
	hostingLines := func() []*models.InvoiceLine {
		return []*models.InvoiceLine{{ItemName: "Hosting", Quantity: 100, Rate: 150000, ProductId: hosting.ProductId}}
	}
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", hostingLines(), nil)

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Empty(t, overview.Products) // quoted is not bought
	assert.Equal(t, 1, len(overview.Timeline))
	assert.Equal(t, models.CUSTOMER_TIMELINE_KIND_ESTIMATE, overview.Timeline[0].Kind)
	assert.Equal(t, "EST/2026-27/01", overview.Timeline[0].InvoiceNumber)

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	invoice := newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01")
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, invoice, hostingLines(), estimate.InvoiceId, 0))

	overview, err = Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(overview.Products))
	assert.Equal(t, 1, overview.Products[0].TimesBought)

	items := make(map[string]*models.CustomerTimelineItemResponse)

	for _, item := range overview.Timeline {
		items[item.Kind] = item
	}

	assert.NotNil(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE])
	assert.NotNil(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED])
	assert.False(t, items[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED].ByClient)
	assert.Equal(t, "INV/2026-27/01", items[models.CUSTOMER_TIMELINE_KIND_INVOICE].InvoiceNumber)
	assert.Equal(t, "EST/2026-27/01", items[models.CUSTOMER_TIMELINE_KIND_INVOICE].EstimateNumber)
}

func TestAcceptedEstimatesNotFullyInvoicedNeedAttention(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	whole := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	staged := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(),
		[]*models.EstimateMilestone{{Label: "Advance", Percent: 5000}, {Label: "Delivery", Percent: 5000}})
	createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/03", testEstimateLines(), nil) // never accepted: never listed

	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, whole.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, staged.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	estimates, progress, err := Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(estimates))
	assert.NotNil(t, progress[staged.InvoiceId])

	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01"), testEstimateLines(), whole.InvoiceId, 0))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates))
	assert.Equal(t, staged.InvoiceId, estimates[0].InvoiceId)

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, staged.InvoiceId)
	assert.Nil(t, err)
	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/02"),
		[]*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 1}}, staged.InvoiceId, milestones[0].MilestoneId))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(estimates)) // Delivery is still open

	assert.Nil(t, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/03"),
		[]*models.InvoiceLine{{ItemName: "Delivery", Quantity: 100, Rate: 1}}, staged.InvoiceId, milestones[1].MilestoneId))
	estimates, _, err = Invoices.GetEstimatesToInvoice(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, estimates)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestCustomerOverviewShowsEstimate|TestAcceptedEstimatesNotFullyInvoiced' -count=1`
Expected: compile FAIL (`undefined: models.CUSTOMER_TIMELINE_KIND_ESTIMATE`, `Invoices.GetEstimatesToInvoice undefined`).

- [ ] **Step 3: Models**

In `pkg/models/invoice.go` extend the timeline kinds:

```go
// Customer timeline entry kinds
const (
	CUSTOMER_TIMELINE_KIND_INVOICE           = "invoice"
	CUSTOMER_TIMELINE_KIND_EMAIL             = "email"
	CUSTOMER_TIMELINE_KIND_PAYMENT           = "payment"
	CUSTOMER_TIMELINE_KIND_ESTIMATE          = "estimate"
	CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED = "estimate_accepted"
	CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED = "estimate_declined"
)
```

and add to `CustomerTimelineItemResponse`:

```go
	EstimateNumber string `json:"estimateNumber,omitempty"` // an invoice made from this estimate
	ByClient       bool   `json:"byClient,omitempty"`       // an estimate answered through the public link
```

(for a declined estimate `Text` carries the client's note).

- [ ] **Step 4: Services**

In `pkg/services/customer_overview.go` replace `GetCustomerOverview` and add `estimateTimelineItems`:

```go
// GetCustomerOverview returns the products a customer bought (from invoices only) and their timeline: invoices, estimates with their
// answers, emails and payments, newest first
func (s *InvoiceService) GetCustomerOverview(c core.Context, uid int64, customerId int64) (*models.CustomerOverviewResponse, error) {
	if _, err := s.GetCustomer(c, uid, customerId); err != nil {
		return nil, err
	}

	var rows []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Find(&rows)

	if err != nil {
		return nil, err
	}

	overview := &models.CustomerOverviewResponse{Products: []*models.CustomerProductResponse{}, Timeline: []*models.CustomerTimelineItemResponse{}}

	if len(rows) < 1 {
		return overview, nil
	}

	rowIds := make([]int64, len(rows))
	invoiceIds := make([]int64, 0, len(rows)) // invoices only: an estimate is never bought or paid
	invoiceMap := make(map[int64]*models.Invoice, len(rows))

	for idx, row := range rows {
		rowIds[idx] = row.InvoiceId
		invoiceMap[row.InvoiceId] = row
	}

	for _, row := range rows {
		if row.Kind == models.INVOICE_KIND_ESTIMATE {
			overview.Timeline = append(overview.Timeline, estimateTimelineItems(row)...)
			continue
		}

		invoiceIds = append(invoiceIds, row.InvoiceId)
		item := &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_INVOICE, UnixTime: row.CreatedUnixTime,
			InvoiceId: utils.Int64ToString(row.InvoiceId), InvoiceNumber: row.Number, Amount: row.Total, Currency: row.Currency, Renewal: row.RecurringId > 0}

		if estimate := invoiceMap[row.EstimateId]; row.EstimateId > 0 && estimate != nil {
			item.EstimateNumber = estimate.Number
		}

		overview.Timeline = append(overview.Timeline, item)
	}

	if len(invoiceIds) > 0 {
		if err := s.addCustomerProducts(c, uid, invoiceIds, invoiceMap, overview); err != nil {
			return nil, err
		}
	}

	var emails []*models.InvoiceEmail
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", rowIds).Omit("email_html").Find(&emails)

	if err != nil {
		return nil, err
	}

	for _, email := range emails {
		invoice := invoiceMap[email.InvoiceId]
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_EMAIL, UnixTime: email.SentUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Text: email.Subject})
	}

	if len(invoiceIds) > 0 {
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
	}

	sort.SliceStable(overview.Timeline, func(i, j int) bool { return overview.Timeline[i].UnixTime > overview.Timeline[j].UnixTime })

	if len(overview.Timeline) > models.CustomerTimelineMaxItems {
		overview.Timeline = overview.Timeline[:models.CustomerTimelineMaxItems]
	}

	return overview, nil
}

// estimateTimelineItems are an estimate's events on the customer page: created, then accepted or declined (with the client's note)
func estimateTimelineItems(estimate *models.Invoice) []*models.CustomerTimelineItemResponse {
	id := utils.Int64ToString(estimate.InvoiceId)
	items := []*models.CustomerTimelineItemResponse{{Kind: models.CUSTOMER_TIMELINE_KIND_ESTIMATE, UnixTime: estimate.CreatedUnixTime,
		InvoiceId: id, InvoiceNumber: estimate.Number, Amount: estimate.Total, Currency: estimate.Currency}}

	kind := ""

	switch estimate.EstimateStatus {
	case models.ESTIMATE_STATUS_ACCEPTED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED
	case models.ESTIMATE_STATUS_DECLINED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED
	}

	if kind != "" && estimate.RespondedUnixTime > 0 {
		items = append(items, &models.CustomerTimelineItemResponse{Kind: kind, UnixTime: estimate.RespondedUnixTime, InvoiceId: id, InvoiceNumber: estimate.Number,
			Amount: estimate.Total, Currency: estimate.Currency, Text: estimate.ResponseNote, ByClient: estimate.RespondedByClient})
	}

	return items
}
```

Create `pkg/services/estimate_attention.go`:

```go
package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// GetEstimatesToInvoice returns the accepted estimates of user that are not fully invoiced yet, oldest answer first, with their
// progress: the home card's "Estimate accepted by <customer>: Convert" rows
func (s *InvoiceService) GetEstimatesToInvoice(c core.Context, uid int64) ([]*models.Invoice, map[int64]*models.EstimateProgress, error) {
	if uid <= 0 {
		return nil, nil, errs.ErrUserIdInvalid
	}

	var accepted []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND estimate_status=?", uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_ACCEPTED).
		OrderBy("responded_unix_time asc, number asc").Find(&accepted)

	if err != nil {
		return nil, nil, err
	}

	estimateIds := make([]int64, len(accepted))

	for idx, estimate := range accepted {
		estimateIds[idx] = estimate.InvoiceId
	}

	progress, err := s.GetEstimateProgress(c, uid, estimateIds)

	if err != nil {
		return nil, nil, err
	}

	open := make([]*models.Invoice, 0, len(accepted))

	for _, estimate := range accepted {
		if !progress[estimate.InvoiceId].FullyInvoiced {
			open = append(open, estimate)
		}
	}

	return open, progress, nil
}
```

- [ ] **Step 5: API and route**

Append to `pkg/api/estimates.go`:

```go
// EstimateToInvoiceListHandler returns the accepted, not fully invoiced estimates of current user for the home card
func (a *InvoicesApi) EstimateToInvoiceListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	estimates, progress, err := a.invoices.GetEstimatesToInvoice(c, uid)

	if err != nil {
		log.Errorf(c, "[estimates.EstimateToInvoiceListHandler] failed to get estimates to invoice for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	resps := make([]*models.InvoiceInfoResponse, len(estimates))

	for idx, estimate := range estimates {
		resps[idx] = estimate.ToInvoiceInfoResponse(nil, nil)
		resps[idx].SetEstimateProgress(progress[estimate.InvoiceId])
	}

	return resps, nil
}
```

In `cmd/webserver.go` after the `/estimates/invoice_draft.json` route:

```go
			apiV1Route.GET("/estimates/to_invoice.json", bindApi(api.Invoices.EstimateToInvoiceListHandler, config))
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && go build ./...`
Expected: PASS (including the existing `customer_overview_test.go`) and a clean build.

- [ ] **Step 7: Commit**

```bash
git add pkg/models/invoice.go pkg/services/customer_overview.go pkg/services/estimate_attention.go pkg/services/estimate_overview_test.go pkg/api/estimates.go cmd/webserver.go
git commit --no-verify -m "estimates: customer timeline events and the accepted-estimates list for the home card"
```

---

### Task 8: Frontend data layer and estimate helpers

**Files:**
- Create: `src/lib/estimate.ts`, `src/lib/__tests__/estimate.test.ts`
- Modify: `src/models/invoice.ts`
- Modify: `src/lib/invoice.ts` (`getInvoicePdfFileName`, `printInvoiceDocument`)
- Modify: `src/lib/invoiceEmail.ts` (`document_type` variable)
- Modify: `src/lib/services.ts` (next to the invoice calls, ~line 1089-1170)
- Modify: `src/stores/invoice.ts`
- Modify: `src/locales/en.json` (`"error"` section and store messages)

**Interfaces:**
- Consumes (Tasks 1–7): JSON `kind`, `milestones`, `estimateStatus`, `respondedUnixTime`, `respondedByClient`, `responseNote`, `estimateId`, `estimateNumber`, `milestoneId`, `convertedInvoices`, `fullyInvoiced` on invoices; `estimateNumberPrefix`, `estimateValidDays` on the profile; `kind`, `estimateStatus`, `respondedUnixTime`, `milestones` on the public view; timeline `estimateNumber`, `byClient`; routes `v1/invoices/list.json?kind=estimate`, `v1/invoices/next_number.json?...&kind=estimate`, `v1/estimates/respond.json`, `v1/estimates/reopen.json`, `v1/estimates/invoice_draft.json`, `v1/estimates/to_invoice.json`, `public/estimates/accept.json`, `public/estimates/decline.json`; error texts of codes 60..68.
- Produces:
  - `src/lib/estimate.ts`: `ESTIMATE_MILESTONE_PERCENT_TOTAL = 10000`, `ESTIMATE_MAX_MILESTONES = 10`, `type EstimateStatus = 'draft' | 'sent' | 'accepted' | 'declined'`, `type EstimateDisplayStatus = EstimateStatus | 'expired' | 'invoiced'`, `ESTIMATE_DISPLAY_STATUS_NAMES`, `getEstimateDisplayStatus(estimate, today)`, `isEstimateClosed(estimate)`, `isEstimateLocked(estimate)`, `canAnswerEstimate(estimate, today)`, `getEstimateShare(amount, percent)`, `getMilestoneAmounts(amount, percents)`, `getMilestonePercentTotal(milestones)`, `getMilestonesProblem(milestones)`, `formatMilestonePercent(basisPoints)`, `unixTimeToTextualDate(unixTime)`.
  - `src/models/invoice.ts`: `type InvoiceKind = 'invoice' | 'estimate'`; interfaces `EstimateMilestone`, `EstimateMilestoneInfoResponse`, `EstimateMilestoneRequest`, `EstimateConvertedInvoice`, `PublicEstimateMilestone`; `Invoice` fields `kind: InvoiceKind`, `estimateStatus`, `respondedUnixTime`, `respondedByClient`, `responseNote`, `estimateId`, `estimateNumber`, `milestoneId`, `milestones`, `convertedInvoices`, `fullyInvoiced`; `Invoice.createNewEstimate()`, `Invoice.createNewMilestone(percent, label?)`; `InvoiceCreateRequest.kind/milestones/estimateId/milestoneId`; `InvoiceProfile.estimateNumberPrefix/estimateValidDays`; `CustomerTimelineItem.kind` adds `'estimate' | 'estimate_accepted' | 'estimate_declined'`, fields `estimateNumber?`, `byClient?`.
  - `getInvoicePdfFileName(number, dueDate, kind = 'invoice')`; `printInvoiceDocument({ number, dueDate, kind? })`.
  - services: `getAllEstimates()`, `getNextInvoiceNumber({ invoiceDate, kind? })`, `setEstimateResponse({ id, status })`, `reopenEstimate({ id })`, `getEstimateInvoiceDraft({ id, milestoneId })`, `getEstimatesToInvoice()`, `respondToPublicEstimate({ token, accept, note })`.
  - store: `allEstimates`, `loadAllEstimates({ force })`, `getNextInvoiceNumber({ invoiceDate, kind? })`, `setEstimateResponse({ estimateId, status })`, `reopenEstimate({ estimateId })`, `getEstimateInvoiceDraft({ estimateId, milestoneId })`, `loadEstimatesToInvoice()`; `deleteInvoice` also removes from `allEstimates`.

- [ ] **Step 1: Write the failing tests**

Create `src/lib/__tests__/estimate.test.ts`:

```ts
import { describe, expect, test } from 'vitest';

import {
    getEstimateDisplayStatus,
    isEstimateClosed,
    isEstimateLocked,
    canAnswerEstimate,
    getMilestoneAmounts,
    getMilestonePercentTotal,
    getMilestonesProblem,
    formatMilestonePercent,
    unixTimeToTextualDate
} from '@/lib/estimate.ts';
import { Invoice } from '@/models/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const estimate = (estimateStatus: '' | 'draft' | 'sent' | 'accepted' | 'declined', dueDate: TextualYearMonthDay, fullyInvoiced = false) =>
    ({ kind: 'estimate', estimateStatus, dueDate, fullyInvoiced, convertedInvoices: [] as { id: string, number: string }[] });

describe('getMilestoneAmounts', () => {
    test('rounds each share half up and gives the last one the remainder', () => {
        expect(getMilestoneAmounts(235999, [3333, 3333, 3334])).toEqual([78658, 78658, 78683]);
        expect(getMilestoneAmounts(5, [5000, 5000])).toEqual([3, 2]);
    });

    test('always adds up to the amount', () => {
        const amounts = getMilestoneAmounts(199999, [1000, 2500, 2500, 4000]);
        expect(amounts.reduce((sum, amount) => sum + amount, 0)).toBe(199999);
    });

    test('shows plain shares while the percents do not add up to 100% yet', () => {
        expect(getMilestoneAmounts(100000, [3000, 3000])).toEqual([30000, 30000]);
        expect(getMilestoneAmounts(100000, [])).toEqual([]);
    });
});

describe('getMilestonesProblem', () => {
    test('none, or 1 to 10 labelled milestones adding up to 100%', () => {
        expect(getMilestonesProblem([])).toBeNull();
        expect(getMilestonesProblem([{ label: 'Advance', percent: 3000 }, { label: 'Delivery', percent: 7000 }])).toBeNull();
        expect(getMilestonesProblem([{ label: ' ', percent: 10000 }])).toBe('Milestone label cannot be blank');
        expect(getMilestonesProblem([{ label: 'Advance', percent: 0 }, { label: 'Delivery', percent: 10000 }])).toBe('Milestone percent must be greater than zero');
        expect(getMilestonesProblem([{ label: 'Advance', percent: 5000 }, { label: 'Delivery', percent: 4999 }])).toBe('Milestones must add up to 100%');
        expect(getMilestonesProblem(Array.from({ length: 11 }, () => ({ label: 'Part', percent: 909 })))).toBe('An estimate can have at most 10 milestones');
        expect(getMilestonePercentTotal([{ percent: 3333 }, { percent: 3333 }])).toBe(6666);
        expect(formatMilestonePercent(3333)).toBe('33.33%');
    });
});

describe('getEstimateDisplayStatus', () => {
    test('expired is a draft or sent estimate past its valid-until date', () => {
        expect(getEstimateDisplayStatus(estimate('sent', '2026-10-29'), '2026-10-30')).toBe('expired');
        expect(getEstimateDisplayStatus(estimate('draft', '2026-10-29'), '2026-10-30')).toBe('expired');
        expect(getEstimateDisplayStatus(estimate('sent', '2026-10-30'), '2026-10-30')).toBe('sent'); // valid until today
        expect(getEstimateDisplayStatus(estimate('accepted', '2026-01-01'), '2026-10-30')).toBe('accepted');
        expect(getEstimateDisplayStatus(estimate('declined', '2026-01-01'), '2026-10-30')).toBe('declined');
        expect(getEstimateDisplayStatus(estimate('', '2099-01-01'), '2026-10-30')).toBe('draft');
    });

    test('a fully invoiced estimate shows as invoiced and is closed', () => {
        expect(getEstimateDisplayStatus(estimate('accepted', '2026-01-01', true), '2026-10-30')).toBe('invoiced');
        expect(isEstimateClosed(estimate('accepted', '2026-01-01', true))).toBe(true);
        expect(isEstimateClosed(estimate('declined', '2099-01-01'))).toBe(true);
        expect(isEstimateClosed(estimate('accepted', '2099-01-01'))).toBe(false);
    });
});

describe('isEstimateLocked and canAnswerEstimate', () => {
    test('editable while draft or sent and nothing invoiced', () => {
        expect(isEstimateLocked(estimate('sent', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked({ ...estimate('sent', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(true);
    });

    test('the client can answer only a sent, still valid estimate', () => {
        expect(canAnswerEstimate(estimate('sent', '2026-10-30'), '2026-10-30')).toBe(true);
        expect(canAnswerEstimate(estimate('sent', '2026-10-29'), '2026-10-30')).toBe(false);
        expect(canAnswerEstimate(estimate('draft', '2099-01-01'), '2026-10-30')).toBe(false);
        expect(canAnswerEstimate({ ...estimate('sent', '2099-01-01'), kind: 'invoice' }, '2026-10-30')).toBe(false);
    });
});

describe('estimate models', () => {
    test('duplicating an answered estimate makes a new draft with the same payment terms', () => {
        const source = Invoice.createNewEstimate();
        source.estimateStatus = 'accepted';
        source.milestones = [{ id: '7', label: 'Advance', percent: 3000, invoiceId: '9', invoiceNumber: 'INV/2026-27/01' }, Invoice.createNewMilestone(7000, 'Delivery')];

        const copy = source.duplicate();
        expect(copy.kind).toBe('estimate');
        expect(copy.estimateStatus).toBe('draft');
        expect(copy.milestones).toEqual([
            { id: '', label: 'Advance', percent: 3000, invoiceId: '', invoiceNumber: '' },
            { id: '', label: 'Delivery', percent: 7000, invoiceId: '', invoiceNumber: '' }
        ]);
        expect(copy.toCreateRequest().milestones).toEqual([{ label: 'Advance', percent: 3000 }, { label: 'Delivery', percent: 7000 }]);
        expect(copy.toCreateRequest().kind).toBe('estimate');
    });

    test('an invoice sends no milestones and no estimate link unless converted', () => {
        const request = Invoice.createNewInvoice().toCreateRequest();
        expect(request.kind).toBe('invoice');
        expect(request.milestones).toEqual([]);
        expect(request.estimateId).toBe('0');
        expect(request.milestoneId).toBe('0');
    });

    test('unixTimeToTextualDate is the local calendar date', () => {
        expect(unixTimeToTextualDate(new Date(2026, 9, 30, 12, 0, 0).getTime() / 1000)).toBe('2026-10-30');
    });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/estimate.test.ts`
Expected: FAIL (`Failed to resolve import "@/lib/estimate.ts"`).

- [ ] **Step 3: Helpers**

Create `src/lib/estimate.ts`:

```ts
import type { TextualYearMonthDay } from '@/core/datetime.ts';

// mirror models.EstimateMilestonePercentTotal / models.EstimateMaxMilestones in pkg/models/estimate.go
export const ESTIMATE_MILESTONE_PERCENT_TOTAL = 10000;
export const ESTIMATE_MAX_MILESTONES = 10;

// mirrors models.ESTIMATE_STATUS_*; expired and invoiced are only ever displayed
export type EstimateStatus = 'draft' | 'sent' | 'accepted' | 'declined';
export type EstimateDisplayStatus = EstimateStatus | 'expired' | 'invoiced';

export const ESTIMATE_DISPLAY_STATUS_NAMES: Record<EstimateDisplayStatus, string> = {
    draft: 'Draft',
    sent: 'Sent',
    accepted: 'Accepted',
    declined: 'Declined',
    expired: 'Expired',
    invoiced: 'Invoiced'
};

interface EstimateState {
    readonly estimateStatus: EstimateStatus | '';
    readonly dueDate: string;
    readonly fullyInvoiced: boolean;
}

// expired: a draft or sent estimate whose valid-until date (dueDate) is before today; extending the date revives it
export function getEstimateDisplayStatus(estimate: EstimateState, today: string): EstimateDisplayStatus {
    if (estimate.fullyInvoiced) {
        return 'invoiced';
    }

    const status = estimate.estimateStatus || 'draft';

    if ((status === 'draft' || status === 'sent') && estimate.dueDate < today) {
        return 'expired';
    }

    return status;
}

// hidden from the list unless "Show closed": declined, or fully invoiced
export function isEstimateClosed(estimate: EstimateState): boolean {
    return estimate.estimateStatus === 'declined' || estimate.fullyInvoiced;
}

// read-only once answered or once anything was invoiced from it, mirroring checkEstimateEditable in pkg/services/estimate_lifecycle.go
export function isEstimateLocked(estimate: { readonly estimateStatus: EstimateStatus | '', readonly convertedInvoices: readonly unknown[] }): boolean {
    const answered = estimate.estimateStatus === 'accepted' || estimate.estimateStatus === 'declined';
    return answered || estimate.convertedInvoices.length > 0;
}

// the public page offers Accept / Decline only while the server would take the answer
export function canAnswerEstimate(estimate: { readonly kind: string, readonly estimateStatus: EstimateStatus | '', readonly dueDate: string }, today: string): boolean {
    return estimate.kind === 'estimate' && estimate.estimateStatus === 'sent' && estimate.dueDate >= today;
}

// amount × percent (basis points) / 10000 rounded half up, mirroring milestoneShare in pkg/invoicepdf/format.go
export function getEstimateShare(amount: number, percent: number): number {
    return Math.round(amount * percent / ESTIMATE_MILESTONE_PERCENT_TOTAL);
}

// each milestone's part of amount (the total incl. tax in the Payment Terms table and the editor); once the percents add up to
// 100% the last one takes what is left, so the parts add up exactly
export function getMilestoneAmounts(amount: number, percents: readonly number[]): number[] {
    const amounts = percents.map(percent => getEstimateShare(amount, percent));

    if (amounts.length && percents.reduce((sum, percent) => sum + percent, 0) === ESTIMATE_MILESTONE_PERCENT_TOTAL) {
        amounts[amounts.length - 1] = amount - amounts.slice(0, -1).reduce((sum, value) => sum + value, 0);
    }

    return amounts;
}

export function getMilestonePercentTotal(milestones: readonly { readonly percent: number }[]): number {
    return milestones.reduce((sum, milestone) => sum + milestone.percent, 0);
}

// mirrors validateEstimateMilestones in pkg/services/estimates.go; null when the milestones can be saved
export function getMilestonesProblem(milestones: readonly { readonly label: string, readonly percent: number }[]): string | null {
    if (!milestones.length) {
        return null;
    } else if (milestones.length > ESTIMATE_MAX_MILESTONES) {
        return 'An estimate can have at most 10 milestones';
    } else if (milestones.some(milestone => !milestone.label.trim())) {
        return 'Milestone label cannot be blank';
    } else if (milestones.some(milestone => milestone.percent <= 0)) {
        return 'Milestone percent must be greater than zero';
    } else if (getMilestonePercentTotal(milestones) !== ESTIMATE_MILESTONE_PERCENT_TOTAL) {
        return 'Milestones must add up to 100%';
    }

    return null;
}

// basis points as a percentage with trailing zeros trimmed: 3000 → "30%", 3333 → "33.33%"
export function formatMilestonePercent(basisPoints: number): string {
    return (basisPoints / 100).toString() + '%';
}

// the local calendar date of a unix time, e.g. for "Accepted on <date>"
export function unixTimeToTextualDate(unixTime: number): TextualYearMonthDay {
    const date = new Date(unixTime * 1000);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}` as TextualYearMonthDay;
}
```

- [ ] **Step 4: Models**

In `src/models/invoice.ts` add `import type { EstimateStatus } from '@/lib/estimate.ts';` next to the other imports, and after `INVOICE_TAX_TYPES`:

```ts
// mirrors models.INVOICE_KIND_* in pkg/models/invoice.go
export type InvoiceKind = 'invoice' | 'estimate';

// one payment term of an estimate as the editor and the detail view hold it ('' ids until saved / invoiced)
export interface EstimateMilestone {
    id: string;
    label: string;
    percent: number; // basis points
    invoiceId: string;
    invoiceNumber: string;
}

export interface EstimateMilestoneInfoResponse {
    readonly id?: string;
    readonly label: string;
    readonly percent: number;
    readonly invoiceId?: string;
    readonly invoiceNumber?: string;
}

export interface EstimateMilestoneRequest {
    readonly label: string;
    readonly percent: number;
}

export interface EstimateConvertedInvoice {
    readonly id: string;
    readonly number: string;
}

export interface PublicEstimateMilestone {
    readonly label: string;
    readonly percent: number;
}
```

Changes to existing types:
- `InvoiceProfileInfoResponse`: add `readonly estimateNumberPrefix: string;` and `readonly estimateValidDays: number;`. `InvoiceProfile`: fields `public estimateNumberPrefix: string = 'EST';` and `public estimateValidDays: number = 30;`; in `fillFrom` `this.estimateNumberPrefix = other.estimateNumberPrefix || 'EST';` and `this.estimateValidDays = other.estimateValidDays || 30;`; in `toUpdateRequest` `estimateNumberPrefix: this.estimateNumberPrefix.trim(), estimateValidDays: this.estimateValidDays`.
- `InvoiceInfoResponse`: add

```ts
    readonly estimateStatus?: EstimateStatus | '';
    readonly respondedUnixTime?: number;
    readonly respondedByClient?: boolean;
    readonly responseNote?: string;
    readonly estimateId?: string; // invoices made from an estimate
    readonly estimateNumber?: string;
    readonly milestoneId?: string; // conversion drafts of one milestone
    readonly milestones?: EstimateMilestoneInfoResponse[];
    readonly convertedInvoices?: EstimateConvertedInvoice[];
    readonly fullyInvoiced?: boolean;
```

- `InvoiceCreateRequest`: add `readonly kind: InvoiceKind; readonly milestones: EstimateMilestoneRequest[]; readonly estimateId: string; readonly milestoneId: string;`.
- `PublicInvoiceResponse`: add `readonly kind?: string; readonly estimateStatus?: EstimateStatus; readonly respondedUnixTime?: number; readonly milestones?: PublicEstimateMilestone[];`.
- `Invoice`: change `public kind: string = 'invoice';` to `public kind: InvoiceKind = 'invoice';` and add

```ts
    public estimateStatus: EstimateStatus | '' = ''; // estimates only
    public respondedUnixTime: number = 0;
    public respondedByClient: boolean = false;
    public responseNote: string = '';
    public estimateId: string = ''; // invoices made from an estimate
    public estimateNumber: string = '';
    public milestoneId: string = ''; // a conversion draft of one milestone; sent back on create
    public milestones: EstimateMilestone[] = [];
    public convertedInvoices: EstimateConvertedInvoice[] = [];
    public fullyInvoiced: boolean = false;
```

  In `fillFrom` replace `this.kind = other.kind;` with

```ts
        this.kind = other.kind === 'estimate' ? 'estimate' : 'invoice';
        this.estimateStatus = other.estimateStatus ?? '';
        this.respondedUnixTime = other.respondedUnixTime ?? 0;
        this.respondedByClient = other.respondedByClient ?? false;
        this.responseNote = other.responseNote ?? '';
        this.estimateId = other.estimateId ?? '';
        this.estimateNumber = other.estimateNumber ?? '';
        this.milestoneId = other.milestoneId ?? '';
        this.milestones = (other.milestones ?? []).map(milestone => ({
            id: milestone.id ?? '', label: milestone.label, percent: milestone.percent, invoiceId: milestone.invoiceId ?? '', invoiceNumber: milestone.invoiceNumber ?? ''
        }));
        this.convertedInvoices = [...(other.convertedInvoices ?? [])];
        this.fullyInvoiced = other.fullyInvoiced ?? false;
```

  In `toCreateRequest` add after `currency: this.currency`:

```ts
            kind: this.kind,
            milestones: this.kind === 'estimate' ? this.milestones.map(milestone => ({ label: milestone.label.trim(), percent: milestone.percent })) : [],
            estimateId: this.estimateId || '0',
            milestoneId: this.milestoneId || '0'
```

  Replace `duplicate()`:

```ts
    // a new unsaved invoice (or draft estimate, with the same payment terms) with this one's customer, currency, lines and notes
    public duplicate(): Invoice {
        const invoice = this.kind === 'estimate' ? Invoice.createNewEstimate() : Invoice.createNewInvoice();
        invoice.customerId = this.customerId;
        invoice.currency = this.currency;
        invoice.notes = this.notes;
        invoice.lines = this.lines.map(line => ({ ...line }));
        invoice.milestones = this.milestones.map(milestone => Invoice.createNewMilestone(milestone.percent, milestone.label));
        return invoice;
    }
```

  In `ofPublic` replace the `fillFrom` call with

```ts
        invoice.fillFrom({
            ...response, id: '', kind: response.kind || 'invoice', customerId: '', payments: [],
            milestones: (response.milestones ?? []).map(milestone => ({ label: milestone.label, percent: milestone.percent }))
        });
```

  Add after `createNewInvoice`:

```ts
    public static createNewEstimate(): Invoice {
        const estimate = Invoice.createNewInvoice();
        estimate.kind = 'estimate';
        estimate.estimateStatus = 'draft';
        return estimate;
    }

    public static createNewMilestone(percent: number, label: string = ''): EstimateMilestone {
        return { id: '', label, percent, invoiceId: '', invoiceNumber: '' };
    }
```

- `CustomerTimelineItem`: `readonly kind: 'invoice' | 'email' | 'payment' | 'estimate' | 'estimate_accepted' | 'estimate_declined';` and add `readonly estimateNumber?: string; readonly byClient?: boolean;`.

In `src/lib/invoice.ts` replace `getInvoicePdfFileName` and `printInvoiceDocument`:

```ts
export function getInvoicePdfFileName(number: string, dueDate: string, kind: string = 'invoice'): string {
    return `${kind === 'estimate' ? 'Estimate' : 'Invoice'}_${number.replace(/[\\/:*?"<>|\s]+/g, '_')}_${dueDate}`;
}

// prints the document; the browser suggests document.title as the PDF file name
export function printInvoiceDocument(invoice: { readonly number: string, readonly dueDate: string, readonly kind?: string }): void {
    const originalTitle = document.title;
    document.title = getInvoicePdfFileName(invoice.number, invoice.dueDate, invoice.kind);
    window.addEventListener('afterprint', () => {
        document.title = originalTitle;
    }, { once: true });
    window.print();
}
```

In `src/lib/invoiceEmail.ts` append `{ key: 'document_type', label: 'Document Type' }` to `INVOICE_EMAIL_EDITOR_VARIABLES`.

- [ ] **Step 5: Services and store**

In `src/lib/services.ts` replace `getNextInvoiceNumber` and add below `getInvoiceRenewals`:

```ts
    getNextInvoiceNumber: ({ invoiceDate, kind }: { invoiceDate: string, kind?: string }): ApiResponsePromise<string> => {
        return axios.get<ApiResponse<string>>('v1/invoices/next_number.json?invoice_date=' + invoiceDate + (kind === 'estimate' ? '&kind=estimate' : ''));
    },
```

```ts
    getAllEstimates: (): ApiResponsePromise<InvoiceInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceInfoResponse[]>>('v1/invoices/list.json?kind=estimate');
    },
    setEstimateResponse: (req: { id: string, status: 'accepted' | 'declined' }): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.post<ApiResponse<InvoiceInfoResponse>>('v1/estimates/respond.json', req);
    },
    reopenEstimate: ({ id }: { id: string }): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.post<ApiResponse<InvoiceInfoResponse>>('v1/estimates/reopen.json', { id });
    },
    getEstimateInvoiceDraft: ({ id, milestoneId }: { id: string, milestoneId: string }): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.get<ApiResponse<InvoiceInfoResponse>>('v1/estimates/invoice_draft.json?id=' + id + (milestoneId ? '&milestone_id=' + milestoneId : ''));
    },
    getEstimatesToInvoice: (): ApiResponsePromise<InvoiceInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceInfoResponse[]>>('v1/estimates/to_invoice.json');
    },
    // unauthenticated like getPublicInvoice: the token is the only key; a refused answer rejects and the page reloads the estimate
    respondToPublicEstimate: ({ token, accept, note }: { token: string, accept: boolean, note: string }): ApiResponsePromise<boolean> => {
        return axios.post<ApiResponse<boolean>>(accept ? 'public/estimates/accept.json' : 'public/estimates/decline.json', { token, note }, {
            noAuth: true,
            ignoreError: true
        } as ApiRequestConfig);
    },
```

In `src/stores/invoice.ts`:
- import `type InvoiceKind` from `@/models/invoice.ts`.
- state `const allEstimates = ref<Invoice[]>([]);`; `resetInvoices` adds `allEstimates.value = [];`; `removeInvoiceFromList` adds `allEstimates.value = allEstimates.value.filter(estimate => estimate.id !== invoiceId);`.
- replace `getNextInvoiceNumber`:

```ts
    function getNextInvoiceNumber({ invoiceDate, kind }: { invoiceDate: string, kind?: InvoiceKind }): Promise<string> {
        return request(services.getNextInvoiceNumber({ invoiceDate, kind }), 'Unable to get next invoice number');
    }
```

- add:

```ts
    // like the invoice list, never served from cache: client answers and conversions change estimates behind the UI's back
    function loadAllEstimates({ force }: { force?: boolean }): Promise<Invoice[]> {
        return request(services.getAllEstimates(), 'Unable to retrieve estimate list').then(result => {
            const estimates = Invoice.ofMulti(result);

            if (force && isEquals(allEstimates.value, estimates)) {
                return Promise.reject({ message: 'Estimate list is up to date', isUpToDate: true });
            }

            allEstimates.value = estimates;
            return estimates;
        });
    }

    function setEstimateResponse({ estimateId, status }: { estimateId: string, status: 'accepted' | 'declined' }): Promise<Invoice> {
        return request(services.setEstimateResponse({ id: estimateId, status }), 'Unable to save estimate').then(result => Invoice.of(result));
    }

    function reopenEstimate({ estimateId }: { estimateId: string }): Promise<Invoice> {
        return request(services.reopenEstimate({ id: estimateId }), 'Unable to save estimate').then(result => Invoice.of(result));
    }

    // the unsaved invoice an estimate (milestoneId '') or one of its milestones turns into, for the editor
    function getEstimateInvoiceDraft({ estimateId, milestoneId }: { estimateId: string, milestoneId: string }): Promise<Invoice> {
        return request(services.getEstimateInvoiceDraft({ id: estimateId, milestoneId }), 'Unable to convert estimate').then(result => Invoice.of(result));
    }

    function loadEstimatesToInvoice(): Promise<Invoice[]> {
        return request(services.getEstimatesToInvoice(), 'Unable to retrieve estimate list').then(result => Invoice.ofMulti(result));
    }
```

- return `allEstimates` with the states and `loadAllEstimates, setEstimateResponse, reopenEstimate, getEstimateInvoiceDraft, loadEstimatesToInvoice` with the functions.

In `src/locales/en.json`, inside `"error"` after `"recurring invoice mode is invalid"`:

```json
        "estimate milestones must be 1 to 10 with labels and add up to 100%": "Milestones need a label each, at most 10 of them, and must add up to 100%",
        "estimate is accepted, declined or invoiced and cannot be edited": "This estimate is accepted, declined or invoiced and can no longer be edited",
        "estimate has invoices created from it": "This estimate has invoices created from it",
        "estimate status does not allow this action": "This estimate's status does not allow this action",
        "this action is not available for estimates": "This action is not available for estimates",
        "estimate or milestone has already been invoiced": "This estimate or milestone has already been invoiced",
        "estimate milestone not found": "Estimate milestone not found",
        "this estimate can no longer be answered": "This estimate can no longer be answered",
        "response note must be at most 500 characters": "The note must be at most 500 characters",
```

and at the top level next to `"Unable to retrieve product list"`:

```json
    "Unable to retrieve estimate list": "Unable to retrieve estimate list",
    "Estimate list is up to date": "Estimate list is up to date",
    "Unable to save estimate": "Unable to save estimate",
    "Unable to convert estimate": "Unable to convert estimate",
    "Document Type": "Document Type",
    "An estimate can have at most 10 milestones": "An estimate can have at most 10 milestones",
    "Milestone label cannot be blank": "Milestone label cannot be blank",
    "Milestone percent must be greater than zero": "Milestone percent must be greater than zero",
    "Milestones must add up to 100%": "Milestones must add up to 100%",
    "Draft": "Draft",
    "Sent": "Sent",
    "Accepted": "Accepted",
    "Declined": "Declined",
    "Expired": "Expired",
    "Invoiced": "Invoiced",
```

- [ ] **Step 6: Run tests and typecheck**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/estimate.test.ts src/lib/__tests__/invoice.test.ts && npx vue-tsc --noEmit && npx eslint src/lib src/models src/stores`
Expected: PASS and clean.

- [ ] **Step 7: Commit**

```bash
git add src/lib/estimate.ts src/lib/__tests__/estimate.test.ts src/models/invoice.ts src/lib/invoice.ts src/lib/invoiceEmail.ts src/lib/services.ts src/stores/invoice.ts src/locales/en.json
git commit --no-verify -m "estimates: frontend models, API calls, store and milestone / status helpers"
```

---

### Task 9: Desktop Estimates view and the editor in estimate mode (with milestones)

**Files:**
- Create: `src/views/base/invoices/EstimateListPageBase.ts`
- Modify: `src/views/base/invoices/InvoiceListPageBase.ts` (invoices only)
- Modify: `src/views/base/invoices/InvoiceEditPageBase.ts`
- Modify: `src/views/desktop/invoices/ListPage.vue`
- Modify: `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Task 8): `allEstimates`, `loadAllEstimates`, `getNextInvoiceNumber({ invoiceDate, kind })`, `getEstimateInvoiceDraft`, `Invoice.createNewEstimate`, `Invoice.createNewMilestone`, `InvoiceKind`, `getEstimateDisplayStatus`, `ESTIMATE_DISPLAY_STATUS_NAMES`, `isEstimateClosed`, `getMilestoneAmounts`, `getMilestonePercentTotal`, `getMilestonesProblem`, `formatMilestonePercent`, `ESTIMATE_MAX_MILESTONES`, `ESTIMATE_MILESTONE_PERCENT_TOTAL`, `InvoiceProfile.estimateValidDays`.
- Produces:
  - `useEstimateListPageBase()` → `{ loading, showClosed, allEstimates, displayEstimates, noAvailableEstimate, getStatus(estimate), getStatusName(estimate), formatAmount(minorUnits, currency), formatDate(date) }`; `ESTIMATE_STATUS_COLORS: Record<EstimateDisplayStatus, string>`.
  - `useInvoiceEditPageBase()` adds `isEstimate`, `numberLabel`, `dateLabel`, `dueDateLabel`, `milestoneAmounts`, `milestonePercentTotal`, `milestonesProblem`, `addMilestone()`, `removeMilestone(index)`, `prepareConvertedInvoice(profile, draft)`; `prepareNewInvoice(profile, duplicateFrom?, kind = 'invoice')`; titles "Add Estimate" / "Edit Estimate".
  - `InvoiceEditDialog.open({ id?, currentInvoice?, duplicateFrom?, kind?, convertFrom? })` with `convertFrom: { estimateId: string, milestoneId: string }`.
  - Desktop Invoices page: third left-nav view `estimates`.

- [ ] **Step 1: Locale strings**

Add to `src/locales/en.json` (top level, next to the invoice strings; skip any key that already exists):

```json
    "Estimates": "Estimates",
    "Estimate": "Estimate",
    "Add Estimate": "Add Estimate",
    "Edit Estimate": "Edit Estimate",
    "Estimate Number": "Estimate Number",
    "Estimate Date": "Estimate Date",
    "Valid Until": "Valid Until",
    "Milestones": "Milestones",
    "Milestone": "Milestone",
    "Percent (%)": "Percent (%)",
    "Add Milestone": "Add Milestone",
    "No available estimate": "No available estimate",
    "Show Closed Estimates": "Show Closed Estimates",
    "Hide Closed Estimates": "Hide Closed Estimates",
    "Estimate number cannot be blank": "Estimate number cannot be blank",
    "Valid until cannot be before the estimate date": "Valid until cannot be before the estimate date",
    "You have added a new estimate": "You have added a new estimate",
    "You have saved this estimate": "You have saved this estimate",
```

- [ ] **Step 2: Bases**

Create `src/views/base/invoices/EstimateListPageBase.ts`:

```ts
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { getTodayTextualDate } from '@/lib/invoice.ts';
import { type EstimateDisplayStatus, ESTIMATE_DISPLAY_STATUS_NAMES, getEstimateDisplayStatus, isEstimateClosed } from '@/lib/estimate.ts';

// the status chip colours of the desktop list and detail
export const ESTIMATE_STATUS_COLORS: Record<EstimateDisplayStatus, string> = {
    draft: 'default',
    sent: 'primary',
    accepted: 'success',
    declined: 'error',
    expired: 'warning',
    invoiced: 'secondary'
};

// the estimate list: open estimates by default, declined and fully invoiced ones behind "Show closed"
export function useEstimateListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const showClosed = ref<boolean>(false);
    const today = getTodayTextualDate();

    const allEstimates = computed<Invoice[]>(() => invoicesStore.allEstimates);
    const displayEstimates = computed<Invoice[]>(() => showClosed.value ? allEstimates.value : allEstimates.value.filter(estimate => !isEstimateClosed(estimate)));
    const noAvailableEstimate = computed<boolean>(() => displayEstimates.value.length < 1);

    function getStatus(estimate: Invoice): EstimateDisplayStatus {
        return getEstimateDisplayStatus(estimate, today);
    }

    function getStatusName(estimate: Invoice): string {
        return ESTIMATE_DISPLAY_STATUS_NAMES[getStatus(estimate)];
    }

    function formatAmount(minorUnits: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        loading,
        showClosed,
        // computed states
        allEstimates,
        displayEstimates,
        noAvailableEstimate,
        // functions
        getStatus,
        getStatusName,
        formatAmount,
        formatDate
    };
}
```

In `src/views/base/invoices/InvoiceListPageBase.ts` make the totals invoice-only even if an estimate ever reaches the list:

```ts
    // invoices only: an estimate is never outstanding or overdue
    const allInvoices = computed<Invoice[]>(() => invoicesStore.allInvoices.filter(invoice => invoice.kind === 'invoice'));
```

In `src/views/base/invoices/InvoiceEditPageBase.ts`:
- imports: `import { type Customer, type InvoiceProfile, type InvoiceTaxType, type InvoiceLine, type InvoiceKind, Invoice, INVOICE_TAX_TYPES } from '@/models/invoice.ts';` and `import { ESTIMATE_MAX_MILESTONES, ESTIMATE_MILESTONE_PERCENT_TOTAL, getMilestoneAmounts, getMilestonePercentTotal, getMilestonesProblem } from '@/lib/estimate.ts';`
- state `const profileEstimateValidDays = ref<number>(30);`
- replace `title` with

```ts
    const isEstimate = computed<boolean>(() => invoice.value.kind === 'estimate');

    const title = computed<string>(() => {
        if (isEstimate.value) {
            return editInvoiceId.value ? 'Edit Estimate' : 'Add Estimate';
        }

        return editInvoiceId.value ? 'Edit Invoice' : 'Add Invoice';
    });

    const numberLabel = computed<string>(() => isEstimate.value ? 'Estimate Number' : 'Invoice Number');
    const dateLabel = computed<string>(() => isEstimate.value ? 'Estimate Date' : 'Invoice Date');
    const dueDateLabel = computed<string>(() => isEstimate.value ? 'Valid Until' : 'Payment Due');
```

- after `totals` add

```ts
    // what each milestone comes to out of the total incl. tax, as the Payment Terms table prints it
    const milestoneAmounts = computed<number[]>(() => getMilestoneAmounts(totals.value.total, invoice.value.milestones.map(milestone => milestone.percent)));
    const milestonePercentTotal = computed<number>(() => getMilestonePercentTotal(invoice.value.milestones));
    const milestonesProblem = computed<string | null>(() => isEstimate.value ? getMilestonesProblem(invoice.value.milestones) : null);
```

- in `inputEmptyProblemMessage`: the number branch returns `isEstimate.value ? 'Estimate number cannot be blank' : 'Invoice number cannot be blank'`, the due-date branch returns `isEstimate.value ? 'Valid until cannot be before the estimate date' : 'Due date cannot be before the invoice date'`, and right before the final `return null;` add `if (milestonesProblem.value) { return milestonesProblem.value; }`.
- add

```ts
    // a new milestone gets whatever percent is still missing, so two clicks on a fresh estimate give a sensible 50 / 50 start
    function addMilestone(): void {
        if (invoice.value.milestones.length >= ESTIMATE_MAX_MILESTONES) {
            return;
        }

        invoice.value.milestones.push(Invoice.createNewMilestone(Math.max(ESTIMATE_MILESTONE_PERCENT_TOTAL - milestonePercentTotal.value, 0)));
    }

    function removeMilestone(index: number): void {
        invoice.value.milestones.splice(index, 1);
    }
```

- `setProfileDefaults` adds `profileEstimateValidDays.value = profile.estimateValidDays;`
- replace `prepareNewInvoice`:

```ts
    // a new invoice (or estimate) dated today with the profile's terms (or validity) and notes, optionally copying another one
    function prepareNewInvoice(profile: InvoiceProfile, duplicateFrom?: Invoice | null, kind: InvoiceKind = 'invoice'): void {
        const newInvoice = duplicateFrom ? duplicateFrom.duplicate() : (kind === 'estimate' ? Invoice.createNewEstimate() : Invoice.createNewInvoice());
        const today = getTodayTextualDate();

        setProfileDefaults(profile);
        newInvoice.invoiceDate = today;
        newInvoice.dueDate = addDaysToDate(today, newInvoice.kind === 'estimate' ? profile.estimateValidDays : getTermsDays(newInvoice.customerId, profile.defaultTermsDays));
        applyCustomerTaxDefaults(newInvoice, newInvoice.customerId);

        if (!duplicateFrom) {
            newInvoice.currency = profile.defaultCurrency || INVOICE_DEFAULT_CURRENCY; // a duplicate keeps its source's currency
            newInvoice.notes = profile.defaultNotes;
            newInvoice.lines = [Invoice.createNewLine(profile.defaultSac)];
        }

        invoice.value = newInvoice;
        lastSuggestedNumber.value = '';
    }

    // the unsaved invoice an estimate turned into; its customer, currency and (for a milestone) amount are set by the server
    function prepareConvertedInvoice(profile: InvoiceProfile, draft: Invoice): void {
        setProfileDefaults(profile);

        const converted = draft.clone();
        converted.id = '';
        invoice.value = converted;
        lastSuggestedNumber.value = converted.number;
    }
```

- in `setInvoiceDate` replace the `getTermsDays(...)` fallback with `(isEstimate.value ? profileEstimateValidDays.value : getTermsDays(invoice.value.customerId, termsDays))`.
- replace the customer `watch`:

```ts
    // choosing a customer on a new invoice re-applies the due date from its terms and its tax defaults (an estimate keeps its validity);
    // an existing invoice, or one made from an estimate, is never changed silently
    watch(() => invoice.value.customerId, customerId => {
        if (editInvoiceId.value || invoice.value.estimateId || !invoice.value.invoiceDate) {
            return;
        }

        if (!isEstimate.value) {
            invoice.value.dueDate = addDaysToDate(invoice.value.invoiceDate, getTermsDays(customerId, profileTermsDays.value));
        }

        applyCustomerTaxDefaults(invoice.value, customerId);
    });
```

- in `suggestNumber` call `invoicesStore.getNextInvoiceNumber({ invoiceDate: invoice.value.invoiceDate, kind: invoice.value.kind })`.
- return the new members: `isEstimate, numberLabel, dateLabel, dueDateLabel, milestoneAmounts, milestonePercentTotal, milestonesProblem` (computed states) and `addMilestone, removeMilestone, prepareConvertedInvoice` (functions).

- [ ] **Step 3: Desktop editor**

In `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`:
- customer `v-autocomplete` and `currency-select`: add `|| !!invoice.estimateId` to `:disabled` (the server bills the estimate's customer in its currency).
- number field: `:label="tt(numberLabel)" :placeholder="tt(numberLabel)"`; invoice date: `:label="tt(dateLabel)"`; due date: `:label="tt(dueDateLabel)"`.
- right after the `Add Line` column add the milestones block (same grid, no extra margins):

```vue
                        <template v-if="isEstimate">
                            <v-col cols="12" class="d-flex align-center">
                                <span class="text-title-small">{{ tt('Milestones') }}</span>
                                <v-chip class="ms-3" size="small" variant="tonal" :color="milestonesProblem ? 'warning' : 'success'"
                                        v-if="invoice.milestones.length">
                                    {{ milestonesProblem ? tt(milestonesProblem) : formatMilestonePercent(milestonePercentTotal) + ' ✓' }}
                                </v-chip>
                            </v-col>
                            <template :key="index" v-for="(milestone, index) in invoice.milestones">
                                <v-col cols="12" md="6">
                                    <v-text-field type="text" autocomplete="off" persistent-placeholder
                                                  :disabled="loading || submitting"
                                                  :label="tt('Milestone')" :placeholder="tt('Milestone')"
                                                  v-model="milestone.label" />
                                </v-col>
                                <v-col cols="5" md="2">
                                    <number-input :disabled="loading || submitting" :persistent-placeholder="true"
                                                  :label="tt('Percent (%)')" :min-value="0.01" :max-value="100" :max-decimal-count="2"
                                                  :model-value="milestone.percent / 100"
                                                  @update:model-value="milestone.percent = Math.round($event * 100)" />
                                </v-col>
                                <v-col cols="5" md="3" class="d-flex align-center justify-end">
                                    <span class="text-body-large">{{ formatAmount(milestoneAmounts[index] ?? 0) }}</span>
                                </v-col>
                                <v-col cols="2" md="1" class="d-flex align-center justify-center">
                                    <v-btn color="error" variant="text" density="comfortable" :icon="mdiDeleteOutline" :aria-label="tt('Remove')"
                                           :disabled="loading || submitting" @click="removeMilestone(index)"></v-btn>
                                </v-col>
                            </template>
                            <v-col cols="12">
                                <v-btn color="primary" variant="text" density="comfortable" :prepend-icon="mdiPlus"
                                       :disabled="loading || submitting || invoice.milestones.length >= ESTIMATE_MAX_MILESTONES"
                                       @click="addMilestone">{{ tt('Add Milestone') }}</v-btn>
                            </v-col>
                        </template>
```

- script: `import { type InvoiceProfile, type InvoiceLine, type InvoiceKind, Invoice } from '@/models/invoice.ts';`, `import { ESTIMATE_MAX_MILESTONES, formatMilestonePercent } from '@/lib/estimate.ts';`; destructure `isEstimate, numberLabel, dateLabel, dueDateLabel, milestoneAmounts, milestonePercentTotal, milestonesProblem, addMilestone, removeMilestone, prepareConvertedInvoice` from `useInvoiceEditPageBase()`.
- replace `open`:

```ts
function open(options: { id?: string, currentInvoice?: Invoice, duplicateFrom?: Invoice, kind?: InvoiceKind, convertFrom?: { estimateId: string, milestoneId: string } }): Promise<InvoiceEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    editInvoiceId.value = options.id ?? null;

    if (options.currentInvoice) {
        setInvoice(options.currentInvoice);
    }

    const loadInvoice = options.id
        ? invoicesStore.getInvoice({ invoiceId: options.id })
        : options.convertFrom
            ? invoicesStore.getEstimateInvoiceDraft({ estimateId: options.convertFrom.estimateId, milestoneId: options.convertFrom.milestoneId })
            : Promise.resolve(null);

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        invoicesStore.loadAllProducts({ force: false }).catch(() => []),
        loadInvoice
    ]).then(([, loadedProfile, , loadedInvoice]) => {
        profile.value = loadedProfile;

        if (loadedInvoice && options.id) {
            setInvoice(loadedInvoice, loadedProfile);
        } else if (loadedInvoice) {
            prepareConvertedInvoice(loadedProfile, loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, options.duplicateFrom, options.kind ?? 'invoice');
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
```

- in `save`, the resolved message becomes `isEstimate.value ? (editInvoiceId.value ? 'You have saved this estimate' : 'You have added a new estimate') : (editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice')`.

- [ ] **Step 4: Desktop Estimates view**

In `src/views/desktop/invoices/ListPage.vue`:
- left nav, after the Recurring item:

```vue
            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'estimates', 'disabled': loading }"
                   @click="activeView = 'estimates'">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentCheckOutline"/>
                    <span class="nav-item-title">{{ tt('Estimates') }}</span>
                </a>
            </li>
```

- the More menu, after the Hide Paid item:

```vue
                                            <v-list-item :prepend-icon="mdiEyeOutline" :title="tt('Show Closed Estimates')"
                                                         v-if="activeView === 'estimates' && !showClosed" @click="showClosed = true"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEyeOffOutline" :title="tt('Hide Closed Estimates')"
                                                         v-if="activeView === 'estimates' && showClosed" @click="showClosed = false"></v-list-item>
```

- after the recurring `v-table`:

```vue
                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'estimates'">
                            <thead>
                            <tr>
                                <th>{{ tt('Estimate Number') }}</th>
                                <th>{{ tt('Customer') }}</th>
                                <th>{{ tt('Estimate Date') }}</th>
                                <th>{{ tt('Valid Until') }}</th>
                                <th class="text-right">{{ tt('Total') }}</th>
                                <th>{{ tt('Status') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="loading && noAvailableEstimate">
                            <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                                <td class="px-0" colspan="6"><v-skeleton-loader type="text" :loading="true"></v-skeleton-loader></td>
                            </tr>
                            </tbody>

                            <tbody v-if="!loading && noAvailableEstimate">
                            <tr><td colspan="6">{{ tt('No available estimate') }}</td></tr>
                            </tbody>

                            <tbody v-if="!noAvailableEstimate">
                            <tr class="cursor-pointer" :key="estimate.id" v-for="estimate in displayEstimates" @click="view(estimate)">
                                <td>{{ estimate.number }}</td>
                                <td>{{ estimate.billToName }}</td>
                                <td>{{ formatDate(estimate.invoiceDate) }}</td>
                                <td>{{ formatDate(estimate.dueDate) }}</td>
                                <td class="text-right">{{ formatAmount(estimate.total, estimate.currency) }}</td>
                                <td>
                                    <v-chip size="small" variant="tonal" :color="ESTIMATE_STATUS_COLORS[getEstimateStatus(estimate)]">{{ tt(getEstimateStatusName(estimate)) }}</v-chip>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>
```

- script: add `mdiFileDocumentCheckOutline` to the `@mdi/js` import; `import { useEstimateListPageBase, ESTIMATE_STATUS_COLORS } from '@/views/base/invoices/EstimateListPageBase.ts';`; `const { showClosed, displayEstimates, noAvailableEstimate, getStatus: getEstimateStatus, getStatusName: getEstimateStatusName } = useEstimateListPageBase();`
- `const activeView = ref<'invoices' | 'recurring' | 'estimates'>('invoices');` and

```ts
const VIEW_TITLES: Record<'invoices' | 'recurring' | 'estimates', string> = { invoices: 'Invoices', recurring: 'Recurring', estimates: 'Estimates' };
const activeViewTitle = computed<string>(() => VIEW_TITLES[activeView.value]);
```

- `reload` adds `invoicesStore.loadAllEstimates({ force: force })` to the `Promise.all`.
- `add` opens `invoiceEditDialog.value?.open({ kind: activeView.value === 'estimates' ? 'estimate' : 'invoice' })` (the rest unchanged); the detail dialog opens estimates through the same `view`.

- [ ] **Step 5: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views && npx vitest run --exclude '.claude/**' src/lib/__tests__`
Expected: clean and PASS.

- [ ] **Step 6: Commit**

```bash
git add src/views src/locales/en.json
git commit --no-verify -m "estimates: desktop Estimates view and the editor in estimate mode with milestones"
```

---

### Task 10: Desktop estimate detail: status, milestones, convert; the document prints estimates

**Files:**
- Modify: `src/views/base/invoices/InvoiceDetailPageBase.ts`
- Modify: `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`
- Modify: `src/components/common/InvoiceDocumentContent.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 8–9): store `setEstimateResponse`, `reopenEstimate`; `getEstimateDisplayStatus`, `ESTIMATE_DISPLAY_STATUS_NAMES`, `isEstimateLocked`, `getMilestoneAmounts`, `formatMilestonePercent`, `EstimateDisplayStatus`; `ESTIMATE_STATUS_COLORS`; `InvoiceEditDialog.open({ convertFrom })`; `getInvoicePdfFileName(number, dueDate, kind)`.
- Produces: `useInvoiceDetailPageBase()` adds `isEstimate`, `estimateStatus`, `estimateStatusName`, `estimateResponseText`, `canEdit`, `canMarkResponse`, `canReopenEstimate`, `canConvert`, `convertNeedsConfirm`, `milestoneAmounts`, `setEstimateResponse(status)`, `reopenEstimate()`. The shared document prints "Estimate", the estimate meta rows, a Total row instead of Amount Due, and the Payment Terms table.

- [ ] **Step 1: Locale strings**

Add to `src/locales/en.json` (top level):

```json
    "Mark Accepted": "Mark Accepted",
    "Mark Declined": "Mark Declined",
    "Convert to Invoice": "Convert to Invoice",
    "Create Invoice": "Create Invoice",
    "No response yet": "No response yet",
    "Client's note": "Client's note",
    "This estimate isn't accepted yet. Convert anyway?": "This estimate isn't accepted yet. Convert anyway?",
    "Send Estimate": "Send Estimate",
    "Estimate has been sent": "Estimate has been sent",
    "Are you sure you want to delete this estimate?": "Are you sure you want to delete this estimate?",
    "Anyone with this link can view this estimate and accept or decline it.": "Anyone with this link can view this estimate and accept or decline it.",
```

and inside `"format"` → `"misc"` after `"timelinePayment"`:

```json
            "estimateAcceptedByClient": "Accepted by the client on {date}",
            "estimateDeclinedByClient": "Declined by the client on {date}",
            "estimateAcceptedByYou": "Marked accepted on {date}",
            "estimateDeclinedByYou": "Marked declined on {date}",
            "invoicedAs": "Invoiced → {number}",
            "fromEstimate": "From estimate {number}",
```

- [ ] **Step 2: Base**

In `src/views/base/invoices/InvoiceDetailPageBase.ts`:
- import `import { type EstimateDisplayStatus, ESTIMATE_DISPLAY_STATUS_NAMES, getEstimateDisplayStatus, isEstimateLocked, getMilestoneAmounts } from '@/lib/estimate.ts';`
- after `sentOnText` add:

```ts
    const isEstimate = computed<boolean>(() => invoice.value?.kind === 'estimate');
    const estimateStatus = computed<EstimateDisplayStatus | null>(() => invoice.value && isEstimate.value ? getEstimateDisplayStatus(invoice.value, getTodayTextualDate()) : null);
    const estimateStatusName = computed<string>(() => estimateStatus.value ? ESTIMATE_DISPLAY_STATUS_NAMES[estimateStatus.value] : '');
    // an estimate is edited only while draft or sent and nothing is invoiced from it; Duplicate always works
    const canEdit = computed<boolean>(() => !!invoice.value && (!isEstimate.value || !isEstimateLocked(invoice.value)));
    // Mark accepted / declined from draft, sent or expired; Reopen an answered estimate while nothing is invoiced
    const canMarkResponse = computed<boolean>(() => isEstimate.value && (invoice.value?.estimateStatus === 'draft' || invoice.value?.estimateStatus === 'sent'));
    const canReopenEstimate = computed<boolean>(() => isEstimate.value && !!invoice.value && !invoice.value.convertedInvoices.length &&
        (invoice.value.estimateStatus === 'accepted' || invoice.value.estimateStatus === 'declined'));
    // converting is offered on accepted estimates, and on sent or expired ones after a confirm
    const canConvert = computed<boolean>(() => estimateStatus.value === 'accepted' || estimateStatus.value === 'sent' || estimateStatus.value === 'expired');
    const convertNeedsConfirm = computed<boolean>(() => estimateStatus.value !== 'accepted');
    const milestoneAmounts = computed<number[]>(() => invoice.value ? getMilestoneAmounts(invoice.value.total, invoice.value.milestones.map(milestone => milestone.percent)) : []);

    // "Accepted by the client on <date>" / "Marked declined on <date>"; empty until answered
    const estimateResponseText = computed<string>(() => {
        const current = invoice.value;

        if (!current || !isEstimate.value || !current.respondedUnixTime) {
            return '';
        }

        const date = formatUnixTime(current.respondedUnixTime);

        if (current.estimateStatus === 'accepted') {
            return tt(current.respondedByClient ? 'format.misc.estimateAcceptedByClient' : 'format.misc.estimateAcceptedByYou', { date });
        }

        return tt(current.respondedByClient ? 'format.misc.estimateDeclinedByClient' : 'format.misc.estimateDeclinedByYou', { date });
    });
```

- add:

```ts
    // your own answer; nothing is emailed to the client
    function setEstimateResponse(status: 'accepted' | 'declined'): Promise<Invoice> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to save estimate' });
        }

        return invoicesStore.setEstimateResponse({ estimateId: current.id, status }).then(updated => {
            invoice.value = updated;
            return updated;
        });
    }

    function reopenEstimate(): Promise<Invoice> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to save estimate' });
        }

        return invoicesStore.reopenEstimate({ estimateId: current.id }).then(updated => {
            invoice.value = updated;
            return updated;
        });
    }
```

- in `sendInvoice` add `invoice.value.estimateStatus = sent.estimateStatus;` next to the other copied fields; in `downloadInvoicePdf` use `getInvoicePdfFileName(current.number, current.dueDate, current.kind)`.
- return `isEstimate, estimateStatus, estimateStatusName, estimateResponseText, canEdit, canMarkResponse, canReopenEstimate, canConvert, convertNeedsConfirm, milestoneAmounts` (computed states) and `setEstimateResponse, reopenEstimate` (functions).

- [ ] **Step 3: Document**

In `src/components/common/InvoiceDocumentContent.vue`:
- the banner text `Invoice` becomes `{{ isEstimate ? 'Estimate' : 'Invoice' }}`.
- replace the four meta rows with:

```vue
                <template v-if="isEstimate">
                    <tr><th>Estimate Number:</th><td>{{ invoice.number }}</td></tr>
                    <tr><th>Estimate Date:</th><td>{{ formatInvoiceDocumentDate(invoice.invoiceDate) }}</td></tr>
                    <tr><th>Valid Until:</th><td>{{ formatInvoiceDocumentDate(invoice.dueDate) }}</td></tr>
                    <tr class="invoice-meta-highlight"><th>Total ({{ invoice.currency }}):</th><td class="invoice-strong">{{ formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}</td></tr>
                </template>
                <template v-else>
                    <tr><th>Invoice Number:</th><td>{{ invoice.number }}</td></tr>
                    <tr><th>Invoice Date:</th><td>{{ formatInvoiceDocumentDate(invoice.invoiceDate) }}</td></tr>
                    <tr><th>Payment Due:</th><td>{{ formatInvoiceDocumentDate(invoice.dueDate) }}</td></tr>
                    <tr class="invoice-meta-highlight"><th>Amount Due ({{ invoice.currency }}):</th><td class="invoice-strong">{{ formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}</td></tr>
                </template>
```

- in the totals table replace the last three rows (`Total`, `Payments received`, `Amount Due`) with:

```vue
            <tr class="invoice-totals-due" v-if="isEstimate"><th>Total ({{ invoice.currency }}):</th><td>{{ formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}</td></tr>
            <template v-else>
                <tr><th>Total:</th><td>{{ formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}</td></tr>
                <tr v-if="invoice.received > 0"><th>Payments received:</th><td>−{{ formatInvoiceDocumentAmount(Math.min(invoice.received, invoice.total), invoice.currency) }}</td></tr>
                <tr class="invoice-totals-due"><th>Amount Due ({{ invoice.currency }}):</th><td>{{ formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}</td></tr>
            </template>
```

- between the totals table and the notes block add:

```vue
        <div class="invoice-payment-terms" v-if="isEstimate && invoice.milestones.length">
            <div class="invoice-strong">Payment Terms</div>
            <table class="invoice-lines">
                <thead>
                <tr>
                    <th class="invoice-left">Milestone</th>
                    <th class="invoice-right">%</th>
                    <th class="invoice-right">Amount</th>
                </tr>
                </thead>
                <tbody>
                <tr :key="index" v-for="(milestone, index) in invoice.milestones">
                    <td>{{ milestone.label }}</td>
                    <td class="invoice-right">{{ formatMilestonePercent(milestone.percent) }}</td>
                    <td class="invoice-right">{{ formatInvoiceDocumentAmount(milestoneAmounts[index] ?? 0, invoice.currency) }}</td>
                </tr>
                </tbody>
            </table>
        </div>
```

- script: `import { formatMilestonePercent, getMilestoneAmounts } from '@/lib/estimate.ts';` and

```ts
const isEstimate = computed<boolean>(() => props.invoice.kind === 'estimate');
// each milestone's share of the total incl. tax, the last taking the remainder (the PDF prints the same)
const milestoneAmounts = computed<number[]>(() => getMilestoneAmounts(props.invoice.total, props.invoice.milestones.map(milestone => milestone.percent)));
```

- style:

```css
.invoice-payment-terms {
    margin-top: 40px;
}

.invoice-payment-terms .invoice-lines {
    margin-top: 8px;
}
```

- [ ] **Step 4: Desktop detail dialog**

In `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`:
- replace the `#after-title` slot:

```vue
            <template #after-title>
                <v-chip class="ms-3" size="small" variant="tonal" :color="ESTIMATE_STATUS_COLORS[estimateStatus ?? 'draft']"
                        v-if="invoice && estimateStatus">{{ tt(estimateStatusName) }}</v-chip>
                <v-chip class="ms-3" size="small" variant="tonal" :color="displayStatus ? statusColors[displayStatus] : 'default'"
                        v-else-if="invoice">{{ tt(displayStatusName) }}</v-chip>
                <span class="text-caption text-medium-emphasis ms-2" v-if="sentOnText">{{ tt('Sent on') }} {{ sentOnText }}</span>
                <a class="text-caption ms-2 cursor-pointer" v-if="invoice?.estimateId"
                   @click="openOther(invoice.estimateId)">{{ tt('format.misc.fromEstimate', { number: invoice.estimateNumber }) }}</a>
            </template>
```

- toolbar: the Edit button gets `:disabled="!invoice || loading || updating || !canEdit"`; the Regenerate, Mark as Paid / Reopen and Repeat buttons get `v-if="!isEstimate"`.
- in `#content`, inside `<v-col cols="12" lg="4">`, wrap the existing totals list, its divider and the Payments block in `<template v-else>` and put the estimate panel before it:

```vue
                        <template v-if="isEstimate">
                            <v-list density="compact">
                                <v-list-item :title="tt('Total')">
                                    <template #append><span class="font-weight-bold">{{ formatAmount(invoice.total) }}</span></template>
                                </v-list-item>
                                <v-list-item :title="tt('Valid Until')">
                                    <template #append><span>{{ formatDate(invoice.dueDate) }}</span></template>
                                </v-list-item>
                            </v-list>

                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Status') }}</div>
                            <div class="text-body-medium px-4 pb-2">{{ estimateResponseText || tt('No response yet') }}</div>
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line" v-if="invoice.responseNote">{{ tt("Client's note") }}: {{ invoice.responseNote }}</div>
                            <div class="px-4 pb-2" v-if="canMarkResponse || canReopenEstimate">
                                <v-btn class="me-2" color="success" variant="tonal" size="small" :disabled="updating"
                                       v-if="canMarkResponse" @click="markResponse('accepted')">{{ tt('Mark Accepted') }}</v-btn>
                                <v-btn class="me-2" color="error" variant="tonal" size="small" :disabled="updating"
                                       v-if="canMarkResponse" @click="markResponse('declined')">{{ tt('Mark Declined') }}</v-btn>
                                <v-btn color="default" variant="tonal" size="small" :disabled="updating"
                                       v-if="canReopenEstimate" @click="reopen">{{ tt('Reopen') }}</v-btn>
                            </div>

                            <v-divider class="my-2" />
                            <template v-if="invoice.milestones.length">
                                <div class="text-title-small px-4 py-2">{{ tt('Milestones') }}</div>
                                <v-list density="compact">
                                    <v-list-item :key="milestone.id" v-for="(milestone, index) in invoice.milestones"
                                                 :title="milestone.label"
                                                 :subtitle="`${formatMilestonePercent(milestone.percent)} · ${formatAmount(milestoneAmounts[index] ?? 0)}`">
                                        <template #append>
                                            <a class="cursor-pointer text-body-medium" v-if="milestone.invoiceId"
                                               @click="openOther(milestone.invoiceId)">{{ tt('format.misc.invoicedAs', { number: milestone.invoiceNumber }) }}</a>
                                            <v-btn color="primary" variant="tonal" size="small" :disabled="updating || !canConvert"
                                                   v-else @click="convert(milestone.id)">{{ tt('Create Invoice') }}</v-btn>
                                        </template>
                                    </v-list-item>
                                </v-list>
                            </template>
                            <template v-else>
                                <div class="text-title-small px-4 py-2">{{ tt('Invoices') }}</div>
                                <v-list density="compact" v-if="invoice.convertedInvoices.length">
                                    <v-list-item :key="converted.id" v-for="converted in invoice.convertedInvoices" link
                                                 :title="tt('format.misc.invoicedAs', { number: converted.number })"
                                                 @click="openOther(converted.id)"></v-list-item>
                                </v-list>
                                <div class="px-4 pb-2" v-else>
                                    <v-btn color="primary" variant="tonal" :disabled="updating || !canConvert" @click="convert('')">{{ tt('Convert to Invoice') }}</v-btn>
                                </div>
                            </template>
                        </template>
```

- the Suggested Matches block condition becomes `v-if="!isEstimate && displayStatus !== 'paid'"`.
- the share dialog text: `{{ tt(isEstimate ? 'Anyone with this link can view this estimate and accept or decline it.' : 'Anyone with this link can view this invoice.') }}`; the send dialog title: `{{ tt(isEstimate ? 'Send Estimate' : 'Send Invoice') }}`.
- script: `import { ESTIMATE_STATUS_COLORS } from '@/views/base/invoices/EstimateListPageBase.ts';`, `import { formatMilestonePercent } from '@/lib/estimate.ts';`; destructure `formatDate, isEstimate, estimateStatus, estimateStatusName, estimateResponseText, canEdit, canMarkResponse, canReopenEstimate, canConvert, convertNeedsConfirm, milestoneAmounts, setEstimateResponse, reopenEstimate` from `useInvoiceDetailPageBase()`.
- in `load`, change `if (displayStatus.value === 'paid') {` to `if (displayStatus.value === 'paid' || loadedInvoice.kind === 'estimate') {`.
- in `remove`, the confirm text is `isEstimate.value ? 'Are you sure you want to delete this estimate?' : 'Are you sure you want to delete this invoice?'`; in `send`, the message is `isEstimate.value ? 'Estimate has been sent' : 'Invoice has been sent'`.
- add:

```ts
// opens another document in this dialog: the estimate an invoice came from, or an invoice made from this estimate
function openOther(id: string): void {
    invoiceId.value = id;
    invoice.value = null;
    suggestions.value = [];
    load();
}

function markResponse(status: 'accepted' | 'declined'): void {
    runAndReload(() => setEstimateResponse(status));
}

function reopen(): void {
    runAndReload(() => reopenEstimate());
}

// opens the editor with the invoice the estimate (milestoneId '') or a milestone turns into; saving it opens the new invoice here
function convert(milestoneId: string): void {
    const current = invoice.value;

    if (!current) {
        return;
    }

    const start = (): void => {
        invoiceEditDialog.value?.open({ convertFrom: { estimateId: current.id, milestoneId } }).then(result => {
            snackbar.value?.showMessage(result.message);
            changed.value = true;
            openOther(result.invoice.id);
        }).catch(error => {
            if (error && !error.processed) {
                snackbar.value?.showError(error);
            }
        });
    };

    if (convertNeedsConfirm.value) {
        confirmDialog.value?.open("This estimate isn't accepted yet. Convert anyway?").then(start);
    } else {
        start();
    }
}
```

- [ ] **Step 5: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views src/components && npx vitest run --exclude '.claude/**' src/lib/__tests__`
Expected: clean and PASS.

- [ ] **Step 6: Commit**

```bash
git add src/views src/components src/locales/en.json
git commit --no-verify -m "estimates: desktop estimate detail with status, milestones and convert; the document prints estimates"
```

---

### Task 11: Public page: Accept and Decline

**Files:**
- Modify: `src/views/desktop/invoices/PublicInvoicePage.vue`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 5, 8, 10): `services.respondToPublicEstimate`, `services.getPublicInvoice`, `Invoice.ofPublic` (kind, estimateStatus, respondedUnixTime, milestones), `canAnswerEstimate`, `getEstimateDisplayStatus`, `unixTimeToTextualDate`, `formatInvoiceDocumentDate`, the estimate-aware document from Task 10.
- Produces: the `/i/<token>` page shows "Estimate" documents with Accept / Decline (optional note, 500 characters) while sent and valid; "Accepted on <date>" / "Declined on <date>" / "This estimate expired on <date>" otherwise.

- [ ] **Step 1: Locale strings**

Add to `src/locales/en.json` (top level): `"Accept": "Accept"`, `"Decline": "Decline"`, `"Decline Estimate": "Decline Estimate"`, `"Note (optional)": "Note (optional)"`, `"This estimate can no longer be answered.": "This estimate can no longer be answered."`; inside `"format"` → `"misc"`:

```json
            "estimateAcceptedOn": "Accepted on {date}",
            "estimateDeclinedOn": "Declined on {date}",
            "estimateExpiredOn": "This estimate expired on {date}",
```

- [ ] **Step 2: Page**

In `src/views/desktop/invoices/PublicInvoicePage.vue` replace the toolbar and message lines of the template with:

```vue
        <div class="public-invoice-toolbar" v-if="invoice">
            <span class="public-invoice-status" v-if="isEstimate">
                {{ estimateStatusText || tt('Total') + ': ' + formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}
            </span>
            <span class="public-invoice-status" :class="{ 'public-invoice-status-paid': isPaid }" v-else>
                {{ isPaid ? tt('Paid') : tt('Amount Due') + ': ' + formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}
            </span>
            <v-spacer />
            <template v-if="canAnswer">
                <v-btn color="success" :loading="answering" :disabled="answering" @click="answer(true)">{{ tt('Accept') }}</v-btn>
                <v-btn color="error" variant="tonal" :disabled="answering" @click="showDeclineDialog = true">{{ tt('Decline') }}</v-btn>
            </template>
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiPrinterOutline" @click="print">{{ tt('Print') }}</v-btn>
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiDownloadOutline" :loading="downloadingPdf" @click="downloadPdf">{{ tt('Download PDF') }}</v-btn>
        </div>

        <div class="public-invoice-message" role="alert" v-if="answerError">{{ tt('This estimate can no longer be answered.') }}</div>
        <div class="public-invoice-message" role="alert" v-if="downloadError">{{ tt('Unable to download invoice PDF') }}</div>
```

and add before the closing root `</div>`:

```vue
        <v-dialog width="480" v-model="showDeclineDialog">
            <v-card>
                <v-card-title>{{ tt('Decline Estimate') }}</v-card-title>
                <v-card-text>
                    <v-textarea rows="3" auto-grow counter="500" maxlength="500" :disabled="answering"
                                :label="tt('Note (optional)')" v-model="declineNote" />
                </v-card-text>
                <v-card-actions>
                    <v-spacer />
                    <v-btn :disabled="answering" @click="showDeclineDialog = false">{{ tt('Cancel') }}</v-btn>
                    <v-btn color="error" :loading="answering" @click="answer(false)">{{ tt('Decline') }}</v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>
```

In the script:
- imports: `import { canAnswerEstimate, getEstimateDisplayStatus, unixTimeToTextualDate } from '@/lib/estimate.ts';` and add `formatInvoiceDocumentDate` to the `@/lib/invoice.ts` import.
- state and computed:

```ts
const today = getTodayTextualDate();
const answering = ref<boolean>(false);
const answerError = ref<boolean>(false);
const showDeclineDialog = ref<boolean>(false);
const declineNote = ref<string>('');

const isEstimate = computed<boolean>(() => invoice.value?.kind === 'estimate');
// the server checks the same: sent and still valid
const canAnswer = computed<boolean>(() => !!invoice.value && canAnswerEstimate(invoice.value, today));

const estimateStatusText = computed<string>(() => {
    const current = invoice.value;

    if (!current || !isEstimate.value) {
        return '';
    }

    if ((current.estimateStatus === 'accepted' || current.estimateStatus === 'declined') && current.respondedUnixTime) {
        const date = formatInvoiceDocumentDate(unixTimeToTextualDate(current.respondedUnixTime));
        return tt(current.estimateStatus === 'accepted' ? 'format.misc.estimateAcceptedOn' : 'format.misc.estimateDeclinedOn', { date });
    }

    if (getEstimateDisplayStatus(current, today) === 'expired') {
        return tt('format.misc.estimateExpiredOn', { date: formatInvoiceDocumentDate(current.dueDate) });
    }

    return '';
});
```

- turn the trailing `services.getPublicInvoice(...)` call into a function and call it once:

```ts
function load(): Promise<void> {
    return services.getPublicInvoice({ token: props.token }).then(response => {
        const data = response.data;

        if (data && data.success && data.result) {
            invoice.value = Invoice.ofPublic(data.result);
            profile.value = InvoiceProfile.ofPublic(data.result.profile);
            document.title = data.result.number;
        }
    }).catch(() => {
        // unknown or disabled link: the page shows "not available" below
    }).finally(() => {
        loading.value = false;
    });
}

// the answer is recorded once; whatever happened, the page reloads the estimate to show its real state
function answer(accept: boolean): void {
    answering.value = true;
    answerError.value = false;

    services.respondToPublicEstimate({ token: props.token, accept, note: accept ? '' : declineNote.value.trim() }).then(response => {
        answerError.value = !response.data?.success;
    }).catch(() => {
        answerError.value = true;
    }).finally(() => {
        answering.value = false;
        showDeclineDialog.value = false;
        load();
    });
}

load();
```

- `print` calls `printInvoiceDocument(invoice.value)` (the kind now picks the file name) and `downloadPdf` uses `getInvoicePdfFileName(current.number, current.dueDate, current.kind)`.

- [ ] **Step 3: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add src/views/desktop/invoices/PublicInvoicePage.vue src/locales/en.json
git commit --no-verify -m "estimates: accept and decline on the public estimate page"
```

---

### Task 12: Mobile estimates: list, editor and detail

**Files:**
- Create: `src/views/mobile/invoices/EstimateListPage.vue`
- Modify: `src/router/mobile.ts`
- Modify: `src/views/mobile/invoices/ListPage.vue` (More menu)
- Modify: `src/views/mobile/invoices/EditPage.vue`
- Modify: `src/views/mobile/invoices/DetailPage.vue`
- Modify: `src/views/mobile/invoices/SendPage.vue`

**Interfaces:**
- Consumes (Tasks 8–10): `useEstimateListPageBase`, `isEstimateLocked`, store `loadAllEstimates`, `deleteInvoice`, `getEstimateInvoiceDraft`; edit base `isEstimate`, `numberLabel`, `dateLabel`, `dueDateLabel`, `milestoneAmounts`, `milestonePercentTotal`, `milestonesProblem`, `addMilestone`, `removeMilestone`, `prepareConvertedInvoice`, `prepareNewInvoice(profile, duplicateFrom, kind)`; detail base `isEstimate`, `estimateStatusName`, `estimateResponseText`, `canEdit`, `canMarkResponse`, `canReopenEstimate`, `canConvert`, `convertNeedsConfirm`, `milestoneAmounts`, `setEstimateResponse`, `reopenEstimate`; `formatMilestonePercent`, `ESTIMATE_MAX_MILESTONES`; locale keys from Tasks 8–10.
- Produces: route `/invoice/estimate/list`; `/invoice/add?kind=estimate` (new estimate); `/invoice/add?fromEstimate=<id>&milestoneId=<id or empty>` (conversion); `/invoice/detail?id=` and `/invoice/edit?id=` work for estimates.

- [ ] **Step 1: Route and entry point**

In `src/router/mobile.ts` import `import InvoiceEstimateListPage from '@/views/mobile/invoices/EstimateListPage.vue';` next to the other invoice pages and add after the `/invoice/recurring/list` route:

```ts
    {
        path: '/invoice/estimate/list',
        async: asyncResolve(InvoiceEstimateListPage),
        beforeEnter: [checkLogin]
    },
```

In `src/views/mobile/invoices/ListPage.vue` add to the More action sheet, after Recurring:

```vue
                <f7-actions-button @click="f7router.navigate('/invoice/estimate/list')">{{ tt('Estimates') }}</f7-actions-button>
```

- [ ] **Step 2: Estimate list page**

Create `src/views/mobile/invoices/EstimateListPage.vue`:

```vue
<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Estimates')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/add?kind=estimate"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="EST/2026-27/01" after="₹000,000.00" footer="Customer · Sent" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableEstimate">
            <f7-list-item :title="tt('No available estimate')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !noAvailableEstimate">
            <f7-list-item swipeout
                          :id="getEstimateDomId(estimate)"
                          :title="estimate.number"
                          :after="formatAmount(estimate.total, estimate.currency)"
                          :link="'/invoice/detail?id=' + estimate.id"
                          :key="estimate.id"
                          v-for="estimate in displayEstimates">
                <template #footer>
                    <div>{{ estimate.billToName }} · {{ tt(getStatusName(estimate)) }} · {{ formatDate(estimate.dueDate) }}</div>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" v-if="!isEstimateLocked(estimate)" @click="edit(estimate)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(estimate, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showClosed" @click="showClosed = true">{{ tt('Show Closed Estimates') }}</f7-actions-button>
                <f7-actions-button v-if="showClosed" @click="showClosed = false">{{ tt('Hide Closed Estimates') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this estimate?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(estimateToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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
import { useEstimateListPageBase } from '@/views/base/invoices/EstimateListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Invoice } from '@/models/invoice.ts';

import { isEstimateLocked } from '@/lib/estimate.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showClosed,
    displayEstimates,
    noAvailableEstimate,
    getStatusName,
    formatAmount,
    formatDate
} = useEstimateListPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const estimateToDelete = ref<Invoice | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getEstimateDomId(estimate: Invoice): string {
    return 'estimate_' + estimate.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllEstimates({ force: false }).then(() => {
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

    invoicesStore.loadAllEstimates({ force: force }).then(() => {
        done?.();
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Estimate list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(estimate: Invoice): void {
    props.f7router.navigate(`/invoice/edit?id=${estimate.id}`);
}

function remove(estimate: Invoice | null, confirm: boolean): void {
    if (!estimate) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        estimateToDelete.value = estimate;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    estimateToDelete.value = null;
    showLoading();

    invoicesStore.deleteInvoice({
        invoice: estimate,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getEstimateDomId(estimate), done);
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

- [ ] **Step 3: Mobile editor**

In `src/views/mobile/invoices/EditPage.vue`:
- customer item: add `:class="{ 'disabled': !!invoice.estimateId }"`; currency item: `:class="{ 'disabled': !!editInvoiceId || !!invoice.estimateId }"` and `:no-chevron="!!editInvoiceId || !!invoice.estimateId"`.
- number input: `:label="tt(numberLabel)" :placeholder="tt(numberLabel)"`; date items: `:header="tt(dateLabel)"` and `:header="tt(dueDateLabel)"`.
- right after the lines `</f7-block>` add:

```vue
            <f7-list strong inset dividers class="margin-vertical" v-if="isEstimate">
                <!-- list-number-input must go in #list, so the whole block goes there to keep its order -->
                <template #list>
                    <f7-list-item group-title>
                        <small>{{ tt('Milestones') }}</small>
                        <small v-if="invoice.milestones.length">{{ milestonesProblem ? tt(milestonesProblem) : formatMilestonePercent(milestonePercentTotal) + ' ✓' }}</small>
                    </f7-list-item>
                    <template :key="index" v-for="(milestone, index) in invoice.milestones">
                        <f7-list-input type="text" autocomplete="off" clear-button
                                       :label="tt('Milestone') + ' #' + (index + 1)" :placeholder="tt('Milestone')"
                                       v-model:value="milestone.label"></f7-list-input>
                        <list-number-input :label="tt('Percent (%)')" :placeholder="tt('Percent (%)')"
                                           :min-value="0.01" :max-value="100" :max-decimal-count="2"
                                           :model-value="milestone.percent / 100"
                                           @update:model-value="milestone.percent = Math.round($event * 100)"></list-number-input>
                        <f7-list-item :title="tt('Amount')" :after="formatAmount(milestoneAmounts[index] ?? 0)"></f7-list-item>
                        <f7-list-button color="red" :title="tt('Remove')" @click="removeMilestone(index)"></f7-list-button>
                    </template>
                    <f7-list-button :class="{ 'disabled': invoice.milestones.length >= ESTIMATE_MAX_MILESTONES }"
                                    :title="tt('Add Milestone')" @click="addMilestone"></f7-list-button>
                </template>
            </f7-list>
```

- More action sheet, first group: add `<f7-actions-button v-if="isEstimate" @click="addMilestone">{{ tt('Add Milestone') }}</f7-actions-button>`.
- script: import `{ ESTIMATE_MAX_MILESTONES, formatMilestonePercent } from '@/lib/estimate.ts'`; destructure `isEstimate, numberLabel, dateLabel, dueDateLabel, milestoneAmounts, milestonePercentTotal, milestonesProblem, addMilestone, removeMilestone, prepareConvertedInvoice`.
- in `init` replace the `Promise.all` with:

```ts
    const fromEstimate = query['fromEstimate'] || '';

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        invoicesStore.loadAllProducts({ force: false }).catch(() => []),
        query['id'] ? invoicesStore.getInvoice({ invoiceId: query['id'] })
            : fromEstimate ? invoicesStore.getEstimateInvoiceDraft({ estimateId: fromEstimate, milestoneId: query['milestoneId'] || '' }) : Promise.resolve(null),
        query['duplicateFrom'] ? invoicesStore.getInvoice({ invoiceId: query['duplicateFrom'] }) : Promise.resolve(null)
    ]).then(([, loadedProfile, , loadedInvoice, duplicateFrom]) => {
        profile.value = loadedProfile;

        if (loadedInvoice && query['id']) {
            setInvoice(loadedInvoice, loadedProfile);
        } else if (loadedInvoice) {
            prepareConvertedInvoice(loadedProfile, loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, duplicateFrom, query['kind'] === 'estimate' ? 'estimate' : 'invoice');
            suggestNumber();
        }

        loading.value = false;
    })
```

(keep the existing `.catch`).
- in `save`, the toast becomes `isEstimate.value ? (editInvoiceId.value ? 'You have saved this estimate' : 'You have added a new estimate') : (editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice')`, and `if (props.f7route.query['duplicateFrom'])` becomes `if (props.f7route.query['duplicateFrom'] || props.f7route.query['fromEstimate'])` (open the new invoice in place of the editor).

- [ ] **Step 4: Mobile detail and send**

In `src/views/mobile/invoices/DetailPage.vue`:
- navbar title stays `invoice.number`.
- replace the first summary `f7-list` (Status … Balance Due) with:

```vue
            <f7-list strong inset dividers class="margin-vertical" v-if="isEstimate">
                <f7-list-item :title="tt('Status')" :after="tt(estimateStatusName)"></f7-list-item>
                <f7-list-item :title="tt('Sent on')" :after="sentOnText" v-if="sentOnText"></f7-list-item>
                <f7-list-item :title="tt('Customer')" :after="invoice.billToName"></f7-list-item>
                <f7-list-item :title="tt('Valid Until')" :after="formatDate(invoice.dueDate)"></f7-list-item>
                <f7-list-item :title="tt('Total')" :after="formatAmount(invoice.total)"></f7-list-item>
                <f7-list-item :title="estimateResponseText" v-if="estimateResponseText"></f7-list-item>
                <f7-list-item :title="tt(`Client's note`)" :footer="invoice.responseNote" v-if="invoice.responseNote"></f7-list-item>
            </f7-list>
            <f7-list strong inset dividers class="margin-vertical" v-else>
                <f7-list-item :title="tt('Status')" :after="tt(displayStatusName)"></f7-list-item>
                <f7-list-item :title="tt('Sent on')" :after="sentOnText" v-if="sentOnText"></f7-list-item>
                <f7-list-item :title="tt('Customer')" :after="invoice.billToName"></f7-list-item>
                <f7-list-item :title="tt('Payment Due')" :after="formatDate(invoice.dueDate)"></f7-list-item>
                <f7-list-item :title="tt('Total')" :after="formatAmount(invoice.total)"></f7-list-item>
                <f7-list-item :title="tt('Received')" :after="formatAmount(invoice.received)"></f7-list-item>
                <f7-list-item :title="tt(shortfallName)" :after="formatAmount(shortfall)" v-if="shortfall > 0"></f7-list-item>
                <f7-list-item :title="tt('Balance Due')" :after="formatAmount(balanceDue)" v-else></f7-list-item>
                <f7-list-item :title="tt('format.misc.fromEstimate', { number: invoice.estimateNumber })"
                              :link="'/invoice/detail?id=' + invoice.estimateId" v-if="invoice.estimateId"></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical" v-if="isEstimate">
                <f7-list-item group-title>
                    <small>{{ tt(invoice.milestones.length ? 'Milestones' : 'Invoices') }}</small>
                </f7-list-item>
                <template v-if="invoice.milestones.length">
                    <f7-list-item :key="milestone.id" v-for="(milestone, index) in invoice.milestones"
                                  :title="milestone.label"
                                  :footer="`${formatMilestonePercent(milestone.percent)} · ${formatAmount(milestoneAmounts[index] ?? 0)}`"
                                  :link="milestone.invoiceId ? '/invoice/detail?id=' + milestone.invoiceId : undefined">
                        <template #after>
                            <span v-if="milestone.invoiceId">{{ tt('format.misc.invoicedAs', { number: milestone.invoiceNumber }) }}</span>
                            <f7-button small fill :class="{ 'disabled': updating || !canConvert }" :text="tt('Create Invoice')"
                                       @click="convert(milestone.id)" v-else></f7-button>
                        </template>
                    </f7-list-item>
                </template>
                <template v-else>
                    <f7-list-item :key="converted.id" v-for="converted in invoice.convertedInvoices"
                                  :title="tt('format.misc.invoicedAs', { number: converted.number })"
                                  :link="'/invoice/detail?id=' + converted.id"></f7-list-item>
                    <f7-list-button :class="{ 'disabled': updating || !canConvert }" :title="tt('Convert to Invoice')"
                                    @click="convert('')" v-if="!invoice.convertedInvoices.length"></f7-list-button>
                </template>
            </f7-list>
```

- the Payments list gets `v-if="!isEstimate"`; the Suggested Matches list condition becomes `v-if="!isEstimate && displayStatus !== 'paid'"`.
- More action sheet, first group:

```vue
                <f7-actions-button v-if="canEdit" @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/add?duplicateFrom=${invoiceId}`)">{{ tt('Duplicate') }}</f7-actions-button>
                <f7-actions-button v-if="!isEstimate" @click="regenerate">{{ tt('Regenerate with Current Customer Details') }}</f7-actions-button>
                <f7-actions-button v-if="!isEstimate" @click="toggleClosed">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</f7-actions-button>
                <f7-actions-button v-if="canMarkResponse" @click="markResponse('accepted')">{{ tt('Mark Accepted') }}</f7-actions-button>
                <f7-actions-button v-if="canMarkResponse" @click="markResponse('declined')">{{ tt('Mark Declined') }}</f7-actions-button>
                <f7-actions-button v-if="canReopenEstimate" @click="reopen">{{ tt('Reopen') }}</f7-actions-button>
                <f7-actions-button @click="printInvoice">{{ tt('Print') }}</f7-actions-button>
                <f7-actions-button v-if="!isEstimate" @click="showRepeatMonthlyPopup = true">{{ tt('Repeat') }}</f7-actions-button>
                <f7-actions-button @click="share">{{ tt('Share Link') }}</f7-actions-button>
                <f7-actions-button @click="downloadPdf" :disabled="downloadingPdf">{{ tt('Download PDF') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/send?id=${invoiceId}`)">{{ tt('Send') }}</f7-actions-button>
```

- the delete label: `{{ tt(isEstimate ? 'Are you sure you want to delete this estimate?' : 'Are you sure you want to delete this invoice?') }}`; the share label: `{{ tt(isEstimate ? 'Anyone with this link can view this estimate and accept or decline it.' : 'Anyone with this link can view this invoice.') }}`.
- script: `import { formatMilestonePercent } from '@/lib/estimate.ts';`; destructure `isEstimate, estimateStatusName, estimateResponseText, canEdit, canMarkResponse, canReopenEstimate, canConvert, convertNeedsConfirm, milestoneAmounts, setEstimateResponse, reopenEstimate` from `useInvoiceDetailPageBase()`; in `load` change `if (displayStatus.value === 'paid') {` to `if (displayStatus.value === 'paid' || loadedInvoice.kind === 'estimate') {`; add:

```ts
function markResponse(status: 'accepted' | 'declined'): void {
    showMoreActionSheet.value = false;
    runAndReload(() => setEstimateResponse(status));
}

function reopen(): void {
    showMoreActionSheet.value = false;
    runAndReload(() => reopenEstimate());
}

// the editor opens with the invoice the estimate (milestoneId '') or a milestone turns into; saving it opens the new invoice
function convert(milestoneId: string): void {
    const open = (): void => {
        props.f7router.navigate(`/invoice/add?fromEstimate=${invoiceId.value}&milestoneId=${milestoneId}`);
    };

    if (convertNeedsConfirm.value) {
        showConfirm("This estimate isn't accepted yet. Convert anyway?", open);
    } else {
        open();
    }
}
```

In `src/views/mobile/invoices/SendPage.vue`: the nav title becomes `:title="tt(invoice?.kind === 'estimate' ? 'Send Estimate' : 'Send Invoice')"` and the toast `showToast(invoice.value?.kind === 'estimate' ? 'Estimate has been sent' : 'Invoice has been sent')`.

- [ ] **Step 5: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/mobile src/router`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add src/views/mobile src/router/mobile.ts
git commit --no-verify -m "estimates: mobile estimate list, editor with milestones and estimate detail"
```

---

### Task 13: Home "Needs attention", estimate profile settings and the customer timeline (desktop + mobile)

**Files:**
- Modify: `src/views/base/invoices/RenewalsCardBase.ts`
- Modify: `src/views/desktop/invoices/RenewalsCard.vue`, `src/views/mobile/invoices/RenewalsCard.vue`
- Modify: `src/views/base/invoices/InvoiceProfilePageBase.ts`
- Modify: `src/views/desktop/settings/InvoiceProfileSettingPage.vue`, `src/views/mobile/invoices/ProfilePage.vue`
- Modify: `src/views/base/invoices/CustomerDetailPageBase.ts`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 7–8): store `loadEstimatesToInvoice`, `loadInvoiceRenewals`; `InvoiceProfile.estimateNumberPrefix/estimateValidDays`; timeline kinds `estimate`, `estimate_accepted`, `estimate_declined`, fields `estimateNumber`, `byClient`.
- Produces: `useRenewalsCardBase()` adds `estimates: Ref<Invoice[]>` and `getEstimateTitle(estimate)`; the card is titled "Needs Attention" and shows while there are renewals or accepted estimates to invoice; profile validation messages for the two estimate fields; `getTimelineTitle` names estimate events; customer Billed / Paid / Outstanding count invoices only.

- [ ] **Step 1: Locale strings**

Add to `src/locales/en.json` (top level): `"Needs Attention": "Needs Attention"`, `"Convert": "Convert"`, `"Estimate Number Prefix": "Estimate Number Prefix"`, `"Estimate Valid For (days)": "Estimate Valid For (days)"`, `"Estimate number prefix cannot be blank": "Estimate number prefix cannot be blank"`, `"Estimate validity must be between 1 and 365 days": "Estimate validity must be between 1 and 365 days"`; inside `"format"` → `"misc"`:

```json
            "estimateAcceptedBy": "Estimate accepted by {customer}",
            "timelineEstimate": "Estimate {number}",
            "timelineEstimateAccepted": "Estimate {number} marked accepted",
            "timelineEstimateAcceptedByClient": "Estimate {number} accepted by the client",
            "timelineEstimateDeclined": "Estimate {number} marked declined",
            "timelineEstimateDeclinedByClient": "Estimate {number} declined by the client",
            "timelineInvoiceFromEstimate": "Invoice {number} from estimate {estimate}",
```

- [ ] **Step 2: Needs attention card**

In `src/views/base/invoices/RenewalsCardBase.ts`:
- state `const estimates = ref<Invoice[]>([]);`
- replace `load`:

```ts
    // renewals and accepted estimates load independently; a failing list just leaves its rows out
    function load(): Promise<void> {
        return Promise.all([
            invoicesStore.loadInvoiceRenewals().catch(() => [] as InvoiceRenewal[]),
            invoicesStore.loadEstimatesToInvoice().catch(() => [] as Invoice[])
        ]).then(([loadedRenewals, loadedEstimates]) => {
            renewals.value = loadedRenewals;
            estimates.value = loadedEstimates;
        });
    }
```

- add

```ts
    function getEstimateTitle(estimate: Invoice): string {
        return tt('format.misc.estimateAcceptedBy', { customer: estimate.billToName });
    }
```

- return `estimates` (states) and `getEstimateTitle` (functions); update the comment above the function to `// the home "Needs attention" card: renewal drafts, failed and recent auto-sends, and accepted estimates not fully invoiced`.

In `src/views/desktop/invoices/RenewalsCard.vue`: the card becomes `<v-card class="mb-4" v-if="renewals.length || estimates.length">` titled `{{ tt('Needs Attention') }}`; after the renewal items, inside the same `v-list`:

```vue
            <v-list-item :key="estimate.id" v-for="estimate in estimates"
                         :title="`${getEstimateTitle(estimate)} · ${estimate.number}`"
                         :subtitle="formatAmount(estimate)">
                <template #append>
                    <v-btn color="primary" variant="tonal" size="small" :to="'/invoice/list?invoiceId=' + estimate.id">{{ tt('Convert') }}</v-btn>
                </template>
            </v-list-item>
```

and destructure `estimates, getEstimateTitle` from `useRenewalsCardBase()`. (`/invoice/list?invoiceId=` opens the estimate's detail, where Convert / Create invoice live.)

In `src/views/mobile/invoices/RenewalsCard.vue`: `<template v-if="renewals.length || estimates.length">`, block title `{{ tt('Needs Attention') }}`, and after the renewal items:

```vue
            <f7-list-item :key="estimate.id" v-for="estimate in estimates"
                          :link="'/invoice/detail?id=' + estimate.id"
                          :title="getEstimateTitle(estimate)"
                          :subtitle="`${estimate.number} · ${formatAmount(estimate)}`"
                          :footer="tt('Convert')">
            </f7-list-item>
```

with `estimates, getEstimateTitle` destructured.

- [ ] **Step 3: Profile settings**

In `src/views/base/invoices/InvoiceProfilePageBase.ts`, `inputInvalidProblemMessage` gains before `return null;`:

```ts
        if (!newProfile.value.estimateNumberPrefix.trim()) {
            return 'Estimate number prefix cannot be blank';
        } else if (newProfile.value.estimateValidDays < 1 || newProfile.value.estimateValidDays > 365) {
            return 'Estimate validity must be between 1 and 365 days';
        }
```

In `src/views/desktop/settings/InvoiceProfileSettingPage.vue` after the Invoice Number Prefix item:

```vue
                        <div class="setting-item">
                            <span>{{ tt('Estimate Number Prefix') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Estimate Number Prefix')"
                                          v-model="newProfile.estimateNumberPrefix" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Estimate Valid For (days)') }}</span>
                            <v-spacer/>
                            <number-input class="ms-4" density="compact" max-width="400px"
                                          :disabled="loading || saving"
                                          :min-value="1" :max-value="365" :max-decimal-count="0"
                                          v-model="newProfile.estimateValidDays" />
                        </div>
```

In `src/views/mobile/invoices/ProfilePage.vue` after the Invoice Number Prefix input:

```vue
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('Estimate Number Prefix')" :placeholder="tt('Estimate Number Prefix')"
                               v-model:value="newProfile.estimateNumberPrefix"></f7-list-input>
                <list-number-input :label="tt('Estimate Valid For (days)')" :placeholder="tt('Estimate Valid For (days)')"
                                   :min-value="1" :max-value="365" :max-decimal-count="0"
                                   v-model="newProfile.estimateValidDays"></list-number-input>
```

- [ ] **Step 4: Customer page**

In `src/views/base/invoices/CustomerDetailPageBase.ts`:
- `invoices` filters invoices only: `invoicesStore.allInvoices.filter(invoice => invoice.kind === 'invoice' && invoice.customerId === customer.value?.id)` (estimates never count in Billed / Paid / Outstanding).
- replace `getTimelineTitle`:

```ts
    function getTimelineTitle(item: CustomerTimelineItem): string {
        switch (item.kind) {
            case 'email':
                return tt('format.misc.timelineEmail', { number: item.invoiceNumber, subject: item.text });
            case 'payment':
                return tt('format.misc.timelinePayment', { number: item.invoiceNumber });
            case 'estimate':
                return tt('format.misc.timelineEstimate', { number: item.invoiceNumber });
            case 'estimate_accepted':
                return tt(item.byClient ? 'format.misc.timelineEstimateAcceptedByClient' : 'format.misc.timelineEstimateAccepted', { number: item.invoiceNumber });
            case 'estimate_declined':
                return tt(item.byClient ? 'format.misc.timelineEstimateDeclinedByClient' : 'format.misc.timelineEstimateDeclined', { number: item.invoiceNumber });
            default:
                if (item.estimateNumber) {
                    return tt('format.misc.timelineInvoiceFromEstimate', { number: item.invoiceNumber, estimate: item.estimateNumber });
                }

                return tt(item.renewal ? 'format.misc.timelineRenewal' : 'format.misc.timelineInvoice', { number: item.invoiceNumber });
        }
    }
```

(The desktop dialog and mobile page already open `item.invoiceId` on click, which opens estimates too.)

- [ ] **Step 5: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views && npx vitest run --exclude '.claude/**' src/lib/__tests__`
Expected: clean and PASS.

- [ ] **Step 6: Commit**

```bash
git add src/views src/locales/en.json
git commit --no-verify -m "estimates: needs-attention home card, estimate profile settings and customer timeline events"
```

---

### Task 14: Full verification and deploy

**Files:** none new.

- [ ] **Step 1: Whole test suites**

Run: `go test ./... -count=1 && npx vitest run --exclude '.claude/**' && npx vue-tsc --noEmit && npx eslint .`
Expected: all PASS / clean. Fix anything red before continuing; do not skip.

- [ ] **Step 2: Manual pass on a copy of real data**

With a local server on a copy of the production database (never the live one), at desktop width and at 375px phone width:
1. Invoice Profile: Estimate Number Prefix shows `EST`, validity 30; existing invoices, outstanding and overdue totals are unchanged.
2. Estimates view → Add: title "Add Estimate", number `EST/2026-27/01`, Valid Until = today + 30; add three milestones 33.33 / 33.33 / 33.34 — the chip reads "100% ✓" and Save is blocked at 99.99%.
3. Detail: document titled "Estimate" with the Payment Terms table; Download PDF (file `Estimate_...pdf`) and Print show the same.
4. Send the estimate to your own address: subject "Estimate EST/... from <business>", status Sent.
5. Open the link in a private window: Accept → "Accepted on <date>", no buttons; your account email gets "<customer> accepted Estimate ...". On a second estimate: Decline with a note → note shown in the app as plain text. On an estimate with a past Valid Until: "This estimate expired on <date>", no buttons.
6. Home card "Needs Attention" lists the accepted estimate → Convert opens it; Create invoice on each milestone in a different order; the three invoices add up to the estimate subtotal; the invoice shows "From estimate <number>"; the estimate shows "Invoiced" once all are done and leaves the card.
7. Delete one milestone invoice → the milestone offers Create invoice again; deleting the estimate is refused while invoices exist.
8. Mark accepted / declined and Reopen; editing is blocked once accepted.
9. Customer page: estimate created / accepted / declined / converted events; Billed / Paid / Outstanding unchanged by estimates.
10. Mobile: Invoices → More → Estimates; add, edit with milestones, detail actions, convert, send.

- [ ] **Step 3: Deploy (ask first)**

The controller must ask the user before pushing or deploying. Only after an explicit yes: follow the project deploy routine (push → GHCR image → Dokploy API redeploy for books.codeation.io). After deploy, open books.codeation.io and confirm: the Invoices page has the Estimates view, an existing invoice still opens with its old totals, and a public estimate link answers once.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit --no-verify -m "estimates: verification fixes"
```
