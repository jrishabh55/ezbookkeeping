# Dashboards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Filters on Overview widgets, a Key Figure widget, financial-year ranges, and several named dashboards, so the user can build a Business P&L dashboard.

**Architecture:** Widgets whose data has to differ from the shared Home data get their own query. `HomePage` collects one query per widget (keyed by widget id), and the overview store loads each distinct query once, as Recent Transactions does today. These widgets use the existing `amounts.json` and `statistics.json` endpoints, which already accept arbitrary named ranges, exclusions and a tag filter. Extra dashboards live in a new cloud setting beside the untouched Home layout.

**Tech Stack:** Vue 3 + Vuetify (desktop), Framework7 (mobile), Pinia, Vitest; Go (two small backend changes).

**Spec:** `docs/superpowers/specs/2026-11-23-dashboards-design.md`

## Global Constraints

- No schema, API or migration changes. Backend edits are limited to two allowed cloud-setting keys and the auto-tagger income rule.
- The existing Home dashboard must look, load and total exactly as today for a user who has not touched the new settings.
- Edits to upstream files are small and additive. New logic goes in new files where possible.
- Desktop + mobile + `src/views/base/` split. New UI strings go in `src/locales/en.json` only.
- Amounts stay as `BigDecimal` in the default currency, converted the way `overviewStore.transactionOverview` converts them today.
- Range names sent to `amounts.json` must match `/^[A-Za-z0-9]+$/`, because the backend splits on `_` and `|`.
- Run frontend tests with `npx vitest run --exclude '.claude/**'` and type checks with `npx vue-tsc --noEmit`. Commit with `--no-verify`.
- **Plan call:** "Extra dashboards ignore the Home-wide filters" applies to the widgets that gain filter settings (Period ×2, Trend, Category Ranking, Key Figure). Other widget types on an extra dashboard behave as they do on Home.

## Review Focus

1. **A widget filtered to a category or account that was later deleted or hidden.** The include list keeps a stale id. The complement must be computed from current ids only. If nothing valid is left, treat it as no filter rather than excluding everything. *Test in Task 2.*
2. **The day rolls over while Home is open.** Widget query cache keys contain timestamps, so the next reload must build new ranges and drop old keys rather than show yesterday's "today". *Test in Task 3.*
3. **A saved extra-dashboards setting that is malformed, or that came from another device with unknown widget types.** It must parse to the valid subset and never break Home. *Test in Task 4.*
4. **A selected-dashboard id that no longer exists** (deleted on another device). Home must fall back to the Home tab. *Test in Task 4 (`resolveSelectedDashboardId`).*
5. **Comparing a negative net with a negative or zero base.** The % change sign must read naturally (−50k vs −100k is an improvement, ▲50%), and a zero base shows no %. *Test in Task 2.*

---

### Task 1: Backend: allow dashboard settings, tag business income

**Files:**
- Modify: `pkg/models/user_app_cloud_setting.go:25-26`
- Modify: `pkg/services/alerts.go` (`classify`, around lines 1160-1166, plus a constant near line 120)
- Test: `pkg/services/alerts_test.go`, `pkg/models/user_app_cloud_setting_test.go` (create if absent)

**Interfaces:**
- Produces: cloud keys `desktopOverviewExtraDashboards`, `mobileOverviewExtraDashboards` (string).

- [ ] **Step 1: Write failing tests**
  - `TestAllowedCloudSettingKeysIncludeExtraDashboards`: both keys are in `ALL_ALLOWED_CLOUD_SYNC_APP_SETTING_KEY_TYPES` with `USER_APPLICATION_CLOUD_SETTING_TYPE_STRING`.
  - `TestIngestTagsBusinessIncomeCredit`: a user with a visible "Business" tag and an income sub-category "Business Income" has a past untagged income transaction from a payee, and a credit SMS from that payee is ingested. Assert the new transaction's tags contain "Business". Build it like `businessTestSetup` (line 400) but with `CATEGORY_TYPE_INCOME` / `TRANSACTION_DB_TYPE_INCOME`. Use a credit SMS format already parsed in `pkg/alerts` tests, so payee history resolves to the "Business Income" category.
  - `TestIngestLeavesOtherIncomeUntagged`: the same setup with category "Interest Income". Assert "Business" is **not** in the tags.
