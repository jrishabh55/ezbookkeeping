# Invoicing extensions: design

Date: 2026-10-26
Status: approved in chat, awaiting spec review
Builds on: `docs/superpowers/specs/2026-10-25-invoicing-design.md` (deployed 2026-10-26, real invoices exist on books.codeation.io)

## Goal

Extend invoicing, in this order, without reworking the deployed schema or core logic:

1. Decimal quantities (hourly billing such as 7.5 h)
2. GST (Indian clients; one rate per invoice, IGST or CGST+SGST)
3. Multi-currency invoices whose payments land in INR accounts
4. Linked-invoice badge on transactions
5. Recurring (monthly) invoices created by cron
6. Emailing the invoice as a PDF attachment

Each phase is shippable on its own. Later phases build on the earlier ones (emailing renders the GST and currency-aware document).

## Shared constraints

- Every schema change is a new column or table, added by xorm `Sync2`. The only data migration is the quantity rescale in phase 1, run once by hand on the live database.
- Totals are computed in one place per side: `computeInvoiceTotal` in `pkg/services/invoices.go` (stored on save) and `computeInvoiceTotals` in `src/lib/invoice.ts` (live preview). Both grow the same tax rules.
- Status (unpaid / partial / paid, 3% tolerance, manual close, overdue) is unchanged and stays in invoice currency.
- Desktop and mobile follow the existing invoice pages; shared logic stays in `src/views/base/invoices/`.
- Amounts are minor units of the invoice currency (two decimals, as everywhere in ezBookkeeping).

---

## Phase 1: Decimal quantities

**Storage.** `InvoiceLine.Quantity int64` keeps its column and now holds hundredths: `1` h is stored as `100`, `7.5` h as `750`. Request validation becomes `min=1,max=1000000` (0.01 to 10,000).

**Math.** `Amount = (Quantity × Rate + 50) / 100` in Go (round half up to the paisa); `Math.round(quantity * rate / 100)` in TS. Line amount stays stored, so printed invoices never drift.

**UI.** Desktop: the existing `NumberInput` with `max-decimal-count="2"`, `min-value="0.01"`, bound through a display value (hundredths ↔ decimal) in `InvoiceEditPageBase`. Mobile: `ListNumberInput` with the same conversion. Default quantity stays 1 (stored `100`). Client validation: quantity ≥ 0.01.

**Document.** Quantity prints with trailing zeros trimmed (`1`, `7.5`, `2.25`); the Quantity column is still hidden when every line is `1`.

**Migration (deploy step, once).** Before the new image serves traffic:

```sql
UPDATE invoice_line SET quantity = quantity * 100;
```

The plan records this as a manual step in the deploy task, with a check that no row exceeds 10,000 afterwards.

**Tests.** Go: amount rounding (`750 × 33333 → 249998` paise, i.e. 7.5 × ₹333.33 = ₹2,499.98). Vitest: totals with fractional quantities; quantity formatting.

---

## Phase 2: GST

**Model additions.**

| Table | New fields |
|---|---|
| `InvoiceProfile` | `Gstin VARCHAR(15)`, `DefaultSac VARCHAR(8)` |
| `Customer` | `Gstin VARCHAR(15)`, `TaxType VARCHAR(16)` (`none` / `igst` / `cgst_sgst`), `TaxRate int32` (basis points, 0–10000) |
| `Invoice` | `BillToGstin VARCHAR(15)` (snapshot), `TaxType VARCHAR(16)`, `TaxRate int32` (bp), `Subtotal int64`, `TaxAmount int64` |
| `InvoiceLine` | `Sac VARCHAR(8)` |

Existing rows read back as `TaxType` empty, treated as `none`; `Subtotal` is backfilled lazily on the next save (the document uses `Total` when `Subtotal` is 0 and `TaxType` is `none`, so old invoices print unchanged).

**Totals (both sides).** `subtotal = Σ line.amount`; `taxAmount = round(subtotal × taxRate / 10000)` when `taxType ≠ none`, else 0; `cgst = taxAmount / 2` (integer), `sgst = taxAmount − cgst`; `total = subtotal + taxAmount`. Validation: `taxType` in the three values; `taxRate` 0–10000; `taxRate` must be > 0 when `taxType ≠ none`.

