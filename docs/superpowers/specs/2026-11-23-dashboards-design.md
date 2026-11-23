# Dashboards: design

Date: 2026-11-23
Status: approved by the user 2026-11-23, after the builder check; open questions answered by the user the same day.

## Why

The user wants several charts and numbers on one page, especially a business profit-and-loss view (revenue, expenses, net profit, margin by month and financial year). Insights Explorer gives one chart per saved view, can't compute income minus expense, and the user doesn't like using it for this. The three saved "Business vs Personal" explorer views are the workaround today.

## What exists (verified 2026-11-23)

- The **Overview** (Home) page is already a dashboard: a grid of widgets you can drag and resize, with a layout editor, an Add Widget dialog and per-widget settings (`src/views/desktop/overview/`, `src/views/mobile/overview/`, definitions in `src/consts/overview_layout.ts`, helpers in `src/lib/overview_layout.ts`, shared widget logic in `src/views/base/overview/`).
- One layout per device, stored as a JSON string in the cloud-synced app settings `desktopOverviewPageLayout` / `mobileOverviewPageLayout` (`src/core/setting.ts`, `src/stores/setting.ts`).
- Data is loaded centrally in `HomePage.vue` per data requirement, not per widget. The one exception is Recent Transactions: it has per-widget queries keyed by widget id, deduped by a cache key (`loadRecentTransactions` in `src/stores/overview.ts`). That is the pattern to follow.
- Number and chart widgets (Period Income and Expense, Period Net Income and Savings Rate, Income/Expense Trend, Expense Category Ranking) have only date-range and display settings. They have no tag, category or account filter. Only Recent Transactions has those filters, and the setting types and pickers (`accountSelect`, `categorySelect`, `tagSelect`) already exist in `WidgetSettingsDialog.vue` / `WidgetSettingsPopup.vue`.
- Backend: `GET v1/transactions/amounts.json` takes any number of named ranges (`query=name_start_end|...`), `exclude_account_ids`, `exclude_category_ids` and `tag_filter`. Trend and statistics endpoints take `tag_filter`. Category statistics takes no account/category filter. `DateRange.ThisFiscalYear` / `LastFiscalYear` exist in `src/core/datetime.ts`, and the user's fiscal year start is in `userStore.currentUserFiscalYearStart`.
- Business data: business expenses carry the tag **Business**, and personal ones have no tag. Business income is in the **Business Income** category and is not tagged yet (section 5 fixes that).

## Goals

1. **Filters on widgets.** The number and chart widgets can be limited by tag, category and account.
2. **A Key Figure widget.** One big number (income, expense, net, or margin) for a date range, with filters, optionally compared with the previous period.
3. **Financial-year ranges.** The Period and Key Figure widgets offer last month, this financial year and last financial year.
4. **Several named dashboards.** "Home" plus any number of others (e.g. "Business"), switchable by tabs.
5. Desktop and mobile both, following the existing patterns.

## Non-goals

- A formula editor, or figures other than income, expense, net and margin.
- Capital vs revenue spending, depreciation, a CA-ready P&L.
- A cash-flow statement.
- An "embed a saved Insights view" widget.
- Sharing or exporting dashboards.
- Dashboard-wide filters. Filters are per widget, so business and personal can sit side by side on one dashboard.
- Backend changes beyond allowing the two new cloud-setting keys (section 4) and the auto-tagger rule (section 5). The widgets' data is already served.

## Anti-goals (a failure even if it works)

- The existing Home dashboard changes for a user who doesn't touch the new settings: different layout, numbers or load time.
- Home loads slower because of dashboards that aren't being viewed.
- Upstream merges into the overview files get painful. Keep edits to upstream files small and additive.

## Approaches considered

