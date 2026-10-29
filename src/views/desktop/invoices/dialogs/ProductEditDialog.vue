<template>
    <v-dialog width="640" :persistent="isProductModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="8">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Product Name')" :placeholder="tt('Product Name')"
                                          v-model="product.name" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('SAC')" :placeholder="tt('SAC')"
                                          v-model="product.sac" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="2"
                                        :disabled="loading || submitting"
                                        :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                        v-model="product.description" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <amount-input :disabled="loading || submitting" :persistent-placeholder="true"
                                          :currency="currency" :show-currency="true"
                                          :label="tt('Default Rate')" :placeholder="tt('Default Rate')"
                                          v-model="product.defaultRate" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-select persistent-placeholder :disabled="loading || submitting"
                                      :label="tt('Renews')" :items="allPeriods" item-title="displayName" item-value="months"
                                      :model-value="product.renewalMonths" @update:model-value="setRenewalMonths" />
                        </v-col>
                        <v-col cols="12" md="4">
                            <v-select persistent-placeholder :disabled="loading || submitting || !product.renewable"
                                      :label="tt('When Due')" :items="allModes" item-title="displayName" item-value="mode"
                                      v-model="product.defaultRenewalMode" />
                        </v-col>
                        <v-col cols="12" md="12" v-if="editProductId">
                            <v-switch density="compact" hide-details color="primary" :disabled="loading || submitting"
                                      :label="tt('Archived')" v-model="product.archived" />
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
import { useProductEditPageBase } from '@/views/base/invoices/ProductEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { Product } from '@/models/product.ts';

import { isEquals } from '@/lib/common.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

interface ProductEditResponse {
    message: string;
    product: Product;
}

const { tt } = useI18n();
const {
    editProductId,
    loading,
    submitting,
    product,
    title,
    saveButtonTitle,
    currency,
    allModes,
    allPeriods,
    inputEmptyProblemMessage,
    inputIsEmpty,
    setRenewalMonths
} = useProductEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: ProductEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const initProduct = ref<Product>(Product.createNewProduct());

const isProductModified = computed<boolean>(() => !isEquals(product.value.toCreateRequest(), initProduct.value.toCreateRequest()));

function open(options: { id?: string, currentProduct?: Product }): Promise<ProductEditResponse> {
    showState.value = true;
    loading.value = false;
    submitting.value = false;
    editProductId.value = options.id ?? null;
    initProduct.value = options.currentProduct ? options.currentProduct.clone() : Product.createNewProduct();
    product.value = initProduct.value.clone();

    invoicesStore.loadInvoiceProfile({ force: false }).catch(() => null);

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

    invoicesStore.saveProduct({ product: product.value, isEdit: !!editProductId.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: editProductId.value ? 'You have saved this product' : 'You have added a new product', product: saved });
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
