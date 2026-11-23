import type { BigDecimal } from '@/core/numeral.ts';
import type { StartEndTime } from '@/core/datetime.ts';
import { DateRange } from '@/core/datetime.ts';
import {
    type OverviewWidgetSettingValue,
    type OverviewWidgetFilter,
    type OverviewWidgetAmountsQuery,
    type OverviewWidgetStatisticsQuery,
    OverviewKeyFigureMetric,
    OverviewComparisonMode
} from '@/core/overview_layout.ts';

import { getUnixTimeBeforeUnixTime } from './datetime.ts';

type ComparisonUnit = 'days' | 'weeks' | 'months' | 'years';

function toStringArray(value: OverviewWidgetSettingValue | undefined): string[] {
    if (!Array.isArray(value)) {
        return [];
    }

    return value.filter(item => typeof item === 'string' || typeof item === 'number').map(item => String(item));
}

export function getWidgetFilter(settings: Record<string, OverviewWidgetSettingValue>): OverviewWidgetFilter {
    const tags = settings ? settings['filterTags'] : undefined;

    return {
        accountIds: toStringArray(settings ? settings['filterAccountIds'] : undefined),
        categoryIds: toStringArray(settings ? settings['filterCategoryIds'] : undefined),
        tagFilter: typeof tags === 'string' ? tags : ''
    };
}

export function isWidgetFilterEmpty(filter: OverviewWidgetFilter): boolean {
    return filter.accountIds.length === 0 && filter.categoryIds.length === 0 && !filter.tagFilter;
}

export function getExcludedIds(includeIds: string[], allIds: string[]): string[] {
    const included = new Set(includeIds);

    if (!allIds.some(id => included.has(id))) {
        return [];
    }

    return allIds.filter(id => !included.has(id));
}

// The ids a transaction list link uses for a widget's include filter, matching the widget's totals query:
// include ids no item has are ignored (an include with no known id is no filter, like getExcludedIds), and ids
// the Home filter excludes are left out. A link cannot express "nothing", so when every known include id is
// excluded the known include ids are used as given. Returns [] when the include filter does not apply.
export function getWidgetLinkIncludeIds(includeIds: string[], allIds: Iterable<string>, isExcluded: (id: string) => boolean): string[] {
    const all = new Set(allIds);
    const known = includeIds.filter(id => all.has(id));
    const remaining = known.filter(id => !isExcluded(id));

    return remaining.length ? remaining : known;
}

export function mergeTagFilters(...filters: string[]): string {
    return filters.filter(f => !!f).join(';');
}

export function getComparisonRange(current: StartEndTime, todayLastUnixTime: number, unit: ComparisonUnit, mode: OverviewComparisonMode): StartEndTime | null {
    if (mode !== OverviewComparisonMode.PreviousPeriod && mode !== OverviewComparisonMode.SamePeriodLastYear) {
        return null;
    }

    const inProgress = current.endTime > todayLastUnixTime;
    const currentEnd = inProgress ? todayLastUnixTime : current.endTime;
    const shiftUnit: ComparisonUnit = mode === OverviewComparisonMode.PreviousPeriod ? unit : 'years';
    const startTime = getUnixTimeBeforeUnixTime(current.startTime, 1, shiftUnit);
    let endTime = inProgress
        ? getUnixTimeBeforeUnixTime(currentEnd, 1, shiftUnit)
        : getUnixTimeBeforeUnixTime(current.endTime + 1, 1, shiftUnit) - 1;

    if (mode === OverviewComparisonMode.PreviousPeriod) {
        endTime = Math.min(endTime, current.startTime - 1);
    }

    return { startTime, endTime };
}

export function getComparisonUnit(dateType: number): ComparisonUnit | null {
    switch (dateType) {
        case DateRange.Today.type:
            return 'days';
        case DateRange.ThisWeek.type:
            return 'weeks';
        case DateRange.ThisMonth.type:
        case DateRange.LastMonth.type:
            return 'months';
        case DateRange.ThisYear.type:
        case DateRange.ThisFiscalYear.type:
        case DateRange.LastFiscalYear.type:
            return 'years';
        default:
            return null;
    }
}

export function getKeyFigureValue(metric: OverviewKeyFigureMetric, income: BigDecimal, expense: BigDecimal): BigDecimal | null {
    switch (metric) {
        case OverviewKeyFigureMetric.Income:
            return income;
        case OverviewKeyFigureMetric.Expense:
            return expense;
        case OverviewKeyFigureMetric.Net:
            return income.subtract(expense);
        case OverviewKeyFigureMetric.Margin:
            if (income.isZero()) {
                return null;
            }
            return income.subtract(expense).divide(income).multiply(100);
        default:
            return null;
    }
}

export function getChangePercent(current: BigDecimal, previous: BigDecimal): number | null {
    if (previous.isZero()) {
        return null;
    }

    return current.subtract(previous).divide(previous.abs()).multiply(100).toDoubleNumber();
}

export function getWidgetQueryCacheKey(query: OverviewWidgetAmountsQuery | OverviewWidgetStatisticsQuery): string {
    if ('ranges' in query) {
        return JSON.stringify({
            ranges: query.ranges.map(r => ({ name: r.name, startTime: r.startTime, endTime: r.endTime })),
            excludeAccountIds: [...query.excludeAccountIds].sort(),
            excludeCategoryIds: [...query.excludeCategoryIds].sort(),
            tagFilter: query.tagFilter
        });
    }

    return JSON.stringify({
        startTime: query.startTime,
        endTime: query.endTime,
        tagFilter: query.tagFilter
    });
}
