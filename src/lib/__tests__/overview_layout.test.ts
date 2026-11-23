import { describe, expect, test } from 'vitest';

import { type OverviewLayoutBase, OverviewWidgetType, OverviewWidgetDataRequirement, OverviewComparisonMode } from '@/core/overview_layout.ts';
import { ALL_TRANSACTION_AMOUNTS_REQUEST_TYPE, type TransactionAmountsRequestType } from '@/models/transaction.ts';
import { DateRange } from '@/core/datetime.ts';

import {
    DESKTOP_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_DESKTOP_OVERVIEW_LAYOUT,
    MOBILE_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_MOBILE_OVERVIEW_LAYOUT
} from '@/consts/overview_layout.ts';

import {
    getOverviewDataRequirements,
    getOverviewTransactionOverviewMonths,
    getOverviewAssetTrendMonths,
    getOverviewCalendarHeatmapMonths,
    getOverviewTransactionCategoryStatisticDateTypes,
    resolveDesktopOverviewWidgetCollisions,
    findDesktopOverviewWidgetPosition,
    findDesktopOverviewWidgetDuplicatePosition,
    normalizeDesktopOverviewLayout,
    compactDesktopOverviewWidgets,
    isDefaultDesktopOverviewLayout,
    serializeDesktopOverviewLayout,
    parseDesktopOverviewLayout,
    isDefaultMobileOverviewLayout,
    normalizeMobileOverviewLayout,
    parseMobileOverviewLayout,
    serializeMobileOverviewLayout,
    OVERVIEW_HOME_DASHBOARD_ID,
    OVERVIEW_MAX_EXTRA_DASHBOARDS,
    type OverviewWidgetQueryContext,
    getOverviewWidgetAmountsQueries,
    getOverviewWidgetStatisticsQueries,
    parseDesktopOverviewExtraDashboards,
    serializeDesktopOverviewExtraDashboards,
    parseMobileOverviewExtraDashboards,
    serializeMobileOverviewExtraDashboards,
    resolveSelectedDashboardId
} from '../overview_layout.ts';

