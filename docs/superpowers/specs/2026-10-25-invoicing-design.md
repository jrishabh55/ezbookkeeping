# Invoicing: design

Date: 2026-10-25
Status: draft (awaiting review)

## Goal

Replace Wave for creating customer invoices, and track each invoice until it is paid, inside ezBookkeeping.

Success means:

- The user creates an invoice in the app and saves a PDF that matches the current Wave layout
  (sample: `Invoice_CA_2026-27_01_2026-04-30.pdf`, invoice `CA/2026-27/01`, ₹3,70,000 to Quber Tech).
- Each invoice shows how much is received and outstanding, using income transactions already in the book
  (from SMS capture or statement import). Invoicing never creates transactions itself.
- The invoice list answers "who owes me what, and what is overdue" at a glance.
- GST, multi-currency, emailing, recurring invoices and quotes can be added later without reworking
  the schema or the core logic.

## Constraints and decisions

- **INR only, no GST** for now (services export, billed in INR).
- **Payments:** an invoice is usually settled by one income transaction, sometimes by several.
  Foreign remittances can arrive slightly short (bank/forex charges).
- **Matching is suggested, never automatic.** The user always clicks to link.
- **PDF = browser print** of a print-styled page (`@media print`, "Save as PDF"). No PDF library.
- **Desktop UI only** in this phase. Invoices get no mobile pages; mobile navigation is unchanged.
- ezBookkeeping is multi-user: every table and query is scoped by `uid`.
- Amounts are `int64` minor units (paise), like `Transaction.Amount`.
- Schema changes go through xorm `Sync2` (`UserDataStore.SyncStructs` in `cmd/database.go`), which adds
  tables and columns. All future features below are additive under it.

## Data model

All tables carry `Uid`, `Deleted`, `CreatedUnixTime`, `UpdatedUnixTime`, `DeletedUnixTime`, following `models.Bucket`.

### `InvoiceProfile` (one row per user)

| Field | Notes |
|---|---|
| `BusinessName` | "Rishabh Jain" |
| `AddressLines` | multi-line text |
| `Phone`, `Website` | optional |
| `LogoDataUrl` | optional; a `data:image/png;base64,…` or `data:image/jpeg;base64,…` string (≤ 350,000 chars, about 250KB image) stored in the row and returned with the profile. No storage plumbing or authenticated image fetch needed for one small logo |
| `DefaultNotes` | multi-line; prefilled into each new invoice (bank details block) |
| `NumberPrefix` | "CA" |
| `DefaultTermsDays` | default 7 |

### `Customer`

`Name`, `ContactName`, `AddressLines`, `Email`. Reused across invoices.

### `Invoice`

| Field | Notes |
|---|---|
| `InvoiceId` | uuid-generated int64, like other models |
| `Kind` | `invoice` (only value now; reserved for quote / credit note) |
| `Number` | required, unique per `(Uid, Number)` among non-deleted invoices |
| `CustomerId` | |
| `BillToSnapshot` | customer name/contact/address/email copied at save time; editing a customer never changes an old invoice |
| `Currency` | `INR` (only value now) |
| `InvoiceDate`, `DueDate` | `YYYY-MM-DD` strings; `DueDate >= InvoiceDate` |
| `Notes` | multi-line |
| `ClosedManually` | bool; the "Mark as paid" override |

### `InvoiceLine`

`InvoiceId`, `DisplayOrder`, `ItemName` ("Consulting"), `Description` (multi-line), `Quantity` (default 1, > 0),
`Rate` (> 0), `Amount` (= quantity × rate, stored so totals never drift from what was printed).

### `InvoicePayment`

`InvoiceId`, `TransactionId` (**unique**: a transaction pays at most one invoice), `LinkedUnixTime`.
The paid amount is **not** stored. It is read from the linked transaction, so editing the transaction keeps the invoice correct,
and a deleted transaction drops out of "received".

## Core logic (single source of truth)

Totals are computed in Go when an invoice is saved (`Invoice.Total` and `InvoiceLine.Amount` are stored). Status is computed only in TS (`src/lib/invoice.ts`), because only the UI needs it. Both are unit-tested.

