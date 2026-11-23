# Invoice Paid Stamp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** PAID stamp (with paid date) on the invoice document and PDF, plus a prominent status banner on the public invoice page.

**Architecture:** The server computes `PaidDate` (latest linked payment's local date, empty unless paid) as a transient field on `models.Invoice`, sets it wherever payments are loaded for rendering, and exposes it as `paidDate` in owner and public responses. Vue and the fpdf renderer each draw the stamp from it.

**Tech Stack:** Go (xorm, go-pdf/fpdf, testify), Vue 3 + Vuetify, vitest.

**Spec:** docs/superpowers/specs/2026-11-23-invoice-paid-stamp-design.md

## Global Constraints
- Paid = `ClosedManually` or balance due ≤ 3% of total (`models.InvoicePaidTolerance`); estimates never stamped.
- Paid date format on wire: `YYYY-MM-DD`; empty when unpaid or when closed manually with no payments.
- Public response adds only `paidDate`; no payment ids, accounts or amounts.

## Review Focus
- Closed manually with no payments: stamp shows "PAID" with no date (no "Invalid Date").
- Payment in IST late evening: date uses the transaction's UTC offset, not UTC.
- Overpaid invoice (received > total): paid, banner shows Paid.
- Estimate with payments linked via milestones: no stamp.
- Print view and PDF: stamp must not overlap the totals text.

---

### Task 1: Paid date on the server

**Files:** Modify `pkg/models/invoice.go`, `pkg/services/invoices.go` (GetInvoiceByPublicToken), `pkg/api/invoices.go` (PDF handler), `pkg/services/invoice_email.go`. Test `pkg/models/invoice_paid_date_test.go`.

**Produces:** `models.InvoiceIsPaid(invoice *Invoice, received int64) bool`, `models.InvoicePaidDate(invoice *Invoice, payments []*InvoicePaymentInfo) string`, `Invoice.PaidDate string xorm:"-"`, `paidDate` json on `InvoiceInfoResponse` and `PublicInvoiceResponse`.

- [ ] Test: unpaid → ""; fully paid by two payments → latest payment's date in its offset (e.g. 2026-11-02T20:00Z with +330 → "2026-11-03"); closed manually, no payments → ""; estimate → "".
- [ ] Implement; set `invoice.PaidDate` at the three render paths; responses copy it.
- [ ] `go test ./pkg/models/ ./pkg/services/ ./pkg/api/` passes.

### Task 2: PAID stamp in the PDF

**Files:** Modify `pkg/invoicepdf/render.go`; test in `pkg/invoicepdf/invoicepdf_test.go`.

- [ ] Test `TestRenderPaidStamp`: paid invoice with `PaidDate "2026-11-02"` → pdftotext contains "PAID" and "02 NOV 2026"; unpaid → no "PAID".
- [ ] `drawPaidStamp` left of the totals block, rotated ~12°, green rounded border, drawn when `balanceDue == 0` for an invoice (not estimate).
- [ ] `go test ./pkg/invoicepdf/` passes.

### Task 3: Vue stamp and public banner

**Files:** Modify `src/models/invoice.ts` (paidDate), `src/lib/invoice.ts` (`getInvoiceDaysOverdue(dueDate, today): number`), `src/components/common/InvoiceDocumentContent.vue` (stamp), `src/views/desktop/invoices/PublicInvoicePage.vue` (banner), `src/locales/en.json`. Test `src/lib/__tests__/invoice.test.ts`.

- [ ] Test `getInvoiceDaysOverdue('2026-11-20', '2026-11-23') === 3`, across month end.
- [ ] Stamp shown when `getInvoiceStatus === 'paid'` and kind is invoice; banner colours: paid green, partial amber, overdue red, unpaid neutral.
- [ ] vitest for the touched test file, `npm run type-check`, eslint on touched files pass.
