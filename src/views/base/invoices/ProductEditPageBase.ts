import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { type RecurringInvoiceMode, RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS } from '@/models/invoice.ts';
import { Product } from '@/models/product.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { INVOICE_DEFAULT_CURRENCY } from '@/lib/invoice.ts';

// ponytail: products are priced in the profile's default currency; the currency column stays for a later per-product choice
export function useProductEditPageBase() {
    const { tt, formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const invoicesStore = useInvoicesStore();

    const editProductId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const product = ref<Product>(Product.createNewProduct());

    const title = computed<string>(() => editProductId.value ? 'Edit Product' : 'Add Product');
    const saveButtonTitle = computed<string>(() => editProductId.value ? 'Save' : 'Add');
    const currency = computed<string>(() => invoicesStore.invoiceProfile?.defaultCurrency || INVOICE_DEFAULT_CURRENCY);

    const allModes = computed<{ mode: RecurringInvoiceMode, displayName: string }[]>(() => RECURRING_INVOICE_MODES.map(item => ({ mode: item.mode, displayName: tt(item.name) })));
    const allPeriods = computed<{ months: number, displayName: string }[]>(() => [
        { months: 0, displayName: tt('One-off') },
        ...RECURRING_INVOICE_PERIODS.map(item => ({ months: item.months, displayName: tt(item.name) }))
    ]);
    const selectedModeName = computed<string>(() => allModes.value.find(item => item.mode === product.value.defaultRenewalMode)?.displayName ?? '');
    const selectedPeriodName = computed<string>(() => getPeriodName(product.value.renewalMonths));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!product.value.name.trim()) {
            return 'Product name cannot be blank';
        } else if (product.value.renewalMonths < 0 || product.value.renewalMonths > 120) {
            return 'Period must be between 1 and 120 months';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function getPeriodName(months: number): string {
        return allPeriods.value.find(item => item.months === months)?.displayName ?? tt('format.misc.everyNMonths', { count: months });
    }

    // a renewable product needs a mode; draft ahead is the default
    function setRenewalMonths(months: number): void {
        product.value.renewalMonths = months;

        if (months > 0 && !product.value.defaultRenewalMode) {
            product.value.defaultRenewalMode = 'draft_ahead';
        }
    }

    // "Every year · Draft ahead, I send it" or "One-off"
    function getProductSummary(item: Product): string {
        if (!item.renewable) {
            return tt('One-off');
        }

        const mode = RECURRING_INVOICE_MODES.find(entry => entry.mode === item.defaultRenewalMode);
        return mode ? `${getPeriodName(item.renewalMonths)} · ${tt(mode.name)}` : getPeriodName(item.renewalMonths);
    }

    function formatRate(item: Product): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(item.defaultRate), item.currency || currency.value);
    }

    return {
        // states
        editProductId,
        loading,
        submitting,
        product,
        // computed states
        title,
        saveButtonTitle,
        currency,
        allModes,
        allPeriods,
        selectedModeName,
        selectedPeriodName,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        setRenewalMonths,
        getProductSummary,
        formatRate
    };
}
