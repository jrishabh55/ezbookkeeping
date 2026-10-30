# Estimate Revisions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Change an estimate after the client has seen it without losing what was sent: a sent or answered estimate is never edited in place; **Revise** creates the next version as a new draft, and every earlier version stays complete, read-only and reachable (in the app and through its old public link).

**Architecture:** A revision is another `invoice` row with `Kind = "estimate"` and three new columns: `RevisionOf` (the chain's first version, 0 on it), `RevisionNumber` (0, 1, 2, …) and `SupersededFromStatus`. A new stored status `superseded` marks every version a newer one replaced, so "latest version" is exactly "not superseded". `POST /api/v1/estimates/revise.json` locks the old row (`lockInvoiceRow`), re-checks it, inserts the copy as a draft numbered `<first number>-R<n>` and supersedes the old row conditionally on its status, all in one transaction. `ModifyEstimate` becomes draft-only; conversion and delete refuse superseded rows (mark / reopen / public answer already only take draft, sent, accepted or declined). Deleting a draft revision restores the previous version's `SupersededFromStatus`. The detail response gains `revisionNumber` and `revisions` (the chain, newest first), the public response gains `supersededBy` (the latest version's token or empty), the estimate list returns latest versions unless `include_superseded=true`, and the customer timeline gets an `estimate_revised` event.

**Tech Stack:** Go (gin, xorm, testify) backend; Vue 3 + Pinia + Vuetify (desktop) + Framework7 (mobile) frontend; vitest.

**Spec:** `docs/superpowers/specs/2026-10-30-estimate-revisions-design.md`

## Global Constraints

- New `Invoice` columns: `RevisionOf` int64 `NOT NULL DEFAULT 0` (indexed with uid as `IDX_invoice_uid_revision_of`), `RevisionNumber` int32 `NOT NULL DEFAULT 0`, `SupersededFromStatus` `VARCHAR(16) NOT NULL DEFAULT ''`. Existing rows are first versions (all three zero / empty); no data migration (xorm `Sync2` already covers the `Invoice` table in `cmd/database.go` and `pkg/services/alerts_testutil_test.go`).
- `RevisionOf` = id of the chain's first version; 0 on the first version itself and on every invoice. `RevisionNumber` = 0 on the first version, then 1, 2, ….
- New stored status `superseded` (`models.ESTIMATE_STATUS_SUPERSEDED`, TS `'superseded'`), display name `Superseded`. Stored estimate statuses are now `draft`, `sent`, `accepted`, `declined`, `superseded`.
- Revision number = first version's number + `-R<n>`, e.g. `EST/2026-27/01-R2`; n = highest `RevisionNumber` in the chain **including deleted rows** + 1 (never reused).
- The per-financial-year estimate sequence ignores revision suffixes: a number `<base>01-R2` counts as `01`, so the next new estimate after `EST/2026-27/01-R2` is `EST/2026-27/02`.
- Revise is allowed only on the latest version of a chain whose status is `sent` (expired included), `declined`, or `accepted` with no non-deleted invoice made from it; refused with `errs.ErrEstimateCannotBeRevised` (code **70**, message `estimate cannot be revised`) otherwise.
- The revision copies customer, currency, tax type and rate, notes, lines (item, description, quantity, rate, SAC, product) and milestones (label, percent, `InvoiceId = 0`); it is a `draft` dated today, valid until today + profile `EstimateValidDays` (30 when unset), with no public token, not sent, no response.
- The old version is set to `superseded` with `SupersededFromStatus` = its previous status, conditionally on its status still being the one read after the lock; concurrent Revise clicks create exactly one revision.
- `ModifyEstimate` accepts only `draft` (was draft or sent); its refusal stays `ErrEstimateLocked` (code 61) with the new message `only a draft estimate can be edited, revise it instead`.
- A superseded version is read-only: no edit, revise, answer, convert, mark accepted / declined, reopen or delete. Superseded delete → `ErrEstimateStatusInvalid` (63); superseded convert → `ErrEstimateStatusInvalid`; public answer → `ErrEstimateCannotBeAnswered` (67).
- Deleting: a draft revision (`RevisionNumber > 0`, `draft`) deletes and in the same transaction the previous non-deleted version returns from `superseded` to its `SupersededFromStatus` (empty → `sent`); deleting the latest version otherwise (a first version, or a sent / answered revision) deletes the whole chain in one transaction when nothing is invoiced from any version (else `ErrEstimateHasInvoices`); a superseded version can't be deleted on its own (`ErrEstimateStatusInvalid`). (Revised in the final review fix wave; earlier a sent / answered revision was refused.)
- Public view: `supersededBy` = the latest version's public token only when the viewed version is `superseded` and the latest version is `sent`, `accepted` or `declined` and has a token; otherwise empty / omitted. No ids are ever added to the public response.
- Routes / API: `POST /api/v1/estimates/revise.json {id}` returns the new version as `InvoiceInfoResponse`; `GET /api/v1/invoices/list.json?kind=estimate` returns latest versions only, `&include_superseded=true` returns every version; the detail response (`/invoices/get.json` and every estimate action) carries `revisionNumber` and `revisions` = `[{id, number, revisionNumber, invoiceDate, dueDate, estimateStatus, total, currency, fullyInvoiced}]` newest first.
- The web client always loads estimates with `include_superseded=true`; superseded versions count as closed, so they appear only with "Show Closed Estimates" (desktop and mobile).
- Customer timeline kind `estimate_revised` (`models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED`): at the revision's `CreatedUnixTime`, `invoiceNumber` = new number, `estimateNumber` = the version it revised; a revision emits it instead of the plain `estimate` event.
- Lock order is unchanged: only estimate rows are locked by Revise and by deleting a revision (the invoice → estimate order holds).
- New UI strings go in `src/locales/en.json` only (`format.misc.*` inside `"format"`, error texts under `"error"`).
- Desktop and mobile both get every screen; shared logic lives in `src/lib/estimate.ts` and `src/views/base/invoices/`.
- Go tests: `go test ./pkg/services/ ./pkg/api/ -count=1` (add `-run <Name>` while iterating). Format: `gofmt -l pkg cmd` must print nothing. Frontend tests: `npx vitest run --exclude '.claude/**' <file>`. Typecheck: `npx vue-tsc --noEmit`. Lint: `npx eslint <paths>`.
- Commit with `git commit --no-verify` (project convention).

## Review Focus

1. **Changing what the client already saw** — a sent or accepted estimate edited in place through the API, or Revise mutating the old version (lines, milestones, recorded acceptance): the old version must stay byte-for-byte what was sent, with its answer, and only drafts are edited. Pinned in Task 1 (`TestReviseCopiesEverythingAndLeavesTheSentVersionIntact`, `TestModifyEstimateRefusesAnythingButDraft`).
2. **Two clicks or two tabs on Revise** — a second Revise from a stale page must not create a second revision or a second superseding. Pinned in Task 1 (`TestConcurrentRevisesCreateOneRevision`).
3. **An old version that still acts** — the client accepting an old link, converting or marking a superseded version, reopening, editing, revising or deleting it: every one refused with nothing changed and no invoice created. Pinned in Task 2 (`TestSupersededVersionRefusesEveryAction`).
4. **Deleting a draft revision breaks the chain or reuses numbers** — the previous version must come back with its exact earlier status (e.g. declined), the next revision must be `-R2` not `-R1`, a sent revision cannot be deleted, and the sequence ignores `-R` suffixes. Pinned in Task 2 (`TestDeletingADraftRevisionRestoresThePreviousVersion`) and Task 1 (`TestEstimateRevisionNumbering`).
5. **The old link leaking the unsent draft or ids** — a superseded version's public page may point only at a latest version that was actually sent and has a link, and its response carries the token only. Pinned in Task 3 (`TestSupersededPublicViewPointsAtTheLatestSharedVersion`) and `pkg/api` (`TestPublicSupersededEstimateResponseCarriesOnlyTheLatestToken`).

---

## File Structure

Backend:
- Modify `pkg/models/estimate.go` — `ESTIMATE_STATUS_SUPERSEDED`, `EstimateRevisionResponse`.
- Modify `pkg/models/invoice.go` — `Invoice.RevisionOf`, `RevisionNumber`, `SupersededFromStatus`; `InvoiceInfoResponse.RevisionNumber`, `.Revisions`; `PublicInvoiceResponse.SupersededBy`; `InvoiceListRequest.IncludeSuperseded`; `CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED`; `SetEstimateRevisions`.
- Modify `pkg/errs/invoice.go` — code 70, new text for 61.
- Create `pkg/services/estimate_revisions.go` — numbering helpers, `checkEstimateRevisable`, `ReviseEstimate`, `checkEstimateDeletable`, `restorePreviousEstimateVersion`, `GetEstimateRevisions`, `GetLatestEstimates`, `GetEstimateSupersededBy`.
- Modify `pkg/services/estimate_lifecycle.go` (`checkEstimateEditable`, `ModifyEstimate`), `pkg/services/estimate_conversion.go` (`estimateInvoiceLines`), `pkg/services/invoices.go` (`nextInvoiceSequence`, `prepareNewInvoice`, `DeleteInvoice`), `pkg/services/customer_overview.go`.
- Modify `pkg/api/estimates.go` (`EstimateReviseHandler`), `pkg/api/invoices.go` (`InvoiceListHandler`, `getInvoiceResponse`, `InvoicePublicViewHandler`), `cmd/webserver.go`.
- Tests: create `pkg/services/estimate_revisions_test.go`, `pkg/services/estimate_revisions_view_test.go`; modify `pkg/services/estimate_lifecycle_test.go`, `pkg/api/estimates_test.go`.

Frontend:
- Modify `src/lib/estimate.ts` + `src/lib/__tests__/estimate.test.ts`.
- Modify `src/models/invoice.ts`, `src/lib/services.ts`, `src/stores/invoice.ts`.
- Modify `src/views/base/invoices/EstimateListPageBase.ts`, `InvoiceDetailPageBase.ts`, `CustomerDetailPageBase.ts`.
- Desktop: modify `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`, `src/views/desktop/invoices/PublicInvoicePage.vue` (the Estimates view in `invoices/ListPage.vue` needs no markup change: it renders `displayEstimates` and `ESTIMATE_STATUS_COLORS` from the base).
- Mobile: modify `src/views/mobile/invoices/DetailPage.vue`, `src/views/mobile/invoices/EstimateListPage.vue`.
- Modify `src/locales/en.json`.

---

### Task 1: Revise: columns, numbering, the Revise service and API; drafts only are edited (backend)

**Files:**
- Modify: `pkg/models/estimate.go` (status constant)
- Modify: `pkg/models/invoice.go` (`Invoice` columns, `InvoiceInfoResponse.RevisionNumber`, `ToInvoiceInfoResponse`)
- Modify: `pkg/errs/invoice.go` (code 70, text of 61)
- Create: `pkg/services/estimate_revisions.go`
- Modify: `pkg/services/invoices.go` (`nextInvoiceSequence`, `prepareNewInvoice`)
- Modify: `pkg/services/estimate_lifecycle.go` (`checkEstimateEditable`, `ModifyEstimate`)
- Modify: `pkg/api/estimates.go` (`EstimateReviseHandler`)
- Modify: `cmd/webserver.go` (one route after `/estimates/reopen.json`)
- Modify: `src/locales/en.json` (`"error"` texts)
- Test: create `pkg/services/estimate_revisions_test.go`; modify `pkg/services/estimate_lifecycle_test.go`, `pkg/api/estimates_test.go`

**Interfaces:**
- Consumes (existing): `lockInvoiceRow(sess, uid, invoiceId, now) error`, `getEstimateInSession(sess, uid, estimateId) (*models.Invoice, error)`, `estimateHasInvoices(sess, uid, estimateId) (bool, error)`, `(*InvoiceService).prepareNewInvoice`, `prepareEstimateMilestones`, `insertInvoice(sess, invoice, lines) error`, `GetInvoice`, `GetEstimateMilestones`, `GetProfile`, `invoiceDateLayout`, `nextInvoiceSequence(numbers []string, base string) int`; test helpers `newAlertTestUser`, `createTestCustomer`, `createTestInvoice`, `createTestProduct`, `createTestEstimate`, `testEstimateLines`, `testEstimateMilestones`, `shareTestEstimate(t, ctx, uid, estimateId) string`, `linkTestInvoiceToEstimate`; API helpers `newTestJsonWebContext`, `(*InvoicesApi).estimateResponse`.
- Produces:
  - `models.ESTIMATE_STATUS_SUPERSEDED = "superseded"`.
  - `Invoice.RevisionOf int64`, `Invoice.RevisionNumber int32`, `Invoice.SupersededFromStatus string`; `InvoiceInfoResponse.RevisionNumber int32` (`json:"revisionNumber,omitempty"`).
  - `errs.ErrEstimateCannotBeRevised` (70).
  - `estimateRevisionNumber(rootNumber string, revisionNumber int32) string`, `nextEstimateRevisionNumber(existing []int32) int32`, `estimateChainRootId(estimate *models.Invoice) int64`, `checkEstimateRevisable(sess *xorm.Session, estimate *models.Invoice) error`.
  - `(*InvoiceService).ReviseEstimate(c core.Context, uid int64, estimateId int64, today string) (*models.Invoice, error)` and `(*InvoiceService).reviseEstimate(c core.Context, source *models.Invoice, sourceLines []*models.InvoiceLine, sourceMilestones []*models.EstimateMilestone, today string) (*models.Invoice, error)`.
  - `ModifyEstimate` refuses anything but `draft` with `ErrEstimateLocked`.
  - Route `POST /api/v1/estimates/revise.json {id}` → `InvoiceInfoResponse` of the new version; handler `(*InvoicesApi).EstimateReviseHandler`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_revisions_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestEstimateRevisionNumbering(t *testing.T) {
	assert.Equal(t, "EST/2026-27/01-R2", estimateRevisionNumber("EST/2026-27/01", 2))
	assert.Equal(t, int32(1), nextEstimateRevisionNumber([]int32{0}))
	assert.Equal(t, int32(4), nextEstimateRevisionNumber([]int32{0, 3, 1})) // the highest wins, whatever was deleted in between
	assert.Equal(t, 2, nextInvoiceSequence([]string{"EST/2026-27/01", "EST/2026-27/01-R2"}, "EST/2026-27/"))
	assert.Equal(t, 2, nextInvoiceSequence([]string{"EST/2026-27/01-R2"}, "EST/2026-27/")) // a revision counts as the estimate it revises

	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01-R1", revision.Number)

	next, err := Invoices.NextEstimateNumber(ctx, uid, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/02", next)
}

func TestReviseCopiesEverythingAndLeavesTheSentVersionIntact(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	hosting := createTestProduct(t, uid, "Hosting", 0, "")

	estimate := &models.Invoice{Uid: uid, Number: "EST/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2099-12-31",
		Notes: "Valid for 30 days", TaxType: models.INVOICE_TAX_TYPE_IGST, TaxRate: 1800, Currency: "USD"}
	lines := testEstimateLines()
	lines[1].ProductId = hosting.ProductId
	assert.Nil(t, Invoices.CreateEstimate(ctx, estimate, lines, testEstimateMilestones()))
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))

	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	reloaded, revisionLines, err := Invoices.GetInvoice(ctx, uid, revision.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_KIND_ESTIMATE, reloaded.Kind)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, reloaded.EstimateStatus)
	assert.Equal(t, "EST/2026-27/01-R1", reloaded.Number)
	assert.Equal(t, estimate.InvoiceId, reloaded.RevisionOf)
	assert.Equal(t, int32(1), reloaded.RevisionNumber)
	assert.Equal(t, customer.CustomerId, reloaded.CustomerId)
	assert.Equal(t, "Quber Tech", reloaded.BillToName)
	assert.Equal(t, "USD", reloaded.Currency)
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, reloaded.TaxType)
	assert.Equal(t, int32(1800), reloaded.TaxRate)
	assert.Equal(t, estimate.Total, reloaded.Total)
	assert.Equal(t, "Valid for 30 days", reloaded.Notes)
	assert.Equal(t, "2026-10-30", reloaded.InvoiceDate)
	assert.Equal(t, "2026-11-29", reloaded.DueDate) // today + the profile's 30 days
	assert.Equal(t, "", reloaded.PublicToken)
	assert.Equal(t, int64(0), reloaded.SentUnixTime)
	assert.Equal(t, int64(0), reloaded.RespondedUnixTime) // the client agreed to another version
	assert.False(t, reloaded.RespondedByClient)
	assert.Equal(t, 2, len(revisionLines))
	assert.Equal(t, "Support", revisionLines[1].ItemName)
	assert.Equal(t, int64(300), revisionLines[1].Quantity)
	assert.Equal(t, int64(33333), revisionLines[1].Rate)
	assert.Equal(t, "998315", revisionLines[1].Sac)
	assert.Equal(t, hosting.ProductId, revisionLines[1].ProductId)

	oldMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	newMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, revision.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(newMilestones))

	for idx, milestone := range newMilestones {
		assert.Equal(t, oldMilestones[idx].Label, milestone.Label)
		assert.Equal(t, oldMilestones[idx].Percent, milestone.Percent)
		assert.Equal(t, int64(0), milestone.InvoiceId)
		assert.NotEqual(t, oldMilestones[idx].MilestoneId, milestone.MilestoneId)
	}

	// the version the client saw keeps everything, its answer included, and is now superseded
	old, oldLines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, old.EstimateStatus)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, old.SupersededFromStatus)
	assert.Greater(t, old.RespondedUnixTime, int64(0))
	assert.Equal(t, "EST/2026-27/01", old.Number)
	assert.Equal(t, "2099-12-31", old.DueDate)
	assert.Equal(t, int64(0), old.RevisionOf)
	assert.Equal(t, int32(0), old.RevisionNumber)
	assert.Equal(t, 2, len(oldLines))
	assert.Equal(t, 3, len(oldMilestones))
}

