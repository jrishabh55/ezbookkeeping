import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { RecurringInvoice } from '@/models/invoice.ts';

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

    return {
        // states
        loading,
        // computed states
        allRecurringInvoices,
        noAvailableRecurringInvoice,
        // functions
        formatDate,
        getSourceNumber
    };
}
