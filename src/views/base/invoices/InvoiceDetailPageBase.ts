import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, type InvoicePayment, type InvoiceProfile, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';
import {
    type InvoiceDisplayStatus,
    getInvoiceDisplayBalanceDue,
    getInvoiceShortfall,
    getInvoiceDisplayStatus,
    getInvoicePublicLinkUrl,
    getTodayTextualDate,
    printInvoiceDocument
} from '@/lib/invoice.ts';

import { getInvoiceDisplayStatusName } from './InvoiceListPageBase.ts';

export function useInvoiceDetailPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDate } = useI18n();

    const accountsStore = useAccountsStore();
    const invoicesStore = useInvoicesStore();

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
    const publicLinkUrl = computed<string>(() => invoice.value?.publicToken ? getInvoicePublicLinkUrl(invoice.value.publicToken) : '');

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

    function printInvoice(): void {
        if (invoice.value) {
            printInvoiceDocument(invoice.value);
        }
    }

    // enabled=true issues a fresh token (first share, or "Reset Link": the old URL stops working); false disables the link
    function setPublicLink(enabled: boolean): Promise<string> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to update public link' });
        }

        return invoicesStore.updateInvoicePublicLink({ invoiceId: current.id, enabled }).then(token => {
            current.publicToken = token;
            return token;
        });
    }

    // resolves false when the browser offers no clipboard (plain http, old WebViews); the page then shows the URL to copy by hand
    function copyPublicLink(): Promise<boolean> {
        if (!publicLinkUrl.value || !navigator.clipboard) {
            return Promise.resolve(false);
        }

        return navigator.clipboard.writeText(publicLinkUrl.value).then(() => true).catch(() => false);
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
        publicLinkUrl,
        // functions
        formatAmount,
        formatDate,
        formatPaymentDescription,
        printInvoice,
        setPublicLink,
        copyPublicLink
    };
}
