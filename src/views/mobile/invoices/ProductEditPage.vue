<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list form strong inset dividers class="margin-vertical">
            <!-- list-number-input must go in #list (f7-list cannot scan it in the default slot), so every item goes there to keep the order -->
            <template #list>
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('Product Name')" :placeholder="tt('Product Name')"
                               v-model:value="product.name"></f7-list-input>
                <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                               :label="tt('Description')" :placeholder="tt('Description (optional)')"
                               v-textarea-auto-size v-model:value="product.description"></f7-list-input>
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('SAC')" :placeholder="tt('SAC')"
                               v-model:value="product.sac"></f7-list-input>
                <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                              :header="tt('Default Rate')" :title="formatRate(product)"
                              @click="showRateSheet = true">
                    <number-pad-sheet :min-value="0" :max-value="TRANSACTION_MAX_AMOUNT" :currency="currency"
                                      v-model:show="showRateSheet" v-model="product.defaultRate"></number-pad-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Renews')" :title="selectedPeriodName" @click="showPeriodSheet = true">
                    <list-item-selection-sheet value-type="item" key-field="months" value-field="months" title-field="displayName"
                                               :items="allPeriods" v-model:show="showPeriodSheet"
                                               :model-value="product.renewalMonths" @update:model-value="setRenewalMonths">
                    </list-item-selection-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('When Due')" :title="selectedModeName" @click="showModeSheet = true"
                              v-if="product.renewable">
                    <list-item-selection-sheet value-type="item" key-field="mode" value-field="mode" title-field="displayName"
                                               :items="allModes" v-model:show="showModeSheet" v-model="product.defaultRenewalMode">
                    </list-item-selection-sheet>
                </f7-list-item>
                <f7-list-item :title="tt('Archived')" v-if="editProductId">
                    <template #after>
                        <f7-toggle :checked="product.archived" @toggle:change="product.archived = $event"></f7-toggle>
                    </template>
                </f7-list-item>
            </template>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useProductEditPageBase } from '@/views/base/invoices/ProductEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editProductId,
    loading,
    submitting,
    product,
    title,
    currency,
    allModes,
    allPeriods,
    selectedModeName,
    selectedPeriodName,
    inputEmptyProblemMessage,
    inputIsEmpty,
    setRenewalMonths,
    formatRate
} = useProductEditPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const showRateSheet = ref<boolean>(false);
const showPeriodSheet = ref<boolean>(false);
const showModeSheet = ref<boolean>(false);

function init(): void {
    const productId = props.f7route.query['id'];

    loading.value = true;

    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.loadInvoiceProfile({ force: false })]).then(() => {
        if (productId) {
            editProductId.value = productId;
            const existing = invoicesStore.allProductsMap[productId];

            if (!existing) {
                loadingError.value = 'Unable to retrieve product';
                showToast('Unable to retrieve product');
                return;
            }

            product.value = existing.clone();
        }

        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    invoicesStore.saveProduct({ product: product.value, isEdit: !!editProductId.value }).then(() => {
        submitting.value = false;
        hideLoading();
        showToast(editProductId.value ? 'You have saved this product' : 'You have added a new product');
        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
