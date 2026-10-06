# SMS auto-capture: design

Date: 2026-10-06
Status: approved

## Goal

Every bank / card transaction alert that reaches the user's iPhone lands in the user's ezBookkeeping book
within seconds, in the right account and category, so balances and spending are always current.
Bank statements stay the monthly source of truth and reconcile against what SMS captured.

Success means:

- A UPI / card / IMPS / NEFT alert creates exactly one transaction, in the right account, usually already categorised.
- Transfers between the user's own accounts (and card bill payments) become one transfer, not an expense plus an income.
- Importing the month's statement afterwards adds only what SMS missed, and reports any SMS transaction the statement does not contain.
- One user's alerts can never reach another user's book.

## Constraints

- iOS gives no app (native or PWA) access to the SMS inbox. The only supported route is a Shortcuts
  personal automation ("When I get a message …"), which can run without confirmation.
- iOS does not let any app or link create that automation. A shared shortcut can be installed from a link,
  but each user adds the automation trigger once by hand.
- ezBookkeeping is multi-user. Everything is scoped by `uid`; this feature must keep that property.
- Statement importers already exist for HDFC (`hdfc_bank_xls`) and PNB ONE (`pnb_one_csv`).

## Architecture

```
iPhone: bank SMS
  └─ Shortcuts automation: "message contains Rs" → Run Immediately → shortcut "Record bank SMS"
       POST /api/v1/alerts/ingest.json   Authorization: Bearer <alert-ingest token>
       {"sender": "AD-HDFCBK", "text": "<sms>", "receivedAt": 1791234567}
server (this fork)
  alerts API  → parser registry → account resolver → deduper → classifier → transaction service
  └─ response {"result": "added" | "duplicate" | "ignored" | "unparsed", "summary": "₹500 · Food · HDFC 1234"}
```

The endpoint is channel-agnostic (plain text + sender), so email alerts can feed it later without redesign.

## Components

### 1. Alert-ingest token (`pkg/core`, `pkg/api/tokens.go`, `pkg/middlewares`)

- New `USER_TOKEN_TYPE_ALERT_INGEST` (next free value, 9), created like the MCP / API tokens
  (password confirmation, expiry choice).
- New middleware `JWTAlertIngestAuthorization`: accepts only this token type and only on
  `/api/v1/alerts/ingest.json`. Existing middlewares reject this token type everywhere else.
- At most one active ingest token per user; creating a new one revokes the previous one.
  The token list in Settings → Security shows it with "last used".
- The user is taken from the token claims only. Request fields never carry or select a user.

### 2. Alerts API (`pkg/api/alerts.go`)

- `POST /api/v1/alerts/ingest.json`, body `{sender, text, receivedAt}`; text capped at 2 KB.
- Rate limit per token (reuse the duplicate-checker / failure counters; e.g. 30 requests per minute).
- Returns only the outcome and, for "added", a one-line summary of the user's own new transaction.
  It never returns balances or other transactions.
- Settings page endpoints for the setup card: create ingest token (returns it once), revoke, status
  (last received alert, counts by outcome), and a "send test" status poll.

### 3. Parsers (`pkg/alerts/parsers`)

- Interface: `Parse(sender, text string, receivedAt time.Time) (*ParsedAlert, error)`; a registry picks
  the parser by sender ID suffix (`HDFCBK`, `YESBNK`, `PNBSMS`, card issuers) and falls back to a
  generic parser.
- `ParsedAlert`: direction (debit / credit), amount, account or card last-4, counterparty (VPA, merchant
  or name), reference number (UPI ref / RRN / IMPS ref), transaction time, optional available balance,
  raw text.
- Non-transaction messages (OTP, promotions, payment-due reminders, mandate notices) return
  "ignored". Anything that looks financial but cannot be parsed safely returns "unparsed". Nothing is guessed.
- Fixtures come from the user's real bank SMS, collected locally and anonymised (amounts, names,
  account numbers and references replaced) before they are committed.

### 4. Account resolver

- Maps last-4 to one of the requesting user's accounts using the trailing digits of account names
  ("HDFC Bank 1234", "Credit Card 5678") plus an optional per-account "alert last-4" field if names differ.
