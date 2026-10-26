import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Customer, type InvoiceProfile, type InvoiceTaxType, Invoice, INVOICE_TAX_TYPES } from '@/models/invoice.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { type InvoiceTotals, INVOICE_DEFAULT_CURRENCY, computeInvoiceTotals, getInvoiceTaxRows, formatInvoiceTaxRate, addDaysToDate, getDaysBetweenDates, getTodayTextualDate } from '@/lib/invoice.ts';

export function useInvoiceEditPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const invoicesStore = useInvoicesStore();

    const editInvoiceId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const invoice = ref<Invoice>(Invoice.createNewInvoice());
    const lastSuggestedNumber = ref<string>('');
    const profileTermsDays = ref<number>(7);
    const profileDefaultSac = ref<string>('');

    const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

    const title = computed<string>(() => editInvoiceId.value ? 'Edit Invoice' : 'Add Invoice');
    const saveButtonTitle = computed<string>(() => editInvoiceId.value ? 'Save' : 'Add');

    const totals = computed<InvoiceTotals>(() => computeInvoiceTotals(invoice.value.lines, invoice.value.taxType, invoice.value.taxRate));

    // the tax lines shown under the subtotal, e.g. "IGST (18%)" or "CGST (9%)" + "SGST (9%)"
    const taxRows = computed<{ label: string, amount: number }[]>(() => getInvoiceTaxRows(invoice.value.taxType, invoice.value.taxRate, totals.value.taxAmount)
        .map(row => ({ label: `${tt(row.name)} (${formatInvoiceTaxRate(row.rate)})`, amount: row.amount })));

    const allTaxTypes = computed<{ type: InvoiceTaxType, displayName: string }[]>(() => INVOICE_TAX_TYPES.map(taxType => ({ type: taxType.type, displayName: tt(taxType.name) })));
    const selectedTaxTypeName = computed<string>(() => allTaxTypes.value.find(taxType => taxType.type === invoice.value.taxType)?.displayName ?? tt('None'));

    // the rate is edited in percent (two decimals) and stored in basis points
    const taxRatePercent = computed<number>({
        get: () => invoice.value.taxRate / 100,
        set: (percent: number) => invoice.value.taxRate = Math.round(percent * 100)
    });

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
        } else if (invoice.value.taxType !== 'none' && invoice.value.taxRate <= 0) {
            return 'Tax rate must be greater than zero';
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
        invoice.value.lines.push(Invoice.createNewLine(profileDefaultSac.value));
    }

    function setProfileDefaults(profile: InvoiceProfile): void {
        profileTermsDays.value = profile.defaultTermsDays;
        profileDefaultSac.value = profile.defaultSac;
    }

    // a new invoice takes its tax type and rate from the customer
    function applyCustomerTaxDefaults(target: Invoice, customerId: string): void {
        const customer = invoicesStore.allCustomersMap[customerId];
        target.taxType = customer?.taxType ?? 'none';
        target.taxRate = customer?.taxRate ?? 0;
    }

    function removeLine(index: number): void {
        invoice.value.lines.splice(index, 1);
    }

    function setInvoice(source: Invoice, profile?: InvoiceProfile): void {
        if (profile) {
            setProfileDefaults(profile);
        }

        invoice.value = source.clone();
        lastSuggestedNumber.value = '';
    }

    // a new invoice dated today with the profile's terms and notes, optionally copying another invoice
    function prepareNewInvoice(profile: InvoiceProfile, duplicateFrom?: Invoice | null): void {
        const newInvoice = duplicateFrom ? duplicateFrom.duplicate() : Invoice.createNewInvoice();
        const today = getTodayTextualDate();

        setProfileDefaults(profile);
        newInvoice.invoiceDate = today;
        newInvoice.dueDate = addDaysToDate(today, getTermsDays(newInvoice.customerId, profile.defaultTermsDays));
        applyCustomerTaxDefaults(newInvoice, newInvoice.customerId);

        if (!duplicateFrom) {
            newInvoice.currency = profile.defaultCurrency || INVOICE_DEFAULT_CURRENCY; // a duplicate keeps its source's currency
            newInvoice.notes = profile.defaultNotes;
            newInvoice.lines = [Invoice.createNewLine(profile.defaultSac)];
        }

        invoice.value = newInvoice;
        lastSuggestedNumber.value = '';
    }

    // the customer's own terms when set (above 0), otherwise the profile's
    function getTermsDays(customerId: string, defaultTermsDays: number): number {
        return invoicesStore.allCustomersMap[customerId]?.termsDays || defaultTermsDays;
    }

    // a new invoice applies the customer's or profile's terms; an existing one keeps its invoice-to-due gap
    function setInvoiceDate(date: TextualYearMonthDay, termsDays: number): void {
        const days = editInvoiceId.value && invoice.value.invoiceDate && invoice.value.dueDate
            ? getDaysBetweenDates(invoice.value.invoiceDate, invoice.value.dueDate)
            : getTermsDays(invoice.value.customerId, termsDays);

        invoice.value.invoiceDate = date;
        invoice.value.dueDate = addDaysToDate(date, Math.max(days, 0));
    }

    // choosing a customer on a new invoice re-applies the due date from its terms and its tax defaults; an existing invoice is never changed silently
    watch(() => invoice.value.customerId, customerId => {
        if (!editInvoiceId.value && invoice.value.invoiceDate) {
            invoice.value.dueDate = addDaysToDate(invoice.value.invoiceDate, getTermsDays(customerId, profileTermsDays.value));
            applyCustomerTaxDefaults(invoice.value, customerId);
        }
    });

    // the suggested number replaces the number only while it is blank or still the last suggestion; resolves whether it was applied
    function suggestNumber(): Promise<boolean> {
        const numberAtRequest = invoice.value.number;

        if (numberAtRequest.trim() && numberAtRequest !== lastSuggestedNumber.value) {
            return Promise.resolve(false);
        }

        return invoicesStore.getNextInvoiceNumber({ invoiceDate: invoice.value.invoiceDate }).then(number => {
            if (invoice.value.number !== numberAtRequest) {
                return false; // the user typed while the suggestion was loading
            }

            invoice.value.number = number;
            lastSuggestedNumber.value = number;
            return true;
        });
    }

    function formatAmount(minorUnits: number): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(minorUnits), invoice.value.currency);
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
        taxRows,
        allTaxTypes,
        selectedTaxTypeName,
        taxRatePercent,
        // functions
        addLine,
        removeLine,
        setInvoice,
        prepareNewInvoice,
        setInvoiceDate,
        suggestNumber,
        formatAmount,
        formatDate
    };
}
