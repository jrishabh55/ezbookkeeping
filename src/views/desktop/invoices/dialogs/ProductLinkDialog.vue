<template>
    <v-dialog width="720" v-model="showState">
        <one-column-dialog-layout :disabled="linkingItemName !== ''" :loading="loading"
                                  :title="tt('Link Past Invoice Lines')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #content>
                <div class="text-body-medium mb-4">{{ tt('Past invoice lines without a product, grouped by item name. Pick the product each group is, then Link.') }}</div>
                <div class="text-body-medium" v-if="!loading && !groups.length">{{ tt('All past invoice lines are linked or skipped.') }}</div>
                <v-row class="align-center" :key="group.itemName" v-for="group in groups">
                    <v-col cols="12" md="5">
                        <div>{{ group.itemName }}</div>
                        <small class="text-medium-emphasis">{{ tt('format.misc.lineCount', { count: group.count }) }}</small>
                    </v-col>
                    <v-col cols="8" md="5">
                        <v-select density="compact" hide-details persistent-placeholder
                                  :label="tt('Product')" :items="activeProducts" item-title="name" item-value="id"
                                  v-model="selections[group.itemName]" />
                    </v-col>
                    <v-col cols="4" md="2">
                        <v-btn color="primary" variant="tonal" :disabled="!selections[group.itemName] || linkingItemName !== ''"
                               :loading="linkingItemName === group.itemName" @click="link(group)">{{ tt('Link') }}</v-btn>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceLineGroup, Product } from '@/models/product.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const showState = ref<boolean>(false);
const loading = ref<boolean>(false);
const groups = ref<InvoiceLineGroup[]>([]);
const selections = ref<Record<string, string>>({});
const linkingItemName = ref<string>('');

const activeProducts = computed<Product[]>(() => invoicesStore.allProducts.filter(product => !product.archived));

function open(): void {
    showState.value = true;
    loading.value = true;
    selections.value = {};

    Promise.all([invoicesStore.loadAllProducts({ force: false }), invoicesStore.getUnlinkedInvoiceLineGroups()]).then(([, loadedGroups]) => {
        groups.value = loadedGroups;
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function link(group: InvoiceLineGroup): void {
    const productId = selections.value[group.itemName];

    if (!productId) {
        return;
    }

    linkingItemName.value = group.itemName;

    invoicesStore.linkInvoiceLinesToProduct({ itemName: group.itemName, productId }).then(count => {
        linkingItemName.value = '';
        groups.value = groups.value.filter(item => item.itemName !== group.itemName);
        snackbar.value?.showMessage(tt('format.misc.linkedLinesCount', { count }));
    }).catch(error => {
        linkingItemName.value = '';

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>
