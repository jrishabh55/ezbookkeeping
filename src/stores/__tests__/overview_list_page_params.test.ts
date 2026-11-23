import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

import { useSettingsStore } from '@/stores/setting.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useOverviewStore } from '@/stores/overview.ts';

import { TransactionType } from '@/core/transaction.ts';
import type { Account } from '@/models/account.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

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
        // Home filters: a tag filter only unless a test sets account/category exclusions
        useSettingsStore().appSettings.overviewTransactionTagFilterInHomePage = HOME_TAG_FILTER;
        useAccountsStore().allAccountsMap = Object.fromEntries(['a1', 'a2', 'a3'].map(id => [id, { id, parentId: '0' } as Account]));
        useTransactionCategoriesStore().allTransactionCategoriesMap = Object.fromEntries([
            ['c1', '0'], ['c2', '0'], ['c3', 'c2']
        ].map(([id, parentId]) => [id, { id, parentId } as TransactionCategory]));
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

    it('intersects the widget include ids with the Home exclusions on Home', () => {
        const settingsStore = useSettingsStore();
        settingsStore.appSettings.overviewAccountFilterInHomePage = { a2: true };
        settingsStore.appSettings.overviewTransactionCategoryFilterInHomePage = { c2: true };
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['a1', 'a2'], categoryIds: ['c1', 'c3'], tagFilter: '', ignoreHomeFilters: false }
        });

        // c3 is a sub category of the excluded c2
        expect(params).toBe('dateType=3&categoryIds=c1&accountIds=a1&tagFilter=' + HOME_TAG_FILTER);
    });

    it('keeps the widget include ids when the Home exclusions remove all of them', () => {
        useSettingsStore().appSettings.overviewAccountFilterInHomePage = { a2: true };
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['a2'], categoryIds: [], tagFilter: '', ignoreHomeFilters: false }
        });

        expect(params).toBe('dateType=3&accountIds=a2&tagFilter=' + HOME_TAG_FILTER);
    });

    it('does not intersect with the Home exclusions when ignoring Home filters', () => {
        useSettingsStore().appSettings.overviewAccountFilterInHomePage = { a2: true };
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['a1', 'a2'], categoryIds: [], tagFilter: '', ignoreHomeFilters: true }
        });

        expect(params).toBe('dateType=3&accountIds=a1,a2');
    });

    it('ignores widget include ids that match no account, like the totals query', () => {
        const store = useOverviewStore();
        const params = store.getTransactionListPageParams({
            dateType: 3,
            filter: { accountIds: ['gone'], categoryIds: ['c1', 'gone'], tagFilter: '', ignoreHomeFilters: true }
        });

        expect(params).toBe('dateType=3&categoryIds=c1');
    });
});