- No match → transaction is created in a per-user "Unmatched alerts" holding account, tagged
  "Needs review" (never silently dropped, never assigned to a guess).

### 5. Deduper

- Primary key: (user, reference number). Same reference already recorded → "duplicate".
- Without a reference: same account, direction, amount and timestamp within 2 minutes → "duplicate".
- Own-account pairs: a debit classified as a transfer to account B leaves a pending expectation;
  the matching credit SMS on B (same amount, within 3 days) is recognised and not added again.

### 6. Classifier (history first, no rules screen)

In order:

1. Own accounts / card bills → transfer to that account (payee is one of the user's accounts by last-4,
   the account holder's own name with a known bank, CRED or a card bill).
2. Known payee → same category or transfer target as the most recent transaction in the user's book
   with the same payee key (e.g. `UPI:<name>`, `MERCHANT:<name>`). Corrections made in the app
   therefore apply to future alerts automatically.
3. Merchant keyword rules (food delivery, telecom, fuel, …), the same set used for the statement imports.
4. Otherwise Other Expense / Other Income, tagged "Needs review".

All SMS-created transactions carry the tag "Auto (SMS)".

### 7. Balance check

If an alert carries an available balance and it differs from the book balance of that account after the
transaction, the transaction is tagged "Balance mismatch" (signals a missed or duplicated alert).

### 8. Statement reconciliation (existing import flow)

During statement parsing, each row is matched against "Auto (SMS)" transactions on the same account:
same amount and direction, date within ±2 days, reference number equal when the statement has one.

| Case | Behaviour |
|---|---|
| Row matches | Not imported again. Shown in the review step as "Already recorded from SMS", unticked by default. On import the SMS transaction takes the statement date and narration, keeps its category, and "Auto (SMS)" becomes "Statement verified". |
| Row without match | Imported normally (missed alert, sweep, interest, charges). |
| SMS transaction without a row in the statement period | Tagged "Not in statement". |

The import summary shows the book balance next to the statement closing balance.

### 9. Setup UI (desktop + mobile settings card "SMS auto-capture")

1. "Set up" → creates the ingest token, shows it once with Copy.
2. "Install shortcut" → opens the shared iCloud shortcut; on import it asks "Paste your setup code".
   The shared shortcut contains no user data.
3. Illustrated steps to add the automation: Shortcuts → Automation → New → Message →
   "Message contains: Rs" → Run Immediately → run "Record bank SMS".
4. "Send test" → waits for the next alert and shows its outcome.

The shortcut posts the message and shows a notification only on failure
("Couldn't record ₹500 — it will be picked up from the statement").

## Data

- New table `alert_message`: uid, sender, raw text, received time, outcome, parsed fields (JSON),
  linked transaction id. Used for "unparsed" review, debugging and reconciliation. Deleted with the user.
- New nullable account field "alert last-4" (only needed when the account name does not end with the digits).

## Error handling

- Shortcut cannot reach the server → local notification; statement import later fills the gap.
- Server error after parsing → the raw message is still stored with outcome "unparsed".
- Expired / revoked / wrong-type token → 401, no details.

## Testing

- Parser unit tests per bank and message type against anonymised fixtures (debit, credit, card spend,
  UPI in/out, IMPS/NEFT, ATM, OTP/promo → ignored).
- API tests: missing / expired / revoked token, other token types rejected, another user's token cannot
  touch this user's accounts, duplicate reference, unknown last-4, oversize body, rate limit.
- Classifier tests: own-account transfer pairing, history reuse, keyword fallback.
- Reconciliation tests on top of the existing HDFC / PNB importer tests.
- Manual check on the local test build with the real shortcut before deploying.

## Out of scope (for now)

- AI fallback between classifier steps 3 and 4 (send only the payee / narration of an unknown payee to the
  configured `[ai]` provider, accept only confident answers). Add if "Needs review" volume justifies it.

- Email-alert ingestion (the endpoint is ready for it; add when SMS gaps show up).
- Android (would use an SMS-forwarding app hitting the same endpoint).
- Splitting EMIs into principal/interest from SMS (the loan schedule handles this).
