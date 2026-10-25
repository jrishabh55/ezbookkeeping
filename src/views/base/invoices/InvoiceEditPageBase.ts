import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Customer, type InvoiceProfile, Invoice, INVOICE_CURRENCY } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { computeInvoiceTotals, addDaysToDate, getTodayTextualDate } from '@/lib/invoice.ts';

export function useInvoiceEditPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const editInvoiceId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const invoice = ref<Invoice>(Invoice.createNewInvoice());

    const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

    const title = computed<string>(() => editInvoiceId.value ? 'Edit Invoice' : 'Add Invoice');
    const saveButtonTitle = computed<string>(() => editInvoiceId.value ? 'Save' : 'Add');

    const totals = computed<{ subtotal: number, total: number }>(() => computeInvoiceTotals(invoice.value.lines));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!invoice.value.customerId) {
            return 'Customer cannot be blank';
        } else if (!invoice.value.number.trim()) {
            return 'Invoice number cannot be blank';
        } else if (!invoice.value.invoiceDate || !invoice.value.dueDate) {
            return 'Invoice date cannot be blank';
        } else if (invoice.value.dueDate < invoice.value.invoiceDate) {
            return 'Due date cannot be before the invoice date';
        } else if (invoice.value.lines.length < 1) {
            return 'Invoice must have at least one line';
        }

        for (const line of invoice.value.lines) {
            if (!line.itemName.trim()) {
                return 'Item name cannot be blank';
            } else if (line.quantity < 1 || line.rate <= 0) {
                return 'Quantity and rate must be greater than zero';
            }
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function addLine(): void {
        invoice.value.lines.push(Invoice.createNewLine());
    }

    function removeLine(index: number): void {
        invoice.value.lines.splice(index, 1);
    }

    function setInvoice(source: Invoice): void {
        invoice.value = source.clone();
    }

    // a new invoice dated today with the profile's terms and notes, optionally copying another invoice
    function prepareNewInvoice(profile: InvoiceProfile, duplicateFrom?: Invoice | null): void {
        const newInvoice = duplicateFrom ? duplicateFrom.duplicate() : Invoice.createNewInvoice();
        const today = getTodayTextualDate();

        newInvoice.invoiceDate = today;
        newInvoice.dueDate = addDaysToDate(today, profile.defaultTermsDays);

        if (!duplicateFrom) {
            newInvoice.notes = profile.defaultNotes;
        }

        invoice.value = newInvoice;
    }

    function setInvoiceDate(date: TextualYearMonthDay, termsDays: number): void {
        invoice.value.invoiceDate = date;
        invoice.value.dueDate = addDaysToDate(date, termsDays);
    }

    function formatAmount(paise: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(paise), INVOICE_CURRENCY);
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        editInvoiceId,
        loading,
        submitting,
        invoice,
        // computed states
        allCustomers,
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        totals,
        // functions
        addLine,
        removeLine,
        setInvoice,
        prepareNewInvoice,
        setInvoiceDate,
        formatAmount,
        formatDate
    };
}