**Defaults.** New invoice: `TaxType`, `TaxRate` from the chosen customer; `Sac` on a new line from the profile's `DefaultSac`. Changing the customer in the edit form re-applies the customer defaults only when the invoice is new (never silently on edit).

**UI.** Invoice edit: a tax row below the lines with a type select (None / IGST / CGST + SGST) and a rate input in percent (two decimals, stored as bp); live totals show Subtotal, tax rows, Total. Customer edit: GSTIN, tax type, rate. Profile: GSTIN, default SAC. Line row: SAC field (desktop inline, mobile list item).

**Document.** GSTIN line under the business block and under BILL TO when present. A SAC column appears when any line has one. Totals block: Subtotal, then `IGST (18%)` or `CGST (9%)` + `SGST (9%)`, then Total, Payments received, Amount Due. Invoices with `TaxType = none` print exactly as today.

**Tests.** Go and vitest: 18% on ₹1,00,000.01 rounds once (`tax = 1800000`, cgst `900000`, sgst `900000`); an odd tax paisa goes to SGST; `none` yields zero tax and `total = subtotal`.

---

## Phase 3: Multi-currency

**Model additions.** `InvoiceProfile.DefaultCurrency VARCHAR(3)` (empty means INR). `InvoicePayment.SettledAmount int64` (0 = same currency, read the transaction; otherwise the amount in invoice currency the user entered when linking).

**Invoice currency.** `Invoice.Currency` becomes a request field on create (`len=3`, must be a known currency code); it is not editable after creation (lines and payments are in it). Default = profile `DefaultCurrency`. Desktop uses `CurrencySelect`; mobile the existing currency selection sheet, as in account edit.

**Received.** `received = Σ (payment.SettledAmount if > 0 else transaction amount)` over linked, non-deleted transactions. Status, tolerance and bank-charge gap are unchanged.

**Linking.** `POST invoices/payment/link.json` gains optional `settledAmount`. Rules:
- Account currency = invoice currency: `settledAmount` must be absent or 0; amount read from the transaction (current behaviour).
- Otherwise: `settledAmount` required (> 0) and ≤ balance due. The transaction amount is the realised amount in the account's currency.
- Everything else (ownership, income type, not already linked) unchanged.

**Suggestions.** Same-currency candidates ranked as today, followed by income transactions in other currencies within the same date window, newest first, capped at the same 10 in total.

**UI.** Link button on a cross-currency suggestion opens an amount input for the settled amount (desktop: `AmountInputDialog`; mobile: `NumberPadSheet`). Payment rows show the realised amount with its currency beside the settled amount when they differ. List sidebar and mobile summary show Outstanding and Overdue per currency (one line each), formatted with the existing currency helpers. The document formats amounts with `Intl.NumberFormat('en-US', { currency })` and labels "Amount Due (USD)".

**Tests.** Go: received with settled amounts; link refuses a cross-currency link without `settledAmount`, and a same-currency link with one; over-balance refused. Vitest: per-currency summaries.

---

## Phase 4: Invoice badge on transactions

**API.** `TransactionInfoResponse` gains `invoiceId string` and `invoiceNumber string` (both `omitempty`). `InvoiceService.GetLinkedInvoicesByTransactionIds(uid, ids) map[int64]*Invoice` (one query joining `invoice_payment` and `invoice`, non-deleted). The transactions API fills the fields in the list, month list and get handlers after building the responses.

**UI.** Desktop transaction list row and detail dialog: a small chip with the invoice number that opens the invoice detail (`/invoice/list` then `InvoiceDetailDialog` by id, via a route query `invoiceId`). Mobile transaction detail: a list item "Invoice" linking to `/invoice/detail?id=`. The invoice side already lists its payments; nothing changes there.

**Tests.** Go API test: a linked transaction returns its number; another user's transaction does not.

---

## Phase 5: Recurring invoices

**Model.** New table `RecurringInvoice`: `RecurringId` (PK, uuid), `Uid`, `Deleted`, `SourceInvoiceId`, `DayOfMonth int32` (1–28), `NextDate VARCHAR(10)`, `Enabled bool`, `LastInvoiceId int64`, `LastRunUnixTime`, created/updated/deleted times. Index on `(Deleted, Enabled, NextDate)`.