- **A. Upgrade Overview (chosen by the user, 2026-11-23).** Add filters, a Key Figure widget, fiscal ranges and several named dashboards to the existing grid. This reuses the editor, the settings dialogs and the existing APIs.
- B. A widget that renders a saved Insights view. Rejected: every tile would still be built in the Insights editor, and it loads all transactions in the browser.
- C. A new dashboard builder with its own query model and formulas. Rejected: it duplicates A's grid and B's query engine, which is too much code for one user.

## Design

### 1. Widget filters

- Add three optional settings, shared like `WIDGET_TITLE_SETTING`:
  - `filterAccountIds` (`accountSelect`, include-only)
  - `filterCategoryIds` (`categorySelect`, include-only)
  - `filterTags` (`tagSelect`, using the existing tag-filter string format, so "has any", "has all" and "does not have" work, e.g. personal = "does not have Business")

  The default for each is empty, meaning no filter, which is today's behaviour.
- Apply them to:
  - **Period Income and Expense**, **Period Net Income and Savings Rate** and **Key Figure**: all three filters.
  - **Income/Expense Trend** (desktop only, as today): all three filters.
  - **Expense Category Ranking**: tag filter only. The statistics endpoint has no account/category filter, and a category filter on a category ranking is pointless. Its date range also gains This fiscal year and Last fiscal year (new date types in `loadTransactionCategoryStatistics`; the endpoint takes start/end times). *Builder-check call: so the Business dashboard doesn't mix calendar and financial years.*
- Include-only selections are sent as the complement through the existing `exclude_account_ids` / `exclude_category_ids` params.
- **Home-wide filters** (`overviewAccountFilterInHomePage`, `overviewTransactionCategoryFilterInHomePage`, `overviewTransactionTagFilterInHomePage`) apply on the **Home** dashboard only, as today. A filtered widget on Home combines them with its own filter: the union of the exclusions, and both tag conditions. Extra dashboards ignore the Home-wide filters, so a Business dashboard matches the Insights totals. *Builder-check call.*
- **Data loading:** a widget with no filter keeps using the shared data (`TransactionOverview`, `TransactionCategoryStatistics`), unchanged. A widget with any filter gets its data from a new per-widget loader in `src/stores/overview.ts`, modelled on `loadRecentTransactions`. Requests are keyed by a filter signature (sorted excluded accounts, excluded categories, tag filter, ranges), deduped across widgets, and invalidated by the same events that invalidate the shared overview data.
- The widget header shows a small filter icon when a filter is set, so a filtered number is never mistaken for an unfiltered one.

### 2. Key Figure widget (new, desktop and mobile)

- Settings: title, metric (Income / Expense / Net = income − expense / Margin = net ÷ income, shown as %), date range, compare (None / Previous period / Same period last year), and the three filters.
- Date ranges: Today, This week, This month, Last month, This year, This fiscal year, Last fiscal year.
- Shows the big number in the default currency, as Period widgets do. With a comparison, it also shows the earlier value and the change as % with ▲/▼. Margin shows "–" when income is 0. Net and margin are coloured by sign.
- Comparison ranges are **like-for-like** (user, 2026-11-23): when the current period isn't over, the earlier range covers the same elapsed span. This FY to date (1 Apr–23 Nov) compares with 1 Apr–23 Nov of last FY. This month to date (1–23 Nov) compares with 1–23 Oct, clamped to the end of a shorter month. Today compares with yesterday. A finished period (Last month, Last fiscal year) compares with the whole period before it. "Same period last year" shifts the range back one year with the same rule. The widget labels the earlier value with its dates (e.g. "1 Apr – 23 Nov 2025"). Both are extra named ranges in the same amounts request, so there are no extra calls.
- Tapping it opens the transaction list with the same range and filters, as Period widgets do today. The list accepts `categoryIds`, `accountIds` and the tag filter, so no filter is lost.
- Default size 3×2 on desktop.

### 3. Financial-year ranges on the Period widgets

Period Income and Expense and Period Net Income and Savings Rate get the same added ranges: Last month, This fiscal year and Last fiscal year. Existing saved settings keep their values.