func TestReviseIsAllowedOnlyOnTheLatestSentOrAnsweredVersionWithNothingInvoiced(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	today := "2026-10-30"

	// a draft is edited in place
	draft := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	_, err := Invoices.ReviseEstimate(ctx, uid, draft.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	sent := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, sent.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, sent.InvoiceId, today)
	assert.Nil(t, err)

	// the superseded version and the new draft are not revisable
	_, err = Invoices.ReviseEstimate(ctx, uid, sent.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)
	_, err = Invoices.ReviseEstimate(ctx, uid, revision.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	// expired (sent, valid-until in the past), declined and accepted are revisable
	expired := &models.Invoice{Uid: uid, Number: "EST/2026-27/03", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-01", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateEstimate(ctx, expired, testEstimateLines(), nil))
	shareTestEstimate(t, ctx, uid, expired.InvoiceId)
	_, err = Invoices.ReviseEstimate(ctx, uid, expired.InvoiceId, today)
	assert.Nil(t, err)

	declined := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/04", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, declined.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	_, err = Invoices.ReviseEstimate(ctx, uid, declined.InvoiceId, today)
	assert.Nil(t, err)

	accepted := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/05", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, accepted.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	_, err = Invoices.ReviseEstimate(ctx, uid, accepted.InvoiceId, today)
	assert.Nil(t, err)

	// never once anything is invoiced from it
	invoiced := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/06", testEstimateLines(), nil)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, invoiced.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV/2026-27/01", 100)
	linkTestInvoiceToEstimate(t, uid, invoice.InvoiceId, invoiced.InvoiceId)
	_, err = Invoices.ReviseEstimate(ctx, uid, invoiced.InvoiceId, today)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoiced.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_ACCEPTED, reloaded.EstimateStatus)

	// an invoice is not an estimate
	_, err = Invoices.ReviseEstimate(ctx, uid, invoice.InvoiceId, today)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
}

func TestConcurrentRevisesCreateOneRevision(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)

	// what a second click read just before the first one wrote
	stale, staleLines, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	staleMilestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	_, err = Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	second, err := Invoices.reviseEstimate(ctx, stale, staleLines, staleMilestones, "2026-10-30")
	assert.Nil(t, second)
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	all, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all)) // the first version and exactly one revision

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, reloaded.EstimateStatus)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.SupersededFromStatus)
}

func TestModifyEstimateRefusesAnythingButDraft(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)

	// the client has not seen a draft: it is edited in place
	estimate.Notes = "First draft"
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	estimate.Notes = "Changed after sending"
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "First draft", reloaded.Notes)
	assert.Equal(t, models.ESTIMATE_STATUS_SENT, reloaded.EstimateStatus)
}
```

In `pkg/services/estimate_lifecycle_test.go`, `TestAnsweredOrInvoicedEstimatesAreLocked`, replace

```go
	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	assert.Nil(t, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), testEstimateMilestones()))
```

with

```go
	assert.Nil(t, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	// reopened means sent again: still never edited in place (it is revised instead)
	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), testEstimateMilestones()))
```

Append to `pkg/api/estimates_test.go`:

```go
func TestEstimateReviseHandlerRefusesABadRequestWithoutTheDatabase(t *testing.T) {
	result, err := Invoices.EstimateReviseHandler(newTestJsonWebContext("/api/v1/estimates/revise.json", `{}`))
	assert.Nil(t, result)
	assert.NotNil(t, err)

	result, err = Invoices.EstimateReviseHandler(newTestJsonWebContext("/api/v1/estimates/revise.json", `{"id":"0"}`))
	assert.Nil(t, result)
	assert.NotNil(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ ./pkg/api/ -run 'TestEstimateRevisionNumbering|TestReviseCopies|TestReviseIsAllowed|TestConcurrentRevises|TestModifyEstimateRefuses|TestAnsweredOrInvoiced|TestEstimateReviseHandler' -count=1`
Expected: compile FAIL (`estimateRevisionNumber undefined`, `Invoices.ReviseEstimate undefined`, `models.ESTIMATE_STATUS_SUPERSEDED undefined`, `Invoices.EstimateReviseHandler undefined`).

- [ ] **Step 3: Models and errors**

In `pkg/models/estimate.go` replace the status block with:

```go
// Estimate statuses stored in Invoice.EstimateStatus. "expired" is never stored: a draft or sent estimate whose
// valid-until date (DueDate) is before today shows as expired, and extending the date revives it. "superseded" is a version
// a newer version (a revision) replaced: read-only history that keeps its lines, milestones, emails and recorded answer.
const (
	ESTIMATE_STATUS_DRAFT      = "draft"
	ESTIMATE_STATUS_SENT       = "sent"
	ESTIMATE_STATUS_ACCEPTED   = "accepted"
	ESTIMATE_STATUS_DECLINED   = "declined"
	ESTIMATE_STATUS_SUPERSEDED = "superseded"
)
```

In `pkg/models/invoice.go`, `Invoice`: give `Uid` the new index and add three columns after `EstimateId`:

```go
	Uid                int64  `xorm:"INDEX(IDX_invoice_uid_deleted) INDEX(IDX_invoice_uid_recurring_id) INDEX(IDX_invoice_uid_estimate_id) INDEX(IDX_invoice_uid_revision_of) NOT NULL"`
```

```go
	// EstimateId is the estimate this invoice was created from; 0 otherwise
	EstimateId int64 `xorm:"INDEX(IDX_invoice_uid_estimate_id) NOT NULL DEFAULT 0"`
	// RevisionOf is the first version of the estimate's revision chain; 0 on a first version and on every invoice
	RevisionOf int64 `xorm:"INDEX(IDX_invoice_uid_revision_of) NOT NULL DEFAULT 0"`
	// RevisionNumber is 0 on a first version, then 1, 2, … within the chain; never reused, deleted revisions included
	RevisionNumber int32 `xorm:"NOT NULL DEFAULT 0"`
	// SupersededFromStatus is the status a superseded version had when it was revised; deleting the draft revision restores it
	SupersededFromStatus string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
}
```

In `InvoiceInfoResponse`, after the `EstimateNumber` field add:

```go
	RevisionNumber     int32                               `json:"revisionNumber,omitempty"`    // estimates: 0 on a first version
```

In `ToInvoiceInfoResponse`, replace

```go
		ResponseNote:       i.ResponseNote,
	}
```

with

```go
		ResponseNote:       i.ResponseNote,
		RevisionNumber:     i.RevisionNumber,
	}
```

In `pkg/errs/invoice.go` replace the `ErrEstimateLocked` line and add code 70 after `ErrEstimateMilestoneInvoiceLineCountInvalid`:

```go
	ErrEstimateLocked                           = NewNormalError(NormalSubcategoryInvoice, 61, http.StatusBadRequest, "only a draft estimate can be edited, revise it instead")
```

```go
	ErrEstimateMilestoneInvoiceLineCountInvalid = NewNormalError(NormalSubcategoryInvoice, 69, http.StatusBadRequest, "a milestone invoice has exactly one line")
	ErrEstimateCannotBeRevised                  = NewNormalError(NormalSubcategoryInvoice, 70, http.StatusBadRequest, "estimate cannot be revised")
```

In `src/locales/en.json` (`"error"` section) replace

```json
        "estimate is accepted, declined or invoiced and cannot be edited": "This estimate is accepted, declined or invoiced and can no longer be edited",
```

with

```json
        "only a draft estimate can be edited, revise it instead": "Only a draft estimate can be edited. Use Revise to change one that was sent",
```

and replace

```json
        "a milestone invoice has exactly one line": "A milestone invoice has exactly one line",
```

with

```json
        "a milestone invoice has exactly one line": "A milestone invoice has exactly one line",
        "estimate cannot be revised": "Only the latest sent, declined or accepted version of an estimate, with nothing invoiced from it, can be revised",
```

- [ ] **Step 4: Services**

In `pkg/services/invoices.go`, `nextInvoiceSequence`, replace

```go
		sequence, err := strconv.Atoi(number[len(base):])
