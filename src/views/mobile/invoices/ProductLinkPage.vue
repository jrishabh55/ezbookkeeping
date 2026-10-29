<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Link Past Invoice Lines')"></f7-nav-title>
        </f7-navbar>

        <f7-block class="margin-vertical"><small>{{ tt('Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.') }}</small></f7-block>

        <f7-list strong inset dividers class="skeleton-text" v-if="loading">
            <f7-list-item title="Item name" footer="3 lines" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers v-if="!loading && !groups.length">
            <f7-list-item :title="tt('All past invoice lines are linked or skipped.')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers v-if="!loading && groups.length">
            <f7-list-item link="#" :key="group.itemName" v-for="group in groups"
                          :title="group.itemName" :footer="tt('format.misc.lineCount', { count: group.count })"
                          @click="groupToLink = group; showProductPopup = true">
            </f7-list-item>
        </f7-list>

        <list-item-selection-popup value-type="item" key-field="id" value-field="id" title-field="name"
                                   :title="tt('Product')" :enable-filter="true"
                                   :filter-placeholder="tt('Product')" :filter-no-items-text="tt('No results')"
                                   :items="activeProducts" v-model:show="showProductPopup"
                                   :model-value="''" @update:model-value="link">
        </list-item-selection-popup>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceLineGroup, Product } from '@/models/product.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();

const invoicesStore = useInvoicesStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const groups = ref<InvoiceLineGroup[]>([]);
const groupToLink = ref<InvoiceLineGroup | null>(null);
const showProductPopup = ref<boolean>(false);

const activeProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

function init(): void {
    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.getUnlinkedInvoiceLineGroups()]).then(([, loadedGroups]) => {
        groups.value = loadedGroups;
        loading.value = false;
    }).catch(error => {
        loadingError.value = error;

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

// picking the product is the confirmation for that one group
function link(productId: string): void {
    const group = groupToLink.value;

    if (!group || !productId) {
        return;
    }

    showLoading();

    invoicesStore.linkInvoiceLinesToProduct({ itemName: group.itemName, productId }).then(count => {
        hideLoading();
        groups.value = groups.value.filter(item => item.itemName !== group.itemName);
        showToast(tt('format.misc.linkedLinesCount', { count }));
    }).catch(error => {
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
