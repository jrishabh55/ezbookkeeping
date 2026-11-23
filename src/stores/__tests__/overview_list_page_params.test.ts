import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

import { useSettingsStore } from '@/stores/setting.ts';
import { useOverviewStore } from '@/stores/overview.ts';

import { TransactionType } from '@/core/transaction.ts';

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

const HOME_TAG_FILTER = 'homeTag';

function emptyFilter(ignoreHomeFilters: boolean) {
    return { accountIds: [], categoryIds: [], tagFilter: '', ignoreHomeFilters };
}

describe('overview store transaction list page params', () => {
    beforeEach(() => {
        localStorage.clear();
        setActivePinia(createPinia());
        // Home filters: a tag filter only (account/category maps resolve against stores that are empty here)
        useSettingsStore().appSettings.overviewTransactionTagFilterInHomePage = HOME_TAG_FILTER;
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    it('keeps the Home filters when no widget filter is given', () => {
        const store = useOverviewStore();

        expect(store.getTransactionListPageParams({ dateType: 3 })).toBe('dateType=3&tagFilter=' + HOME_TAG_FILTER);
    });

    it('defaults activeDashboardIsHome to true', () => {
        expect(useOverviewStore().activeDashboardIsHome).toBe(true);
    });

    it('keeps the Home filters for an empty widget filter on Home', () => {
        const store = useOverviewStore();

        expect(store.getTransactionListPageParams({ dateType: 3, filter: emptyFilter(false) })).toBe('dateType=3&tagFilter=' + HOME_TAG_FILTER);
    });

    it('skips the Home filters when ignoreHomeFilters is set', () => {
        const store = useOverviewStore();

        expect(store.getTransactionListPageParams({ dateType: 3, filter: emptyFilter(true) })).toBe('dateType=3');
    });

    it('adds the widget include ids and tag filter without Home filters', () => {
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['a1', 'a2'], categoryIds: ['c1'], tagFilter: 'w', ignoreHomeFilters: true }
        });

        expect(params).toBe('dateType=3&categoryIds=c1&accountIds=a1,a2&tagFilter=w');
    });

    it('merges the widget tag filter with the Home tag filter on Home', () => {
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['a1'], categoryIds: [], tagFilter: 'w', ignoreHomeFilters: false }
        });

        expect(params).toBe('dateType=3&accountIds=a1&tagFilter=' + encodeURIComponent(HOME_TAG_FILTER + ';w'));
    });

    it('prefers explicit category ids over the widget category filter', () => {
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            type: TransactionType.Expense,
            dateType: 3,
            categoryIds: ['c9'],
            filter: { accountIds: [], categoryIds: ['c1'], tagFilter: '', ignoreHomeFilters: true }
        });

        expect(params).toBe(`type=${TransactionType.Expense}&dateType=3&categoryIds=c9`);
    });
});