```

with

```go
		rest := number[len(base):]

		// a revision (EST/2026-27/01-R2) counts as the estimate it revises, so the suffix never moves the sequence
		if idx := strings.Index(rest, "-R"); idx > 0 {
			rest = rest[:idx]
		}

		sequence, err := strconv.Atoi(rest)
```

In `prepareNewInvoice`, replace

```go
	invoice.EstimateId = 0
	invoice.CreatedUnixTime = time.Now().Unix()
```

with

```go
	invoice.EstimateId = 0
	invoice.RevisionOf = 0
	invoice.RevisionNumber = 0
	invoice.SupersededFromStatus = ""
	invoice.CreatedUnixTime = time.Now().Unix()
```

In `pkg/services/estimate_lifecycle.go` replace `checkEstimateEditable` with:

```go
// checkEstimateEditable returns ErrEstimateLocked unless the estimate is a draft with nothing invoiced from it: once the client
// may have seen it (sent, answered, superseded) it is changed only through ReviseEstimate
func checkEstimateEditable(sess *xorm.Session, estimate *models.Invoice) error {
	if estimate.EstimateStatus != models.ESTIMATE_STATUS_DRAFT {
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
```

In `ModifyEstimate`, replace the doc comment and the conditional update:

```go
// ModifyEstimate saves a draft estimate and replaces its lines and milestones, like ModifyInvoice does for lines.
// The update is conditional on the status still being draft, so a send or an answer that lands meanwhile wins.
```

```go
		updatedRows, err := sess.Cols(invoiceModifyColumns...).
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=? AND estimate_status=?", estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.InvoiceId, models.ESTIMATE_STATUS_DRAFT).
			Update(estimate)
```

Create `pkg/services/estimate_revisions.go`:

```go
package services

import (
	"fmt"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateRevisionNumber is a revision's number: the first version's number plus "-R<n>", e.g. EST/2026-27/01-R2
func estimateRevisionNumber(rootNumber string, revisionNumber int32) string {
	return fmt.Sprintf("%s-R%d", rootNumber, revisionNumber)
}

// nextEstimateRevisionNumber is one more than the highest revision number of a chain; the caller passes every row of the
// chain, deleted ones included, so a deleted draft revision's number is never handed out again
func nextEstimateRevisionNumber(existing []int32) int32 {
	highest := int32(0)

	for _, revisionNumber := range existing {
		if revisionNumber > highest {
			highest = revisionNumber
		}
	}

	return highest + 1
}

// estimateChainRootId is the id of the first version of the estimate's revision chain
func estimateChainRootId(estimate *models.Invoice) int64 {
	if estimate.RevisionOf > 0 {
		return estimate.RevisionOf
	}

	return estimate.InvoiceId
}

// checkEstimateRevisable returns ErrEstimateCannotBeRevised unless the estimate is sent (expired included), declined or accepted,
// nothing was invoiced from it, and no non-deleted version of its chain has a higher revision number
func checkEstimateRevisable(sess *xorm.Session, estimate *models.Invoice) error {
	switch estimate.EstimateStatus {
	case models.ESTIMATE_STATUS_SENT, models.ESTIMATE_STATUS_DECLINED, models.ESTIMATE_STATUS_ACCEPTED:
	default:
		return errs.ErrEstimateCannotBeRevised
	}

	invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

	if err != nil {
		return err
	} else if invoiced {
		return errs.ErrEstimateCannotBeRevised
	}

	rootId := estimateChainRootId(estimate)
	newer, err := sess.Where("uid=? AND deleted=? AND kind=? AND revision_number>? AND (invoice_id=? OR revision_of=?)",
		estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.RevisionNumber, rootId, rootId).Exist(&models.Invoice{})

	if err != nil {
		return err
	} else if newer {
		return errs.ErrEstimateCannotBeRevised
	}

	return nil
}

// ReviseEstimate creates the next version of a sent or answered estimate as a new draft and supersedes the old version, which keeps
// everything it had (lines, milestones, PDF, emails, recorded answer). It returns the new version for the editor.
func (s *InvoiceService) ReviseEstimate(c core.Context, uid int64, estimateId int64, today string) (*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return nil, errs.ErrInvoiceIdInvalid
	}

	source, lines, err := s.GetInvoice(c, uid, estimateId)

	if err != nil {
		return nil, err
	} else if source.Kind != models.INVOICE_KIND_ESTIMATE {
		return nil, errs.ErrInvoiceNotFound
	}

	milestones, err := s.GetEstimateMilestones(c, uid, estimateId)

	if err != nil {
		return nil, err
	}

	return s.reviseEstimate(c, source, lines, milestones, today)
}

// reviseEstimate copies source (as read before the transaction) into a new draft version. Inside one transaction it locks the source
// row (only estimate rows are locked, so the invoice → estimate lock order holds), re-reads and re-checks it, numbers and inserts the
// copy, and supersedes the source conditionally on the status it just read: of two concurrent Revise clicks exactly one goes through.
func (s *InvoiceService) reviseEstimate(c core.Context, source *models.Invoice, sourceLines []*models.InvoiceLine, sourceMilestones []*models.EstimateMilestone, today string) (*models.Invoice, error) {
	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return nil, errs.ErrInvoiceDateInvalid
	}

	profile, err := s.GetProfile(c, source.Uid)

	if err != nil {
		return nil, err
	}

	validDays := profile.EstimateValidDays

	if validDays <= 0 {
		validDays = models.EstimateValidDaysDefault
	}

	revision := &models.Invoice{Uid: source.Uid, Number: source.Number, CustomerId: source.CustomerId, Currency: source.Currency, InvoiceDate: today,
		DueDate: todayTime.AddDate(0, 0, int(validDays)).Format(invoiceDateLayout), Notes: source.Notes, TaxType: source.TaxType, TaxRate: source.TaxRate}

	lines := make([]*models.InvoiceLine, len(sourceLines))

	for idx, line := range sourceLines {
		lines[idx] = &models.InvoiceLine{ItemName: line.ItemName, Description: line.Description, Quantity: line.Quantity, Rate: line.Rate, Sac: line.Sac, ProductId: line.ProductId}
	}

	milestones := make([]*models.EstimateMilestone, len(sourceMilestones))

	for idx, milestone := range sourceMilestones {
		milestones[idx] = &models.EstimateMilestone{Label: milestone.Label, Percent: milestone.Percent}
	}

	if err := s.prepareNewInvoice(c, revision, lines); err != nil {
		return nil, err
	}

	revision.Kind = models.INVOICE_KIND_ESTIMATE
	revision.EstimateStatus = models.ESTIMATE_STATUS_DRAFT

	if err := s.prepareEstimateMilestones(revision, milestones); err != nil {
		return nil, err
	}

	err = s.UserDataDB(source.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := lockInvoiceRow(sess, source.Uid, source.InvoiceId, revision.CreatedUnixTime); err != nil {
			return err
		}

		stored, err := getEstimateInSession(sess, source.Uid, source.InvoiceId)

		if err != nil {
			return err
		}

		if err := checkEstimateRevisable(sess, stored); err != nil {
			return err
		}

		rootId := estimateChainRootId(stored)
		root := &models.Invoice{}
		has, err := sess.Where("uid=? AND invoice_id=?", stored.Uid, rootId).Get(root)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrInvoiceNotFound
		}

		// deleted versions count too, so a deleted draft revision's number is never reused
		var revisionNumbers []int32
		err = sess.Table(&models.Invoice{}).Where("uid=? AND kind=? AND (invoice_id=? OR revision_of=?)", stored.Uid, models.INVOICE_KIND_ESTIMATE, rootId, rootId).
			Cols("revision_number").Find(&revisionNumbers)

		if err != nil {
			return err
		}

		revision.RevisionOf = rootId
		revision.RevisionNumber = nextEstimateRevisionNumber(revisionNumbers)
		revision.Number = estimateRevisionNumber(root.Number, revision.RevisionNumber)

		if err := insertInvoice(sess, revision, lines); err != nil {
			return err
		}

		if len(milestones) > 0 {
			if _, err := sess.Insert(milestones); err != nil {
				return err
			}
		}

		updatedRows, err := sess.Cols("estimate_status", "superseded_from_status", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=? AND estimate_status=?", stored.Uid, false, models.INVOICE_KIND_ESTIMATE, stored.InvoiceId, stored.EstimateStatus).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SUPERSEDED, SupersededFromStatus: stored.EstimateStatus, UpdatedUnixTime: revision.CreatedUnixTime})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateCannotBeRevised
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return revision, nil
}
```

- [ ] **Step 5: API and route**

In `pkg/api/estimates.go`, add `"time"` to the imports:

```go
import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)
```

and add after `EstimateReopenHandler`:

```go
// EstimateReviseHandler creates the next version of a sent or answered estimate of current user as a new draft and returns it;
// the old version is superseded and stays as it was sent
func (a *InvoicesApi) EstimateReviseHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.InvoiceIdRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf(c, "[estimates.EstimateReviseHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	revision, err := a.invoices.ReviseEstimate(c, uid, req.Id, time.Now().Format("2006-01-02"))

	if err != nil {
		log.Errorf(c, "[estimates.EstimateReviseHandler] failed to revise estimate \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.estimateResponse(c, "EstimateReviseHandler", uid, revision.InvoiceId)
}
```

In `cmd/webserver.go` replace

```go
			apiV1Route.POST("/estimates/reopen.json", bindApi(api.Invoices.EstimateReopenHandler, config))
```

with

```go
			apiV1Route.POST("/estimates/reopen.json", bindApi(api.Invoices.EstimateReopenHandler, config))
			apiV1Route.POST("/estimates/revise.json", bindApi(api.Invoices.EstimateReviseHandler, config))
```

Then: `gofmt -w pkg/models/estimate.go pkg/models/invoice.go pkg/errs/invoice.go pkg/services/estimate_revisions.go pkg/services/estimate_lifecycle.go pkg/services/invoices.go pkg/api/estimates.go cmd/webserver.go`

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && gofmt -l pkg cmd`
Expected: PASS (the whole packages, so every earlier estimate test still holds with draft-only editing); gofmt prints nothing.

- [ ] **Step 7: Commit**

```bash
git add pkg/models pkg/errs pkg/services pkg/api cmd/webserver.go src/locales/en.json
git commit --no-verify -m "estimates: revise a sent or answered estimate into a new draft version; only drafts are edited in place"
```

---

### Task 2: Superseded versions are read-only; deleting a draft revision restores the previous version (backend)

**Files:**
- Modify: `pkg/services/estimate_conversion.go` (`estimateInvoiceLines`)
- Modify: `pkg/services/estimate_revisions.go` (`checkEstimateDeletable`, `restorePreviousEstimateVersion`)
- Modify: `pkg/services/invoices.go` (`DeleteInvoice`)
- Test: modify `pkg/services/estimate_revisions_test.go`

**Interfaces:**
- Consumes (Task 1): `ReviseEstimate`, `ESTIMATE_STATUS_SUPERSEDED`, `Invoice.RevisionOf / RevisionNumber / SupersededFromStatus`, `ErrEstimateCannotBeRevised`; existing `SetEstimateResponse`, `ReopenEstimate`, `RespondToEstimateByPublicToken`, `GetEstimateInvoiceDraft`, `CreateInvoiceFromEstimate`, `DeleteInvoice`, `NextEstimateNumber`; test helpers `shareTestEstimate`, `newTestInvoiceDraft(uid, customerId, number) *models.Invoice`.
- Produces:
  - `checkEstimateDeletable(estimate *models.Invoice) error` (`ErrEstimateStatusInvalid` for a superseded version or a revision that is no longer a draft).
  - `restorePreviousEstimateVersion(sess *xorm.Session, revision *models.Invoice, now int64) error`.
  - Conversion (`GetEstimateInvoiceDraft`, `CreateInvoiceFromEstimate`) refuses `superseded` with `ErrEstimateStatusInvalid`.
  - `DeleteInvoice` refuses superseded versions and sent / answered revisions; deleting a draft revision restores the previous version's status.

- [ ] **Step 1: Write the failing tests**

Append to `pkg/services/estimate_revisions_test.go`:

```go
func TestSupersededVersionRefusesEveryAction(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), testEstimateMilestones())
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	_, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	milestones, err := Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)

	assert.Equal(t, errs.ErrEstimateLocked, Invoices.ModifyEstimate(ctx, estimate, testEstimateLines(), nil))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_ACCEPTED))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.ReopenEstimate(ctx, uid, estimate.InvoiceId))
	assert.Equal(t, errs.ErrEstimateCannotBeAnswered, Invoices.RespondToEstimateByPublicToken(ctx, token, models.ESTIMATE_STATUS_ACCEPTED, "", "2026-10-30"))

	_, _, err = Invoices.GetEstimateInvoiceDraft(ctx, uid, estimate.InvoiceId, milestones[0].MilestoneId, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateStatusInvalid, err)
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.CreateInvoiceFromEstimate(ctx, newTestInvoiceDraft(uid, customer.CustomerId, "INV/2026-27/01"),
		[]*models.InvoiceLine{{ItemName: "Advance", Quantity: 100, Rate: 1}}, estimate.InvoiceId, milestones[0].MilestoneId))

	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.DeleteInvoice(ctx, uid, estimate.InvoiceId))
	_, err = Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Equal(t, errs.ErrEstimateCannotBeRevised, err)

	// nothing changed: still superseded, still there, no invoice made, the payment terms untouched
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, reloaded.EstimateStatus)
	assert.Equal(t, int64(0), reloaded.RespondedUnixTime)

	invoices, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Empty(t, invoices)

	milestones, err = Invoices.GetEstimateMilestones(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), milestones[0].InvoiceId)
}

func TestDeletingADraftRevisionRestoresThePreviousVersion(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	assert.Nil(t, Invoices.SetEstimateResponse(ctx, uid, estimate.InvoiceId, models.ESTIMATE_STATUS_DECLINED))

	first, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, first.InvoiceId))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, estimate.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.ESTIMATE_STATUS_DECLINED, reloaded.EstimateStatus) // exactly what it was before Revise
	assert.Equal(t, "", reloaded.SupersededFromStatus)
	assert.Greater(t, reloaded.RespondedUnixTime, int64(0))

	// the deleted draft's number is never handed out again
	second, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01-R2", second.Number)
	assert.Equal(t, int32(2), second.RevisionNumber)

	// once sent, a revision is answered or revised, never deleted: deleting it would leave the chain without a live version
	shareTestEstimate(t, ctx, uid, second.InvoiceId)
	assert.Equal(t, errs.ErrEstimateStatusInvalid, Invoices.DeleteInvoice(ctx, uid, second.InvoiceId))

	// a first-version draft is deleted as before
	draft := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, draft.InvoiceId))

	// 01 and 01-R2 remain: the next new estimate is 02 again, the -R suffix does not count
	next, err := Invoices.NextEstimateNumber(ctx, uid, "2026-10-30")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/02", next)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ -run 'TestSupersededVersionRefusesEveryAction|TestDeletingADraftRevisionRestores' -count=1`
Expected: FAIL — `GetEstimateInvoiceDraft` / `CreateInvoiceFromEstimate` return nil instead of `ErrEstimateStatusInvalid`, `DeleteInvoice` of the superseded version returns nil, and after deleting the draft revision the first version is still `superseded` instead of `declined`.

- [ ] **Step 3: Conversion refuses superseded versions**

In `pkg/services/estimate_conversion.go`, `estimateInvoiceLines`, replace

```go
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED {
		return nil, errs.ErrEstimateStatusInvalid
	}
```

with

```go
	// a declined estimate is not invoiced, and only the latest version of a revised estimate can be
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED || estimate.EstimateStatus == models.ESTIMATE_STATUS_SUPERSEDED {
		return nil, errs.ErrEstimateStatusInvalid
	}
```

- [ ] **Step 4: Delete rules**

Append to `pkg/services/estimate_revisions.go`:

```go
// checkEstimateDeletable refuses a superseded version (history the newer version depends on) and a revision that is no longer a
// draft (the client may have seen it); a first version, or a draft revision, can be deleted
func checkEstimateDeletable(estimate *models.Invoice) error {
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_SUPERSEDED {
		return errs.ErrEstimateStatusInvalid
	}

	if estimate.RevisionNumber > 0 && estimate.EstimateStatus != models.ESTIMATE_STATUS_DRAFT {
		return errs.ErrEstimateStatusInvalid
	}

	return nil
}

// restorePreviousEstimateVersion returns the version a deleted draft revision superseded (the chain's highest lower revision number
// still there) to the status it had before Revise, inside the caller's transaction. The update is conditional on it still being
// superseded; a version stored without SupersededFromStatus goes back to sent.
func restorePreviousEstimateVersion(sess *xorm.Session, revision *models.Invoice, now int64) error {
	previous := &models.Invoice{}
	has, err := sess.Where("uid=? AND deleted=? AND kind=? AND estimate_status=? AND revision_number<? AND (invoice_id=? OR revision_of=?)",
		revision.Uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED, revision.RevisionNumber, revision.RevisionOf, revision.RevisionOf).
		OrderBy("revision_number desc").Get(previous)

	if err != nil {
		return err
	} else if !has {
		return nil
	}

	status := previous.SupersededFromStatus

	if status == "" {
		status = models.ESTIMATE_STATUS_SENT
	}

	_, err = sess.Cols("estimate_status", "superseded_from_status", "updated_unix_time").
		Where("uid=? AND deleted=? AND invoice_id=? AND estimate_status=?", revision.Uid, false, previous.InvoiceId, models.ESTIMATE_STATUS_SUPERSEDED).
		Update(&models.Invoice{EstimateStatus: status, SupersededFromStatus: "", UpdatedUnixTime: now})

	return err
}
```

In `pkg/services/invoices.go`, `DeleteInvoice`, replace the doc comment with:

```go
// DeleteInvoice soft-deletes an invoice or estimate and removes its payment links; the linked transactions are untouched.
// An estimate with invoices made from it, a superseded estimate version and a revision that is no longer a draft are refused; an
// invoice made from a milestone frees that milestone; deleting a draft revision returns the version it superseded to its old status.
```

replace

```go
		if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
			invoiced, err := estimateHasInvoices(sess, uid, invoiceId)
```

with

```go
		if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
			if err := checkEstimateDeletable(invoice); err != nil {
				return err
			}

			invoiced, err := estimateHasInvoices(sess, uid, invoiceId)
```

and replace

```go
		_, err = sess.Where("uid=? AND invoice_id=?", uid, invoiceId).Delete(&models.InvoicePayment{})
		return err
	})
}
```

with

```go
		if invoice.Kind == models.INVOICE_KIND_ESTIMATE && invoice.RevisionNumber > 0 {
			if err := restorePreviousEstimateVersion(sess, invoice, now); err != nil {
				return err
			}
		}

		_, err = sess.Where("uid=? AND invoice_id=?", uid, invoiceId).Delete(&models.InvoicePayment{})
		return err
	})
}
```

Then: `gofmt -w pkg/services/estimate_revisions.go pkg/services/invoices.go pkg/services/estimate_conversion.go`

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && gofmt -l pkg cmd`
Expected: PASS; gofmt prints nothing.

- [ ] **Step 6: Commit**

```bash
git add pkg/services
git commit --no-verify -m "estimates: superseded versions are read-only; deleting a draft revision restores the previous version"
```

---

### Task 3: Revisions in the detail, latest-only list, public "revised" pointer and the customer timeline (backend)

**Files:**
- Modify: `pkg/models/estimate.go` (`EstimateRevisionResponse`)
- Modify: `pkg/models/invoice.go` (`CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED`, `InvoiceListRequest.IncludeSuperseded`, `InvoiceInfoResponse.Revisions`, `PublicInvoiceResponse.SupersededBy`, `SetEstimateRevisions`)
- Modify: `pkg/services/estimate_revisions.go` (`GetEstimateRevisions`, `GetLatestEstimates`, `GetEstimateSupersededBy`)
- Modify: `pkg/services/customer_overview.go`
- Modify: `pkg/api/invoices.go` (`InvoiceListHandler`, `getInvoiceResponse`, `InvoicePublicViewHandler`)
- Test: create `pkg/services/estimate_revisions_view_test.go`; modify `pkg/api/estimates_test.go`

**Interfaces:**
- Consumes (Tasks 1–2): `ReviseEstimate`, `estimateChainRootId`, `ESTIMATE_STATUS_SUPERSEDED`, `RevisionNumber`; existing `GetEstimateProgress`, `GetAllEstimates`, `GetInvoiceByPublicToken`, `SetInvoicePublicToken`, `generateInvoicePublicToken`, `GetCustomerOverview`, `shareTestEstimate`.
- Produces:
  - `models.EstimateRevisionResponse{Id, Number string; RevisionNumber int32; InvoiceDate, DueDate, EstimateStatus string; Total int64; Currency string; FullyInvoiced bool}` (JSON `id, number, revisionNumber, invoiceDate, dueDate, estimateStatus, total, currency, fullyInvoiced`).
  - `InvoiceInfoResponse.Revisions []*EstimateRevisionResponse` (`json:"revisions,omitempty"`); `(*InvoiceInfoResponse).SetEstimateRevisions(revisions []*Invoice, progress map[int64]*EstimateProgress)`.
  - `PublicInvoiceResponse.SupersededBy string` (`json:"supersededBy,omitempty"`).
  - `InvoiceListRequest.IncludeSuperseded bool` (`form:"include_superseded"`).
  - `models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED = "estimate_revised"`.
  - `(*InvoiceService).GetEstimateRevisions(c, uid int64, estimate *models.Invoice) ([]*models.Invoice, error)`, `GetLatestEstimates(c, uid int64) ([]*models.Invoice, error)`, `GetEstimateSupersededBy(c, estimate *models.Invoice) (string, error)`.
  - `previousEstimateVersions(rows []*models.Invoice) map[int64]*models.Invoice`; `estimateTimelineItems(estimate, previous *models.Invoice)`.

- [ ] **Step 1: Write the failing tests**

Create `pkg/services/estimate_revisions_view_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

func TestSupersededPublicViewPointsAtTheLatestSharedVersion(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	// the old link still opens the version the client saw
	old, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, token)
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01", old.Number)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, old.EstimateStatus)

	// an unsent draft is never pointed at, not even when it somehow has a token
	supersededBy, err := Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	draftToken, err := generateInvoicePublicToken()
	assert.Nil(t, err)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("public_token").
		Where("uid=? AND invoice_id=?", uid, revision.InvoiceId).Update(&models.Invoice{PublicToken: draftToken})
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	// once the revision is sent with a link, the old page links to it
	revisionToken := shareTestEstimate(t, ctx, uid, revision.InvoiceId)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, revisionToken, supersededBy)

	// the latest version itself is superseded by nothing
	latest, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, revisionToken)
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, latest)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	// a disabled link on the latest version leaves only the notice
	_, _, err = Invoices.SetInvoicePublicToken(ctx, uid, revision.InvoiceId, false)
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)
}

