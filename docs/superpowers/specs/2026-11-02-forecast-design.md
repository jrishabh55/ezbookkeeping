# Forecast: design

Date: 2026-11-02
Status: approved 2026-11-02. Trimmed for v1 (user, 2026-11-02): no Forecast page, no suggest-from-history; an overview widget shows this month. Engine, API and MCP tool unchanged.

## Context

On 2026-11-02 the user's November forecast was worked out by hand in a Claude session: fixed costs from Sep–Oct history (with Rakshit's contract ended, Anoop's salary less his advance recovery, one Jio fibre line), the Quber invoice as uncertain income, card bills by statement cycle, and a shortfall on HDFC 9260 before the 5 Nov car EMI. This feature makes that a page in the platform.

Nothing in the code projects the future today. Scheduled templates post real transactions (cron every 15 min, `pkg/services/transactions.go:766`), which would double-count against SMS capture. Budgets, invoices, recurring invoices and credit-card statement dates exist and are reused.

## Goal

One forecast engine, three views:

1. **This month (cash)**: month-end bank balance with and without uncertain income, every expected item on a timeline, and the first date any account goes below zero.
2. **3 months (cash)**: the same rolled forward, one column per month.
3. **Business P&L**: expected business income vs business costs per month, next to actuals to date.

## Shared constraints

- Schema changes are new columns/enum values only; existing rows keep today's behaviour.
- Amounts in minor units, account currency. The forecast is computed in the user's default currency using the latest exchange rates, as the overview does; v1 users are INR-only, so no conversion edge cases are designed beyond that.
- Desktop and mobile follow existing page patterns; shared logic in `src/views/base/forecast/`.

## 1. Inputs

### 1.1 Forecast templates (fixed items)

- New `TRANSACTION_TEMPLATE_TYPE_FORECAST = 3` in `pkg/models/transaction_template.go`. It uses the same schedule fields as type 2 (frequency, days, start/end, account, category, tags, amount, comment).
- The scheduled-transaction cron already filters `template_type = SCHEDULE`, so forecast templates never post. A test pins this.
- API: `transaction_templates.go` accepts type 3 for list/create/modify (the `> SCHEDULE` bound checks become `> FORECAST`). Type 3 does **not** depend on `EnableScheduledTransaction`.
- UI: the existing template list page with `initType = Forecast`, route `/template/list?type=forecast` (mobile likewise), menu entry "Forecast items" under the templates menu. The template editor's schedule section shows for type 3 exactly as for type 2.
- **Suggest from history** button on that page: scans the last 3 full months of expenses and income (not transfers), groups by (account, category, normalised payee key: the same `PayeeKey` the SMS auto-tag uses), and proposes groups seen in at least 2 of the 3 months with amount spread ≤ 25%. Each suggestion shows payee, typical amount (median), typical day; the user ticks the ones to create. Groups already covered by a forecast template (same account + category + amount within 20%) are not suggested.
- Transfers are allowed as forecast templates (e.g. SIP to Mutual Funds, Anoop advance recovery as an expense from "Anoop - Advances").

### 1.2 Expected income from invoices (uncertain)

- Unpaid invoices (received amount below total less `InvoicePaidTolerance`, not cancelled, not estimates): one income item for the outstanding amount on `DueDate`; if `DueDate` has passed, on today.
- Enabled recurring invoices: one income item per occurrence of `NextDate` (stepping `PeriodMonths`) inside the horizon, amount = source invoice `Total`, date = occurrence + the source invoice's due-date offset (DueDate − InvoiceDate).
- Target account: the account of the most recent payment linked to any invoice of that customer; else the user's account with the most Business Income in the last 6 months.
- All invoice income is **uncertain**: views show balances with and without it.

### 1.3 Everyday spending (variable)

Per month in the horizon, per expense category:

- If a monthly budget scoped to that category (directly, no tag/account scope) covers the month: its amount.
- Else: the average monthly spend in that category over the last 3 full months, **excluding** transactions matched to a forecast template (§2) and expenses of one transaction ≥ 3× that category's median (one-offs such as the ₹35k phone).
- Category amount is split across accounts by each account's share of that category in the same 3 months, and spread evenly across the remaining days of the month (for the current month: the rest of the month minus what is already spent in that category this month, floored at 0).
- Shown as one "Everyday spending" line per account per month (expandable to categories), not per day.

### 1.4 Credit cards

- `AccountExtend` gets two optional fields: `CreditCardPaymentDueDay *int` (1–28) and `CreditCardPaymentAccountId *int64` (an asset account of the same user). Account create/modify requests and the existing credit-card section of the account editor (desktop + mobile) get the two inputs. Both empty = card is skipped for bill placement and the view says so.
- Card charges (fixed items and everyday spending on the card) reduce the card balance. On each statement date the projected outstanding is "billed"; on the next due day the billed amount moves from the paying account to the card (a transfer item, category Credit Card Repayment). Today's outstanding that predates the last statement date is billed already and pays on the next due day.