describe('desktop overview layout', () => {
    test('merges data requirements', () => {
        const requirements = getOverviewDataRequirements(normalizeDesktopOverviewLayout({
            widgets: [{ id: 'trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 0, y: 0, w: 6, h: 6 }]
        }), DESKTOP_OVERVIEW_WIDGET_DEFINITIONS);
        expect(!!requirements[OverviewWidgetDataRequirement.TransactionOverview]).toBe(true);
        expect(!!requirements[OverviewWidgetDataRequirement.TransactionOverviewLast12Months]).toBe(true);
        expect(!!requirements[OverviewWidgetDataRequirement.Accounts]).toBe(false);
    });

    test('gets the maximum transaction overview months required by widgets', () => {
        const sixMonthLayout = normalizeDesktopOverviewLayout({
            widgets: [{ id: 'trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 0, y: 0, w: 6, h: 6, settings: { months: 6 } }]
        });
        expect(getOverviewTransactionOverviewMonths(sixMonthLayout)).toBe(6);

        const mixedLayout = normalizeDesktopOverviewLayout({
            widgets: [
                { id: 'six-month-trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 0, y: 0, w: 6, h: 6, settings: { months: 6 } },
                { id: 'twelve-month-trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 6, y: 0, w: 6, h: 6, settings: { months: 12 } }
            ]
        });
        expect(getOverviewTransactionOverviewMonths(mixedLayout)).toBe(12);
    });

    test('gets maximum widget data ranges', () => {
        const layout = normalizeDesktopOverviewLayout({
            widgets: [
                { id: 'progress', type: OverviewWidgetType.CurrentMonthExpenseProgress, x: 0, y: 0, w: 4, h: 4 },
                { id: 'recent', type: OverviewWidgetType.RecentTransactions, x: 4, y: 0, w: 4, h: 6, settings: { itemCount: 10 } },
                { id: 'worth', type: OverviewWidgetType.NetAssetsTrend, x: 0, y: 6, w: 6, h: 6, settings: { months: 12 } },
                { id: 'calendar', type: OverviewWidgetType.TransactionCalendarHeatmap, x: 0, y: 12, w: 8, h: 6, settings: { months: 6 } }
            ]
        });

        expect(getOverviewTransactionOverviewMonths(layout)).toBe(2);
        expect(getOverviewAssetTrendMonths(layout)).toBe(12);
        expect(getOverviewCalendarHeatmapMonths(layout)).toBe(6);
    });

    test('gets only the category statistic date types used by ranking widgets', () => {
        const layout = normalizeDesktopOverviewLayout({
            widgets: [
                { id: 'month-ranking', type: OverviewWidgetType.ExpenseCategoryRanking, x: 0, y: 0, w: 3, h: 4, settings: { dateRange: DateRange.ThisMonth.type } },
                { id: 'year-ranking', type: OverviewWidgetType.ExpenseCategoryRanking, x: 3, y: 0, w: 3, h: 4, settings: { dateRange: DateRange.ThisYear.type } },
                { id: 'month-ranking-2', type: OverviewWidgetType.ExpenseCategoryRanking, x: 6, y: 0, w: 3, h: 4, settings: { dateRange: DateRange.ThisMonth.type } }
            ]
        });

        expect(getOverviewTransactionCategoryStatisticDateTypes(layout)).toEqual([DateRange.ThisMonth.type, DateRange.ThisYear.type]);
    });

    test('pushes colliding widget down and compacts it back up', () => {
        const widgets = [
            { id: 'active', type: OverviewWidgetType.CurrentMonthOverview, x: 0, y: 0, w: 4, h: 3, settings: {} },
            { id: 'other', type: OverviewWidgetType.AssetSummary, x: 3, y: 1, w: 4, h: 3, settings: {} }
        ];
        const resolved = resolveDesktopOverviewWidgetCollisions(widgets, 'active');
        expect(resolved.find(widget => widget.id === 'other')?.y).toBe(3);
        expect(compactDesktopOverviewWidgets(resolved).find(widget => widget.id === 'other')?.y).toBe(3);
    });

    test('moves another widget into available space above the active widget', () => {
        const widgets = [
            { id: 'active', type: OverviewWidgetType.CurrentMonthOverview, x: 0, y: 3, w: 4, h: 3, settings: {} },
            { id: 'other', type: OverviewWidgetType.AssetSummary, x: 0, y: 3, w: 4, h: 3, settings: {} }
        ];
        const resolved = resolveDesktopOverviewWidgetCollisions(widgets, 'active');

        expect(resolved.find(widget => widget.id === 'active')?.y).toBe(3);
        expect(resolved.find(widget => widget.id === 'other')?.y).toBe(6);

        const compacted = compactDesktopOverviewWidgets(resolved, 'active');
        expect(compacted.find(widget => widget.id === 'other')?.y).toBe(0);
        expect(compacted.find(widget => widget.id === 'active')?.y).toBe(3);
    });

    test('keeps another widget below when space above the active widget is too small', () => {
        const widgets = [
            { id: 'active', type: OverviewWidgetType.CurrentMonthOverview, x: 0, y: 2, w: 4, h: 3, settings: {} },
            { id: 'other', type: OverviewWidgetType.AssetSummary, x: 0, y: 2, w: 4, h: 3, settings: {} }
        ];
        const resolved = resolveDesktopOverviewWidgetCollisions(widgets, 'active');
        const compacted = compactDesktopOverviewWidgets(resolved, 'active');

        expect(compacted.find(widget => widget.id === 'active')?.y).toBe(2);
        expect(compacted.find(widget => widget.id === 'other')?.y).toBe(5);
    });

    test('resolves full-height collisions without looping', () => {
        const resolved = resolveDesktopOverviewWidgetCollisions([
            { id: 'one', type: OverviewWidgetType.AssetSummary, x: 0, y: 0, w: 12, h: 1000, settings: {} },
            { id: 'two', type: OverviewWidgetType.AssetSummary, x: 0, y: 0, w: 12, h: 1000, settings: {} }
        ]);
        expect(resolved[1]?.y).toBe(1000);
    });

    test('normalizes sizes and widget settings', () => {
        const layout = normalizeDesktopOverviewLayout({
            widgets: [
                { id: 'one', type: OverviewWidgetType.PeriodIncomeExpense, x: -2, y: -1, w: 1, h: 1, settings: { dateRange: 'invalid' } },
                { id: 'ranking', type: OverviewWidgetType.ExpenseCategoryRanking, x: 0, y: 0, w: 4, h: 6, settings: { dateRange: 'invalid', categoryLevel: 'invalid', itemCount: 100 } },
                { id: 'accounts', type: OverviewWidgetType.AccountBalanceList, x: 4, y: 0, w: 4, h: 6, settings: { accountIds: ['1', '2', 3, '2'], accountCategories: [1, 2, 3], itemCount: 10, sortBy: 'balance' } },
                { id: 'income-trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 0, y: 0, w: 6, h: 6, settings: { months: 6, showXAxisLabels: false, showLegend: false } },
                { id: 'assets-trend', type: OverviewWidgetType.NetAssetsTrend, x: 6, y: 0, w: 6, h: 6, settings: { months: 12, showXAxisLabels: false, showLegend: false } },
                { id: 'default-trend', type: OverviewWidgetType.NetAssetsTrend, x: 0, y: 6, w: 6, h: 6, settings: { months: 12 } }
            ]
        });

        expect(layout.widgets).toHaveLength(6);
        expect(layout.widgets[0]).toMatchObject({ id: 'income-trend', type: OverviewWidgetType.IncomeExpenseTrend, x: 0, y: 0, w: 6, h: 6, settings: { months: 6, showXAxisLabels: false, showLegend: false } });
        expect(layout.widgets[1]).toMatchObject({ id: 'assets-trend', type: OverviewWidgetType.NetAssetsTrend, x: 6, y: 0, w: 6, h: 6, settings: { months: 12, showXAxisLabels: false, showLegend: false } });
        expect(layout.widgets[2]).toMatchObject({ id: 'one', type: OverviewWidgetType.PeriodIncomeExpense, x: 0, y: 6, w: 2, h: 3, settings: { dateRange: DateRange.Today.type } });
        expect(layout.widgets[3]).toMatchObject({ id: 'accounts', type: OverviewWidgetType.AccountBalanceList, x: 4, y: 6, w: 4, h: 6, settings: { accountIds: ['1', '2', '2'], itemCount: 10, sortBy: 'balance' } });
        expect(layout.widgets[3]?.settings).not.toHaveProperty('accountCategories');
        expect(layout.widgets[4]).toMatchObject({ id: 'ranking', type: OverviewWidgetType.ExpenseCategoryRanking, x: 0, y: 9, w: 4, h: 6, settings: { dateRange: DateRange.ThisMonth.type, categoryLevel: 'primary', itemCount: 3 } });
        expect(layout.widgets[5]).toMatchObject({ id: 'default-trend', type: OverviewWidgetType.NetAssetsTrend, x: 0, y: 15, w: 6, h: 6, settings: { months: 12, showXAxisLabels: true, showLegend: true } });
    });

    test('allows repeated widget types and removes duplicate ids', () => {
        const layout = normalizeDesktopOverviewLayout({
            widgets: [
                { id: 'one', type: OverviewWidgetType.PeriodIncomeExpense, x: 0, y: 0, w: 3, h: 3 },
                { id: 'two', type: OverviewWidgetType.PeriodIncomeExpense, x: 3, y: 0, w: 3, h: 3 },
                { id: 'two', type: OverviewWidgetType.AssetSummary, x: 6, y: 0, w: 6, h: 3 }
            ]
        });

        expect(layout.widgets).toHaveLength(2);
        expect(layout.widgets.every(widget => widget.type === OverviewWidgetType.PeriodIncomeExpense)).toBe(true);
    });

    test('finds first available position', () => {
        const position = findDesktopOverviewWidgetPosition([
            { id: 'one', type: OverviewWidgetType.AssetSummary, x: 0, y: 0, w: 12, h: 3, settings: {} }
        ], 4, 3);
        expect(position).toEqual({ x: 0, y: 3 });
    });

    test('places a duplicated widget on the right when possible, otherwise below', () => {
        const source = { id: 'source', type: OverviewWidgetType.AssetSummary, x: 0, y: 0, w: 4, h: 3, settings: {} };

        expect(findDesktopOverviewWidgetDuplicatePosition([source], source)).toEqual({ x: 4, y: 0 });

        const rightWidget = { id: 'right', type: OverviewWidgetType.AssetSummary, x: 4, y: 0, w: 4, h: 3, settings: {} };
        expect(findDesktopOverviewWidgetDuplicatePosition([source, rightWidget], source)).toEqual({ x: 0, y: 3 });

        const rightEdgeWidget = { ...source, x: 8 };
        expect(findDesktopOverviewWidgetDuplicatePosition([rightEdgeWidget], rightEdgeWidget)).toEqual({ x: 8, y: 3 });
    });

    test('round trips serialized layout', () => {
        const json = serializeDesktopOverviewLayout(DEFAULT_DESKTOP_OVERVIEW_LAYOUT, true);
        expect(serializeDesktopOverviewLayout(parseDesktopOverviewLayout(json))).toBe(serializeDesktopOverviewLayout(DEFAULT_DESKTOP_OVERVIEW_LAYOUT));
    });

    test('empty setting uses default layout', () => {
        const layout = parseDesktopOverviewLayout('');
        expect(isDefaultDesktopOverviewLayout(layout)).toBe(true);
        expect(layout).not.toBe(DEFAULT_DESKTOP_OVERVIEW_LAYOUT);
    });
});

describe('mobile overview layout', () => {
    test('keeps the single-column order and ignores duplicate widgets', () => {
        const layout = normalizeMobileOverviewLayout({
            widgets: [
                { id: 'period', type: OverviewWidgetType.PeriodIncomeExpense, settings: { invalid: true } },
                { id: 'asset-summary', type: OverviewWidgetType.AssetSummary },
                { id: 'accounts', type: OverviewWidgetType.AccountBalanceList },
                { id: 'month', type: OverviewWidgetType.CurrentMonthOverview },
                { id: 'period', type: OverviewWidgetType.CurrentMonthOverview }
            ]
        });

        expect(layout.widgets).toEqual([
            {
                id: 'period',
                type: OverviewWidgetType.PeriodIncomeExpense,
                settings: { dateRanges: [DateRange.Today.type, DateRange.ThisWeek.type, DateRange.ThisMonth.type, DateRange.ThisYear.type] }
            },
            {
                id: 'asset-summary',
                type: OverviewWidgetType.AssetSummary,
                settings: { height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' }
            },
            {
                id: 'accounts',
                type: OverviewWidgetType.AccountBalanceList,
                settings: { accountIds: [], itemCount: 4, showTitle: false, sortBy: 'displayOrder', alwaysShowAmount: false, showAvailableCreditForCreditCard: false }
            },
            {
                id: 'month',
                type: OverviewWidgetType.CurrentMonthOverview,
                settings: { height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' }
            }
        ]);
    });

    test('normalizes mobile widget settings', () => {
        const layout = normalizeMobileOverviewLayout({
            widgets: [
                { id: 'small-month', type: OverviewWidgetType.CurrentMonthOverview, settings: { height: 1, lightBackgroundColor: '112233', darkBackgroundColor: 'abcdef' } },
                { id: 'invalid-month', type: OverviewWidgetType.CurrentMonthOverview, settings: { height: 99, lightBackgroundColor: 'invalid', darkBackgroundColor: '#ffffff' } },
                { id: 'selected-periods', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRanges: [DateRange.ThisYear.type, DateRange.Today.type] } },
                { id: 'empty-periods', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRanges: [] } }
            ]
        });

        expect(layout.widgets[0]?.settings).toEqual({ height: 1, lightBackgroundColor: '112233', darkBackgroundColor: 'abcdef' });
        expect(layout.widgets[1]?.settings).toEqual({ height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' });
        expect(layout.widgets[2]?.settings).toEqual({ dateRanges: [DateRange.ThisYear.type, DateRange.Today.type] });
        expect(layout.widgets[3]?.settings).toEqual({ dateRanges: [DateRange.Today.type, DateRange.ThisWeek.type, DateRange.ThisMonth.type, DateRange.ThisYear.type] });
    });

    test('round trips layout and collects registered data requirements', () => {
        const json = serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT, true);
        const layout = parseMobileOverviewLayout(json);

        expect(serializeMobileOverviewLayout(layout)).toBe(serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT));
        expect(getOverviewDataRequirements(layout, MOBILE_OVERVIEW_WIDGET_DEFINITIONS)).toEqual({[OverviewWidgetDataRequirement.TransactionOverview]: true});
    });

    test('empty setting uses a cloned default layout', () => {
        const layout = parseMobileOverviewLayout('');

        expect(isDefaultMobileOverviewLayout(layout)).toBe(true);
        expect(layout).not.toBe(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
        expect(layout.widgets).not.toBe(DEFAULT_MOBILE_OVERVIEW_LAYOUT.widgets);
    });
});

function makeContext(homeFilter: OverviewWidgetQueryContext['homeFilter']): OverviewWidgetQueryContext {
    const monthRanges = {} as Record<TransactionAmountsRequestType, { startTime: number; endTime: number }>;

    ALL_TRANSACTION_AMOUNTS_REQUEST_TYPE.forEach((type, index) => {
        monthRanges[type] = { startTime: 1000 + index * 10, endTime: 1009 + index * 10 };
    });

    return {
        allAccountIds: ['a1', 'a2', 'a3'],
        allCategoryIds: ['c1', 'c2'],
        homeFilter,
        todayLastUnixTime: 1767225599,
        firstDayOfWeek: 0,
        fiscalYearStart: 101,
        monthRanges
    };
}

const HOME_CTX = makeContext({ excludeAccountIds: ['a3'], excludeCategoryIds: ['c2'], tagFilter: '0:H' });
const EXTRA_CTX = makeContext(null);

function layoutOf(...widgets: { id: string; type: OverviewWidgetType | string; settings: Record<string, string | number | boolean | (string | number)[]> }[]): OverviewLayoutBase {
    return { widgets: widgets.map(w => ({ x: 0, y: 0, w: 6, h: 6, ...w })) } as unknown as OverviewLayoutBase;
}

describe('overview widget queries', () => {
    test('Home with no filters produces no queries', () => {
        expect(getOverviewWidgetAmountsQueries(DEFAULT_DESKTOP_OVERVIEW_LAYOUT, HOME_CTX)).toEqual({});
        expect(getOverviewWidgetStatisticsQueries(DEFAULT_DESKTOP_OVERVIEW_LAYOUT, HOME_CTX)).toEqual({});
    });

    test('period widget with own tag merges with home tag and home exclusions', () => {
        const layout = layoutOf({ id: 'p', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRange: DateRange.ThisMonth.type, filterTags: '0:T1', filterAccountIds: ['a1'] } });
        const query = getOverviewWidgetAmountsQueries(layout, HOME_CTX)['p'];

        expect(query?.tagFilter).toBe('0:H;0:T1');
        expect(query?.excludeAccountIds).toEqual(expect.arrayContaining(['a3', 'a2']));
        expect(query?.excludeAccountIds).not.toContain('a1');
        expect(query?.excludeCategoryIds).toEqual(['c2']);
        expect(query?.ranges.map(r => r.name)).toEqual(['current']);
    });

    test('same widget without home filter uses only its own tag', () => {
        const layout = layoutOf({ id: 'p', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRange: DateRange.ThisMonth.type, filterTags: '0:T1' } });
        const query = getOverviewWidgetAmountsQueries(layout, EXTRA_CTX)['p'];

        expect(query?.tagFilter).toBe('0:T1');
        expect(query?.excludeAccountIds).toEqual([]);
    });

    test('unfiltered period widget: Home keeps shared data, non-standard range or extra dashboard gets a query', () => {
        const mk = (dateRange: number) => layoutOf({ id: 'p', type: OverviewWidgetType.PeriodNetIncomeAndSavingsRate, settings: { dateRange } });

        expect(getOverviewWidgetAmountsQueries(mk(DateRange.ThisYear.type), HOME_CTX)['p']).toBeUndefined();
        expect(getOverviewWidgetAmountsQueries(mk(DateRange.LastMonth.type), HOME_CTX)['p']).toBeDefined();
        expect(getOverviewWidgetAmountsQueries(mk(DateRange.ThisYear.type), EXTRA_CTX)['p']).toBeDefined();
    });

    test('key figure with comparison gets current and compare ranges', () => {
        const layout = layoutOf({ id: 'k', type: OverviewWidgetType.KeyFigure, settings: { metric: 'net', dateRange: DateRange.ThisMonth.type, comparison: OverviewComparisonMode.PreviousPeriod } });
        const query = getOverviewWidgetAmountsQueries(layout, HOME_CTX)['k'];

        expect(query?.ranges.map(r => r.name)).toEqual(['current', 'compare']);
        expect(query!.ranges[1]!.endTime).toBeLessThan(query!.ranges[0]!.startTime);
    });

    test('key figure without comparison has only current range, and always gets a query', () => {
        const layout = layoutOf({ id: 'k', type: OverviewWidgetType.KeyFigure, settings: { metric: 'income', dateRange: DateRange.Today.type, comparison: OverviewComparisonMode.None } });

        expect(getOverviewWidgetAmountsQueries(layout, HOME_CTX)['k']?.ranges.map(r => r.name)).toEqual(['current']);
    });

    test('trend widget uses shared month names and boundaries only when filtered or off Home', () => {
        const layout = layoutOf({ id: 't', type: OverviewWidgetType.IncomeExpenseTrend, settings: { months: 3 } });

        expect(getOverviewWidgetAmountsQueries(layout, HOME_CTX)['t']).toBeUndefined();

        const query = getOverviewWidgetAmountsQueries(layout, EXTRA_CTX)['t'];

        expect(query?.ranges.map(r => r.name)).toEqual(['monthBeforeLastMonth', 'lastMonth', 'thisMonth']);
        expect(query?.ranges[2]).toEqual({ name: 'thisMonth', ...EXTRA_CTX.monthRanges.thisMonth });
    });

    test('category ranking gets a statistics query when tag filtered, fiscal range or off Home', () => {
        const mk = (dateRange: number, filterTags?: string) => layoutOf({ id: 'r', type: OverviewWidgetType.ExpenseCategoryRanking, settings: filterTags ? { dateRange, filterTags } : { dateRange } });

        expect(getOverviewWidgetStatisticsQueries(mk(DateRange.ThisMonth.type), HOME_CTX)['r']).toBeUndefined();
        expect(getOverviewWidgetStatisticsQueries(mk(DateRange.ThisFiscalYear.type), HOME_CTX)['r']).toBeDefined();
        expect(getOverviewWidgetStatisticsQueries(mk(DateRange.ThisMonth.type, '0:T1'), HOME_CTX)['r']?.tagFilter).toBe('0:H;0:T1');
        expect(getOverviewWidgetStatisticsQueries(mk(DateRange.ThisMonth.type), EXTRA_CTX)['r']?.tagFilter).toBe('');
    });
});

describe('extra dashboards', () => {
    const widget = { id: 'w1', type: OverviewWidgetType.AssetSummary, x: 0, y: 0, w: 6, h: 4, settings: {} };
    const entry = (id: string, name: string, widgets: unknown[] = [widget]) => ({ id, name, layout: { widgets } });

    test('invalid JSON yields empty list', () => {
        expect(parseDesktopOverviewExtraDashboards('not json')).toEqual([]);
        expect(parseDesktopOverviewExtraDashboards('')).toEqual([]);
        expect(parseDesktopOverviewExtraDashboards('{"a":1}')).toEqual([]);
    });

    test('drops duplicate ids, the home id and empty names', () => {
        const value = JSON.stringify([entry('a', 'First'), entry('a', 'Dup'), entry(OVERVIEW_HOME_DASHBOARD_ID, 'Home'), entry('b', '  '), entry('c', 'Valid')]);
        const result = parseDesktopOverviewExtraDashboards(value);

        expect(result.map(d => d.id)).toEqual(['a', 'c']);
    });

    test('trims and caps names at 40 characters', () => {
        const result = parseDesktopOverviewExtraDashboards(JSON.stringify([entry('a', '  ' + 'x'.repeat(60) + '  ')]));

        expect(result[0]?.name).toBe('x'.repeat(40));
    });

    test('keeps dashboard but drops widget with unknown type', () => {
        const result = parseDesktopOverviewExtraDashboards(JSON.stringify([entry('a', 'A', [widget, { ...widget, id: 'w2', type: 'no-such-widget' }])]));

        expect(result).toHaveLength(1);
        expect(result[0]?.layout.widgets.map(w => w.id)).toEqual(['w1']);
    });

    test('keeps only the first 20 dashboards', () => {
        const value = JSON.stringify(Array.from({ length: 25 }, (_, i) => entry('d' + i, 'D' + i)));
        const result = parseDesktopOverviewExtraDashboards(value);

        expect(result).toHaveLength(OVERVIEW_MAX_EXTRA_DASHBOARDS);
        expect(result[19]?.id).toBe('d19');
    });

    test('serialize returns empty string for empty list and round trips', () => {
        expect(serializeDesktopOverviewExtraDashboards([])).toBe('');
        expect(serializeMobileOverviewExtraDashboards([])).toBe('');

        const dashboards = parseDesktopOverviewExtraDashboards(JSON.stringify([entry('a', 'A')]));
        expect(parseDesktopOverviewExtraDashboards(serializeDesktopOverviewExtraDashboards(dashboards))).toEqual(dashboards);
    });

    test('mobile parse uses the mobile layout normaliser', () => {
        const mobileWidget = { id: 'w1', type: OverviewWidgetType.AssetSummary, settings: {} };
        const result = parseMobileOverviewExtraDashboards(JSON.stringify([entry('a', 'A', [mobileWidget])]));

        expect(result).toHaveLength(1);
        expect(parseMobileOverviewExtraDashboards(serializeMobileOverviewExtraDashboards(result))).toEqual(result);
    });

    test('resolveSelectedDashboardId falls back to home', () => {
        expect(resolveSelectedDashboardId('gone', [{ id: 'a' }])).toBe('home');
        expect(resolveSelectedDashboardId('a', [{ id: 'a' }])).toBe('a');
        expect(resolveSelectedDashboardId('home', [{ id: 'a' }])).toBe('home');
    });
});
