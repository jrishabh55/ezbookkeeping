import { ref } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, InvoiceRenewal } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { createInvoiceEmailSendRequest } from '@/lib/invoiceEmail.ts';

const STATE_NAMES: Record<InvoiceRenewal['state'], string> = {
    due: 'Renewal due',
    send_failed: 'Auto-send failed',
    auto_sent: 'Sent automatically'
};

// the home "Needs attention" card: renewal drafts, failed and recent auto-sends, and accepted estimates not fully invoiced
export function useRenewalsCardBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const renewals = ref<InvoiceRenewal[]>([]);
    const estimates = ref<Invoice[]>([]);
    const sendingId = ref<string>('');

    // renewals and accepted estimates load independently; a failing list just leaves its rows out
    function load(): Promise<void> {
        return Promise.all([
            invoicesStore.loadInvoiceRenewals().catch(() => [] as InvoiceRenewal[]),
            invoicesStore.loadEstimatesToInvoice().catch(() => [] as Invoice[])
        ]).then(([loadedRenewals, loadedEstimates]) => {
            renewals.value = loadedRenewals;
            estimates.value = loadedEstimates;
        });
    }

    function canSend(renewal: InvoiceRenewal): boolean {
        return renewal.state !== 'auto_sent' && !!renewal.invoice.billToEmail;
    }

    // one click: the saved email template to the customer's address, exactly as the send form would prefill it
    function send(renewal: InvoiceRenewal): Promise<void> {
        sendingId.value = renewal.invoice.id;

        return invoicesStore.loadInvoiceProfile({ force: false })
            .then(profile => invoicesStore.sendInvoice(createInvoiceEmailSendRequest(renewal.invoice, profile)))
            .then(() => load())
            .finally(() => {
                sendingId.value = '';
            });
    }

    function getStateName(renewal: InvoiceRenewal): string {
        return tt(STATE_NAMES[renewal.state]);
    }

    function getEstimateTitle(estimate: Invoice): string {
        return tt('format.misc.estimateAcceptedBy', { customer: estimate.billToName });
    }

    function formatAmount(invoice: Invoice): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(invoice.total), invoice.currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        renewals,
        estimates,
        sendingId,
        // functions
        load,
        canSend,
        send,
        getStateName,
        getEstimateTitle,
        formatAmount,
        formatDate
    };
}