### 1.5 Starting balances

Current balance of every non-hidden asset and liability account (`query_all_accounts_balance` equivalent). Receivables (e.g. "Anoop - Advances") and investments are included in net figures but excluded from "bank cash" totals; the cash total = cash, checking, savings and virtual accounts.

## 2. Matching (no double counting)

A forecast item for a date in the current month is **fulfilled** and dropped when a real transaction in the same calendar month has the same type, same source account, same category and an amount within ±20%, and is not already matched to another item. Matching is greedy by date proximity. Recurring-invoice and unpaid-invoice items are fulfilled when a payment is linked to the invoice (already reflected as "paid"). Card bill items are fulfilled when a Credit Card Repayment transfer into that card ≥ 90% of the billed amount occurs between the statement date and the due day.

## 3. Engine

New `pkg/services/forecast.go`, `ForecastService.GetForecast(c, uid, startMonth, months)` (months 1–3):

1. Load balances, forecast templates, invoices/recurring invoices, budgets, card accounts, and transactions from 3 full months before `startMonth` to now.
2. Expand forecast templates into dated items (reuse the frequency/day expansion used by the scheduled cron; extract it into a pure function `ExpandSchedule(template, from, to) []time.Time` with tests).
3. Drop fulfilled items (§2); add invoice income (§1.2), everyday spending (§1.3), card bill transfers (§1.4).
4. Walk items by date per account, producing running balances; record for each asset account that starts at ≥ 0 the first date its balance goes below 0. Accounts already negative at the start (overdraft accounts such as HDFC 4071, where negative is normal) are not alerted.
5. Return:

```
ForecastResponse {
  months: [{ month: "2026-11",
             openingCash, closingCash, closingCashWithoutUncertain,
             businessIncome, businessCost, personalCost,          // P&L, expected + actual
             actualBusinessIncome, actualBusinessCost }]
  items:  [{ date, type, accountId, destinationAccountId?, categoryId?, amount,
             source: "template"|"invoice"|"recurring_invoice"|"everyday"|"card_bill",
             sourceId?, uncertain, business, label }]
  shortfalls: [{ accountId, date, lowestBalance, lowestDate }]
  skippedCards: [accountId]          // cards without due day / pay-from
}
```

"Business" = item carries the Business tag (templates) or is invoice income or a Business-category expense; everyday spending uses the Business-tagged share of that category's history.

Pure functions (schedule expansion, matching, everyday estimate, card billing, balance walk) take plain structs and are unit-tested without a DB.

API: `GET /api/v1/forecast/get.json?start_month=2026-11&months=3`. MCP: add a `query_forecast` tool returning the same summary (months + shortfalls + top items).

## 4. UI

- New page **Forecast** in the analytics area, top navigation next to Insights (`/forecast`, mdiCrystalBall icon), desktop and mobile.
- Tabs:
  - **This month**: summary cards (opening cash, expected in, expected out, closing cash with / without uncertain income); shortfall alerts ("HDFC 9260 goes to −₹13,050 on 5 Nov: move ₹15,000 before then"); timeline table of items (date, label, account, amount, running cash) with uncertain rows marked; everyday spending rows expandable.
  - **3 months**: one column per month: opening, in (certain / uncertain), fixed out, everyday out, card bills, closing; click a month to open it in the This month view.
  - **Business P&L**: per month expected vs actual business income, business cost, personal cost, net.
- Empty state links to Forecast items with the "Suggest from history" button.
- Reuse existing table, card and amount components; no new chart in v1.

## 5. Error handling

- Missing card fields: card listed in `skippedCards`, warning chip on the page; forecast still computes.
- Template with an account or category since deleted: item skipped, logged.
- Months outside 1–3 or bad `start_month`: 400 with the existing parameter-invalid error.
- No history (new user): everyday spending 0, page says so.

## 6. Testing

- Go unit tests: schedule expansion (monthly multi-day, negative days, end date), matching (within/outside 20%, one transaction cannot fulfil two items), everyday estimate (budget wins, one-off exclusion, current-month remainder), card billing (statement/due across month boundary, already-paid bill), balance walk shortfall date; cron test that type 3 templates never post.
- One API test for `/forecast/get.json` shape.
- Frontend: vitest for the base view-model (tab totals from a fixed response).
- Acceptance: on prod data, November 2026 forecast is within ±5% of the hand-worked figures (closing cash without Quber ≈ ₹98.5k, 9260 shortfall before 5 Nov) once the user's forecast templates are entered.

## Out of scope (v1)

Charts, scenario editing in the page, notifications for shortfalls, multi-currency beyond conversion at latest rates, forecasting investments/market value.
