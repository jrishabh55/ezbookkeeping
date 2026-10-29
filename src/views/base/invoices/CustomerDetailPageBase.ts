import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Customer, CustomerOverview, CustomerTimelineItem, Invoice, RecurringInvoice } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { type InvoiceCurrencyAmount, getInvoiceOutstandingByCurrency } from '@/lib/invoice.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';

export function useCustomerDetailPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate, formatDateTimeToLongDateTime } = useI18n();

    const invoicesStore = useInvoicesStore();

    const loading = ref<boolean>(true);
    const customer = ref<Customer | null>(null);
    const overview = ref<CustomerOverview>({ products: [], timeline: [] });

    const invoices = computed<Invoice[]>(() => customer.value ? invoicesStore.allInvoices.filter(invoice => invoice.customerId === customer.value?.id) : []);
    const recurrings = computed<RecurringInvoice[]>(() => customer.value ? invoicesStore.allRecurringInvoices.filter(recurring => recurring.customerId === customer.value?.id) : []);

    // one line per currency the customer was billed in
    const billedTotals = computed<InvoiceCurrencyAmount[]>(() => {
        const totals = new Map<string, number>();

        for (const invoice of invoices.value) {
            totals.set(invoice.currency, (totals.get(invoice.currency) ?? 0) + invoice.total);
        }

        return [...totals.entries()].map(([currency, amount]) => ({ currency, amount }));
    });

    const outstandingTotals = computed<InvoiceCurrencyAmount[]>(() => getInvoiceOutstandingByCurrency(invoices.value).filter(total => total.amount > 0));

    function load(customerId: string): Promise<void> {
        loading.value = true;

        return Promise.all([
            invoicesStore.loadAllCustomers({ force: false }),
            invoicesStore.loadAllInvoices({ force: false }),
            invoicesStore.loadAllRecurringInvoices({ force: false }),
            invoicesStore.getCustomerOverview({ customerId })
        ]).then(([, , , loadedOverview]) => {
            customer.value = invoicesStore.allCustomersMap[customerId] ?? null;
            overview.value = loadedOverview;
            loading.value = false;

            if (!customer.value) {
                throw { message: 'Unable to retrieve customer' };
            }
        });
    }

    function formatAmount(amount: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    function formatTime(unixTime: number): string {
        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTimeWithTimezoneOffset(unixTime, getTimezoneOffsetMinutes(unixTime)));
    }

    function getTimelineTitle(item: CustomerTimelineItem): string {
        switch (item.kind) {
            case 'email':
                return tt('format.misc.timelineEmail', { number: item.invoiceNumber, subject: item.text });
            case 'payment':
                return tt('format.misc.timelinePayment', { number: item.invoiceNumber });
            default:
                return tt(item.renewal ? 'format.misc.timelineRenewal' : 'format.misc.timelineInvoice', { number: item.invoiceNumber });
        }
    }

    return {
        // states
        loading,
        customer,
        overview,
        // computed states
        invoices,
        recurrings,
        billedTotals,
        outstandingTotals,
        // functions
        load,
        formatAmount,
        formatDate,
        formatTime,
        getTimelineTitle
    };
}
