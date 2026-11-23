# Invoice paid stamp and public status banner

## Why
Clients open the public link (`/i/<token>`) and can't tell at a glance whether an invoice is paid; the toolbar shows a small "Paid" / "Amount Due" text. A paid invoice should double as the client's payment receipt, so the document itself (view, print, PDF, email attachment) must carry a PAID stamp.

## Goals
- Prominent status banner on the public page: Paid (green, paid date), Partly paid (amber, received + due), Overdue (red, days late), Due (neutral, due date). Estimates keep their current text.
- PAID stamp with the paid date on the invoice document near the totals: Vue `InvoiceDocumentContent` (public page, in-app preview, print) and the server PDF (`pkg/invoicepdf`, used by download and email attachments).

## Non-goals
- Client uploads of payment proof (decided: the stamped invoice is the receipt).
- Separate receipt document or receipt numbering.
- Stamps for estimates or partly paid invoices.

## Decisions
- "Paid" = the existing rule: closed manually, or balance due within the 3% tolerance.
- Paid date = local date (transaction UTC offset) of the latest linked payment. Closed manually with no payments → stamp shows "PAID" without a date.
- Server computes `paidDate` (YYYY-MM-DD, empty unless paid) and returns it in the owner and public invoice responses; the public response gains only this date, no payment details.

## Approaches
1. **Server-computed paid date, stamp drawn in both renderers (picked).** One rule for the date; Vue and PDF both draw a stamp.
2. Compute the date in the browser from payments: public view has no payments, and the PDF can't use it.
3. Stamp only in the browser and rely on "Save as PDF": the downloaded PDF and email attachment (the actual receipt) would be unstamped.

## Left to the builder
Exact stamp look (tilted, green border, uppercase), placement beside the totals, banner styling consistent with the public page.