- [ ] **Step 2: Run them and see them fail**
  `go test ./pkg/services/ -run 'TestIngest(TagsBusinessIncome|LeavesOtherIncome)' ./pkg/models/ -run TestAllowedCloudSettingKeys` → the first two FAIL. The "other income" test may already pass.
- [ ] **Step 3: Implement**
  - Add both keys as `USER_APPLICATION_CLOUD_SETTING_TYPE_STRING` next to `mobileOverviewPageLayout`.
  - In `classify`, add `const businessIncomeCategoryName = "Business Income"`. When `businessTagId > 0 && parsed.Direction == alerts.Credit && !classification.IsTransfer` and the classified category's name equals it, set `classification.Business = true`. Update the doc comment above `classify`.
- [ ] **Step 4: Run** `go test ./pkg/services/ ./pkg/models/` → PASS.
- [ ] **Step 5: Commit** `feat(overview): allow extra dashboard settings; auto-tag Business Income credits as Business`

---

### Task 2: Pure widget-query logic

**Files:**
- Modify: `src/core/overview_layout.ts` (add types and enum values only)
- Create: `src/lib/overview_widget_query.ts`
- Test: `src/lib/__tests__/overview_widget_query.test.ts`

**Interfaces:**
- Produces, in `src/core/overview_layout.ts`:
  ```ts
  OverviewWidgetType.KeyFigure = 'key-figure'
  OverviewWidgetDataRequirement.WidgetAmounts = 'widgetAmounts'
  OverviewWidgetDataRequirement.WidgetCategoryStatistics = 'widgetCategoryStatistics'
  export enum OverviewKeyFigureMetric { Income = 'income', Expense = 'expense', Net = 'net', Margin = 'margin' }
  export enum OverviewComparisonMode { None = 'none', PreviousPeriod = 'previous', SamePeriodLastYear = 'lastYear' }
  export interface OverviewWidgetFilter { accountIds: string[]; categoryIds: string[]; tagFilter: string; }
  export interface OverviewNamedRange { name: string; startTime: number; endTime: number; }
  export interface OverviewWidgetAmountsQuery { ranges: OverviewNamedRange[]; excludeAccountIds: string[]; excludeCategoryIds: string[]; tagFilter: string; }
  export interface OverviewWidgetStatisticsQuery { startTime: number; endTime: number; tagFilter: string; }
  export interface OverviewHomeFilter { excludeAccountIds: string[]; excludeCategoryIds: string[]; tagFilter: string; }
  ```
- Produces, in `src/lib/overview_widget_query.ts`:
  ```ts
  getWidgetFilter(settings: Record<string, OverviewWidgetSettingValue>): OverviewWidgetFilter   // reads filterAccountIds, filterCategoryIds, filterTags
  isWidgetFilterEmpty(filter: OverviewWidgetFilter): boolean
  getExcludedIds(includeIds: string[], allIds: string[]): string[]      // complement over allIds; [] when no includeId is in allIds
  mergeTagFilters(...filters: string[]): string                          // non-empty parts joined with ';'
  getComparisonRange(current: StartEndTime, todayLastUnixTime: number, unit: 'days' | 'weeks' | 'months' | 'years', mode: OverviewComparisonMode): StartEndTime | null
  getComparisonUnit(dateType: number): 'days' | 'weeks' | 'months' | 'years' | null
  getKeyFigureValue(metric: OverviewKeyFigureMetric, income: BigDecimal, expense: BigDecimal): BigDecimal | null   // Margin = net ÷ income × 100; null when income is zero
  getChangePercent(current: BigDecimal, previous: BigDecimal): number | null   // (current − previous) ÷ |previous| × 100; null when previous is zero
  getWidgetQueryCacheKey(query: OverviewWidgetAmountsQuery | OverviewWidgetStatisticsQuery): string   // JSON with sorted id arrays
  ```

