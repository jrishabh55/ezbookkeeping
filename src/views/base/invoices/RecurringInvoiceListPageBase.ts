import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS, type RecurringInvoice } from '@/models/invoice.ts';

export function useRecurringInvoiceListPageBase() {
    const { tt, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);

    const allRecurringInvoices = computed<RecurringInvoice[]>(() => invoicesStore.allRecurringInvoices);
    const noAvailableRecurringInvoice = computed<boolean>(() => allRecurringInvoices.value.length < 1);

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    // the source number is blank when the source invoice has been deleted; the row then never creates anything
    function getSourceNumber(recurring: RecurringInvoice): string {
        return recurring.sourceInvoiceNumber || tt('Deleted invoice');
    }

    // "Every year · Draft ahead, I send it"
    function getScheduleText(recurring: RecurringInvoice): string {
        const period = RECURRING_INVOICE_PERIODS.find(item => item.months === recurring.periodMonths);
        const mode = RECURRING_INVOICE_MODES.find(item => item.mode === recurring.mode);
        const periodText = period ? tt(period.name) : tt('format.misc.everyNMonths', { count: recurring.periodMonths });
        return mode ? `${periodText} · ${tt(mode.name)}` : periodText;
    }

    return {
        // states
        loading,
        // computed states
        allRecurringInvoices,
        noAvailableRecurringInvoice,
        // functions
        formatDate,
        getSourceNumber,
        getScheduleText
    };
}
