# Estimates with milestones: design

Date: 2026-10-30
Status: draft, awaiting review
Builds on: `docs/superpowers/specs/2026-10-29-crm-part-a-products-renewals-design.md` (products, renewals, customer page, Invoices / Customers / Products navigation)

## Goal

Send clients estimates (quotes) that use the same editor, products, PDF, public link and email as invoices; let the client accept or decline online; turn an accepted estimate into invoices, either all at once or milestone by milestone, where the milestones are the estimate's payment terms.

## Approach

An estimate is a row in the existing `invoice` table with `Kind = "estimate"` (the column exists; every saved row is `"invoice"`). It reuses lines, products, tax, totals, PDF, public link, email sending, email history and the customer timeline. Every invoice-only feature (payments and suggestions, outstanding / overdue totals, recurring schedules, renewals, Create renewal, invoice numbering) gets an explicit `kind = invoice` filter. A separate table was rejected: it would duplicate most of the invoicing code.

## Shared constraints

- Schema changes are new columns / tables via xorm `Sync2`; no data migration (existing rows are already `kind = invoice`).
- Amounts are minor units of the document currency.
- Desktop and mobile get every screen; shared logic in `src/views/base/invoices/`; new strings in `src/locales/en.json` only.
- Existing invoices, schedules and their behaviour do not change.

## 1. Data model

### Invoice (new columns)

| Field | Applies to | Notes |
|---|---|---|
| Kind | both | `invoice` (existing rows) or `estimate` |
| EstimateStatus | estimate | `draft`, `sent`, `accepted`, `declined`; empty on invoices |
| RespondedUnixTime | estimate | when accepted / declined; 0 otherwise |
| RespondedByClient | estimate | true when answered through the public link |
| ResponseNote | estimate | client's optional decline note, plain text, max 500 |
| EstimateId | invoice | the estimate this invoice was created from; 0 otherwise |

- An estimate's `DueDate` is its **valid until** date (no new date column).
- **Expired** is never stored: an estimate in `draft` or `sent` whose valid-until date is before today is displayed as Expired. Extending valid-until revives it.
- Sending an estimate by email moves `draft` → `sent` (and records the send like invoices do).

### EstimateMilestone (new table)

| Field | Notes |
|---|---|
| MilestoneId, Uid | |
| EstimateId | indexed |
| DisplayOrder | 1..10 |
| Label | VARCHAR(128), required, e.g. "Advance on acceptance" |
| Percent | basis points, > 0 |
| InvoiceId | 0 until invoiced |

- An estimate has 0 or 1..10 milestones; when present, percents sum to exactly 10000 (100%).
- Percentages only (no fixed amounts), so milestones stay correct when lines change before sending.

### InvoiceProfile (new columns)

- `EstimateNumberPrefix` VARCHAR(16), default `EST`.
- `EstimateValidDays` int32, default 30, range 1..365.

### Numbering

Estimates: `{EstimateNumberPrefix}/{financial year}/{sequence}` (e.g. `EST/2026-27/01`), a separate sequence. Invoice numbering counts only `kind = invoice`.

## 2. Converting to invoices

- **No milestones:** **Convert to invoice** creates one invoice with every line copied (products, SAC, quantities, rates), the estimate's tax settings and currency, dated today, due by the customer's / profile's terms, `EstimateId` set. It opens in the editor to adjust before saving.
- **With milestones:** each milestone has **Create invoice**, in any order. The invoice has one line: `"<label>: <pct>% of Estimate <number>"`, SAC from the estimate's first line, quantity 1, rate = the milestone's share of the estimate **subtotal** (pre-tax), rounded half up; the estimate's tax settings apply on top. The **last open** milestone's rate is `subtotal − sum of the other milestones' invoiced subtotals`, so the parts always add up exactly. The milestone's `InvoiceId` is set.
- Deleting an invoice made from an estimate frees its milestone (or, without milestones, makes the estimate convertible again).
- An estimate with no milestones can be converted once (while that invoice exists).
- Converting does not require Accepted, but the UI only offers it on Accepted estimates and on Sent / Expired ones after a confirm ("This estimate isn't accepted yet. Convert anyway?").

## 3. Locking and lifecycle