- [ ] **Step 1: Write failing tests.** Use `moment.tz.setDefault('Asia/Kolkata')` and a `unix('YYYY-MM-DD HH:mm:ss')` helper, as `datetime.test.ts` does.
  - `getComparisonRange`, with today ending at `2026-11-23 23:59:59`:
    - this month in progress (`11-01 00:00` → `11-30 23:59:59`), previous → `10-01 00:00:00` → `10-23 23:59:59`
    - the same with last year → `2025-11-01 00:00:00` → `2025-11-23 23:59:59`
    - this FY in progress (`2026-04-01` → `2027-03-31 23:59:59`), previous → `2025-04-01 00:00:00` → `2025-11-23 23:59:59`
    - last month (finished, `10-01` → `10-31 23:59:59`), previous → `09-01 00:00:00` → `09-30 23:59:59`
    - finished Feb 2029 (`2029-02-01` → `2029-02-28 23:59:59`, today far after), last year → `2028-02-01 00:00:00` → `2028-02-29 23:59:59`
    - today ending `2026-03-31 23:59:59` with this month, previous → `2026-02-01 00:00:00` → `2026-02-28 23:59:59`
    - today (`11-23 00:00` → `11-23 23:59:59`), previous → `11-22 00:00:00` → `11-22 23:59:59`
    - mode None → `null`
  - `getComparisonUnit`: Today → days, ThisWeek → weeks, ThisMonth/LastMonth → months, ThisYear/ThisFiscalYear/LastFiscalYear → years, Custom → null.
  - `getExcludedIds(['a'], ['a','b','c'])` → `['b','c']`; `getExcludedIds([], ['a'])` → `[]`; `getExcludedIds(['gone'], ['a','b'])` → `[]` (Review Focus 1).
  - `mergeTagFilters('', '0:1', '', '2:3')` → `'0:1;2:3'`; `mergeTagFilters('', '')` → `''`.
  - `getKeyFigureValue`: Net(100, 30) → 70; Margin(200, 150) → 25; Margin(0, 50) → null; Income/Expense return their input.
  - `getChangePercent`: (120, 100) → 20; (−50, −100) → 50 (Review Focus 5); (80, 100) → −20; (10, 0) → null.
  - `getWidgetQueryCacheKey` is equal for two queries whose id arrays differ only in order.
- [ ] **Step 2: Run** `npx vitest run src/lib/__tests__/overview_widget_query.test.ts` → FAIL (module missing).
- [ ] **Step 3: Implement** the types and functions above. `getComparisonRange` (moment does the month-end clamping):
  ```
  inProgress = current.endTime > todayLast;  curEnd = inProgress ? todayLast : current.endTime
  shiftUnit = mode === PreviousPeriod ? unit : 'years'
  start = getUnixTimeBeforeUnixTime(current.startTime, 1, shiftUnit)
  end   = inProgress ? getUnixTimeBeforeUnixTime(curEnd, 1, shiftUnit)
                     : getUnixTimeBeforeUnixTime(current.endTime + 1, 1, shiftUnit) - 1
  if (mode === PreviousPeriod) end = min(end, current.startTime - 1)
  ```
- [ ] **Step 4: Run** the same command → PASS.
- [ ] **Step 5: Commit** `feat(overview): widget filter, comparison range and key figure helpers`

---

### Task 3: Overview store: per-widget amounts and statistics

**Files:**
- Modify: `src/lib/services.ts` (add one function after `getTransactionAmounts`, line ~707)
- Modify: `src/stores/overview.ts`
- Test: `src/stores/__tests__/overview_widget_data.test.ts` (or the pure part in `src/lib/__tests__/overview_widget_query.test.ts` if the store is hard to instantiate)

**Interfaces:**
- Consumes: Task 2 types and `getWidgetQueryCacheKey`.
- Produces:
  ```ts
  services.getTransactionAmountsByRanges(ranges: OverviewNamedRange[], excludeAccountIds: string[], excludeCategoryIds: string[], tagFilter: string, useTransactionTimezone: boolean): ApiResponsePromise<TransactionAmountsResponse>
  // overview store
  loadWidgetAmounts({ force, queries }: { force: boolean, queries: Record<string /*widgetId*/, OverviewWidgetAmountsQuery> }): Promise<void>
  getWidgetAmounts(widgetId: string): Record<string /*range name*/, TransactionOverviewDataItem> | undefined
  getWidgetAmountsQuery(widgetId: string): OverviewWidgetAmountsQuery | undefined
  loadWidgetCategoryStatistics({ force, queries }: { force: boolean, queries: Record<string, OverviewWidgetStatisticsQuery> }): Promise<void>
  getWidgetCategoryStatistics(widgetId: string): TransactionStatisticResponse | undefined
  ```

