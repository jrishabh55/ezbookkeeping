import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';

import type { ForecastSummary } from '@/lib/forecast.ts';
import { summarise } from '@/lib/forecast.ts';
import { getCurrentDateTime } from '@/lib/datetime.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

export interface CommonForecastSummaryWidgetProps {
    title?: string;
}

// unlike the other overview widgets, this one does not share the dashboard's bulk-loaded data
// (transaction overview, asset trends, etc.); it fetches the current month's forecast on its own.
export function useForecastSummaryWidgetBase(props: CommonForecastSummaryWidgetProps) {
    const {
        tt,
        formatAmountToLocalizedNumeralsWithCurrency,
        formatGregorianTextualYearMonthDayToLongDate
    } = useI18n();

    const userStore = useUserStore();
    const accountsStore = useAccountsStore();

    const loading = ref<boolean>(true);
    const errorMessage = ref<string>('');
    const summary = ref<ForecastSummary | null>(null);
    const hasTemplateItems = ref<boolean>(false);

    const title = computed<string>(() => props.title || tt('Forecast'));
    const isEmpty = computed<boolean>(() => !loading.value && !errorMessage.value && !hasTemplateItems.value);

    function load(): Promise<void> {
        loading.value = true;
        errorMessage.value = '';

        const now = getCurrentDateTime();
        const startMonth = now.getGregorianCalendarYearDashMonth();
        const today = now.getGregorianCalendarYearDashMonthDashDay();

        return Promise.all([
            accountsStore.loadAllAccounts({ force: false }),
            services.getForecast({ startMonth: startMonth, months: 1 })
        ]).then(([, response]) => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                errorMessage.value = tt('Unable to retrieve forecast');
                summary.value = null;
                hasTemplateItems.value = false;
                return;
            }

            hasTemplateItems.value = data.result.items.some(item => item.source === 'template');
            summary.value = summarise(data.result, today);
        }).catch(error => {
            logger.error('failed to load forecast summary widget data', error);
            errorMessage.value = (error && error.message) || tt('Unable to retrieve forecast');
            summary.value = null;
            hasTemplateItems.value = false;
        }).finally(() => {
            loading.value = false;
        });
    }

    function getAccountName(accountId: string): string {
        return accountsStore.allAccountsMap[accountId]?.name ?? '';
    }

    function formatAmount(amount: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), userStore.currentUserDefaultCurrency);
    }

    function formatItemDate(date: string): string {
        return formatGregorianTextualYearMonthDayToLongDate(date as TextualYearMonthDay);
    }

    return {
        // states
        loading,
        errorMessage,
        summary,
        isEmpty,
        title,
        // functions
        load,
        getAccountName,
        formatAmount,
        formatItemDate
    };
}