- `computeInvoiceTotals(lines) → { subtotal, total }`. Every total (API, print page, status) goes through this.
  GST later adds `taxes[]` here.
- **Status** is derived on read, never stored:
  - `received` = sum of amounts of linked, non-deleted transactions; `balanceDue = total − received`.
  - **Paid** if `ClosedManually`, or `balanceDue <= total × 3%` (constant `InvoicePaidTolerance = 0.03`).
    When paid with `balanceDue > 0`, the gap is shown as **Bank charges**.
  - **Partially paid** if `received > 0` and not paid.
  - **Unpaid** if `received == 0` and not paid.
  - **Overdue** is a flag on unpaid/partially-paid invoices whose `DueDate` is before today.
- **Numbering:** `next_number` suggests `{NumberPrefix}/{FY}/{seq:02}`, where FY runs April–March (`2026-27`)
  and `seq` = highest existing sequence for that prefix+FY, plus 1. The user can type any number; uniqueness is the only rule.
  Past Wave invoices are entered with their real numbers so the sequence continues.
- **Creation** always goes through `InvoiceService.CreateInvoice(invoice, lines)`. Future recurring invoices call the same method from a cron job.

## Backend

Files, following the buckets/budgets pattern:

- `pkg/models/invoice.go`: models, request/response structs, validation tags
- `pkg/services/invoices.go`: CRUD, totals, numbering, received amounts, suggestions, link/unlink
- `pkg/api/invoices.go`: handlers
- `pkg/errs/invoice.go`: error codes
- `cmd/webserver.go`: routes; `cmd/database.go`: `SyncStructs` for the five tables
- `pkg/uuid/uuid_type.go`: `UUID_TYPE_INVOICE = 15`. Only 4 bits exist for UUID types and 15 is the last free value, so customers, invoices, lines and payments all share it (sequence numbers are per type, so IDs stay unique).

Endpoints (`/api/v1/...`):

