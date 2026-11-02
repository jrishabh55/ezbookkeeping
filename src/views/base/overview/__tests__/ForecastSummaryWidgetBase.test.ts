import { beforeEach, describe, expect, it, vi } from 'vitest';
import { reactive } from 'vue';

import type { ForecastResponse } from '@/models/forecast.ts';

const mocks = vi.hoisted(() => ({
    loadAllAccounts: vi.fn(),
    loadAllTemplates: vi.fn(),
    getForecast: vi.fn()
}));

vi.mock('@/locales/helpers.ts', () => ({
    useI18n: () => ({
        tt: (text: string) => text,
        formatAmountToLocalizedNumeralsWithCurrency: (amount: unknown) => String(amount),
        formatGregorianTextualYearMonthDayToLongDate: (date: string) => date
    })
}));
vi.mock('@/stores/user.ts', () => ({ useUserStore: () => ({ currentUserDefaultCurrency: 'INR' }) }));
vi.mock('@/stores/account.ts', () => ({ useAccountsStore: () => ({ loadAllAccounts: mocks.loadAllAccounts, allAccountsMap: {} }) }));
vi.mock('@/stores/overview.ts', () => {
    const store = reactive({ dashboardReloadCount: 0 });
    return { useOverviewStore: () => store };
});
vi.mock('@/stores/transactionTemplate.ts', () => ({ useTransactionTemplatesStore: () => ({ loadAllTemplates: mocks.loadAllTemplates }) }));
vi.mock('@/lib/services.ts', () => ({ default: { getForecast: mocks.getForecast } }));
vi.mock('@/lib/datetime.ts', () => ({
    getCurrentDateTime: () => ({
        getGregorianCalendarYearDashMonth: () => '2026-11',
        getGregorianCalendarYearDashMonthDashDay: () => '2026-11-28'
    })
}));
vi.mock('@/lib/logger.ts', () => ({ default: { error: vi.fn() } }));

import { useForecastSummaryWidgetBase } from '../ForecastSummaryWidgetBase.ts';

// late in the month every template item is fulfilled: only everyday spending and a shortfall remain
const lateMonthForecast: ForecastResponse = {
    months: [{ month: '2026-11', openingCash: 500, closingCash: 300, closingCashWithoutUncertain: 300, businessIncome: 0, businessCost: 0, personalCost: 200 }],
    items: [{ date: '2026-11-28', type: 1, accountId: '1', amount: 200, source: 'everyday', uncertain: false, business: false, label: 'Everyday spending' }],
    shortfalls: [{ accountId: '1', date: '2026-11-29', lowestBalance: -100, lowestDate: '2026-11-30' }],
    skippedCards: []
};

describe('useForecastSummaryWidgetBase', () => {
    beforeEach(() => {
        mocks.loadAllAccounts.mockResolvedValue([]);
        mocks.getForecast.mockResolvedValue({ data: { success: true, result: lateMonthForecast } });
    });

    it('is not empty when the user has forecast templates, even with no template items left this month', async () => {
        mocks.loadAllTemplates.mockResolvedValue([{ id: '1' }]);

        const widget = useForecastSummaryWidgetBase({});
        await widget.load();

        expect(mocks.loadAllTemplates).toHaveBeenCalledWith({ templateType: 3, force: false });
        expect(widget.isEmpty.value).toBe(false);
        expect(widget.summary.value?.closingCash).toBe(300);
    });

    it('is empty when the user has no forecast templates at all', async () => {
        mocks.loadAllTemplates.mockResolvedValue([]);

        const widget = useForecastSummaryWidgetBase({});
        await widget.load();

        expect(widget.isEmpty.value).toBe(true);
    });
});
