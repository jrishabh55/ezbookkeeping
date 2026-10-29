<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Products')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="link" :aria-label="tt('Link Past Invoice Lines')" href="/invoice/product/link"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/product/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Product Name" footer="Every year · Draft ahead, I send it" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !products.length">
            <f7-list-item :title="tt('No available product')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && products.length">
            <f7-list-item swipeout
                          :id="getProductDomId(product)"
                          :title="product.name"
                          :footer="getProductSummary(product) + ' · ' + formatRate(product)"
                          :key="product.id"
                          v-for="product in products">
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(product)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(product, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this product?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(productToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';
import { useProductEditPageBase } from '@/views/base/invoices/ProductEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Product } from '@/models/product.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const { getProductSummary, formatRate } = useProductEditPageBase();

const invoicesStore = useInvoicesStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const productToDelete = ref<Product | null>(null);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());
const products = computed<Product[]>(() => invoicesStore.allProducts);

function getProductDomId(product: Product): string {
    return 'product_' + product.id;
}

function init(): void {
    loading.value = true;

    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.loadInvoiceProfile({ force: false })]).then(() => {
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

function reload(done?: () => void): void {
    const force = !!done;

    invoicesStore.loadAllProducts({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Product list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Product list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(product: Product): void {
    props.f7router.navigate(`/invoice/product/edit?id=${product.id}`);
}

function remove(product: Product | null, confirm: boolean): void {
    if (!product) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        productToDelete.value = product;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    productToDelete.value = null;
    showLoading();

    invoicesStore.deleteProduct({ product: product }).then(() => {
        onSwipeoutDeleted(getProductDomId(product), () => hideLoading());
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
