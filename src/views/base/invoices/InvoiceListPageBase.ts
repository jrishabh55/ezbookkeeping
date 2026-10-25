import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    type InvoiceDisplayStatus,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    getInvoiceDisplayStatus,
    isInvoiceOverdue,
    getTodayTextualDate
} from '@/lib/invoice.ts';

export function getInvoiceDisplayStatusName(status: InvoiceDisplayStatus): string {
    const names: Record<InvoiceDisplayStatus, string> = {
        unpaid: 'Unpaid',
        partial: 'Partially Paid',
        paid: 'Paid',
        overdue: 'Overdue'
    };

    return names[status];
}

export function useInvoiceListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const showPaid = ref<boolean>(true);
    const today = getTodayTextualDate();

    const allInvoices = computed<Invoice[]>(() => invoicesStore.allInvoices);
    const displayInvoices = computed<Invoice[]>(() => showPaid.value ? allInvoices.value : allInvoices.value.filter(invoice => getInvoiceStatus(invoice) !== 'paid'));
    const noAvailableInvoice = computed<boolean>(() => displayInvoices.value.length < 1);

    const outstandingTotal = computed<number>(() => allInvoices.value
        .filter(invoice => getInvoiceStatus(invoice) !== 'paid')
        .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

    const overdueTotal = computed<number>(() => allInvoices.value
        .filter(invoice => isInvoiceOverdue(invoice, today))
        .reduce((sum, invoice) => sum + getInvoiceBalanceDue(invoice), 0));

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    function getDisplayStatus(invoice: Invoice): InvoiceDisplayStatus {
        return getInvoiceDisplayStatus(invoice, today);
    }

    function getDisplayStatusName(invoice: Invoice): string {
        return getInvoiceDisplayStatusName(getDisplayStatus(invoice));
    }

    return {
        // states
        loading,
        showPaid,
        today,
        // computed states
        allInvoices,
        displayInvoices,
        noAvailableInvoice,
        outstandingTotal,
        overdueTotal,
        // functions
        formatAmount,
        formatDate,
        getDisplayStatus,
        getDisplayStatusName
    };
}
