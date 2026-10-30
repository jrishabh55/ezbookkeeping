import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import {
    type InvoiceDisplayStatus,
    type InvoiceCurrencyAmount,
    getInvoiceStatus,
    getInvoiceDisplayStatus,
    getInvoiceOutstandingByCurrency,
    getInvoiceOverdueByCurrency,
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

    // invoices only: an estimate is never outstanding or overdue
    const allInvoices = computed<Invoice[]>(() => invoicesStore.allInvoices.filter(invoice => invoice.kind === 'invoice'));
    const displayInvoices = computed<Invoice[]>(() => showPaid.value ? allInvoices.value : allInvoices.value.filter(invoice => getInvoiceStatus(invoice) !== 'paid'));
    const noAvailableInvoice = computed<boolean>(() => displayInvoices.value.length < 1);

    // one line per currency, INR first
    const outstandingTotals = computed<InvoiceCurrencyAmount[]>(() => getInvoiceOutstandingByCurrency(allInvoices.value));
    const overdueTotals = computed<InvoiceCurrencyAmount[]>(() => getInvoiceOverdueByCurrency(allInvoices.value, today));

    function formatAmount(minorUnits: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), currency);
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
        outstandingTotals,
        overdueTotals,
        // functions
        formatAmount,
        formatDate,
        getDisplayStatus,
        getDisplayStatusName
    };
}
