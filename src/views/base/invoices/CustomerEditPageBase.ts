import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { type InvoiceTaxType, Customer, INVOICE_TAX_TYPES } from '@/models/invoice.ts';

export function useCustomerEditPageBase() {
    const { tt } = useI18n();

    const editCustomerId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const customer = ref<Customer>(Customer.createNewCustomer());

    const title = computed<string>(() => editCustomerId.value ? 'Edit Customer' : 'Add Customer');
    const saveButtonTitle = computed<string>(() => editCustomerId.value ? 'Save' : 'Add');

    const allTaxTypes = computed<{ type: InvoiceTaxType, displayName: string }[]>(() => INVOICE_TAX_TYPES.map(taxType => ({ type: taxType.type, displayName: tt(taxType.name) })));
    const selectedTaxTypeName = computed<string>(() => allTaxTypes.value.find(taxType => taxType.type === customer.value.taxType)?.displayName ?? tt('None'));

    // the rate is edited in percent (two decimals) and stored in basis points
    const taxRatePercent = computed<number>({
        get: () => customer.value.taxRate / 100,
        set: (percent: number) => customer.value.taxRate = Math.round(percent * 100)
    });

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!customer.value.name.trim()) {
            return 'Customer name cannot be blank';
        } else if (customer.value.taxType !== 'none' && customer.value.taxRate <= 0) {
            return 'Tax rate must be greater than zero';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    return {
        // states
        editCustomerId,
        loading,
        submitting,
        customer,
        // computed states
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        allTaxTypes,
        selectedTaxTypeName,
        taxRatePercent
    };
}
