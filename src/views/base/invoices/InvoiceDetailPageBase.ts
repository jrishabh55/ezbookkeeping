import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';

import services from '@/lib/services.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, InvoicePayment, InvoiceProfile } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';
import {
    type InvoiceDisplayStatus,
    INVOICE_DEFAULT_CURRENCY,
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

    // in the invoice currency unless another one is given
    function formatAmount(minorUnits: number, currency?: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), currency ?? invoice.value?.currency ?? INVOICE_DEFAULT_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    // a suggestion in another currency needs the settled amount (in invoice currency) typed in before it can be linked
    function needsSettledAmount(payment: InvoicePayment): boolean {
        return !!invoice.value && payment.currency !== invoice.value.currency;
    }

    // what the payment counts for: the settled amount in invoice currency, otherwise the transaction amount in its own currency
    function formatPaymentAmount(payment: InvoicePayment): string {
        return payment.settledAmount > 0 ? formatAmount(payment.settledAmount) : formatAmount(payment.amount, payment.currency);
    }

    // "April 29, 2026 · PNB Savings · NEFT-QUBER TECH", led by the realised amount ("$4,400.00") when it is in another currency
    function formatPaymentDescription(payment: InvoicePayment): string {
        const realised = payment.settledAmount > 0 ? formatAmount(payment.amount, payment.currency) : '';
        const date = formatDateTimeToLongDate(parseDateTimeFromUnixTimeWithTimezoneOffset(payment.transactionTime, getTimezoneOffsetMinutes(payment.transactionTime)));
        const accountName = accountsStore.allAccountsMap[payment.accountId]?.name ?? '';
        return [realised, date, accountName, payment.comment].filter(part => !!part).join(' · ');
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
        publicLinkUrl,
        // functions
        formatAmount,
        formatDate,
        needsSettledAmount,
        formatPaymentAmount,
        formatPaymentDescription,
        printInvoice,
        setPublicLink,
        copyPublicLink
,
        downloadInvoicePdf
    };
}