- [ ] **Step 1: Write failing tests.** Mock `services`.
  - Two widgets with equal queries cause one `getTransactionAmountsByRanges` call. `getWidgetAmounts` returns the same converted items for both widget ids.
  - A widget id not in the last `queries` → `getWidgetAmounts` is `undefined`.
  - After `updateTransactionOverviewInvalidState(true)`, a non-forced load calls the service again. Without it, it doesn't.
  - Loading with a query whose ranges differ (the next day) drops the old cache key: `widgetAmountsData` holds only keys requested in the latest call (Review Focus 2).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement**
  - Extract the per-item conversion body of the `transactionOverview` computed (lines ~199-230) into a local `toOverviewDataItem(item: TransactionAmountsResponseItem): TransactionOverviewDataItem`. The computed calls it, so its behaviour is unchanged.
  - New state: `widgetQueryKeys: Record<widgetId, cacheKey>`, `widgetAmountsData: Record<cacheKey, TransactionAmountsResponse>`, `widgetAmountsStateInvalid: Record<cacheKey, boolean>`, and the same three for statistics.
  - The statistics loader calls `services.getTransactionStatistics` with the query's start/end/tagFilter, `keyword: ''`, `matchMode: KeywordMatchMode.Default.type` and the Home timezone setting.
  - Add the new maps to `updateTransactionOverviewInvalidState` and `resetTransactionOverview`.
  - The amounts request uses `settingsStore.appSettings.timezoneUsedForStatisticsInHomePage`, as `loadTransactionOverview` does.
- [ ] **Step 4: Run** tests and `npx vue-tsc --noEmit` → PASS.
- [ ] **Step 5: Commit** `feat(overview): per-widget amounts and category statistics loaders`

---

### Task 4: Layout lib: widget queries from a layout, extra dashboards, settings

**Files:**
- Modify: `src/lib/overview_layout.ts`
- Modify: `src/core/setting.ts` (three fields), `src/stores/setting.ts` (three setters)
- Test: `src/lib/__tests__/overview_layout.test.ts`

**Interfaces:**
- Consumes: Task 2.
- Produces:
  ```ts
  export const OVERVIEW_HOME_DASHBOARD_ID = 'home';
  export const OVERVIEW_MAX_EXTRA_DASHBOARDS = 20;
  export interface OverviewDashboard<L extends OverviewLayoutBase> { id: string; name: string; layout: L; }
  export interface OverviewWidgetQueryContext {
      allAccountIds: string[]; allCategoryIds: string[];
      homeFilter: OverviewHomeFilter | null;          // non-null only on the Home dashboard
      todayLastUnixTime: number; firstDayOfWeek: WeekDayValue; fiscalYearStart: number;
      monthRanges: Record<TransactionAmountsRequestType, StartEndTime>;   // overviewStore.transactionDataRange
  }
  getOverviewWidgetAmountsQueries(layout: OverviewLayoutBase, ctx: OverviewWidgetQueryContext): Record<string, OverviewWidgetAmountsQuery>
  getOverviewWidgetStatisticsQueries(layout: OverviewLayoutBase, ctx: OverviewWidgetQueryContext): Record<string, OverviewWidgetStatisticsQuery>
  parseDesktopOverviewExtraDashboards(value: string): OverviewDashboard<DesktopOverviewLayout>[]
  serializeDesktopOverviewExtraDashboards(dashboards: OverviewDashboard<DesktopOverviewLayout>[]): string   // '' when empty
  parseMobileOverviewExtraDashboards / serializeMobileOverviewExtraDashboards   // same, mobile layout
  resolveSelectedDashboardId(selectedId: string, extraDashboards: { id: string }[]): string   // falls back to OVERVIEW_HOME_DASHBOARD_ID
  // settings
  appSettings.desktopOverviewExtraDashboards: string   // cloud String
  appSettings.mobileOverviewExtraDashboards: string    // cloud String
  appSettings.overviewSelectedDashboardId: string      // local only, default 'home'
  settingsStore.setDesktopOverviewExtraDashboards(value: string) / setMobileOverviewExtraDashboards(value: string) / setOverviewSelectedDashboardId(value: string)
  ```
- Which widgets get a query from `getOverviewWidgetAmountsQueries`:
  - **Key Figure:** always. Ranges `current`, plus `compare` when comparison is not None.
  - **Period Income and Expense / Period Net Income:** when the filter is non-empty, OR `dateRange` is not one of Today/ThisWeek/ThisMonth/ThisYear, OR `ctx.homeFilter` is null. Range: `current`.
  - **Income/Expense Trend:** when the filter is non-empty or `ctx.homeFilter` is null. Ranges: the same names and boundaries as the shared overview months (`thisMonth`, `lastMonth`, `monthBeforeLastMonth` … up to `months`), taken from `ctx.monthRanges`.
  - In every case: exclusions = union(homeFilter exclusions, `getExcludedIds(include, all)`), tagFilter = `mergeTagFilters(homeFilter?.tagFilter ?? '', widget tag)`. `current` = `getDateRangeByDateType(dateRange, …)`, with `endTime` kept as is.
