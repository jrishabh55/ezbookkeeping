import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import moment from 'moment-timezone';

import { useUserStore } from '@/stores/user.ts';
import { useOverviewStore } from '@/stores/overview.ts';

import { EMPTY_USER_BASIC_INFO } from '@/models/user.ts';
import type { OverviewWidgetAmountsQuery, OverviewWidgetStatisticsQuery } from '@/core/overview_layout.ts';

import services from '@/lib/services.ts';

// The stores read window.location and localStorage when they are loaded and the test environment is node
vi.hoisted(() => {
    const data = new Map<string, string>();
    const storage = {
        getItem: (key: string) => data.has(key) ? data.get(key) as string : null,
        setItem: (key: string, value: string) => { data.set(key, String(value)); },
        removeItem: (key: string) => { data.delete(key); },
        clear: () => { data.clear(); },
        key: (index: number) => Array.from(data.keys())[index] ?? null,
        get length() { return data.size; }
    };

    Object.assign(globalThis, {
        window: { location: { pathname: '/', origin: 'http://localhost', href: 'http://localhost/' }, localStorage: storage, sessionStorage: storage },
        localStorage: storage,
        sessionStorage: storage
    });
});

function amountsQuery(startTime: number): OverviewWidgetAmountsQuery {
    return { ranges: [{ name: 'a', startTime, endTime: startTime + 99 }], excludeAccountIds: ['x'], excludeCategoryIds: [], tagFilter: '' };
}

function mockAmounts() {
    return vi.spyOn(services, 'getTransactionAmountsByRanges').mockResolvedValue({
        data: { success: true, result: { a: { startTime: 1, endTime: 2, amounts: [{ currency: 'INR', incomeAmount: '1000', expenseAmount: '400' }] } } }
    } as never);
}