func TestEstimateListShowsLatestVersionsUnlessSupersededAreRequested(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	other := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)

	latest, err := Invoices.GetLatestEstimates(ctx, uid)
	assert.Nil(t, err)
	latestIds := make([]int64, len(latest))

	for idx, row := range latest {
		latestIds[idx] = row.InvoiceId
	}

	assert.ElementsMatch(t, []int64{revision.InvoiceId, other.InvoiceId}, latestIds)

	all, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(all))
}

func TestEstimateRevisionsListTheWholeChainNewestFirst(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	first, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	shareTestEstimate(t, ctx, uid, first.InvoiceId)
	second, err := Invoices.ReviseEstimate(ctx, uid, first.InvoiceId, "2026-10-31")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01-R2", second.Number)

	for _, version := range []*models.Invoice{estimate, first, second} {
		revisions, err := Invoices.GetEstimateRevisions(ctx, uid, version)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(revisions))
		assert.Equal(t, "EST/2026-27/01-R2", revisions[0].Number)
		assert.Equal(t, "EST/2026-27/01-R1", revisions[1].Number)
		assert.Equal(t, "EST/2026-27/01", revisions[2].Number)
	}

	revisions, err := Invoices.GetEstimateRevisions(ctx, uid, estimate)
	assert.Nil(t, err)
	progress, err := Invoices.GetEstimateProgress(ctx, uid, []int64{revisions[0].InvoiceId, revisions[1].InvoiceId, revisions[2].InvoiceId})
	assert.Nil(t, err)

	resp := estimate.ToInvoiceInfoResponse(nil, nil)
	resp.SetEstimateRevisions(revisions, progress)
	assert.Equal(t, utils.Int64ToString(second.InvoiceId), resp.Revisions[0].Id)
	assert.Equal(t, int32(2), resp.Revisions[0].RevisionNumber)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, resp.Revisions[0].EstimateStatus)
	assert.Equal(t, "2026-10-31", resp.Revisions[0].InvoiceDate)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, resp.Revisions[2].EstimateStatus)
	assert.Equal(t, estimate.Total, resp.Revisions[2].Total)
	assert.False(t, resp.Revisions[2].FullyInvoiced)

	// a first version that was never revised lists only itself
	lone := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	revisions, err = Invoices.GetEstimateRevisions(ctx, uid, lone)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(revisions))
}