### 4. Several dashboards

- The existing `desktopOverviewPageLayout` / `mobileOverviewPageLayout` settings stay as they are and are always the **Home** dashboard. There's no migration and no upstream conflict.
- New cloud settings `desktopOverviewExtraDashboards` / `mobileOverviewExtraDashboards`: a JSON array of `{ id, name, layout }`, where `layout` uses the same format and normalizer as the Home layout. The default is an empty array. The server drops unknown keys, so both keys are added to `ALL_ALLOWED_CLOUD_SYNC_APP_SETTING_KEY_TYPES` in `pkg/models/user_app_cloud_setting.go` as string settings.
- Home page: when at least one extra dashboard exists, a tab bar above the grid shows Home followed by the extras in order. With no extras, nothing changes visually. The selected tab is remembered per device in local app settings (not synced).
- Only the visible dashboard's data requirements are loaded. Switching tabs loads that dashboard's data. Cached shared data is reused.
- Layout editor: edits the dashboard of the current tab. Desktop gets a menu with "New dashboard", "Rename", "Duplicate", "Delete" (not on Home) and "Move left/right". Mobile gets the same actions in the existing editor's action sheet. A new dashboard starts empty.
- Desktop and mobile dashboards are separate, as layouts are today. They are not shared between devices.
- Mobile has no trend widget, as today, so a mobile Business dashboard uses Key Figure, Period and Category Ranking widgets. A mobile trend widget is not in scope. *Builder-check call.*

### 5. Business income tagging

Decided (user, 2026-11-23): business income gets the Business tag too, so "tag = Business" alone selects all business rows.
- **Code:** in the SMS auto-tagger (`pkg/services/alerts.go`, where `classification.Business` adds `tagBusiness`, and the classifier around line 1041), an income row whose category is Business Income is marked Business. Wherever the invoice screen creates an income transaction for a payment, it adds the Business tag when the category is Business Income. If payments only link existing rows, nothing changes there.
- **Data, at deploy:** one bulk update via the REST API: every income transaction in the Business Income category gets the Business tag. Count the rows before and after.

After this, "tag = Business" alone gives business revenue, costs, net and margin.

### 6. The Business dashboard (configuration after deploy)

After deploy, created through the UI or the cloud setting, not shipped as a default:

| Row | Widgets |
|---|---|
| 1 | Key Figure: Revenue this FY vs last FY · Expenses this FY vs last FY · Net profit this FY vs last FY · Margin this FY |
| 2 | Key Figure: Net this month vs last month · Period Income and Expense (last month) |
| 3 | Income/Expense Trend, 12 months · Expense Category Ranking, this fiscal year |

All are filtered to tag = Business.

## Constraints

- No schema, API or migration changes. Backend edits are limited to the two allowed cloud-setting keys and the auto-tagger rule.
- Follow the existing desktop + mobile + `src/views/base/` split. New shared logic goes in `src/lib/overview_layout.ts` and `src/views/base/overview/`.
- New strings go in `en` (other locales fall back to English).
- Amounts stay in minor units with the existing currency formatting.

## Testing

- Vitest (`--exclude '.claude/**'`): extra-dashboard normalisation (bad JSON → `[]`, bad layouts dropped), filter-signature dedupe, include → exclude complement, comparison range calculation (partial month, partial FY, finished month, finished FY, today, year shift, 31st clamped into a 30-day month, leap day), and Key Figure values (net, margin with zero income, % change with a zero base).
- Go: the auto-tagger marks a Business Income row Business and still leaves other income untagged.
- Manual: the existing Home is unchanged with no new settings; a filtered widget matches the Insights "Business vs Personal" totals for the same range.

## Left to the builder

Icons, exact layout of the tab bar and Key Figure, widget minimum sizes, how the filter icon looks, and how filtered and unfiltered loaders share invalidation.