- `getOverviewWidgetStatisticsQueries`: an **Expense Category Ranking** widget gets a query when its tag filter is non-empty, or `dateRange` is ThisFiscalYear/LastFiscalYear, or `ctx.homeFilter` is null.
- Adding these requirement types to `getOverviewDataRequirements` happens in Task 5, through the definitions.

- [ ] **Step 1: Write failing tests**
  - On Home with no filters, `getOverviewWidgetAmountsQueries(DEFAULT_DESKTOP_OVERVIEW_LAYOUT, homeCtx)` → `{}` (anti-goal: Home unchanged).
  - A Period widget with `filterTags: '0:T1'` on Home with a home tag `'0:H'` → tagFilter `'0:H;0:T1'`, and exclusions include the home exclusions.
  - The same widget with `ctx.homeFilter = null` → tagFilter `'0:T1'`.
  - A Key Figure with comparison PreviousPeriod and ThisMonth → range names `['current','compare']`.
  - `parseDesktopOverviewExtraDashboards`: `'not json'` → `[]`. A list with a duplicate id, an id `'home'`, an empty name and one valid entry → only the valid entry. A layout with an unknown widget type keeps the dashboard and drops the widget. More than 20 entries → the first 20 (Review Focus 3).
  - `resolveSelectedDashboardId('gone', [{id:'a'}])` → `'home'`; `('a', [{id:'a'}])` → `'a'` (Review Focus 4).
- [ ] **Step 2: Run** `npx vitest run src/lib/__tests__/overview_layout.test.ts` → FAIL.
- [ ] **Step 3: Implement.** Parse and serialize reuse `normalizeDesktopOverviewLayout` / `normalizeMobileOverviewLayout`. Names are trimmed and capped at 40 characters. Settings fields mirror `desktopOverviewPageLayout` (cloud type `String`, default `''`). The `overviewSelectedDashboardId` default is `'home'`, and it is not in the cloud type map.
- [ ] **Step 4: Run** tests + `npx vue-tsc --noEmit` → PASS.
- [ ] **Step 5: Commit** `feat(overview): widget queries from layout, extra dashboards setting`

---

### Task 5: Widget definitions: filters, financial-year ranges, Key Figure

**Files:**
- Modify: `src/consts/overview_layout.ts`
- Modify: `src/locales/en.json`
- Test: `src/lib/__tests__/overview_layout.test.ts`

**Interfaces:**
- Produces: setting names `filterAccountIds` (`accountSelect`), `filterCategoryIds` (`categorySelect`), `filterTags` (`tagSelect`), as one shared `WIDGET_FILTER_SETTINGS` array with display names "Filter Accounts", "Filter Categories" and "Filter Tags". Key Figure setting names: `metric`, `dateRange`, `comparison`.

