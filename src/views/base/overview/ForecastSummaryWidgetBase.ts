import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useOverviewStore } from '@/stores/overview.ts';
import { useTransactionTemplatesStore } from '@/stores/transactionTemplate.ts';

import { TemplateType } from '@/core/template.ts';

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
    const overviewStore = useOverviewStore();
    const transactionTemplatesStore = useTransactionTemplatesStore();

    const loading = ref<boolean>(true);
    const errorMessage = ref<string>('');
    const summary = ref<ForecastSummary | null>(null);
    const forecastTemplateCount = ref<number>(0);

    const title = computed<string>(() => props.title || tt('Forecast'));
    // empty only when the user has no forecast templates at all: once this month's templates are all
    // fulfilled the forecast still has month-end cash and shortfalls worth showing
    const isEmpty = computed<boolean>(() => !loading.value && !errorMessage.value && forecastTemplateCount.value === 0);

    function load(): Promise<void> {
        loading.value = true;
        errorMessage.value = '';

        const now = getCurrentDateTime();
        const startMonth = now.getGregorianCalendarYearDashMonth();
        const today = now.getGregorianCalendarYearDashMonthDashDay();

        return Promise.all([
            accountsStore.loadAllAccounts({ force: false }),
            transactionTemplatesStore.loadAllTemplates({ templateType: TemplateType.Forecast.type, force: false }),
            services.getForecast({ startMonth: startMonth, months: 1 })
        ]).then(([, templates, response]) => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                errorMessage.value = tt('Unable to retrieve forecast');
                summary.value = null;
                forecastTemplateCount.value = 0;
                return;
            }

            forecastTemplateCount.value = templates.length;
            summary.value = summarise(data.result, today);
        }).catch(error => {
            logger.error('failed to load forecast summary widget data', error);
            errorMessage.value = (error && error.message) || tt('Unable to retrieve forecast');
            summary.value = null;
            forecastTemplateCount.value = 0;
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

    // neither desktop nor mobile count this widget in the dashboard's dataRequirements, so its data is never
    // part of the shared Promise.all the home pages await; watching this counter (bumped by those home pages'
    // reload functions on initial load, manual refresh, pull-to-refresh and returning to the page) is what
    // makes the dashboard's "Refresh" action - and mobile's pull-to-refresh, which has no other hook - actually
    // refresh this widget too, on both platforms, from this one shared composable.
    watch(() => overviewStore.dashboardReloadCount, () => {
        load();
    });

    load();

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
