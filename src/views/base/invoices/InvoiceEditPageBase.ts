import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Customer, type InvoiceProfile, type InvoiceTaxType, type InvoiceLine, type InvoiceKind, Invoice, INVOICE_TAX_TYPES } from '@/models/invoice.ts';
import type { Product } from '@/models/product.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { type InvoiceTotals, INVOICE_DEFAULT_CURRENCY, computeInvoiceTotals, getInvoiceTaxRows, formatInvoiceTaxRate, addDaysToDate, getDaysBetweenDates, getTodayTextualDate } from '@/lib/invoice.ts';
import { filterProductSuggestions, applyProductToLine } from '@/lib/product.ts';
import { ESTIMATE_MAX_MILESTONES, ESTIMATE_MILESTONE_PERCENT_TOTAL, getMilestoneAmounts, getMilestonePercentTotal, getMilestonesProblem } from '@/lib/estimate.ts';

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
    const profileEstimateValidDays = ref<number>(30);

    const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);
    const allProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

    const isEstimate = computed<boolean>(() => invoice.value.kind === 'estimate');

    const title = computed<string>(() => {
        if (isEstimate.value) {
            return editInvoiceId.value ? 'Edit Estimate' : 'Add Estimate';
        }

        return editInvoiceId.value ? 'Edit Invoice' : 'Add Invoice';
    });

    const numberLabel = computed<string>(() => isEstimate.value ? 'Estimate Number' : 'Invoice Number');
    const dateLabel = computed<string>(() => isEstimate.value ? 'Estimate Date' : 'Invoice Date');
    const dueDateLabel = computed<string>(() => isEstimate.value ? 'Valid Until' : 'Payment Due');

    const saveButtonTitle = computed<string>(() => editInvoiceId.value ? 'Save' : 'Add');

    const totals = computed<InvoiceTotals>(() => computeInvoiceTotals(invoice.value.lines, invoice.value.taxType, invoice.value.taxRate));

    // what each milestone comes to out of the total incl. tax, as the Payment Terms table prints it
    const milestoneAmounts = computed<number[]>(() => getMilestoneAmounts(totals.value.total, invoice.value.milestones.map(milestone => milestone.percent)));
    const milestonePercentTotal = computed<number>(() => getMilestonePercentTotal(invoice.value.milestones));
    const milestonesProblem = computed<string | null>(() => isEstimate.value ? getMilestonesProblem(invoice.value.milestones) : null);

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
            return isEstimate.value ? 'Estimate number cannot be blank' : 'Invoice number cannot be blank';
        } else if (!invoice.value.invoiceDate || !invoice.value.dueDate) {
            return 'Invoice date cannot be blank';
        } else if (invoice.value.dueDate < invoice.value.invoiceDate) {
            return isEstimate.value ? 'Valid until cannot be before the estimate date' : 'Due date cannot be before the invoice date';
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

        if (milestonesProblem.value) {
            return milestonesProblem.value;
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function addLine(): void {
        invoice.value.lines.push(Invoice.createNewLine(profileDefaultSac.value));
    }

    function getProductSuggestions(query: string): Product[] {
        return filterProductSuggestions(invoicesStore.allProducts, query);
    }

    function selectProduct(line: InvoiceLine, product: Product): void {
        applyProductToLine(line, product);
    }

    // the catalog name of a line's product, also for archived products; '' when typed by hand
    function getLinkedProductName(line: InvoiceLine): string {
        return line.productId ? invoicesStore.allProductsMap[line.productId]?.name ?? '' : '';
    }

    function setProfileDefaults(profile: InvoiceProfile): void {
        profileTermsDays.value = profile.defaultTermsDays;
        profileDefaultSac.value = profile.defaultSac;
        profileEstimateValidDays.value = profile.estimateValidDays;
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

    // a new milestone gets whatever percent is still missing, so two clicks on a fresh estimate give a sensible 50 / 50 start
    function addMilestone(): void {
        if (invoice.value.milestones.length >= ESTIMATE_MAX_MILESTONES) {
            return;
        }

        invoice.value.milestones.push(Invoice.createNewMilestone(Math.max(ESTIMATE_MILESTONE_PERCENT_TOTAL - milestonePercentTotal.value, 0)));
    }

    function removeMilestone(index: number): void {
        invoice.value.milestones.splice(index, 1);
    }

    function setInvoice(source: Invoice, profile?: InvoiceProfile): void {
        if (profile) {
            setProfileDefaults(profile);
        }

        invoice.value = source.clone();
        lastSuggestedNumber.value = '';
    }

    // a new invoice (or estimate) dated today with the profile's terms (or validity) and notes, optionally copying another one
    function prepareNewInvoice(profile: InvoiceProfile, duplicateFrom?: Invoice | null, kind: InvoiceKind = 'invoice'): void {
        const newInvoice = duplicateFrom ? duplicateFrom.duplicate() : (kind === 'estimate' ? Invoice.createNewEstimate() : Invoice.createNewInvoice());
        const today = getTodayTextualDate();

        setProfileDefaults(profile);
        newInvoice.invoiceDate = today;
        newInvoice.dueDate = addDaysToDate(today, newInvoice.kind === 'estimate' ? profile.estimateValidDays : getTermsDays(newInvoice.customerId, profile.defaultTermsDays));
        applyCustomerTaxDefaults(newInvoice, newInvoice.customerId);

        if (!duplicateFrom) {
            newInvoice.currency = profile.defaultCurrency || INVOICE_DEFAULT_CURRENCY; // a duplicate keeps its source's currency
            newInvoice.notes = profile.defaultNotes;
            newInvoice.lines = [Invoice.createNewLine(profile.defaultSac)];
        }

        invoice.value = newInvoice;
        lastSuggestedNumber.value = '';
    }

    // the unsaved invoice an estimate turned into; its customer, currency and (for a milestone) amount are set by the server
    function prepareConvertedInvoice(profile: InvoiceProfile, draft: Invoice): void {
        setProfileDefaults(profile);

        const converted = draft.clone();
        converted.id = '';
        invoice.value = converted;
        lastSuggestedNumber.value = converted.number;
    }

    // the customer's own terms when set (above 0), otherwise the profile's
    function getTermsDays(customerId: string, defaultTermsDays: number): number {
        return invoicesStore.allCustomersMap[customerId]?.termsDays || defaultTermsDays;
    }

    // a new invoice applies the customer's or profile's terms; an existing one keeps its invoice-to-due gap
    function setInvoiceDate(date: TextualYearMonthDay, termsDays: number): void {
        const days = editInvoiceId.value && invoice.value.invoiceDate && invoice.value.dueDate
            ? getDaysBetweenDates(invoice.value.invoiceDate, invoice.value.dueDate)
            : (isEstimate.value ? profileEstimateValidDays.value : getTermsDays(invoice.value.customerId, termsDays));

        invoice.value.invoiceDate = date;
        invoice.value.dueDate = addDaysToDate(date, Math.max(days, 0));
    }

    // choosing a customer on a new invoice re-applies the due date from its terms and its tax defaults (an estimate keeps its validity);
    // an existing invoice, or one made from an estimate, is never changed silently
    watch(() => invoice.value.customerId, customerId => {
        if (editInvoiceId.value || invoice.value.estimateId || !invoice.value.invoiceDate) {
            return;
        }

        if (!isEstimate.value) {
            invoice.value.dueDate = addDaysToDate(invoice.value.invoiceDate, getTermsDays(customerId, profileTermsDays.value));
        }

        applyCustomerTaxDefaults(invoice.value, customerId);
    });

    // the suggested number replaces the number only while it is blank or still the last suggestion; resolves whether it was applied
    function suggestNumber(): Promise<boolean> {
        const numberAtRequest = invoice.value.number;

        if (numberAtRequest.trim() && numberAtRequest !== lastSuggestedNumber.value) {
            return Promise.resolve(false);
        }

        return invoicesStore.getNextInvoiceNumber({ invoiceDate: invoice.value.invoiceDate, kind: invoice.value.kind }).then(number => {
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
        allProducts,
        isEstimate,
        title,
        numberLabel,
        dateLabel,
        dueDateLabel,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        totals,
        taxRows,
        allTaxTypes,
        selectedTaxTypeName,
        taxRatePercent,
        milestoneAmounts,
        milestonePercentTotal,
        milestonesProblem,
        // functions
        addLine,
        getProductSuggestions,
        selectProduct,
        getLinkedProductName,
        removeLine,
        addMilestone,
        removeMilestone,
        setInvoice,
        prepareNewInvoice,
        prepareConvertedInvoice,
        setInvoiceDate,
        suggestNumber,
        formatAmount,
        formatDate
    };
}