describe('overview store widget data', () => {
    beforeEach(() => {
        moment.tz.setDefault('UTC');
        localStorage.clear();
        setActivePinia(createPinia());
        useUserStore().storeUserBasicInfo({ ...EMPTY_USER_BASIC_INFO, username: 'test', defaultCurrency: 'INR' });
    });

    afterEach(() => {
        vi.restoreAllMocks();
        moment.tz.setDefault();
    });

    it('shares one request between widgets with equal queries', async () => {
        const store = useOverviewStore();
        const spy = mockAmounts();

        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10), w2: amountsQuery(10) } });

        expect(spy).toHaveBeenCalledTimes(1);
        expect(spy.mock.calls[0]).toEqual([amountsQuery(10).ranges, ['x'], [], '', expect.any(Boolean)]);
        const a1 = store.getWidgetAmounts('w1');
        expect(a1?.['a']?.incomeAmount.toString()).toBe('1000');
        expect(a1?.['a']?.expenseAmount.toString()).toBe('400');
        expect(store.getWidgetAmounts('w2')).toEqual(a1);
        expect(store.getWidgetAmountsQuery('w1')).toEqual(amountsQuery(10));
    });

    it('returns undefined for a widget not in the latest queries', async () => {
        const store = useOverviewStore();
        mockAmounts();

        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10), w2: amountsQuery(10) } });
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });

        expect(store.getWidgetAmounts('w1')).toBeDefined();
        expect(store.getWidgetAmounts('w2')).toBeUndefined();
    });

    it('reloads only when forced or invalidated', async () => {
        const store = useOverviewStore();
        const spy = mockAmounts();

        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        expect(spy).toHaveBeenCalledTimes(1);

        store.updateTransactionOverviewInvalidState(true);
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        expect(spy).toHaveBeenCalledTimes(2);

        await store.loadWidgetAmounts({ force: true, queries: { w1: amountsQuery(10) } });
        expect(spy).toHaveBeenCalledTimes(3);
    });

    it('drops cache keys that the latest call did not request', async () => {
        const store = useOverviewStore();
        const spy = mockAmounts();

        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(500) } });
        expect(Object.keys(store.widgetAmountsData)).toHaveLength(1);

        // The first range is no longer cached, so asking for it again requests it again
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        expect(spy).toHaveBeenCalledTimes(3);
    });

    it('makes no request and clears widgets when there are no queries', async () => {
        const store = useOverviewStore();
        const amountsSpy = mockAmounts();
        const statsSpy = vi.spyOn(services, 'getTransactionStatistics').mockResolvedValue({ data: { success: true, result: { items: [] } } } as never);

        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        await store.loadWidgetCategoryStatistics({ force: false, queries: { w1: { startTime: 1, endTime: 2, tagFilter: '' } } });
        expect(store.getWidgetAmounts('w1')).toBeDefined();
        expect(store.getWidgetCategoryStatistics('w1')).toBeDefined();
        amountsSpy.mockClear();
        statsSpy.mockClear();

        await store.loadWidgetAmounts({ force: true, queries: {} });
        await store.loadWidgetCategoryStatistics({ force: true, queries: {} });

        expect(amountsSpy).not.toHaveBeenCalled();
        expect(statsSpy).not.toHaveBeenCalled();
        expect(store.getWidgetAmounts('w1')).toBeUndefined();
        expect(store.getWidgetCategoryStatistics('w1')).toBeUndefined();
    });

    it('rejects when the amounts request fails', async () => {
        const store = useOverviewStore();
        vi.spyOn(services, 'getTransactionAmountsByRanges').mockRejectedValue({ response: { data: { errorMessage: 'boom' } } });

        await expect(store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } })).rejects.toEqual({ error: { errorMessage: 'boom' } });
    });

    it('loads and shares category statistics per query', async () => {
        const store = useOverviewStore();
        const result = { startTime: 1, endTime: 2, items: [] };
        const spy = vi.spyOn(services, 'getTransactionStatistics').mockResolvedValue({ data: { success: true, result } } as never);
        const query: OverviewWidgetStatisticsQuery = { startTime: 1, endTime: 2, tagFilter: 'a' };

        await store.loadWidgetCategoryStatistics({ force: false, queries: { w1: query, w2: { ...query } } });

        expect(spy).toHaveBeenCalledTimes(1);
        expect(spy.mock.calls[0]?.[0]).toMatchObject({ startTime: 1, endTime: 2, tagFilter: 'a', keyword: '' });
        expect(store.getWidgetCategoryStatistics('w1')).toEqual(result);
        expect(store.getWidgetCategoryStatistics('w2')).toEqual(result);

        store.updateTransactionOverviewInvalidState(true);
        await store.loadWidgetCategoryStatistics({ force: false, queries: { w1: query } });
        expect(spy).toHaveBeenCalledTimes(2);
    });

    it('ignores a slow amounts response that resolves after a newer call', async () => {
        const store = useOverviewStore();
        const ok = (income: string) => ({ data: { success: true, result: { a: { startTime: 1, endTime: 2, amounts: [{ currency: 'INR', incomeAmount: income, expenseAmount: '0' }] } } } });
        let resolveSlow: (v: unknown) => void = () => {};
        const spy = vi.spyOn(services, 'getTransactionAmountsByRanges')
            .mockImplementationOnce(() => new Promise(resolve => { resolveSlow = resolve; }) as never)
            .mockResolvedValueOnce(ok('7') as never);

        const slow = store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(500) } });
        resolveSlow(ok('1'));
        await slow;

        expect(spy).toHaveBeenCalledTimes(2);
        expect(Object.keys(store.widgetAmountsData)).toHaveLength(1);
        expect(store.getWidgetAmounts('w1')?.['a']?.incomeAmount.toString()).toBe('7');
    });

    it('ignores a slow statistics response that resolves after a newer call', async () => {
        const store = useOverviewStore();
        let resolveSlow: (v: unknown) => void = () => {};
        vi.spyOn(services, 'getTransactionStatistics')
            .mockImplementationOnce(() => new Promise(resolve => { resolveSlow = resolve; }) as never)
            .mockResolvedValueOnce({ data: { success: true, result: { items: ['new'] } } } as never);

        const slow = store.loadWidgetCategoryStatistics({ force: false, queries: { w1: { startTime: 1, endTime: 2, tagFilter: '' } } });
        await store.loadWidgetCategoryStatistics({ force: false, queries: { w1: { startTime: 3, endTime: 4, tagFilter: '' } } });
        resolveSlow({ data: { success: true, result: { items: ['old'] } } });
        await slow;

        expect(store.getWidgetCategoryStatistics('w1')).toEqual({ items: ['new'] });
    });

    it('keeps the invalid flag set when invalidated while a request is in flight', async () => {
        const store = useOverviewStore();
        let resolveSlow: (v: unknown) => void = () => {};
        const spy = vi.spyOn(services, 'getTransactionAmountsByRanges')
            .mockImplementationOnce(() => new Promise(resolve => { resolveSlow = resolve; }) as never)
            .mockResolvedValue({ data: { success: true, result: { a: { startTime: 1, endTime: 2, amounts: [] } } } } as never);

        const pending = store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });
        store.updateTransactionOverviewInvalidState(true);
        resolveSlow({ data: { success: true, result: { a: { startTime: 1, endTime: 2, amounts: [] } } } });
        await pending;
        await store.loadWidgetAmounts({ force: false, queries: { w1: amountsQuery(10) } });

        expect(spy).toHaveBeenCalledTimes(2);
    });
});
