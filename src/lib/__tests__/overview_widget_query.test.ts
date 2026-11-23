import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import moment from 'moment-timezone';

import { DateRange } from '@/core/datetime.ts';
import { OverviewComparisonMode, OverviewKeyFigureMetric } from '@/core/overview_layout.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    getWidgetFilter,
    isWidgetFilterEmpty,
    getExcludedIds,
    mergeTagFilters,
    getComparisonRange,
    getComparisonUnit,
    getKeyFigureValue,
    getChangePercent,
    getWidgetQueryCacheKey
} from '@/lib/overview_widget_query.ts';

function unix(dateTime: string): number {
    return moment.tz(dateTime, 'YYYY-MM-DD HH:mm:ss', 'Asia/Kolkata').unix();
}

const TODAY_LAST = '2026-11-23 23:59:59';

describe('overview widget query helpers', () => {
    beforeEach(() => {
        moment.tz.setDefault('Asia/Kolkata');
    });

    afterEach(() => {
        moment.tz.setDefault();
    });

    describe('getComparisonRange', () => {
        function range(a: string, b: string) {
            return { startTime: unix(a), endTime: unix(b) };
        }

        it('compares an in-progress month with the previous month up to the same day', () => {
            expect(getComparisonRange(range('2026-11-01 00:00:00', '2026-11-30 23:59:59'), unix(TODAY_LAST), 'months', OverviewComparisonMode.PreviousPeriod))
                .toEqual(range('2026-10-01 00:00:00', '2026-10-23 23:59:59'));
        });

        it('compares an in-progress month with the same days last year', () => {
            expect(getComparisonRange(range('2026-11-01 00:00:00', '2026-11-30 23:59:59'), unix(TODAY_LAST), 'months', OverviewComparisonMode.SamePeriodLastYear))
                .toEqual(range('2025-11-01 00:00:00', '2025-11-23 23:59:59'));
        });

        it('compares an in-progress fiscal year with the previous year up to the same day', () => {
            expect(getComparisonRange(range('2026-04-01 00:00:00', '2027-03-31 23:59:59'), unix(TODAY_LAST), 'years', OverviewComparisonMode.PreviousPeriod))
                .toEqual(range('2025-04-01 00:00:00', '2025-11-23 23:59:59'));
        });

        it('compares a finished month with the whole previous month', () => {
            expect(getComparisonRange(range('2026-10-01 00:00:00', '2026-10-31 23:59:59'), unix(TODAY_LAST), 'months', OverviewComparisonMode.PreviousPeriod))
                .toEqual(range('2026-09-01 00:00:00', '2026-09-30 23:59:59'));
        });

        it('handles leap-year February for a finished month', () => {
            expect(getComparisonRange(range('2029-02-01 00:00:00', '2029-02-28 23:59:59'), unix('2030-01-01 23:59:59'), 'months', OverviewComparisonMode.SamePeriodLastYear))
                .toEqual(range('2028-02-01 00:00:00', '2028-02-29 23:59:59'));
        });

        it('clamps the end day to the previous month length', () => {
            expect(getComparisonRange(range('2026-03-01 00:00:00', '2026-03-31 23:59:59'), unix('2026-03-31 23:59:59'), 'months', OverviewComparisonMode.PreviousPeriod))
                .toEqual(range('2026-02-01 00:00:00', '2026-02-28 23:59:59'));
        });

        it('compares today with yesterday', () => {
            expect(getComparisonRange(range('2026-11-23 00:00:00', '2026-11-23 23:59:59'), unix(TODAY_LAST), 'days', OverviewComparisonMode.PreviousPeriod))
                .toEqual(range('2026-11-22 00:00:00', '2026-11-22 23:59:59'));
        });

        it('returns null when comparison is off', () => {
            expect(getComparisonRange(range('2026-11-01 00:00:00', '2026-11-30 23:59:59'), unix(TODAY_LAST), 'months', OverviewComparisonMode.None)).toBeNull();
        });
    });

    describe('getComparisonUnit', () => {
        it('maps date ranges to units', () => {
            expect(getComparisonUnit(DateRange.Today.type)).toBe('days');
            expect(getComparisonUnit(DateRange.ThisWeek.type)).toBe('weeks');
            expect(getComparisonUnit(DateRange.ThisMonth.type)).toBe('months');
            expect(getComparisonUnit(DateRange.LastMonth.type)).toBe('months');
            expect(getComparisonUnit(DateRange.ThisYear.type)).toBe('years');
            expect(getComparisonUnit(DateRange.ThisFiscalYear.type)).toBe('years');
            expect(getComparisonUnit(DateRange.LastFiscalYear.type)).toBe('years');
            expect(getComparisonUnit(DateRange.Custom.type)).toBeNull();
        });
    });

    describe('getExcludedIds', () => {
        it('returns the complement over all ids', () => {
            expect(getExcludedIds(['a'], ['a', 'b', 'c'])).toEqual(['b', 'c']);
        });

        it('returns empty when nothing is included', () => {
            expect(getExcludedIds([], ['a'])).toEqual([]);
        });

        it('returns empty when no included id exists any more', () => {
            expect(getExcludedIds(['gone'], ['a', 'b'])).toEqual([]);
        });
    });

    describe('mergeTagFilters', () => {
        it('joins non-empty parts', () => {
            expect(mergeTagFilters('', '0:1', '', '2:3')).toBe('0:1;2:3');
            expect(mergeTagFilters('', '')).toBe('');
        });
    });

    describe('getWidgetFilter', () => {
        it('reads settings and tolerates missing or wrongly typed values', () => {
            expect(getWidgetFilter({ filterAccountIds: ['1', '2'], filterCategoryIds: ['3'], filterTags: '0:1' }))
                .toEqual({ accountIds: ['1', '2'], categoryIds: ['3'], tagFilter: '0:1' });
            expect(getWidgetFilter({})).toEqual({ accountIds: [], categoryIds: [], tagFilter: '' });
            expect(getWidgetFilter({ filterAccountIds: 'x', filterCategoryIds: true, filterTags: 5 }))
                .toEqual({ accountIds: [], categoryIds: [], tagFilter: '' });
        });

        it('detects an empty filter', () => {
            expect(isWidgetFilterEmpty({ accountIds: [], categoryIds: [], tagFilter: '' })).toBe(true);
            expect(isWidgetFilterEmpty({ accountIds: [], categoryIds: ['1'], tagFilter: '' })).toBe(false);
            expect(isWidgetFilterEmpty({ accountIds: [], categoryIds: [], tagFilter: '0:1' })).toBe(false);
        });
    });

    describe('getKeyFigureValue', () => {
        const d = (n: number) => parseBigDecimal(n);

        it('computes each metric', () => {
            expect(getKeyFigureValue(OverviewKeyFigureMetric.Net, d(100), d(30))?.toDoubleNumber()).toBe(70);
            expect(getKeyFigureValue(OverviewKeyFigureMetric.Margin, d(200), d(150))?.toDoubleNumber()).toBe(25);
            expect(getKeyFigureValue(OverviewKeyFigureMetric.Margin, d(0), d(50))).toBeNull();
            expect(getKeyFigureValue(OverviewKeyFigureMetric.Income, d(200), d(150))?.toDoubleNumber()).toBe(200);
            expect(getKeyFigureValue(OverviewKeyFigureMetric.Expense, d(200), d(150))?.toDoubleNumber()).toBe(150);
        });
    });

    describe('getChangePercent', () => {
        const d = (n: number) => parseBigDecimal(n);

        it('computes percent change against the absolute previous value', () => {
            expect(getChangePercent(d(120), d(100))).toBe(20);
            expect(getChangePercent(d(-50), d(-100))).toBe(50);
            expect(getChangePercent(d(80), d(100))).toBe(-20);
            expect(getChangePercent(d(10), d(0))).toBeNull();
        });
    });

    describe('getWidgetQueryCacheKey', () => {
        it('ignores id order', () => {
            const ranges = [{ name: 'current', startTime: 1, endTime: 2 }];
            expect(getWidgetQueryCacheKey({ ranges, excludeAccountIds: ['a', 'b'], excludeCategoryIds: ['c', 'd'], tagFilter: 't' }))
                .toBe(getWidgetQueryCacheKey({ ranges, excludeAccountIds: ['b', 'a'], excludeCategoryIds: ['d', 'c'], tagFilter: 't' }));
        });

        it('differs for different statistics queries', () => {
            expect(getWidgetQueryCacheKey({ startTime: 1, endTime: 2, tagFilter: '' }))
                .not.toBe(getWidgetQueryCacheKey({ startTime: 1, endTime: 3, tagFilter: '' }));
        });
    });
});