- [ ] **Step 1: Write failing tests**
  - Desktop and mobile Period ×2 and Key Figure, plus desktop Trend, list all three filter settings. Ranking lists only `filterTags`.
  - The Period ×2 `dateRange` values include LastMonth, ThisFiscalYear and LastFiscalYear. Ranking includes ThisFiscalYear and LastFiscalYear.
  - Key Figure defaults: `{ metric: 'net', dateRange: ThisMonth, comparison: 'previous' }`. Desktop default size 3×2, min 2×2. Its data requirement is `WidgetAmounts`.
  - Period ×2 and Trend definitions also list `WidgetAmounts`. Ranking also lists `WidgetCategoryStatistics`.
  - `normalizeDesktopOverviewLayout` on the old default layout is unchanged (anti-goal).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement.** The filter settings have no `defaultSettings` entries (absent means no filter). Key Figure date ranges: Today, ThisWeek, ThisMonth, LastMonth, ThisYear, ThisFiscalYear, LastFiscalYear. Metric labels: "Income", "Expense", "Net Income", "Margin". Comparison labels: "No Comparison", "Previous Period", "Same Period Last Year". Add every new string to `en.json`.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(overview): filter and fiscal-year settings, Key Figure definition`

---

### Task 6: Existing widgets read filtered data

**Files:**
- Modify: `src/views/base/overview/PeriodStatisticsWidgetBase.ts`, `src/views/base/overview/ExpenseCategoryRankingWidgetBase.ts`
- Modify: `src/views/desktop/overview/OverviewWidget.vue`, `src/views/mobile/overview/OverviewWidget.vue` (pass `:widget-id="widget.id"` and `:filtered`)
- Modify: desktop `PeriodIncomeExpenseWidget.vue`, `PeriodNetIncomeAndSavingsRateWidget.vue`, `IncomeExpenseTrendWidget.vue`, `ExpenseCategoryRankingWidget.vue`, `OverviewWidgetHeader.vue`. Mobile: the matching three widgets.
- Modify: `src/stores/overview.ts` (`getTransactionListPageParams` gains an optional `filter`)

**Interfaces:**
- Consumes: `getWidgetAmounts`, `getWidgetAmountsQuery`, `getWidgetCategoryStatistics` (Task 3).
- Produces:
  - `CommonPeriodStatisticsWidgetProps` gains `widgetId?: string`.
  - The ranking base props gain `widgetId?: string`.
  - `OverviewWidgetHeader` gains a `filtered?: boolean` prop that shows a small `mdiFilterOutline` icon with the tooltip "Filtered".
  - `getTransactionListPageParams({ …, filter?: { accountIds: string[]; categoryIds: string[]; tagFilter: string; ignoreHomeFilters: boolean } })`.

- [ ] **Step 1: Implement data selection.** `getOverviewItem` returns `overviewStore.getWidgetAmounts(widgetId)?.current` when present, otherwise the shared item, as today. The display date for non-legacy ranges is formatted with `formatRange` from `getWidgetAmountsQuery(widgetId)`'s `current` range. Trend uses `getWidgetAmounts(widgetId) ?? overviewStore.transactionOverview`. Ranking uses `getWidgetCategoryStatistics(widgetId) ?? transactionCategoryStatisticsData[dateType]`.
- [ ] **Step 2: Implement details links.** Pass the widget's filter. With `ignoreHomeFilters`, the Home filter params are skipped. Include-only `categoryIds` / `accountIds` and the merged `tagFilter` are added.
- [ ] **Step 3: Verify.** `npx vue-tsc --noEmit` and `npx vitest run --exclude '.claude/**'` → PASS.
- [ ] **Step 4: Manual check.** `npm run serve`. With no new settings, Home totals match production for the same day. Then add `filterTags` = Business to a Period widget: its total matches the Insights "Business vs Personal" view for the month, and the filter icon shows.
- [ ] **Step 5: Commit** `feat(overview): period, trend and ranking widgets honour widget filters`

---

### Task 7: Key Figure widget

**Files:**
- Create: `src/views/base/overview/KeyFigureWidgetBase.ts`, `src/views/desktop/overview/widgets/KeyFigureWidget.vue`, `src/views/mobile/overview/widgets/KeyFigureWidget.vue`
- Modify: desktop and mobile `OverviewWidget.vue` (register it)
- Test: covered by Task 2's pure functions. No component test.

**Interfaces:**
- Consumes: `getWidgetAmounts(widgetId)` → `current` / `compare` items, `getWidgetAmountsQuery(widgetId)` for the comparison date label, `getKeyFigureValue`, `getChangePercent`.
- Props: `widgetId: string`, `title?: string`, `metric: OverviewKeyFigureMetric`, `dateType: number`, `comparison: OverviewComparisonMode`, `filtered: boolean`, `loading: boolean`, `editing?: boolean`.

- [ ] **Step 1: Implement the base composable.** It returns `displayValue` (amount via `getDisplayAmount` from the period base, or a `formatPercentToLocalizedNumerals` value for Margin, "–" when null), `valueSign` (−1/0/1), `displayCompareValue`, `displayCompareRange` (e.g. "1 Apr – 23 Nov 2025" via `formatRange`), `changePercent: number | null`, and `detailsUrl`. It honours `showAmountInHomePage`, as other widgets do.
- [ ] **Step 2: Implement the desktop and mobile components.** Show the title (default: the metric label plus the period name), the big value coloured by sign for Net/Margin (`text-income` / `text-expense`), and a line with ▲/▼ and |%| (green when the change is favourable: up for Income/Net/Margin, down for Expense) plus "vs {range}". Clicking opens `detailsUrl`. Show the filter icon when `filtered`. Show a skeleton while `loading` and nothing has been loaded yet.
- [ ] **Step 3: Verify.** `npx vue-tsc --noEmit` → PASS. In the dev server, add a Key Figure (Net, this month, vs previous period) and check the figures against the Insights view.
- [ ] **Step 4: Commit** `feat(overview): Key Figure widget`

---

### Task 8: Dashboard tabs on Home

**Files:**
- Modify: `src/views/desktop/HomePage.vue`, `src/views/mobile/HomePage.vue`

**Interfaces:**
- Consumes: Task 4 (`parse*ExtraDashboards`, `resolveSelectedDashboardId`, settings), Task 3 loaders, `getOverviewWidgetAmountsQueries` / `getOverviewWidgetStatisticsQueries`.

- [ ] **Step 1: Implement**
  - `extraDashboards` is parsed from the setting. `selectedId = resolveSelectedDashboardId(settingsStore.appSettings.overviewSelectedDashboardId, extraDashboards)`. `layout` is the Home layout when `selectedId === 'home'`, else that dashboard's layout.
  - The tab bar shows only when `extraDashboards.length > 0`: desktop `v-tabs`, mobile f7 segmented control. Selecting a tab calls `setOverviewSelectedDashboardId` and `reload(false)`.
  - `reload` builds `OverviewWidgetQueryContext` (`homeFilter` is non-null only for Home, from the three Home filter settings, with account/category exclusions as arrays of ids). It calls `loadWidgetAmounts` / `loadWidgetCategoryStatistics` when the layout requires them, next to the existing loaders.
- [ ] **Step 2: Verify.** `npx vue-tsc --noEmit` → PASS. In the dev server, with no extra dashboards Home looks identical and the network panel shows the same requests as before. Putting a hand-made extra dashboard in the setting shows tabs. Switching tabs only loads that tab's data.
- [ ] **Step 3: Commit** `feat(overview): named dashboard tabs on Home`

---

### Task 9: Manage dashboards in the layout editor

**Files:**
- Modify: `src/views/desktop/overview/LayoutEditorPage.vue`, `src/views/mobile/overview/LayoutEditorPage.vue`
- Modify: the desktop Home "edit layout" entry point and the mobile one, so they pass the current dashboard id

**Interfaces:**
- The editor takes the route query `dashboard=<id>` (desktop vue-router; mobile f7 `props.f7route.query`), default `home`.
- Save writes the Home layout setting for `home`, otherwise the edited entry's `layout` in the extra-dashboards setting.

- [ ] **Step 1: Implement.** The editor title shows the dashboard name. Its menu has:
  - **New dashboard:** asks for a name, appends an empty dashboard with id `Date.now().toString(36)`, then opens the editor on it.
  - **Rename:** not on Home.
  - **Duplicate:** copies the layout with new widget ids (`cloneWidget`). Duplicating Home copies the Home layout.
  - **Delete:** not on Home. Confirm with the existing confirm dialog, then go back to Home.
  - **Move left / Move right:** extras only.

  The New, Duplicate and Delete actions save immediately. Layout edits still use Save/Cancel, as today. The limit is `OVERVIEW_MAX_EXTRA_DASHBOARDS`, with "New" disabled at the limit.
- [ ] **Step 2: Verify.** `npx vue-tsc --noEmit` and `npx vitest run --exclude '.claude/**'` → PASS. In the dev server: create "Business", add a Key Figure, save, see the tab, rename, duplicate, delete, and reload to confirm the setting persisted.
- [ ] **Step 3: Commit** `feat(overview): create, rename, duplicate, delete and reorder dashboards`

---

### Task 10: Whole-branch check, deploy and data setup (with the user)

- [ ] **Step 1:** Run `npx vitest run --exclude '.claude/**'`, `npx vue-tsc --noEmit`, `go test ./pkg/...` → all PASS.
- [ ] **Step 2:** Deploy per the usual books.codeation.io flow (push → GHCR → Dokploy). **Ask the user before pushing.**
- [ ] **Step 3: Retag business income (production data, ask first).** Over REST, list income transactions in the "Business Income" category. Report the count and the count already tagged Business. After the user's OK, add the Business tag to each untagged one. Re-count and report before/after.
- [ ] **Step 4:** Build the Business dashboard from spec section 6 (in the UI or via the cloud setting), all filtered to tag = Business. Compare its FY revenue and expense with the Insights "Business vs Personal" view and report any difference.
