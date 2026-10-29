# CRM part A: products, renewals and customer view: design

Date: 2026-10-29
Status: draft, awaiting review
Builds on: `docs/superpowers/specs/2026-10-26-invoicing-extensions-design.md` (recurring invoices, email send, deployed)

## Context

CodeBooks is growing from a personal finance app into a small-business tool. The CRM work is split in three:

- **A. Products and customer view** (this spec): product catalog, renewals, customer detail page.
- **B. Leads and quotes**: deferred. Evo CRM (self-hosted, `evolution-foundation/evo-crm-community`) will be trialled for 2-3 weeks for leads, pipeline and WhatsApp. If it sticks, B becomes a one-way "deal won → CodeBooks customer" webhook; if not, lightweight leads get built here.
- **C. Follow-ups and labels**: deferred, sits on top of A and B.

Evo cannot cover part A: its product currencies are BRL/USD/EUR only, it has no renewal concept, and no invoices or GST.

## Goal

1. Know which products each customer has bought.
2. Never miss a renewal (domains, hosting, licences, AMCs, retainers), with a per-schedule choice of how hands-on the renewal is.
3. One page per customer showing everything about them.

Products are a mix of one-off services and renewable items.

## Shared constraints

- Every schema change is a new column or table, added by xorm `Sync2`. No data migration; existing rows get defaults that keep today's behaviour.
- Renewals are the existing recurring invoices, extended. There is no separate subscription entity: a recurring invoice copies a whole source invoice, so one invoice = one renewal period.
- Desktop and mobile follow the existing invoice pages; shared logic in `src/views/base/invoices/`.
- Amounts are minor units of the invoice currency, as everywhere else.

## 1. Data model

### Product (new table)

| Field | Notes |
|---|---|
| ProductId, Uid, Deleted, timestamps | as other tables |
| Name | VARCHAR(128), required |
| Description | VARCHAR(512) |
| Sac | VARCHAR(8) |
| DefaultRate | int64, minor units |
| Currency | VARCHAR(3); empty = profile default |
| RenewalMonths | int32; 0 = one-off, else 1..120 |
| DefaultRenewalMode | VARCHAR(16): `create`, `draft_ahead`, `auto_send`; ignored when RenewalMonths = 0 |
| Archived | bool; hidden from the line picker, kept in history |

A product cannot be deleted while any invoice line links to it; archive instead.

### InvoiceLine (new column)

- `ProductId int64 NOT NULL DEFAULT 0`; 0 = free text, as all lines today.
- Picking a product in the editor fills ItemName, Description, Sac and Rate; all stay editable. The link is kept even if the text is edited.

"Products bought" by a customer = their non-deleted invoices' lines with ProductId > 0. No extra table.

### RecurringInvoice (new columns)

