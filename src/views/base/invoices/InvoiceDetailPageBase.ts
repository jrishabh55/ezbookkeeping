import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';

import services from '@/lib/services.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, type InvoicePayment, type InvoiceProfile, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';
import {
    type InvoiceDisplayStatus,
    getInvoiceDisplayBalanceDue,
    getInvoiceShortfall,
    getInvoiceDisplayStatus,
    getInvoicePdfFileName,
    getTodayTextualDate
} from '@/lib/invoice.ts';

import { getInvoiceDisplayStatusName } from './InvoiceListPageBase.ts';

export function useInvoiceDetailPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDate } = useI18n();

    const accountsStore = useAccountsStore();

    const loading = ref<boolean>(true);
    const updating = ref<boolean>(false);
    const invoice = ref<Invoice | null>(null);
    const profile = ref<InvoiceProfile | null>(null);
    const suggestions = ref<InvoicePayment[]>([]);

    const displayStatus = computed<InvoiceDisplayStatus | null>(() => invoice.value ? getInvoiceDisplayStatus(invoice.value, getTodayTextualDate()) : null);
    const displayStatusName = computed<string>(() => displayStatus.value ? getInvoiceDisplayStatusName(displayStatus.value) : '');
    const balanceDue = computed<number>(() => invoice.value ? getInvoiceDisplayBalanceDue(invoice.value) : 0);
    const shortfall = computed<number>(() => invoice.value ? getInvoiceShortfall(invoice.value).amount : 0);
    const shortfallName = computed<string>(() => {
        const kind = invoice.value ? getInvoiceShortfall(invoice.value).kind : null;

        if (kind === 'bankCharges') {
            return 'Bank Charges';
        } else if (kind === 'writtenOff') {
            return 'Written Off';
        }

        return '';
    });

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    // "April 29, 2026 · PNB Savings · NEFT-QUBER TECH"
    function formatPaymentDescription(payment: InvoicePayment): string {
        const date = formatDateTimeToLongDate(parseDateTimeFromUnixTimeWithTimezoneOffset(payment.transactionTime, getTimezoneOffsetMinutes(payment.transactionTime)));
        const accountName = accountsStore.allAccountsMap[payment.accountId]?.name ?? '';
        return [date, accountName, payment.comment].filter(part => !!part).join(' · ');
    }

    // the browser suggests document.title as the PDF file name
    function printInvoice(): void {
        if (!invoice.value) {
            return;
        }

        const originalTitle = document.title;
        document.title = getInvoicePdfFileName(invoice.value.number, invoice.value.dueDate);
        window.addEventListener('afterprint', () => {
            document.title = originalTitle;
        }, { once: true });
        window.print();
    }

    // the server-rendered PDF; the response is an attachment, so the new tab turns into a download
    function downloadInvoicePdf(): void {
        if (!invoice.value) {
            return;
        }

        window.open(services.getInvoicePdfUrlWithToken(invoice.value.id), '_blank');
    }

    return {
        // states
        loading,
        updating,
        invoice,
        profile,
        suggestions,
        // computed states
        displayStatus,
        displayStatusName,
        balanceDue,
        shortfall,
        shortfallName,
        // functions
        formatAmount,
        formatDate,
        formatPaymentDescription,
        printInvoice,
        downloadInvoicePdf
    };
}
