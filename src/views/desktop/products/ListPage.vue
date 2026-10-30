<template>
    <main-page-layout no-navbar>
        <template #content>
            <v-row class="match-height">
                <v-col cols="12">
                    <v-card>
                        <template #title>
                            <div class="title-and-toolbar d-flex align-center">
                                <span>{{ tt('Products') }}</span>
                                <v-btn class="ms-3" color="default" variant="outlined"
                                       :disabled="loading || updating" @click="add">{{ tt('Add') }}</v-btn>
                                <v-btn density="compact" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('Refresh')" :icon="true" :disabled="loading || updating"
                                       :loading="loading" @click="reload(true)">
                                    <template #loader>
                                        <v-progress-circular indeterminate size="20"/>
                                    </template>
                                    <v-icon :icon="mdiRefresh" size="24" />
                                    <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                                </v-btn>
                                <v-spacer/>
                                <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('More')" :disabled="loading || updating" :icon="true">
                                    <v-icon :icon="mdiDotsVertical" />
                                    <v-menu activator="parent">
                                        <v-list>
                                            <v-list-item :prepend-icon="mdiLinkVariant" :title="tt('Link Past Invoice Lines')"
                                                         @click="productLinkDialog?.open()"></v-list-item>
                                        </v-list>
                                    </v-menu>
                                </v-btn>
                            </div>
                        </template>

                        <v-table class="table-striped" :hover="!loading">
                            <thead>
                            <tr>
                                <th>{{ tt('Product Name') }}</th>
                                <th>{{ tt('Renews') }}</th>
                                <th class="text-right">{{ tt('Default Rate') }}</th>
                                <th class="text-right">{{ tt('Operation') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="!loading && !allProducts.length">
                            <tr><td colspan="4">{{ tt('No available product') }}</td></tr>
                            </tbody>

                            <tbody>
                            <tr :key="product.id" v-for="product in allProducts"
                                @mouseover="hoveredProductId = product.id" @mouseleave="hoveredProductId = ''">
                                <td>
                                    <span :class="{ 'text-medium-emphasis': product.archived }">{{ product.name }}</span>
                                    <v-chip class="ms-2" size="x-small" variant="tonal" v-if="product.archived">{{ tt('Archived') }}</v-chip>
                                </td>
                                <td>{{ getProductSummary(product) }}</td>
                                <td class="text-right">{{ formatRate(product) }}</td>
                                <td class="text-right">
                                    <template v-if="hoveredProductId === product.id && !loading">
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                               @click="editProduct(product)">{{ tt('Edit') }}</v-btn>
                                        <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                               :prepend-icon="mdiDeleteOutline" :disabled="loading || updating"
                                               @click="removeProduct(product)">{{ tt('Delete') }}</v-btn>
                                    </template>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>
                    </v-card>
                </v-col>
            </v-row>
        </template>
    </main-page-layout>

    <product-edit-dialog ref="productEditDialog" />
    <product-link-dialog ref="productLinkDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue';

import {
    mdiRefresh,
    mdiDotsVertical,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiLinkVariant
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import ProductEditDialog from '../invoices/dialogs/ProductEditDialog.vue';
import ProductLinkDialog from '../invoices/dialogs/ProductLinkDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useProductEditPageBase } from '@/views/base/invoices/ProductEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Product } from '@/models/product.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type ProductEditDialogType = InstanceType<typeof ProductEditDialog>;
type ProductLinkDialogType = InstanceType<typeof ProductLinkDialog>;

const { tt } = useI18n();
const { getProductSummary, formatRate } = useProductEditPageBase();

const invoicesStore = useInvoicesStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const productEditDialog = useTemplateRef<ProductEditDialogType>('productEditDialog');
const productLinkDialog = useTemplateRef<ProductLinkDialogType>('productLinkDialog');

const loading = ref<boolean>(true);
const updating = ref<boolean>(false);
const hoveredProductId = ref<string>('');

const allProducts = computed<Product[]>(() => invoicesStore.allProducts);

function reload(force: boolean): void {
    loading.value = true;

    Promise.all([
        invoicesStore.loadAllProducts({ force: force }),
        invoicesStore.loadInvoiceProfile({ force: false })
    ]).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Product list has been updated');
        }
    }).catch(error => {
        loading.value = false;

        if (error.isUpToDate) {
            snackbar.value?.showMessage('Product list is up to date');
        } else if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    productEditDialog.value?.open({}).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function editProduct(product: Product): void {
    productEditDialog.value?.open({ id: product.id, currentProduct: product }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeProduct(product: Product): void {
    confirmDialog.value?.open('Are you sure you want to delete this product?').then(() => {
        updating.value = true;

        invoicesStore.deleteProduct({ product }).then(() => {
            updating.value = false;
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error); // "product is used by invoices, archive it instead"
            }
        });
    });
}

reload(false);
</script>
