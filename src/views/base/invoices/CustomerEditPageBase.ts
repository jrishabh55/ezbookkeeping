import { ref, computed } from 'vue';

import { Customer } from '@/models/invoice.ts';

export function useCustomerEditPageBase() {
    const editCustomerId = ref<string | null>(null);
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const customer = ref<Customer>(Customer.createNewCustomer());

    const title = computed<string>(() => editCustomerId.value ? 'Edit Customer' : 'Add Customer');
    const saveButtonTitle = computed<string>(() => editCustomerId.value ? 'Save' : 'Add');

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!customer.value.name.trim()) {
            return 'Customer name cannot be blank';
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
        inputIsEmpty
    };
}
