import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { getTodayTextualDate } from '@/lib/invoice.ts';
import { type EstimateDisplayStatus, ESTIMATE_DISPLAY_STATUS_NAMES, getEstimateDisplayStatus, isEstimateClosed } from '@/lib/estimate.ts';

// the status chip colours of the desktop list and detail
export const ESTIMATE_STATUS_COLORS: Record<EstimateDisplayStatus, string> = {
    draft: 'default',
    sent: 'primary',
    accepted: 'success',
    declined: 'error',
    expired: 'warning',
    invoiced: 'secondary',
    superseded: 'grey'
};

// the estimate list: the latest version of each open estimate by default; declined, fully invoiced and superseded ones behind "Show closed"
export function useEstimateListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const showClosed = ref<boolean>(false);
    const today = getTodayTextualDate();

    const allEstimates = computed<Invoice[]>(() => invoicesStore.allEstimates);
    const displayEstimates = computed<Invoice[]>(() => showClosed.value ? allEstimates.value : allEstimates.value.filter(estimate => !isEstimateClosed(estimate)));
    const noAvailableEstimate = computed<boolean>(() => displayEstimates.value.length < 1);

    function getStatus(estimate: Invoice): EstimateDisplayStatus {
        return getEstimateDisplayStatus(estimate, today);
    }

    function getStatusName(estimate: Invoice): string {
        return ESTIMATE_DISPLAY_STATUS_NAMES[getStatus(estimate)];
    }

    function formatAmount(minorUnits: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        loading,
        showClosed,
        // computed states
        allEstimates,
        displayEstimates,
        noAvailableEstimate,
        // functions
        getStatus,
        getStatusName,
        formatAmount,
        formatDate
    };
}