- Editable while `draft` or `sent` and nothing has been invoiced from it.
- Read-only once `accepted`, `declined`, or any invoice exists from it. **Duplicate** always works and makes a new draft.
- **Mark accepted / Mark declined** (by you) record the response (`RespondedByClient = false`); allowed from `draft`, `sent` or expired.
- **Reopen** returns accepted / declined to `sent` and clears the response; only while nothing has been invoiced.
- An estimate cannot be deleted while invoices created from it exist.

## 4. Public link: accept and decline

- The public page (`/i/<token>`) shows estimates titled "ESTIMATE", with the Payment Terms table.
- While `sent` and not expired: **Accept** and **Decline** buttons (Decline asks for an optional note).
- After a response: "Accepted on <date>" / "Declined on <date>", no buttons. Expired: "This estimate expired on <date>", no buttons.
- New unauthenticated routes `POST /api/public/estimates/accept.json` and `/decline.json` with `{ token, note? }`:
  - the public token is the only key (existing 43-char random token, can be disabled or reset);
  - only acts when the estimate is `sent`, not expired and unanswered; otherwise "This estimate can no longer be answered", no change;
  - the update is conditional on `estimate_status = 'sent'`, so concurrent clicks record one response;
  - the note is limited to 500 characters and stored and shown as plain text;
  - the token never reveals or changes anything but its own estimate; responses carry no ids.

## 5. Notifications

- Client accept / decline emails the user's account address: "<customer> accepted Estimate <number> (<total>)" (or declined, with the note) and a link to the estimate. SMTP off or a send failure is logged; the response still counts.
- Marking accepted / declined yourself sends nothing.

## 6. Screens

### Desktop (Invoices section)

- Left nav: Invoices, Recurring, **Estimates**.
- Estimates list: number, customer, date, valid until, total, status badge (Draft / Sent / Accepted / Declined / Expired / Invoiced). "Show closed" reveals declined and fully invoiced ones.
- Add / edit: the invoice editor in estimate mode — title "Add Estimate" / "Edit Estimate", number from the estimate series, "Valid Until" instead of "Payment Due" (default today + EstimateValidDays), and a **Milestones** block under the lines (label, %, computed amount; add / remove; running total "100% ✓" or a warning; Save blocked unless 100% or none).
- Detail: the invoice detail view, document titled "ESTIMATE" with a Payment Terms table when there are milestones. Side panel: **Status** (who responded and when, the client's note; Mark accepted / Mark declined / Reopen), **Milestones** (each "Create invoice" or "Invoiced → <number>"), or **Convert to invoice** without milestones. Send, Share link, PDF, Print, Duplicate, Edit (when unlocked) and Delete work as for invoices.
- Invoice detail shows "From estimate <number>" (link) when `EstimateId` is set.

### Mobile

- Invoice list More menu: **Estimates** (list page like the invoice list).
- Add / edit: the mobile invoice editor in estimate mode with the milestones block.
- Detail: the mobile invoice detail with the same status, milestones and convert actions.

### Elsewhere

- Home: the Renewals card becomes **Needs attention**, listing renewals plus "Estimate accepted by <customer>: Convert" rows until fully invoiced.
- Customer page timeline: estimate created, sent, accepted / declined, converted. Estimates never count in Billed / Paid / Outstanding.
- Invoice profile: "Estimate Number Prefix", "Estimate Valid For (days)".
- PDF, public page and email: title "ESTIMATE", "Valid until" instead of "Due", Payment Terms table (milestone, %, amount computed from the estimate total incl. tax), no "Balance due / Received".

## 7. Testing

Go: estimate numbering is separate from invoice numbering; status transitions (email → sent, mark accepted / declined, client accept / decline, reopen); public accept / decline guards (wrong status, expired, concurrent clicks, disabled token, note length); milestone validation (sum 100%, 1..10, labels); milestone invoice amounts and last-milestone remainder; full conversion copies lines; deleting an invoice frees its milestone; locking; estimates excluded from outstanding, payments, suggestions, recurring, renewals and Create renewal; accept notification email (captured mailer).
Vitest: milestone amount / remainder calculation; estimate display status (incl. Expired).
Manual: desktop and mobile flows; public link in a private window.

## Out of scope

Fixed-amount milestones; milestone due dates; estimate versions / revisions history; e-signatures; converting one estimate into invoices for several customers; leads and pipeline (part B).