func TestCustomerTimelineShowsRevisedEvents(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)

	kinds := make(map[string]int)
	var revised *models.CustomerTimelineItemResponse

	for _, item := range overview.Timeline {
		kinds[item.Kind]++

		if item.Kind == models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED {
			revised = item
		}
	}

	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_ESTIMATE]) // the first version only: a revision is a revised event
	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED])
	assert.NotNil(t, revised)
	assert.Equal(t, "EST/2026-27/01-R1", revised.InvoiceNumber)
	assert.Equal(t, "EST/2026-27/01", revised.EstimateNumber)
	assert.Equal(t, utils.Int64ToString(revision.InvoiceId), revised.InvoiceId)
	assert.Equal(t, revision.CreatedUnixTime, revised.UnixTime)
}
```

Append to `pkg/api/estimates_test.go`:

```go
func TestPublicSupersededEstimateResponseCarriesOnlyTheLatestToken(t *testing.T) {
	estimate := &models.Invoice{InvoiceId: 1234, Kind: models.INVOICE_KIND_ESTIMATE, Number: "EST/2026-27/01", Currency: "INR", Total: 235999,
		EstimateStatus: models.ESTIMATE_STATUS_SUPERSEDED, SupersededFromStatus: models.ESTIMATE_STATUS_ACCEPTED, RevisionOf: 0, RevisionNumber: 0}
	resp := estimate.ToPublicInvoiceResponse(nil, 0, &models.InvoiceProfile{})
	resp.SupersededBy = "tokenOfTheLatestVersion0000000000000000000"

	data, err := json.Marshal(resp)
	assert.Nil(t, err)

	var body map[string]any
	assert.Nil(t, json.Unmarshal(data, &body))
	assert.Equal(t, "superseded", body["estimateStatus"])
	assert.Equal(t, "tokenOfTheLatestVersion0000000000000000000", body["supersededBy"])
	assert.NotContains(t, string(data), "1234")
	assert.NotContains(t, string(data), "accepted") // the status it had before is internal
	assert.NotContains(t, string(data), "revision")

	// a version nobody replaced carries no supersededBy at all
	estimate.EstimateStatus = models.ESTIMATE_STATUS_SENT
	data, err = json.Marshal(estimate.ToPublicInvoiceResponse(nil, 0, &models.InvoiceProfile{}))
	assert.Nil(t, err)
	assert.NotContains(t, string(data), "supersededBy")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/services/ ./pkg/api/ -run 'TestSupersededPublicView|TestEstimateListShowsLatest|TestEstimateRevisionsList|TestCustomerTimelineShowsRevised|TestPublicSupersededEstimateResponse' -count=1`
Expected: compile FAIL (`Invoices.GetEstimateSupersededBy undefined`, `Invoices.GetLatestEstimates undefined`, `resp.SetEstimateRevisions undefined`, `models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED undefined`, `resp.SupersededBy undefined`).

- [ ] **Step 3: Models**

Append to `pkg/models/estimate.go`:

```go
// EstimateRevisionResponse is one version of an estimate's revision chain as the detail view lists it, newest first
type EstimateRevisionResponse struct {
	Id             string `json:"id"`
	Number         string `json:"number"`
	RevisionNumber int32  `json:"revisionNumber"`
	InvoiceDate    string `json:"invoiceDate"`
	DueDate        string `json:"dueDate"`
	EstimateStatus string `json:"estimateStatus"`
	Total          int64  `json:"total"`
	Currency       string `json:"currency"`
	FullyInvoiced  bool   `json:"fullyInvoiced"`
}
```

In `pkg/models/invoice.go`:

Replace the timeline kinds block with:

```go
// Customer timeline entry kinds
const (
	CUSTOMER_TIMELINE_KIND_INVOICE           = "invoice"
	CUSTOMER_TIMELINE_KIND_EMAIL             = "email"
	CUSTOMER_TIMELINE_KIND_PAYMENT           = "payment"
	CUSTOMER_TIMELINE_KIND_ESTIMATE          = "estimate"
	CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED = "estimate_accepted"
	CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED = "estimate_declined"
	CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED  = "estimate_revised" // invoiceNumber is the new version, estimateNumber the one it revised
)
```

Replace `InvoiceListRequest` with:

```go
// InvoiceListRequest picks invoices (default) or estimates for the list; estimates are the latest version of each chain unless
// IncludeSuperseded asks for every version (the web client's "Show closed" filters them on its side)
type InvoiceListRequest struct {
	Kind              string `form:"kind" binding:"omitempty,oneof=invoice estimate"`
	IncludeSuperseded bool   `form:"include_superseded"`
}
```

In `InvoiceInfoResponse`, after the `RevisionNumber` field add:

```go
	Revisions          []*EstimateRevisionResponse         `json:"revisions,omitempty"`         // estimates: every version of the chain, newest first
```

In `PublicInvoiceResponse`, after `Milestones` add:

```go
	SupersededBy      string                             `json:"supersededBy,omitempty"` // a superseded version: the latest sent version's public token
```

Append to `pkg/models/invoice.go`:

```go
// SetEstimateRevisions fills every version of the estimate's revision chain (newest first) with whether each is fully invoiced
func (r *InvoiceInfoResponse) SetEstimateRevisions(revisions []*Invoice, progress map[int64]*EstimateProgress) {
	r.Revisions = make([]*EstimateRevisionResponse, len(revisions))

	for idx, revision := range revisions {
		r.Revisions[idx] = &EstimateRevisionResponse{
			Id:             utils.Int64ToString(revision.InvoiceId),
			Number:         revision.Number,
			RevisionNumber: revision.RevisionNumber,
			InvoiceDate:    revision.InvoiceDate,
			DueDate:        revision.DueDate,
			EstimateStatus: revision.EstimateStatus,
			Total:          revision.Total,
			Currency:       revision.Currency,
			FullyInvoiced:  progress[revision.InvoiceId] != nil && progress[revision.InvoiceId].FullyInvoiced,
		}
	}
}
```

- [ ] **Step 4: Services**

Append to `pkg/services/estimate_revisions.go`:

```go
// GetEstimateRevisions returns every non-deleted version of the estimate's revision chain, newest (highest revision number) first;
// a first version that was never revised returns just itself
func (s *InvoiceService) GetEstimateRevisions(c core.Context, uid int64, estimate *models.Invoice) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	rootId := estimateChainRootId(estimate)

	var revisions []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND (invoice_id=? OR revision_of=?)", uid, false, models.INVOICE_KIND_ESTIMATE, rootId, rootId).
		OrderBy("revision_number desc").Find(&revisions)

	return revisions, err
}

// GetLatestEstimates returns the latest version of every estimate of user (never a superseded one), newest estimate date first
func (s *InvoiceService) GetLatestEstimates(c core.Context, uid int64) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var estimates []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND estimate_status<>?", uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED).
		OrderBy("invoice_date desc, number desc").Find(&estimates)

	return estimates, err
}