**Creation.** `InvoiceService.CreateRecurringInvoices(c, today string)`: for each user DB, select enabled, non-deleted rows with `NextDate <= today`; for each, load the source invoice (skip and log if deleted), the current customer (for a fresh bill-to snapshot; fall back to the source snapshot if the customer was deleted), the profile (terms days), then call `CreateInvoice` with: number = `NextInvoiceNumber(nextDate)`, invoice date = `NextDate`, due date = invoice date + terms, lines, notes, currency, tax copied from the source. On success set `LastInvoiceId`, `LastRunUnixTime`, and `NextDate` = same day next month (day capped at 28 by validation, so no clamping). On error log and continue; the row is retried next day.

**Cron.** `CreateRecurringInvoicesJob` in `pkg/cron/cron_jobs.go`, `CronJobFixedHourPeriod{Hour: 1}`, passing the server's local date.

**API.** `recurring_invoices/list.json`, `add.json` (`sourceInvoiceId`, `dayOfMonth`, `nextDate`), `modify.json` (`dayOfMonth`, `nextDate`, `enabled`), `delete.json`.

**UI.** Invoice detail toolbar/ellipsis: "Repeat Monthly" opens a small dialog/sheet with day of month (default: the source invoice's day, capped at 28) and first date (default: next occurrence). Invoice list sidebar (desktop) and ellipsis (mobile) gain "Recurring": rows with customer, source number, next date, enabled switch, delete. Generated invoices are ordinary unpaid invoices.

**Tests.** Go: next date advances across the year boundary; a disabled row is skipped; a row whose source invoice is deleted is skipped without error; the generated invoice copies lines and tax and gets the next number.

---

## Phase 6: Emailing

**PDF renderer.** New package `pkg/invoicepdf` using `github.com/go-pdf/fpdf` with `DejaVuSans.ttf` and `DejaVuSans-Bold.ttf` embedded via `go:embed` (needed for ₹ and non-Latin text; core fonts are WinAnsi only). `Render(profile, invoice, lines) ([]byte, error)` mirrors `InvoiceDocumentContent.vue`: logo (decoded from the data URL, PNG or JPEG) top-left, business block top-right, "Invoice" banner, BILL TO and meta table, lines table (Services, optional SAC, optional Quantity, Rate, Amount), totals block (Subtotal / tax rows / Total / Payments received / Amount Due), Notes / Terms. Amount formatting matches the document's en-US style per currency.

**Mail.** `mail.MailMessage` gains `Attachments []MailAttachment{Name string, Content []byte}`; `DefaultMailer.SendMail` attaches each with `gopkg.in/mail.v2`'s `AttachReader`.

**API.**
- `GET invoices/pdf?id=` returns the PDF (`Content-Disposition: attachment; filename=Invoice_<number>_<dueDate>.pdf`).
- `POST invoices/send.json {id, to, subject, message}`: `to` required (valid email, max 128); `subject` max 256; `message` max 2048. Refused with the existing SMTP-not-enabled error when mail is off. Body is `message` as plain paragraphs in a minimal HTML wrapper. On success sets `Invoice.SentUnixTime` (new column) and returns the invoice.

**UI.** Invoice detail gains "Send" (desktop dialog, mobile page) with To (prefilled from bill-to email), Subject (`Invoice <number> from <business name>`), Message (default: amount due, due date, thank-you line), Send button; "Sent on <date>" under the status once sent. "Download PDF" sits next to Print and calls the server PDF. The print path is unchanged.

**Deploy prerequisite.** SMTP settings in the Dokploy environment: `EBK_MAIL_ENABLE_SMTP=true`, `EBK_MAIL_SMTP_HOST`, `EBK_MAIL_SMTP_USER`, `EBK_MAIL_SMTP_PASSWD`, `EBK_MAIL_FROM_ADDRESS` (section `mail` of `conf/ezbookkeeping.ini`, read via `getConfigItemValueEnvironmentKey`). Without them the Send button reports that SMTP is disabled.

**Tests.** Go: `Render` succeeds for an invoice with logo, GST and a non-INR currency and the output starts with `%PDF`; `send` with SMTP disabled returns the SMTP error; cross-user send is refused.

---

## Error handling

- All new validation errors are API-boundary errors with locale strings, like the existing ones.
- Recurring creation never fails the cron run on one bad row; it logs and continues.
- A send failure leaves `SentUnixTime` untouched.

## Out of scope

Automatic emailing of generated recurring invoices, quotes and credit notes, per-line tax rates, editing an invoice's currency after creation, FX gain/loss accounting, PDF layouts other than the Wave one.
