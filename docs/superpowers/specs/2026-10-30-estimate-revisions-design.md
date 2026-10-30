# Estimate revisions: design

Date: 2026-10-30
Status: approved 2026-10-30 (design approved in chat; user asked to proceed through plan and build)
Builds on: `docs/superpowers/specs/2026-10-30-estimates-design.md` (deployed 2026-10-30)

## Goal

Change an estimate after the client has seen it without losing what was sent: every change after sending creates a new version, and every earlier version stays complete and read-only.

## Rules

- A **draft** is edited in place (the client hasn't seen it).
- Once an estimate is **sent** (emailed or its link shared), it is never edited in place. Changes go through **Revise**, which creates the next version as a new draft.
- **Revise** is allowed on the latest version of a chain when it is `sent` (incl. expired), `declined`, or `accepted` with no non-deleted invoice made from it. Never once anything is invoiced from it, never on a superseded version, never on a draft.
- Revising an accepted estimate clears the acceptance (the client agreed to a different version): the old version keeps its recorded acceptance as history; the new draft starts unanswered.

## Data

### Invoice (new columns, estimates only)

| Field | Notes |
|---|---|
| RevisionOf | id of the chain's first version; 0 on the first version itself |
| RevisionNumber | 0 on the first version, then 1, 2, … |

New stored status `superseded` (in addition to draft / sent / accepted / declined), set on a version when a newer one is created. A superseded version is read-only: no edit, revise, answer, convert, mark accepted / declined, reopen or delete. It keeps its lines, milestones, PDF, sent emails and recorded response.

Existing estimates are first versions (`RevisionOf = 0`, `RevisionNumber = 0`); nothing changes for them.

### Numbering

A revision's number is the first version's number plus `-R<n>` (e.g. `EST/2026-27/01-R2`). The per-financial-year estimate sequence ignores revision suffixes: the next new estimate after `EST/2026-27/01-R2` is `EST/2026-27/02`. Revision numbers are never reused, even after a draft revision is deleted (n = highest existing RevisionNumber in the chain, including deleted rows, + 1).

## The Revise action

`POST /api/v1/estimates/revise.json {id}` — in one transaction, with the estimate row locked (`lockInvoiceRow`; only estimate rows are locked, so the invoice → estimate lock order holds):

1. Re-read the version; refuse unless it is the latest of its chain (no non-deleted version with a higher RevisionNumber in the chain) and its status allows Revise (rules above), with `ErrEstimateCannotBeRevised` (new error code 70) otherwise.
2. Insert the new version: same customer, currency, tax type and rate, notes, lines (with products, SAC, quantities, rates) and milestones (labels, percents, `InvoiceId = 0`); `Kind = estimate`, status `draft`, invoice date today, valid until today + profile `EstimateValidDays`, number as above, `RevisionOf` = chain root id, `RevisionNumber` = n; no public token, not sent, no response.
3. Update the old version to `superseded` conditionally on its status still being the one read in step 1 (concurrent Revise clicks create exactly one revision).

Returns the new version (the frontend opens it in the editor).

Server-side lock: `ModifyEstimate` refuses anything but `draft` (previously `draft` or `sent`), so a sent estimate can't be edited in place through the API either. Mark accepted / declined, reopen, convert and the public answer refuse `superseded`.

## Deleting

- A draft revision (the latest version, `RevisionNumber > 0`) can be deleted; in the same transaction the previous version returns from `superseded` to the status it had before (stored on the superseded row as `SupersededFromStatus`, a VARCHAR(16) column set in step 3 of Revise).
- A superseded version cannot be deleted. Deleting a first-version draft works as today.

## Public link

- A superseded version's public page shows "This estimate has been revised." with no Accept / Decline. When the latest version of the chain is `sent` or later and has a public token, it adds "View the latest version" linking to that token's page. The public response gains `supersededBy` (the latest version's public token or empty) — no ids.
- The public answer endpoints refuse superseded versions (existing "can no longer be answered").

## In the app

- **Estimates list (desktop + mobile):** only the latest version of each chain; superseded versions appear when "Show closed" is on.
- **Estimate detail:** a **Revisions** section listing every version of the chain (number, date, status, total), newest first; selecting one opens it read-only. Draft → Edit; sent / expired / declined / accepted-not-invoiced → **Revise** (opens the editor on the new draft); superseded → read-only with "Superseded by <number>".
- **Customer timeline:** "Estimate <old number> revised as <new number>" event at the new version's creation time.
- **Needs attention, conversion, invoices:** unchanged; only latest versions can be accepted or converted.

## Testing

Go: revise allowed / refused per status, invoiced and superseded; copies lines, milestones (InvoiceId reset), tax, currency, customer, notes, dates; numbering `-R1`, `-R2`, sequence ignores suffixes, numbers not reused after delete; concurrent revise creates one; ModifyEstimate refuses sent; superseded refuses answer / convert / mark / reopen / delete; deleting a draft revision restores the previous status; public view of a superseded version (no ids, supersededBy token only when the latest is sent with a link); list returns latest versions only unless closed are requested; timeline revised event.
Vitest: display status `superseded`; revision label helper.
Manual: desktop and mobile revise flow; old public link in a private window.

## Out of scope

Side-by-side diff of two versions; reverting to an older version (revise from the latest and re-enter); revising invoices.