// GetEstimateSupersededBy returns, for a superseded version, the public token of the latest version of its chain when that version was
// sent (sent, accepted or declined) and its link is enabled; otherwise empty. The public page links to the token only, never to an id,
// and never to a draft the client has not been sent.
func (s *InvoiceService) GetEstimateSupersededBy(c core.Context, estimate *models.Invoice) (string, error) {
	if estimate.Kind != models.INVOICE_KIND_ESTIMATE || estimate.EstimateStatus != models.ESTIMATE_STATUS_SUPERSEDED {
		return "", nil
	}

	rootId := estimateChainRootId(estimate)
	latest := &models.Invoice{}
	has, err := s.UserDataDB(estimate.Uid).NewSession(c).
		Where("uid=? AND deleted=? AND kind=? AND estimate_status<>? AND (invoice_id=? OR revision_of=?)",
			estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED, rootId, rootId).
		OrderBy("revision_number desc").Get(latest)

	if err != nil {
		return "", err
	} else if !has || latest.EstimateStatus == models.ESTIMATE_STATUS_DRAFT {
		return "", nil
	}

	return latest.PublicToken, nil
}
```

In `pkg/services/customer_overview.go`, replace

```go
	for _, row := range rows {
		if row.Kind == models.INVOICE_KIND_ESTIMATE {
			overview.Timeline = append(overview.Timeline, estimateTimelineItems(row)...)
			continue
		}
```

with

```go
	previous := previousEstimateVersions(rows)

	for _, row := range rows {
		if row.Kind == models.INVOICE_KIND_ESTIMATE {
			overview.Timeline = append(overview.Timeline, estimateTimelineItems(row, previous[row.InvoiceId])...)
			continue
		}
```

and replace the whole `estimateTimelineItems` function with:

```go
// estimateTimelineItems are an estimate's events on the customer page: created (or, for a revision, "<previous> revised as <number>"),
// then accepted or declined (with the client's note)
func estimateTimelineItems(estimate *models.Invoice, previous *models.Invoice) []*models.CustomerTimelineItemResponse {
	id := utils.Int64ToString(estimate.InvoiceId)
	first := &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_ESTIMATE, UnixTime: estimate.CreatedUnixTime,
		InvoiceId: id, InvoiceNumber: estimate.Number, Amount: estimate.Total, Currency: estimate.Currency}

	if previous != nil {
		first.Kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED
		first.EstimateNumber = previous.Number
	}

	items := []*models.CustomerTimelineItemResponse{first}

	kind := ""

	switch estimate.EstimateStatus {
	case models.ESTIMATE_STATUS_ACCEPTED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED
	case models.ESTIMATE_STATUS_DECLINED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED
	case models.ESTIMATE_STATUS_SUPERSEDED:
		// the answer it had before it was revised is still history
		switch estimate.SupersededFromStatus {
		case models.ESTIMATE_STATUS_ACCEPTED:
			kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED
		case models.ESTIMATE_STATUS_DECLINED:
			kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED
		}
	}

	if kind != "" && estimate.RespondedUnixTime > 0 {
		items = append(items, &models.CustomerTimelineItemResponse{Kind: kind, UnixTime: estimate.RespondedUnixTime, InvoiceId: id, InvoiceNumber: estimate.Number,
			Amount: estimate.Total, Currency: estimate.Currency, Text: estimate.ResponseNote, ByClient: estimate.RespondedByClient})
	}

	return items
}

// previousEstimateVersions maps each revision among rows to the version it revised: the highest lower revision number of the same
// chain still among rows. A revision whose earlier versions belong to another customer gets none and shows as a plain new estimate.
func previousEstimateVersions(rows []*models.Invoice) map[int64]*models.Invoice {
	previous := make(map[int64]*models.Invoice)

	for _, row := range rows {
		if row.Kind != models.INVOICE_KIND_ESTIMATE || row.RevisionNumber == 0 {
			continue
		}

		for _, other := range rows {
			if other.Kind != models.INVOICE_KIND_ESTIMATE || other.RevisionNumber >= row.RevisionNumber ||
				(other.InvoiceId != row.RevisionOf && other.RevisionOf != row.RevisionOf) {
				continue
			}

			if current := previous[row.InvoiceId]; current == nil || other.RevisionNumber > current.RevisionNumber {
				previous[row.InvoiceId] = other
			}
		}
	}

	return previous
}
```

- [ ] **Step 5: API**

In `pkg/api/invoices.go`, `InvoiceListHandler`, replace the doc comment and the fetch:

```go
// InvoiceListHandler returns all invoices (or, with kind=estimate, the latest version of every estimate; every version with
// include_superseded=true) of current user with received amounts
```

```go
	if req.Kind == models.INVOICE_KIND_ESTIMATE && req.IncludeSuperseded {
		invoices, err = a.invoices.GetAllEstimates(c, uid)
	} else if req.Kind == models.INVOICE_KIND_ESTIMATE {
		invoices, err = a.invoices.GetLatestEstimates(c, uid)
	} else {
		invoices, err = a.invoices.GetAllInvoices(c, uid)
	}
```

In `getInvoiceResponse`, replace

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		progress, err := a.invoices.GetEstimateProgress(c, uid, []int64{invoiceId})

		if err != nil {
			return nil, err
		}

		resp.SetEstimateProgress(progress[invoiceId])
	} else if invoice.EstimateId > 0 {
```

with

```go
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		// the chain always includes this estimate itself, so one progress query covers the detail and the Revisions list
		revisions, err := a.invoices.GetEstimateRevisions(c, uid, invoice)

		if err != nil {
			return nil, err
		}

		revisionIds := make([]int64, len(revisions))

		for idx, revision := range revisions {
			revisionIds[idx] = revision.InvoiceId
		}

		progress, err := a.invoices.GetEstimateProgress(c, uid, revisionIds)

		if err != nil {
			return nil, err
		}

		resp.SetEstimateProgress(progress[invoiceId])
		resp.SetEstimateRevisions(revisions, progress)
	} else if invoice.EstimateId > 0 {
```

In `InvoicePublicViewHandler`, replace

```go
		resp.Milestones = models.ToPublicEstimateMilestoneResponses(milestones)
	}

	return resp, nil
}
```

with

```go
		resp.Milestones = models.ToPublicEstimateMilestoneResponses(milestones)
		resp.SupersededBy, err = a.invoices.GetEstimateSupersededBy(c, invoice)

		if err != nil {
			log.Errorf(c, "[invoices.InvoicePublicViewHandler] failed to get the latest version of estimate \"id:%d\", because %s", invoice.InvoiceId, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	return resp, nil
}
```

Then: `gofmt -w pkg/models/estimate.go pkg/models/invoice.go pkg/services/estimate_revisions.go pkg/services/customer_overview.go pkg/api/invoices.go`

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/services/ ./pkg/api/ -count=1 && gofmt -l pkg cmd`
Expected: PASS (including `TestCustomerOverviewShowsEstimateEventsButNeverCountsEstimatesAsBought`, unchanged for first versions); gofmt prints nothing.

- [ ] **Step 7: Commit**

```bash
git add pkg/models pkg/services pkg/api
git commit --no-verify -m "estimates: revisions in the detail, latest-only list, public link to the latest version, revised timeline event"
```

---

### Task 4: Frontend data layer, helpers and shared bases

**Files:**
- Modify: `src/lib/estimate.ts`, `src/lib/__tests__/estimate.test.ts`
- Modify: `src/models/invoice.ts`, `src/lib/services.ts`, `src/stores/invoice.ts`
- Modify: `src/views/base/invoices/EstimateListPageBase.ts`, `InvoiceDetailPageBase.ts`, `CustomerDetailPageBase.ts`
- Modify: `src/locales/en.json`

**Interfaces:**
- Consumes (Tasks 1–3): `POST v1/estimates/revise.json {id}` → `InvoiceInfoResponse`; `list.json?kind=estimate&include_superseded=true`; response fields `revisionNumber`, `revisions[]`, `supersededBy`; timeline kind `estimate_revised`.
- Produces:
  - `EstimateStatus` gains `'superseded'`; `EstimateDisplayStatus` gains `'superseded'`; `ESTIMATE_DISPLAY_STATUS_NAMES.superseded = 'Superseded'`.
  - `getEstimateDisplayStatus` → `'superseded'` for superseded; `isEstimateClosed` true for superseded; `isEstimateLocked` true unless draft and nothing invoiced.
  - `canReviseEstimate(estimate: { estimateStatus: EstimateStatus | '', convertedInvoices: readonly unknown[] }): boolean`, `canDeleteEstimate(estimate: { estimateStatus: EstimateStatus | '', revisionNumber: number }): boolean`, `getEstimateRevisionLabel(revisionNumber: number): { key: string, params: { number: number } }`, `getSupersedingRevision<T extends { readonly revisionNumber: number }>(revisions: readonly T[], revisionNumber: number): T | null`.
  - `interface EstimateRevision { id, number: string; revisionNumber: number; invoiceDate, dueDate: TextualYearMonthDay; estimateStatus: EstimateStatus; total: number; currency: string; fullyInvoiced: boolean }`; `InvoiceInfoResponse.revisionNumber?`, `.revisions?`; `Invoice.revisionNumber: number`, `Invoice.revisions: EstimateRevision[]`; `PublicInvoiceResponse.supersededBy?: string`; `CustomerTimelineItem.kind` gains `'estimate_revised'`.
  - `services.reviseEstimate({ id }): ApiResponsePromise<InvoiceInfoResponse>`; `services.getAllEstimates()` asks for `include_superseded=true`.
  - Store `reviseEstimate({ estimateId }): Promise<Invoice>`.
  - `ESTIMATE_STATUS_COLORS.superseded = 'grey'`.
  - `useInvoiceDetailPageBase()` adds `canRevise`, `canDelete`, `canSend`, `isSuperseded`, `supersededBy: EstimateRevision | null`, `hasRevisions`, `getRevisionLabel(revision)`, `getRevisionStatusName(revision)`, `reviseEstimate(): Promise<Invoice>`.
  - Locale keys: `Superseded`, `Revise`, `Revisions`, `Unable to revise estimate`, `This estimate has been revised.`, `View the latest version`, `format.misc.estimateRevision`, `format.misc.supersededBy`, `format.misc.timelineEstimateRevised`.

- [ ] **Step 1: Write the failing tests**

In `src/lib/__tests__/estimate.test.ts` replace the import block and the `estimate` helper with:

```ts
import { describe, expect, test } from 'vitest';

import {
    type EstimateStatus,
    ESTIMATE_DISPLAY_STATUS_NAMES,
    getEstimateDisplayStatus,
    isEstimateClosed,
    isEstimateLocked,
    canAnswerEstimate,
    canReviseEstimate,
    canDeleteEstimate,
    getEstimateRevisionLabel,
    getSupersedingRevision,
    getMilestoneAmounts,
    getMilestonePercentTotal,
    getMilestonesProblem,
    addMilestonePercent,
    formatMilestonePercent,
    unixTimeToTextualDate
} from '@/lib/estimate.ts';
import { type EstimateRevision, Invoice } from '@/models/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const estimate = (estimateStatus: '' | EstimateStatus, dueDate: TextualYearMonthDay, fullyInvoiced = false) =>
    ({ kind: 'estimate', estimateStatus, dueDate, fullyInvoiced, revisionNumber: 0, convertedInvoices: [] as { id: string, number: string }[] });
```

Replace the test

```ts
    test('editable while draft or sent and nothing invoiced', () => {
        expect(isEstimateLocked(estimate('sent', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked({ ...estimate('sent', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(true);
    });
```

with

```ts
    test('edited in place only while a draft with nothing invoiced', () => {
        expect(isEstimateLocked(estimate('draft', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('', '2099-01-01'))).toBe(false);
        expect(isEstimateLocked(estimate('sent', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked(estimate('superseded', '2099-01-01'))).toBe(true);
        expect(isEstimateLocked({ ...estimate('draft', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(true);
    });
```

Append:

```ts
describe('estimate revisions', () => {
    test('a superseded version shows as superseded, whatever its dates, and is closed', () => {
        expect(getEstimateDisplayStatus(estimate('superseded', '2026-01-01'), '2026-10-30')).toBe('superseded');
        expect(getEstimateDisplayStatus(estimate('superseded', '2099-01-01'), '2026-10-30')).toBe('superseded');
        expect(ESTIMATE_DISPLAY_STATUS_NAMES.superseded).toBe('Superseded');
        expect(isEstimateClosed(estimate('superseded', '2099-01-01'))).toBe(true);
        expect(canAnswerEstimate(estimate('superseded', '2099-01-01'), '2026-10-30')).toBe(false);
    });

    test('Revise is offered on sent (also expired), declined and accepted-not-invoiced versions', () => {
        expect(canReviseEstimate(estimate('sent', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('sent', '2026-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('declined', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate(estimate('accepted', '2099-01-01'))).toBe(true);
        expect(canReviseEstimate({ ...estimate('accepted', '2099-01-01'), convertedInvoices: [{ id: '1', number: 'INV/2026-27/01' }] })).toBe(false);
        expect(canReviseEstimate(estimate('draft', '2099-01-01'))).toBe(false);
        expect(canReviseEstimate(estimate('superseded', '2099-01-01'))).toBe(false);
    });

    test('a superseded version or a revision that was sent cannot be deleted', () => {
        expect(canDeleteEstimate({ estimateStatus: 'sent', revisionNumber: 0 })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'draft', revisionNumber: 2 })).toBe(true);
        expect(canDeleteEstimate({ estimateStatus: 'sent', revisionNumber: 2 })).toBe(false);
        expect(canDeleteEstimate({ estimateStatus: 'superseded', revisionNumber: 0 })).toBe(false);
    });

    test('revision labels', () => {
        expect(getEstimateRevisionLabel(0)).toEqual({ key: 'Original', params: { number: 0 } });
        expect(getEstimateRevisionLabel(2)).toEqual({ key: 'format.misc.estimateRevision', params: { number: 2 } });
    });

    test('a version is superseded by the next version still in the chain', () => {
        const revisions = [{ id: 'c', revisionNumber: 3 }, { id: 'a', revisionNumber: 0 }]; // R1 and R2 were deleted drafts
        expect(getSupersedingRevision(revisions, 0)?.id).toBe('c');
        expect(getSupersedingRevision(revisions, 3)).toBeNull();
        expect(getSupersedingRevision([], 0)).toBeNull();
    });

    test('the detail response carries the revision number and every version of the chain', () => {
        const revisions: EstimateRevision[] = [
            { id: '9', number: 'EST/2026-27/01-R1', revisionNumber: 1, invoiceDate: '2026-10-30', dueDate: '2026-11-29', estimateStatus: 'draft', total: 100, currency: 'INR', fullyInvoiced: false },
            { id: '7', number: 'EST/2026-27/01', revisionNumber: 0, invoiceDate: '2026-04-23', dueDate: '2099-12-31', estimateStatus: 'superseded', total: 100, currency: 'INR', fullyInvoiced: false }
        ];
        const old = Invoice.of({ ...Invoice.createNewEstimate(), id: '7', estimateStatus: 'superseded', revisions });

        expect(old.revisionNumber).toBe(0);
        expect(old.revisions.map(revision => revision.number)).toEqual(['EST/2026-27/01-R1', 'EST/2026-27/01']);
        expect(getSupersedingRevision(old.revisions, old.revisionNumber)?.number).toBe('EST/2026-27/01-R1');
        expect(old.clone().revisions).toEqual(revisions);
        expect(old.duplicate().revisions).toEqual([]); // a duplicate is a new first version
        expect(Invoice.of({ ...Invoice.createNewEstimate(), id: '9', revisionNumber: 1 }).revisionNumber).toBe(1);
    });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/estimate.test.ts`
Expected: FAIL (`canReviseEstimate is not a function`, `EstimateRevision` has no exported member, `'superseded'` not assignable to `EstimateStatus`).

- [ ] **Step 3: Helpers**

In `src/lib/estimate.ts` replace

```ts
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
```

with

```ts
// mirrors models.ESTIMATE_STATUS_*; expired and invoiced are only ever displayed; superseded is a version a revision replaced
export type EstimateStatus = 'draft' | 'sent' | 'accepted' | 'declined' | 'superseded';
export type EstimateDisplayStatus = EstimateStatus | 'expired' | 'invoiced';

export const ESTIMATE_DISPLAY_STATUS_NAMES: Record<EstimateDisplayStatus, string> = {
    draft: 'Draft',
    sent: 'Sent',
    accepted: 'Accepted',
    declined: 'Declined',
    superseded: 'Superseded',
    expired: 'Expired',
    invoiced: 'Invoiced'
};
```

Replace `getEstimateDisplayStatus`, `isEstimateClosed` and `isEstimateLocked` with:

```ts
// expired: a draft or sent estimate whose valid-until date (dueDate) is before today; extending the date revives it.
// A superseded version is history: it never shows as expired or invoiced
export function getEstimateDisplayStatus(estimate: EstimateState, today: string): EstimateDisplayStatus {
    if (estimate.estimateStatus === 'superseded') {
        return 'superseded';
    }

    if (estimate.fullyInvoiced) {
        return 'invoiced';
    }

    const status = estimate.estimateStatus || 'draft';

    if ((status === 'draft' || status === 'sent') && estimate.dueDate < today) {
        return 'expired';
    }

    return status;
}

// hidden from the list unless "Show closed": declined, fully invoiced, or superseded by a newer version
export function isEstimateClosed(estimate: EstimateState): boolean {
    return estimate.estimateStatus === 'declined' || estimate.estimateStatus === 'superseded' || estimate.fullyInvoiced;
}

// read-only unless a draft with nothing invoiced from it, mirroring checkEstimateEditable in pkg/services/estimate_lifecycle.go;
// once sent it is changed through Revise
export function isEstimateLocked(estimate: { readonly estimateStatus: EstimateStatus | '', readonly convertedInvoices: readonly unknown[] }): boolean {
    return (estimate.estimateStatus || 'draft') !== 'draft' || estimate.convertedInvoices.length > 0;
}

// Revise is offered on a version that is sent (expired too), declined, or accepted with nothing invoiced from it; a superseded version is
// never the latest, so this is also "latest only". Mirrors checkEstimateRevisable in pkg/services/estimate_revisions.go
export function canReviseEstimate(estimate: { readonly estimateStatus: EstimateStatus | '', readonly convertedInvoices: readonly unknown[] }): boolean {
    const status = estimate.estimateStatus;
    return (status === 'sent' || status === 'declined' || status === 'accepted') && !estimate.convertedInvoices.length;
}

// a superseded version is history and a revision that was sent stays (answer or revise it); a first version or a draft revision can be
// deleted. Mirrors checkEstimateDeletable in pkg/services/estimate_revisions.go
export function canDeleteEstimate(estimate: { readonly estimateStatus: EstimateStatus | '', readonly revisionNumber: number }): boolean {
    if (estimate.estimateStatus === 'superseded') {
        return false;
    }

    return estimate.revisionNumber === 0 || (estimate.estimateStatus || 'draft') === 'draft';
}

// a version's name in the Revisions list as a locale key and its parameters: "Original", then "Revision 1", "Revision 2", …
export function getEstimateRevisionLabel(revisionNumber: number): { key: string, params: { number: number } } {
    return revisionNumber > 0 ? { key: 'format.misc.estimateRevision', params: { number: revisionNumber } } : { key: 'Original', params: { number: 0 } };
}

// the version that replaced this one: the lowest higher revision number still in the chain; null for the latest version
export function getSupersedingRevision<T extends { readonly revisionNumber: number }>(revisions: readonly T[], revisionNumber: number): T | null {
    let next: T | null = null;

    for (const revision of revisions) {
        if (revision.revisionNumber > revisionNumber && (!next || revision.revisionNumber < next.revisionNumber)) {
            next = revision;
        }
    }

    return next;
}
```

- [ ] **Step 4: Models**

In `src/models/invoice.ts`, after the `EstimateConvertedInvoice` interface add:

```ts
// one version of an estimate's revision chain, as the detail response lists it (newest first)
export interface EstimateRevision {
    readonly id: string;
    readonly number: string;
    readonly revisionNumber: number; // 0 = the first version
    readonly invoiceDate: TextualYearMonthDay;
    readonly dueDate: TextualYearMonthDay;
    readonly estimateStatus: EstimateStatus;
    readonly total: number;
    readonly currency: string;
    readonly fullyInvoiced: boolean;
}
```

In `InvoiceInfoResponse` replace

```ts
    readonly convertedInvoices?: EstimateConvertedInvoice[];
    readonly fullyInvoiced?: boolean;
}
```

with

```ts
    readonly convertedInvoices?: EstimateConvertedInvoice[];
    readonly fullyInvoiced?: boolean;
    readonly revisionNumber?: number; // estimates: absent on a first version
    readonly revisions?: EstimateRevision[]; // estimate detail: every version of the chain, newest first
}
```

In `PublicInvoiceResponse` replace

```ts
    readonly milestones?: PublicEstimateMilestone[];
}
```

with

```ts
    readonly milestones?: PublicEstimateMilestone[];
    readonly supersededBy?: string; // a superseded version: the public token of the latest version that was sent
}
```

In the `Invoice` class replace

```ts
    public fullyInvoiced: boolean = false;
```

with

```ts
    public fullyInvoiced: boolean = false;
    public revisionNumber: number = 0; // estimates: 0 on the first version
    public revisions: EstimateRevision[] = []; // estimate detail only
```

and in `fillFrom` replace

```ts
        this.fullyInvoiced = other.fullyInvoiced ?? false;
```

with

```ts
        this.fullyInvoiced = other.fullyInvoiced ?? false;
        this.revisionNumber = other.revisionNumber ?? 0;
        this.revisions = [...(other.revisions ?? [])];
```

In `CustomerTimelineItem` replace

```ts
    readonly kind: 'invoice' | 'email' | 'payment' | 'estimate' | 'estimate_accepted' | 'estimate_declined';
```

with

```ts
    readonly kind: 'invoice' | 'email' | 'payment' | 'estimate' | 'estimate_accepted' | 'estimate_declined' | 'estimate_revised';
```

- [ ] **Step 5: Services, store and bases**

In `src/lib/services.ts` replace

```ts
    getAllEstimates: (): ApiResponsePromise<InvoiceInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceInfoResponse[]>>('v1/invoices/list.json?kind=estimate');
    },
```

with

```ts
    // every version: superseded ones are hidden on the client side unless "Show closed", like declined and invoiced estimates
    getAllEstimates: (): ApiResponsePromise<InvoiceInfoResponse[]> => {
        return axios.get<ApiResponse<InvoiceInfoResponse[]>>('v1/invoices/list.json?kind=estimate&include_superseded=true');
    },
```

and after `reopenEstimate` add:

```ts
    reviseEstimate: ({ id }: { id: string }): ApiResponsePromise<InvoiceInfoResponse> => {
        return axios.post<ApiResponse<InvoiceInfoResponse>>('v1/estimates/revise.json', { id });
    },
```

In `src/stores/invoice.ts`, after `reopenEstimate` add:

```ts
    // the next version of a sent or answered estimate, created as a new draft; the old version is superseded
    function reviseEstimate({ estimateId }: { estimateId: string }): Promise<Invoice> {
        return request(services.reviseEstimate({ id: estimateId }), 'Unable to revise estimate').then(result => Invoice.of(result));
    }
```

and in the returned object replace

```ts
        reopenEstimate,
        getEstimateInvoiceDraft,
```

with

```ts
        reopenEstimate,
        reviseEstimate,
        getEstimateInvoiceDraft,
```

In `src/views/base/invoices/EstimateListPageBase.ts` replace

```ts
    expired: 'warning',
    invoiced: 'secondary'
};

// the estimate list: open estimates by default, declined and fully invoiced ones behind "Show closed"
```

with

```ts
    expired: 'warning',
    invoiced: 'secondary',
    superseded: 'grey'
};

// the estimate list: the latest version of each open estimate by default; declined, fully invoiced and superseded ones behind "Show closed"
```

In `src/views/base/invoices/CustomerDetailPageBase.ts` replace

```ts
            case 'estimate_declined':
                return tt(item.byClient ? 'format.misc.timelineEstimateDeclinedByClient' : 'format.misc.timelineEstimateDeclined', { number: item.invoiceNumber });
```

with

```ts
            case 'estimate_declined':
                return tt(item.byClient ? 'format.misc.timelineEstimateDeclinedByClient' : 'format.misc.timelineEstimateDeclined', { number: item.invoiceNumber });
            case 'estimate_revised':
                return tt('format.misc.timelineEstimateRevised', { estimate: item.estimateNumber ?? '', number: item.invoiceNumber });
```

In `src/views/base/invoices/InvoiceDetailPageBase.ts`:

Replace the two imports

```ts
import type { Invoice, InvoiceEmail, InvoicePayment, InvoiceProfile, InvoiceSendRequest, InvoiceRenewalOption } from '@/models/invoice.ts';
```

```ts
import { type EstimateDisplayStatus, ESTIMATE_DISPLAY_STATUS_NAMES, getEstimateDisplayStatus, isEstimateLocked, getMilestoneAmounts } from '@/lib/estimate.ts';
```

with

```ts
import type { Invoice, InvoiceEmail, InvoicePayment, InvoiceProfile, InvoiceSendRequest, InvoiceRenewalOption, EstimateRevision } from '@/models/invoice.ts';
```

```ts
import {
    type EstimateDisplayStatus,
    ESTIMATE_DISPLAY_STATUS_NAMES,
    getEstimateDisplayStatus,
    isEstimateLocked,
    canReviseEstimate,
    canDeleteEstimate,
    getEstimateRevisionLabel,
    getSupersedingRevision,
    getMilestoneAmounts
} from '@/lib/estimate.ts';
```

Replace

```ts
    // an estimate is edited only while draft or sent and nothing is invoiced from it; Duplicate always works
    const canEdit = computed<boolean>(() => !!invoice.value && (!isEstimate.value || !isEstimateLocked(invoice.value)));
```

with

```ts
    // an estimate is edited in place only while a draft with nothing invoiced from it; Duplicate always works
    const canEdit = computed<boolean>(() => !!invoice.value && (!isEstimate.value || !isEstimateLocked(invoice.value)));
    // a sent (or expired), declined or accepted-not-invoiced estimate is changed by revising it into a new draft version
    const canRevise = computed<boolean>(() => isEstimate.value && !!invoice.value && canReviseEstimate(invoice.value));
    const isSuperseded = computed<boolean>(() => isEstimate.value && invoice.value?.estimateStatus === 'superseded');
    // a superseded version, or a revision that was sent, is never deleted; invoices and first versions are
    const canDelete = computed<boolean>(() => !!invoice.value && (!isEstimate.value || canDeleteEstimate(invoice.value)));
    // emailing a superseded version would send terms that no longer stand
    const canSend = computed<boolean>(() => !!invoice.value && !isSuperseded.value);
    const supersededBy = computed<EstimateRevision | null>(() => invoice.value && isSuperseded.value ? getSupersedingRevision(invoice.value.revisions, invoice.value.revisionNumber) : null);
    // the Revisions section appears once the estimate has more than one version
    const hasRevisions = computed<boolean>(() => isEstimate.value && (invoice.value?.revisions.length ?? 0) > 1);
```

After `reopenEstimate()` add:

```ts
    // creates the next version as a new draft and resolves with it; the page then shows it and opens it in the editor
    function reviseEstimate(): Promise<Invoice> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to revise estimate' });
        }

        return invoicesStore.reviseEstimate({ estimateId: current.id });
    }

    // "Original" / "Revision 2"
    function getRevisionLabel(revision: EstimateRevision): string {
        const label = getEstimateRevisionLabel(revision.revisionNumber);
        return tt(label.key, label.params);
    }

    function getRevisionStatusName(revision: EstimateRevision): string {
        return ESTIMATE_DISPLAY_STATUS_NAMES[getEstimateDisplayStatus(revision, getTodayTextualDate())];
    }
```

In the returned object replace

```ts
        milestoneAmounts,
        // functions
```

with

```ts
        milestoneAmounts,
        canRevise,
        isSuperseded,
        canDelete,
        canSend,
        supersededBy,
        hasRevisions,
        // functions
```

and replace

```ts
        setEstimateResponse,
        reopenEstimate
    };
```

with

```ts
        setEstimateResponse,
        reopenEstimate,
        reviseEstimate,
        getRevisionLabel,
        getRevisionStatusName
    };
```

- [ ] **Step 6: Locale strings**

In `src/locales/en.json` replace

```json
            "estimateExpiredOn": "This estimate expired on {date}",
```

with

```json
            "estimateExpiredOn": "This estimate expired on {date}",
            "estimateRevision": "Revision {number}",
            "supersededBy": "Superseded by {number}",
            "timelineEstimateRevised": "Estimate {estimate} revised as {number}",
```

and replace

```json
    "Invoiced": "Invoiced",
```

with

```json
    "Invoiced": "Invoiced",
    "Superseded": "Superseded",
    "Revise": "Revise",
    "Revisions": "Revisions",
    "Unable to revise estimate": "Unable to revise estimate",
    "This estimate has been revised.": "This estimate has been revised.",
    "View the latest version": "View the latest version",
```

- [ ] **Step 7: Run tests and typecheck**

Run: `npx vitest run --exclude '.claude/**' src/lib/__tests__/estimate.test.ts && npx vue-tsc --noEmit && npx eslint src/lib src/models src/stores src/views/base`
Expected: PASS / clean.

- [ ] **Step 8: Commit**

```bash
git add src/lib src/models src/stores src/views/base src/locales/en.json
git commit --no-verify -m "estimates: revision data layer, superseded status, revise / delete / edit rules and revision helpers"
```

---

### Task 5: Desktop: Revise, the Revisions section, superseded read-only, the public page's revised notice

**Files:**
- Modify: `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue`
- Modify: `src/views/desktop/invoices/PublicInvoicePage.vue`

**Interfaces:**
- Consumes (Task 4): `canRevise`, `canDelete`, `canSend`, `isSuperseded`, `supersededBy`, `hasRevisions`, `getRevisionLabel`, `getRevisionStatusName`, `reviseEstimate`, `ESTIMATE_STATUS_COLORS.superseded`, `Invoice.revisions`, `PublicInvoiceResponse.supersededBy`, `getInvoicePublicLinkUrl(token)` (existing, `src/lib/invoice.ts`), `InvoiceEditDialog.open({ id, currentInvoice })` (existing), locale keys from Task 4.
- Produces: desktop detail toolbar shows **Revise** in place of Edit when `canRevise`; Delete disabled unless `canDelete`; Send disabled unless `canSend`; "Superseded by <number>" link; Revisions list (newest first, current one active, any row opens that version); the public page shows "This estimate has been revised." and, with `supersededBy`, "View the latest version". The Estimates view of `invoices/ListPage.vue` is unchanged: `displayEstimates` from the base now hides superseded versions until "Show Closed Estimates", and the status chip uses `ESTIMATE_STATUS_COLORS.superseded`.

- [ ] **Step 1: Detail toolbar**

In `src/views/desktop/invoices/dialogs/InvoiceDetailDialog.vue` replace

```html
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Edit')" :disabled="!invoice || loading || updating || !canEdit" @click="edit">
                    <v-icon :icon="mdiPencilOutline" />
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
```

with

```html
                <!-- a sent or answered estimate is never edited in place: Revise makes the next version as a new draft -->
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Revise')" :disabled="loading || updating" @click="revise"
                       v-if="canRevise">
                    <v-icon :icon="mdiFileDocumentEditOutline" />
                    <v-tooltip activator="parent">{{ tt('Revise') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Edit')" :disabled="!invoice || loading || updating || !canEdit" @click="edit"
                       v-else>
                    <v-icon :icon="mdiPencilOutline" />
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
```

Replace

```html
                       :aria-label="tt('Delete')" :disabled="!invoice || loading || updating" @click="remove">
```

with

```html
                       :aria-label="tt('Delete')" :disabled="!invoice || loading || updating || !canDelete" @click="remove">
```

Replace

```html
                       :aria-label="tt('Send')" :disabled="!invoice || !profile || loading || updating" @click="openSend">
```

with

```html
                       :aria-label="tt('Send')" :disabled="!invoice || !profile || loading || updating || !canSend" @click="openSend">
```

- [ ] **Step 2: Superseded notice and Revisions section**

Replace

```html
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line" v-if="invoice.responseNote">{{ tt("Client's note") }}: {{ invoice.responseNote }}</div>
```

with

```html
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line" v-if="invoice.responseNote">{{ tt("Client's note") }}: {{ invoice.responseNote }}</div>
                            <div class="text-body-medium px-4 pb-2" v-if="supersededBy">
                                <a class="cursor-pointer" @click="openOther(supersededBy.id)">{{ tt('format.misc.supersededBy', { number: supersededBy.number }) }}</a>
                            </div>
```

Replace

```html
                            <v-divider class="my-2" />
                            <template v-if="invoice.milestones.length">
```

with

```html
                            <template v-if="hasRevisions">
                                <v-divider class="my-2" />
                                <div class="text-title-small px-4 py-2">{{ tt('Revisions') }}</div>
                                <v-list density="compact">
                                    <v-list-item :key="revision.id" v-for="revision in invoice.revisions" link
                                                 :active="revision.id === invoice.id"
                                                 :title="revision.number"
                                                 :subtitle="`${getRevisionLabel(revision)} · ${formatDate(revision.invoiceDate)} · ${tt(getRevisionStatusName(revision))}`"
                                                 @click="openOther(revision.id)">
                                        <template #append><span>{{ formatAmount(revision.total, revision.currency) }}</span></template>
                                    </v-list-item>
                                </v-list>
                            </template>

                            <v-divider class="my-2" />
                            <template v-if="invoice.milestones.length">
```

- [ ] **Step 3: Detail script**

Replace

```ts
    mdiShareVariantOutline,
    mdiEmailOutline
} from '@mdi/js';
```

with

```ts
    mdiShareVariantOutline,
    mdiEmailOutline,
    mdiFileDocumentEditOutline
} from '@mdi/js';
```

In the `useInvoiceDetailPageBase()` destructuring replace

```ts
    milestoneAmounts,
    setEstimateResponse,
    reopenEstimate,
```

with

```ts
    milestoneAmounts,
    canRevise,
    canDelete,
    canSend,
    supersededBy,
    hasRevisions,
    getRevisionLabel,
    getRevisionStatusName,
    reviseEstimate,
    setEstimateResponse,
    reopenEstimate,
```

After `function reopen(): void { ... }` add:

```ts
// creates the next version as a new draft, shows it here and opens it in the editor; the version the client saw stays as it was
function revise(): void {
    updating.value = true;

    reviseEstimate().then(revised => {
        updating.value = false;
        changed.value = true;
        openOther(revised.id);

        invoiceEditDialog.value?.open({ id: revised.id, currentInvoice: revised }).then(result => {
            snackbar.value?.showMessage(result.message);
            load();
        }).catch(error => {
            if (error && !error.processed) {
                snackbar.value?.showError(error);
            }
        });
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}
```

- [ ] **Step 4: Public page**

In `src/views/desktop/invoices/PublicInvoicePage.vue` replace

```html
            <span class="public-invoice-status" :class="{ 'public-invoice-status-paid': isPaid }" v-else>
                {{ isPaid ? tt('Paid') : tt('Amount Due') + ': ' + formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}
            </span>
            <v-spacer />
```

with

```html
            <span class="public-invoice-status" :class="{ 'public-invoice-status-paid': isPaid }" v-else>
                {{ isPaid ? tt('Paid') : tt('Amount Due') + ': ' + formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}
            </span>
            <a class="public-invoice-latest-link" :href="latestVersionUrl" v-if="latestVersionUrl">{{ tt('View the latest version') }}</a>
            <v-spacer />
```

Replace

```ts
import { formatInvoiceDocumentAmount, formatInvoiceDocumentDate, getInvoicePdfFileName, getInvoiceDisplayBalanceDue, getInvoiceDisplayStatus, getTodayTextualDate, printInvoiceDocument } from '@/lib/invoice.ts';
```

with

```ts
import {
    formatInvoiceDocumentAmount,
    formatInvoiceDocumentDate,
    getInvoicePdfFileName,
    getInvoiceDisplayBalanceDue,
    getInvoiceDisplayStatus,
    getInvoicePublicLinkUrl,
    getTodayTextualDate,
    printInvoiceDocument
} from '@/lib/invoice.ts';
```

Replace

```ts
const declineNote = ref<string>('');
```

with

```ts
const declineNote = ref<string>('');
// a superseded version: the public token of the latest version that was sent, empty when there is none to show
const supersededBy = ref<string>('');
const latestVersionUrl = computed<string>(() => supersededBy.value ? getInvoicePublicLinkUrl(supersededBy.value) : '');
```

Replace

```ts
    if (!current || !isEstimate.value) {
        return '';
    }

    if ((current.estimateStatus === 'accepted'
```

with

```ts
    if (!current || !isEstimate.value) {
        return '';
    }

    if (current.estimateStatus === 'superseded') {
        return tt('This estimate has been revised.');
    }

    if ((current.estimateStatus === 'accepted'
```

Replace

```ts
            invoice.value = Invoice.ofPublic(data.result);
            profile.value = InvoiceProfile.ofPublic(data.result.profile);
```

with

```ts
            invoice.value = Invoice.ofPublic(data.result);
            profile.value = InvoiceProfile.ofPublic(data.result.profile);
            supersededBy.value = data.result.supersededBy ?? '';
```

Replace

```css
.public-invoice-status-paid {
    color: #15803d;
}
```

with

```css
.public-invoice-status-paid {
    color: #15803d;
}

.public-invoice-latest-link {
    color: #1d4ed8;
    font-weight: 600;
}
```

- [ ] **Step 5: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/desktop/invoices`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add src/views/desktop/invoices
git commit --no-verify -m "estimates: desktop Revise, Revisions section, superseded read-only view and the public page's revised notice"
```

---

### Task 6: Mobile parity: Revise, Revisions, superseded read-only, list delete rules

**Files:**
- Modify: `src/views/mobile/invoices/DetailPage.vue`
- Modify: `src/views/mobile/invoices/EstimateListPage.vue`

**Interfaces:**
- Consumes (Task 4): the same base members as Task 5; `canDeleteEstimate`; routes `/invoice/detail?id=` and `/invoice/edit?id=` (existing).
- Produces: the More sheet offers **Revise** (in place of Edit) when `canRevise`; Send hidden unless `canSend`; Delete hidden unless `canDelete`; a "Superseded by <number>" row linking to that version; a Revisions list whose rows open the other versions; Revise turns this page into the new draft and opens `/invoice/edit?id=<new>`; the estimate list hides the Delete swipe for versions that cannot be deleted. The list's Edit swipe already follows `isEstimateLocked` (drafts only now).

- [ ] **Step 1: Detail page markup**

In `src/views/mobile/invoices/DetailPage.vue` replace

```html
                <f7-list-item :title="tt(`Client's note`)" :footer="invoice.responseNote" v-if="invoice.responseNote"></f7-list-item>
            </f7-list>
```

with

```html
                <f7-list-item :title="tt(`Client's note`)" :footer="invoice.responseNote" v-if="invoice.responseNote"></f7-list-item>
                <f7-list-item :title="tt('format.misc.supersededBy', { number: supersededBy.number })"
                              :link="'/invoice/detail?id=' + supersededBy.id" v-if="supersededBy"></f7-list-item>
            </f7-list>
```

Replace

```html
            <f7-list strong inset dividers class="margin-vertical" v-if="!isEstimate">
                <f7-list-item group-title>
                    <small>{{ tt('Payments') }}</small>
```

with

```html
            <f7-list strong inset dividers class="margin-vertical" v-if="hasRevisions">
                <f7-list-item group-title>
                    <small>{{ tt('Revisions') }}</small>
                </f7-list-item>
                <f7-list-item :key="revision.id" v-for="revision in invoice.revisions"
                              :title="revision.number"
                              :footer="`${getRevisionLabel(revision)} · ${formatDate(revision.invoiceDate)} · ${tt(getRevisionStatusName(revision))}`"
                              :after="formatAmount(revision.total, revision.currency)"
                              :link="revision.id !== invoice.id ? '/invoice/detail?id=' + revision.id : undefined"></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical" v-if="!isEstimate">
                <f7-list-item group-title>
                    <small>{{ tt('Payments') }}</small>
```

Replace

```html
                <f7-actions-button v-if="canEdit" @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
```

with

```html
                <f7-actions-button v-if="canRevise" @click="revise">{{ tt('Revise') }}</f7-actions-button>
                <f7-actions-button v-else-if="canEdit" @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
```

Replace

```html
                <f7-actions-button @click="f7router.navigate(`/invoice/send?id=${invoiceId}`)">{{ tt('Send') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button color="red" @click="showDeleteActionSheet = true">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
```

with

```html
                <f7-actions-button v-if="canSend" @click="f7router.navigate(`/invoice/send?id=${invoiceId}`)">{{ tt('Send') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group v-if="canDelete">
                <f7-actions-button color="red" @click="showDeleteActionSheet = true">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
```

- [ ] **Step 2: Detail page script**

In the `useInvoiceDetailPageBase()` destructuring replace

```ts
    milestoneAmounts,
    formatAmount,
```

with

```ts
    milestoneAmounts,
    canRevise,
    canDelete,
    canSend,
    supersededBy,
    hasRevisions,
    getRevisionLabel,
    getRevisionStatusName,
    reviseEstimate,
    formatAmount,
```

After `function reopen(): void { ... }` add:

```ts
// creates the next version as a new draft; this page switches to it (back from the editor reloads it) and the editor opens on it
function revise(): void {
    showMoreActionSheet.value = false;
    updating.value = true;
    showLoading(() => updating.value);

    reviseEstimate().then(revised => {
        updating.value = false;
        hideLoading();
        invoiceId.value = revised.id;
        props.f7router.navigate(`/invoice/edit?id=${revised.id}`);
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}
```

- [ ] **Step 3: Estimate list**

In `src/views/mobile/invoices/EstimateListPage.vue` replace

```html
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(estimate, false)">
```

with

```html
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')"
                                        v-if="canDeleteEstimate(estimate)" @click="remove(estimate, false)">
```

and replace

```ts
import { isEstimateLocked } from '@/lib/estimate.ts';
```

with

```ts
import { isEstimateLocked, canDeleteEstimate } from '@/lib/estimate.ts';
```

- [ ] **Step 4: Typecheck and lint**

Run: `npx vue-tsc --noEmit && npx eslint src/views/mobile/invoices`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add src/views/mobile/invoices
git commit --no-verify -m "estimates: mobile Revise, Revisions list, superseded read-only view and list delete rules"
```

---

### Task 7: Full verification and deploy

**Files:** none new.

- [ ] **Step 1: Whole test suites**

Run: `go test ./... -count=1 && gofmt -l pkg cmd && npx vitest run --exclude '.claude/**' && npx vue-tsc --noEmit && npx eslint .`
Expected: all PASS / clean, gofmt prints nothing. Fix anything red before continuing; do not skip.

- [ ] **Step 2: Manual pass on a copy of real data**

With a local server on a copy of the production database (never the live one), at desktop width and at 375px phone width:
1. Existing estimates open unchanged; none shows a Revisions section; the Estimates list looks as before.
2. A draft estimate shows **Edit** (no Revise); edit and save works.
3. Send a draft to your own address (status Sent). The toolbar now shows **Revise** instead of Edit. Revise → the editor opens on `EST/…-R1`, dated today, valid until today + 30, same customer, lines, tax, notes and milestones; change a rate and save.
4. The detail now shows R1 (Draft) with a Revisions section: R1 and Original, newest first; selecting Original shows it read-only, status **Superseded**, "Superseded by EST/…-R1", Delete and Send disabled, no Mark / Reopen / Convert.
5. The Estimates list shows only R1; "Show Closed Estimates" also shows the original as Superseded.
6. Open the original's public link in a private window: "This estimate has been revised.", no Accept / Decline, no "View the latest version" yet. Send R1 → reload the old link: "View the latest version" opens R1's page, which can be accepted.
7. Accept R1 from its link, then Revise R1 → R2 draft; R1 keeps "Accepted by the client on <date>" as history and leaves Needs Attention; delete the R2 draft → R1 is Accepted again (back on Needs Attention); Revise again → the new number is `-R3`, not `-R2`.
8. Add a new estimate: its suggested number is the next plain sequence (e.g. `EST/2026-27/02`), not affected by `-R` numbers.
9. An accepted estimate with an invoice made from it shows no Revise.
10. Customer page timeline: "Estimate EST/…/01 revised as EST/…/01-R1".
11. Mobile: Estimates list → an estimate → More → Revise opens the editor on the new draft; back shows the new version with the Revisions list; tapping Original opens it read-only; the list hides Delete on superseded versions.

- [ ] **Step 3: Deploy (ask first)**

The controller must ask the user before pushing or deploying. Only after an explicit yes: bring the `budgets-buckets` branch up to this work and `git push origin budgets-buckets` → wait for the "Publish Docker Image to GHCR" workflow → `POST $DOKPLOY_API/application.deploy {"applicationId": ...}` (credentials in `~/projects/dokploy/.env`, app id in `~/projects/dokploy/ezbookkeeping-app-id`, browser User-Agent required). After deploy, open books.codeation.io and confirm: an existing sent estimate shows Revise, an existing draft shows Edit, and an existing public estimate link still opens.

- [ ] **Step 4: Commit any fixes**

```bash
git add -A
git commit --no-verify -m "estimates: revision verification fixes"
```
