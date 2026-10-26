<template>
    <v-dialog width="640" :persistent="isCustomerModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Customer Name')" :placeholder="tt('Customer Name')"
                                          v-model="customer.name" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Contact Person')" :placeholder="tt('Contact Person (optional)')"
                                          v-model="customer.contactName" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="3"
                                        :disabled="loading || submitting"
                                        :label="tt('Address')" :placeholder="tt('Address')"
                                        v-model="customer.addressLines" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-text-field type="email" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Email')" :placeholder="tt('Email (optional)')"
                                          v-model="customer.email" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Phone')" :placeholder="tt('Phone')"
                                          v-model="customer.phone" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Website')" :placeholder="tt('Website')"
                                          v-model="customer.website" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('PAN')" :placeholder="tt('PAN')"
                                          v-model="customer.pan" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Account Number')" :placeholder="tt('Account Number')"
                                          v-model="customer.accountNumber" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('GSTIN')" :placeholder="tt('GSTIN')"
                                          v-model="customer.gstin" />
                        </v-col>
                        <v-col cols="6" md="3">
                            <v-select item-title="displayName" item-value="type" persistent-placeholder
                                      :disabled="loading || submitting"
                                      :label="tt('Tax')" :placeholder="tt('Tax')"
                                      :items="allTaxTypes"
                                      v-model="customer.taxType" />
                        </v-col>
                        <v-col cols="6" md="3">
                            <number-input :disabled="loading || submitting || customer.taxType === 'none'" persistent-placeholder
                                          :label="tt('Tax Rate (%)')" :placeholder="tt('Tax Rate (%)')"
                                          :min-value="0" :max-value="100" :max-decimal-count="2"
                                          v-model="taxRatePercent" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <number-input :disabled="loading || submitting" persistent-placeholder
                                          :label="tt('Payment Terms (days, 0 = default)')" :placeholder="tt('Payment Terms (days, 0 = default)')"
                                          :min-value="0" :max-value="365" :max-decimal-count="0"
                                          v-model="customer.termsDays" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="2"
                                        :disabled="loading || submitting"
                                        :label="tt('Internal Notes')" :placeholder="tt('Not printed on invoices')"
                                        v-model="customer.notes" />
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCustomerEditPageBase } from '@/views/base/invoices/CustomerEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { Customer } from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

interface CustomerEditResponse {
    message: string;
    customer: Customer;
}

const { tt } = useI18n();
const {
    editCustomerId,
    loading,
    submitting,
    customer,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allTaxTypes,
    taxRatePercent
} = useCustomerEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: CustomerEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const initCustomer = ref<Customer>(Customer.createNewCustomer());

const isCustomerModified = computed<boolean>(() => !isEquals(customer.value.toCreateRequest(), initCustomer.value.toCreateRequest()));

function open(options: { id?: string, currentCustomer?: Customer }): Promise<CustomerEditResponse> {
    showState.value = true;
    loading.value = false;
    submitting.value = false;
    editCustomerId.value = options.id ?? null;
    initCustomer.value = options.currentCustomer ? options.currentCustomer.clone() : Customer.createNewCustomer();
    customer.value = initCustomer.value.clone();

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    invoicesStore.saveCustomer({ customer: customer.value, isEdit: !!editCustomerId.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: editCustomerId.value ? 'You have saved this customer' : 'You have added a new customer', customer: saved });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
