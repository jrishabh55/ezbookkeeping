import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';

import { createInvoiceEmailSendRequest } from '@/lib/invoiceEmail.ts';

import { startDownloadFile } from '@/lib/ui/common.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, InvoiceEmail, InvoicePayment, InvoiceProfile, InvoiceSendRequest, InvoiceRenewalOption } from '@/models/invoice.ts';
import { RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';
import {
    type InvoiceDisplayStatus,
    INVOICE_DEFAULT_CURRENCY,
    getInvoiceDisplayBalanceDue,
    getInvoiceShortfall,
    getInvoiceDisplayStatus,
    getInvoicePublicLinkUrl,
    getInvoicePdfFileName,
    getTodayTextualDate,
    printInvoiceDocument
} from '@/lib/invoice.ts';
import { type EstimateDisplayStatus, ESTIMATE_DISPLAY_STATUS_NAMES, getEstimateDisplayStatus, isEstimateLocked, getMilestoneAmounts } from '@/lib/estimate.ts';

import { getInvoiceDisplayStatusName } from './InvoiceListPageBase.ts';

export function useInvoiceDetailPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDate, formatDateTimeToLongDateTime } = useI18n();

    const accountsStore = useAccountsStore();
    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const updating = ref<boolean>(false);
    const downloadingPdf = ref<boolean>(false);
    const invoice = ref<Invoice | null>(null);
    const profile = ref<InvoiceProfile | null>(null);
    const suggestions = ref<InvoicePayment[]>([]);
    const viewedEmail = ref<InvoiceEmail | null>(null); // a sent email opened with its full content
    const loadingEmail = ref<boolean>(false);

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
    // "Sent on" date once the invoice was emailed, empty otherwise
    const sentOnText = computed<string>(() => invoice.value && invoice.value.sentUnixTime > 0 ? formatUnixTime(invoice.value.sentUnixTime) : '');

    const isEstimate = computed<boolean>(() => invoice.value?.kind === 'estimate');
    const estimateStatus = computed<EstimateDisplayStatus | null>(() => invoice.value && isEstimate.value ? getEstimateDisplayStatus(invoice.value, getTodayTextualDate()) : null);
    const estimateStatusName = computed<string>(() => estimateStatus.value ? ESTIMATE_DISPLAY_STATUS_NAMES[estimateStatus.value] : '');
    // an estimate is edited only while draft or sent and nothing is invoiced from it; Duplicate always works
    const canEdit = computed<boolean>(() => !!invoice.value && (!isEstimate.value || !isEstimateLocked(invoice.value)));
    // Mark accepted / declined from draft, sent or expired; Reopen an answered estimate while nothing is invoiced
    const canMarkResponse = computed<boolean>(() => isEstimate.value && (invoice.value?.estimateStatus === 'draft' || invoice.value?.estimateStatus === 'sent'));
    const canReopenEstimate = computed<boolean>(() => isEstimate.value && !!invoice.value && !invoice.value.convertedInvoices.length &&
        (invoice.value.estimateStatus === 'accepted' || invoice.value.estimateStatus === 'declined'));
    // converting is offered on accepted estimates, and on sent or expired ones after a confirm
    const canConvert = computed<boolean>(() => estimateStatus.value === 'accepted' || estimateStatus.value === 'sent' || estimateStatus.value === 'expired');
    const convertNeedsConfirm = computed<boolean>(() => estimateStatus.value !== 'accepted');
    const milestoneAmounts = computed<number[]>(() => invoice.value ? getMilestoneAmounts(invoice.value.total, invoice.value.milestones.map(milestone => milestone.percent)) : []);

    // "Accepted by the client on <date>" / "Marked declined on <date>"; empty until answered
    const estimateResponseText = computed<string>(() => {
        const current = invoice.value;

        if (!current || !isEstimate.value || !current.respondedUnixTime) {
            return '';
        }

        const date = formatUnixTime(current.respondedUnixTime);

        if (current.estimateStatus === 'accepted') {
            return tt(current.respondedByClient ? 'format.misc.estimateAcceptedByClient' : 'format.misc.estimateAcceptedByYou', { date });
        }

        return tt(current.respondedByClient ? 'format.misc.estimateDeclinedByClient' : 'format.misc.estimateDeclinedByYou', { date });
    });

    // in the invoice currency unless another one is given
    function formatAmount(minorUnits: number, currency?: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), currency ?? invoice.value?.currency ?? INVOICE_DEFAULT_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    function formatUnixTime(unixTime: number): string {
        return formatDateTimeToLongDate(parseDateTimeFromUnixTimeWithTimezoneOffset(unixTime, getTimezoneOffsetMinutes(unixTime)));
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

    // shown in the send form only while no schedule renews this invoice yet
    const renewalOption = computed<InvoiceRenewalOption | null>(() => {
        const option = invoice.value?.renewalOption;
        return option && !option.scheduled ? option : null;
    });

    const renewalOptionText = computed<string>(() => {
        const option = renewalOption.value;

        if (!option) {
            return '';
        }

        const period = RECURRING_INVOICE_PERIODS.find(item => item.months === option.periodMonths);
        const mode = RECURRING_INVOICE_MODES.find(item => item.mode === option.mode);

        return tt('format.misc.createRenewal', {
            period: period ? tt(period.name) : tt('format.misc.everyNMonths', { count: option.periodMonths }),
            mode: mode ? tt(mode.name) : option.mode
        });
    });

    // Each compose starts from unresolved saved templates; editing this request never mutates the profile.
    function buildSendRequest(): InvoiceSendRequest {
        if (!invoice.value) {
            return { id: '', to: '', subject: '', bodyHtml: '', message: '' };
        }

        // pre-ticked only on the first send: re-sending an old invoice should not start a schedule unless asked to
        const option = renewalOption.value;
        return { ...createInvoiceEmailSendRequest(invoice.value, profile.value), createRenewal: !!option && !option.mixedPeriods && invoice.value.sentUnixTime === 0 };
    }

    // Preserve the existing public-link creation flow before opening the actual-invoice preview.
    function formatEmailSentTime(email: InvoiceEmail): string {
        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTimeWithTimezoneOffset(email.sentUnixTime, getTimezoneOffsetMinutes(email.sentUnixTime)));
    }

    // fetches the HTML that was sent, so it can be shown exactly as the customer received it
    function viewEmail(email: InvoiceEmail): Promise<InvoiceEmail> {
        loadingEmail.value = true;

        return invoicesStore.getInvoiceEmail({ emailId: email.id }).then(full => {
            viewedEmail.value = full;
            return full;
        }).finally(() => {
            loadingEmail.value = false;
        });
    }

    function prepareSendRequest(): Promise<InvoiceSendRequest> {
        if (invoice.value?.publicToken) {
            return Promise.resolve(buildSendRequest());
        }

        return setPublicLink(true).then(() => buildSendRequest());
    }

    // emails the invoice; the detail keeps the stored sent time and public token afterwards
    function sendInvoice(req: InvoiceSendRequest): Promise<Invoice> {
        return invoicesStore.sendInvoice(req).then(sent => {
            if (invoice.value && invoice.value.id === sent.id) {
                invoice.value.publicToken = sent.publicToken;
                invoice.value.sentUnixTime = sent.sentUnixTime;
                invoice.value.emails = sent.emails;
                invoice.value.renewalOption = sent.renewalOption;
                invoice.value.estimateStatus = sent.estimateStatus;
            }

            return sent;
        });
    }

    // your own answer; nothing is emailed to the client
    function setEstimateResponse(status: 'accepted' | 'declined'): Promise<Invoice> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to save estimate' });
        }

        return invoicesStore.setEstimateResponse({ estimateId: current.id, status }).then(updated => {
            invoice.value = updated;
            return updated;
        });
    }

    function reopenEstimate(): Promise<Invoice> {
        const current = invoice.value;

        if (!current) {
            return Promise.reject({ message: 'Unable to save estimate' });
        }

        return invoicesStore.reopenEstimate({ estimateId: current.id }).then(updated => {
            invoice.value = updated;
            return updated;
        });
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

    async function downloadInvoicePdf(): Promise<void> {
        const current = invoice.value;
        if (!current || downloadingPdf.value) {
            return;
        }

        downloadingPdf.value = true;
        try {
            const pdf = await invoicesStore.getInvoicePdf(current.id);
            startDownloadFile(`${getInvoicePdfFileName(current.number, current.dueDate, current.kind)}.pdf`, pdf);
        } finally {
            downloadingPdf.value = false;
        }
    }

    return {
        // states
        loading,
        updating,
        downloadingPdf,
        invoice,
        profile,
        suggestions,
        viewedEmail,
        loadingEmail,
        // computed states
        displayStatus,
        displayStatusName,
        balanceDue,
        shortfall,
        shortfallName,
        publicLinkUrl,
        sentOnText,
        renewalOption,
        renewalOptionText,
        isEstimate,
        estimateStatus,
        estimateStatusName,
        estimateResponseText,
        canEdit,
        canMarkResponse,
        canReopenEstimate,
        canConvert,
        convertNeedsConfirm,
        milestoneAmounts,
        // functions
        formatAmount,
        formatDate,
        needsSettledAmount,
        formatPaymentAmount,
        formatPaymentDescription,
        formatEmailSentTime,
        viewEmail,
        printInvoice,
        setPublicLink,
        copyPublicLink,
        downloadInvoicePdf,
        buildSendRequest,
        prepareSendRequest,
        sendInvoice,
        setEstimateResponse,
        reopenEstimate
    };
}