| Endpoint | Purpose |
|---|---|
| `GET invoices/list.json` | all invoices with total and received (filtering by status/customer happens in the page; one user's invoices are a small list) |
| `GET invoices/get.json?id=` | invoice, lines, payments (with transaction summary) |
| `POST invoices/add.json`, `modify.json`, `delete.json` | CRUD (delete is soft, and removes its payment links) |
| `GET invoices/next_number.json?invoice_date=` | suggested number |
| `GET invoices/payment/suggestions.json?id=` | candidate transactions |
| `POST invoices/payment/link.json`, `unlink.json` | `{ invoiceId, transactionId }` |
| `POST invoices/close.json`, `reopen.json` | set/clear `ClosedManually` |
| `customers/list.json`, `add.json`, `modify.json`, `delete.json` | customers (delete refused if used by a non-deleted invoice) |
| `GET invoice_profile/get.json`, `POST invoice_profile/update.json` | profile |

### Validation (at the API boundary)

- Number required and unique per user. At least one line. Quantity and rate > 0. `DueDate >= InvoiceDate`. Customer belongs to the user.
- Link: the transaction belongs to the user, is an income, is not deleted, is in an INR account, and is not already linked.
  Amount > current balance due is refused with "payment exceeds the invoice balance due" (an overpayment is almost always a wrong pick).
- Concurrent links of the same transaction are stopped by the unique constraint and reported as "already linked".

### Payment suggestions

Candidates: income transactions in INR accounts, not linked, not deleted, dated from `InvoiceDate` to `DueDate + 60 days`.
Ranked: exact `balanceDue` first, then amounts in `[97%, 100%)` of balance due, then smaller amounts (possible part-payments).
Top 10.

## Frontend (desktop)

### Navigation

- `src/components/desktop/MainPageLayout.vue` top header gets three icon buttons after Accounts, with tooltip and active state like the existing ones:
  **Budgets** (`mdiWalletOutline`, `/budget/list`), **Buckets** (`mdiPiggyBankOutline`, `/bucket/list`), **Invoices** (`mdiFileDocumentOutline`, `/invoice/list`).
- Budgets and Buckets are removed from the desktop Settings sidebar ("Planning" section in `SettingsPageLayout.vue`). Their routes move out from under `SettingsPageLayout` into a new parent route using `src/views/desktop/common/NoNavbarPageLayout.vue` (`<main-page-layout no-navbar>` + `<router-view/>`), alongside the invoice routes.
- Invoice profile lives in Settings (new "Invoice Profile" item); the Invoices page toolbar also links to it.
- Mobile: no navigation change.

### Pages and components

- `src/views/desktop/invoices/ListPage.vue`: table (number, customer, invoice date, due date, total, received, balance due, status chip);
  filter tabs All / Outstanding / Paid; customer filter; header strip with **Outstanding total** and **Overdue total**.
- `src/views/desktop/invoices/dialogs/InvoiceEditDialog.vue`: customer picker with "+ Add customer"; number (prefilled from `next_number`);
  invoice date (today); due date (invoice date + default terms); line grid (item, multi-line description, qty, rate, computed amount; add/remove);
  notes (prefilled from profile); live totals. **Duplicate** action on existing invoices.
- `src/views/desktop/invoices/dialogs/CustomerEditDialog.vue`
- `src/views/desktop/invoices/InvoiceViewPage.vue` (`/invoice/view?id=`): toolbar (Edit, Print / Save PDF, Duplicate, Mark as paid / Reopen, Delete),
  the document, and a **Payments panel** (linked transactions with unlink; received / balance due / bank charges; suggested matches with one-click link;
  "Search transactions…" to pick any income transaction).
- `src/components/desktop/InvoiceDocument.vue`: the print layout, a pure component taking a plain invoice + profile object.
  Matches the sample: logo and business block top-right; double-rule "Invoice" banner; bill-to left; number/date/due/amount-due box right;
  Services / Rate / Amount table with dotted rules (qty column shown only if any line has qty ≠ 1); totals; Notes / Terms.
  Uses a font with the ₹ glyph.
- `@media print` hides everything except the document. `document.title` is set to `Invoice_{number with / → _}_{dueDate}` while printing
  so the saved file is named like `Invoice_CA_2026-27_01_2026-04-30.pdf`.
- `src/views/desktop/settings/tabs/InvoiceProfileSettingTab.vue` (or page, following existing settings structure): profile fields and logo upload.
- `src/models/invoice.ts`, `src/stores/invoice.ts`, `src/lib/invoice.ts`, `src/lib/services.ts` additions, `src/router/desktop.ts` routes,
  `src/locales/en.json` strings.

## Error handling

- Deleting a linked transaction: it drops out of "received"; the invoice can become partially paid again, which the list shows.
- Editing a paid invoice is allowed; status is recomputed.
- Deleting an invoice removes its payment links; transactions are never modified.
- Duplicate number: clear error naming the existing invoice.

## Future extensions (not built now)

| Feature | Plugs in by |
|---|---|
| GST | optional `TaxName`/`TaxRate` per line; GSTIN/SAC on profile and customer; `taxes[]` in `computeInvoiceTotals` |
| Multi-currency | UI for `Invoice.Currency`; FX difference shown at settlement |
| Emailing | server-side renderer mirroring `InvoiceDocument`; existing `pkg/mail` mailer; a "sent" timestamp |
| Recurring | `RecurringInvoice` template table + cron job (`pkg/cron`) calling `CreateInvoice` |
| Quotes / credit notes | new `Invoice.Kind` values |
| Mobile pages | reuse store, models and `src/lib/invoice.ts` |
| Linked-invoice badge on transactions | join via `InvoicePayment.TransactionId` |

## Testing

- Go: tests for totals; numbering across the April FY rollover and gaps; received amounts with partial payments and deleted transactions; suggestion ranking;
  link validation (ownership, type, currency, already linked, overpayment). API tests for cross-user access.
- Vitest: `src/lib/invoice.ts` totals, status at the 3% boundary, partial payments, manual close, overdue, amount/date/file-name formatting.
- Manual: recreate `CA/2026-27/01` and compare the saved PDF against the Wave sample side by side;
  link a short payment (e.g. ₹3,68,500) and confirm Paid with ₹1,500 bank charges.

## Out of scope

GST, other currencies, emailing, recurring invoices, quotes, mobile invoice pages, PDF libraries, automatic linking.