| Field | Default | Notes |
|---|---|---|
| PeriodMonths | 1 | 1..120 |
| Mode | `create` | `create` (today's behaviour), `draft_ahead`, `auto_send` |

Existing rows therefore keep running monthly and create-only. Pausing and cancelling use the existing Enabled flag.

### InvoiceProfile (new column)

- `RenewalDraftDays int32 NOT NULL DEFAULT 15`: how many days before the date a `draft_ahead` invoice is created. Edited on the existing profile page.

## 2. Screens

### Customer detail page (new, desktop and mobile)

Opened from the customer list.

- **Header**: name, contact, GSTIN; totals billed, paid, outstanding (in invoice currency, grouped by currency when mixed).
- **Products bought**: per product, times bought, last bought date, and its active recurring schedule if any ("renews 12 Mar 2027, draft ahead").
- **Renewals**: the customer's recurring schedules (next date, period, mode, enabled) with edit and add, reusing the existing recurring popup.
- **Timeline**, newest first, built from existing data: invoice created, invoice sent, payment linked, email sent (from the email history), renewal invoice created. No new table.
- **Notes**: the existing customer Notes field. Timestamped notes belong to part C.

### Product catalog (new list and edit pages, desktop and mobile)

- Placed beside Customers in the invoices area.
- Edit form: fields from section 1.
- **Link past invoice lines**: lists unlinked lines grouped by exact ItemName (trimmed, case-insensitive), with counts. The user picks a product per group (or skips) and confirms; the matching lines get ProductId set. Never automatic.

### Invoice editor

- Item name field suggests non-archived products as you type; picking one fills the line.
- Send dialog, when the invoice has lines with renewable products and no recurring schedule uses it as source yet:
  - all renewable lines share one RenewalMonths: a checked option "Create renewal: every N months, <mode>" (mode = the product's default; if products differ in mode, the most hands-on wins: `draft_ahead` > `create` > `auto_send`). Unchecking skips it.
  - renewable lines with different RenewalMonths: a warning "These products renew on different periods; split them into separate invoices to track renewals". Send still works; no schedule is created.

### Home page

A **Renewals** card, shown when non-empty:

- `draft_ahead` invoices not yet sent whose invoice date is within the draft window, with **Send** (existing send flow, one click with the saved template) and **Open**.
- Unsent invoices from `auto_send` schedules (auto-send failed), labelled "Auto-send failed".
- `auto_send` invoices sent in the last 7 days, labelled "Sent automatically", no action.

### Recurring invoices list and popup

- List gains period and mode.
- Popup gains "Every N months" and a mode picker.

## 3. Scheduler

Extend `CreateRecurringInvoices` (same cron job, no new one). For each enabled, non-deleted schedule:

| Mode | Create when | Invoice date | After creating |
|---|---|---|---|
| `create` | today ≥ NextDate | NextDate | nothing (today's behaviour) |
| `draft_ahead` | today ≥ NextDate − RenewalDraftDays | NextDate | left unsent; appears on the home card |
| `auto_send` | today ≥ NextDate | NextDate | sent to the customer's email with the saved template, outside the DB transaction |

- `nextRecurringDate(date, dayOfMonth, periodMonths)` steps N months; `catchUpRecurringDate` steps by N months too.
- Invoice insert and NextDate advance stay in one transaction, so a crash cannot create the same period twice.
- The created invoice copies the source invoice, as today. Price changes: edit the draft, or edit the source invoice.
- Auto-send failure (SMTP error, customer has no email): the invoice stays, unsent; the home card shows "Auto-send failed". No retry loop.

### Auto-created schedules

On a successful send that meets the single-period rule in section 2 with the option checked, create a RecurringInvoice: source = this invoice, PeriodMonths = the products' RenewalMonths, Mode as chosen, DayOfMonth = min(invoice day, 28), NextDate = invoice date + PeriodMonths on that day. A renewal invoice created by the scheduler does not create another schedule (its source already has one).

## 4. Edge cases

- Product archived: schedules keep running; only the picker hides it.
- Customer deleted: already blocked while invoices exist.
- Source invoice deleted: existing behaviour (schedule skipped with a warning log).
- Paused then resumed: catch-up skips missed periods.
- Schedule created with a past NextDate: runs on the next job.
- Draft created, then user changes Mode to `create` before the date: draft stays an ordinary unsent invoice; the card stops listing it.

## 5. Testing

Go unit tests (`pkg/services`):

- `nextRecurringDate` and catch-up with periods 1, 3, 12; day 28; year rollover.
- Draft window: created at exactly NextDate − N, not a day before.
- Auto-send success marks sent; failure leaves an unsent invoice and still advances NextDate.
- Renewals-due query: draft in window, failed auto-send, recent auto-send.
- Auto-schedule on send: single period creates one; mixed periods creates none; already-scheduled source creates none.
- Past-line linking by ItemName.
- Product delete refused while linked.

Vitest: product suggestion filtering in the line editor.

Manual pass on desktop and mobile before deploy.

## Out of scope

Leads, pipeline, quotes (part B); follow-up reminders, labels, timestamped notes (part C); Evo CRM integration; per-line renewal periods within one invoice; stock or inventory.
