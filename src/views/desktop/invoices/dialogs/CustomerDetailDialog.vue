<template>
    <v-dialog width="1000" v-model="showState">
        <one-column-dialog-layout :loading="loading" :title="customer ? customer.name : tt('Customer Details')"
                                  :cancel-button-title="tt('Close')" @cancel="showState = false">
            <template #content>
                <v-row v-if="customer">
                    <v-col cols="12" md="5">
                        <div class="text-body-medium">
                            <div v-if="customer.contactName">{{ customer.contactName }}</div>
                            <div v-if="customer.email">{{ customer.email }}</div>
                            <div v-if="customer.phone">{{ customer.phone }}</div>
                            <div v-if="customer.gstin">{{ tt('GSTIN') }}: {{ customer.gstin }}</div>
                        </div>
                        <v-list density="compact" class="mt-2">
                            <v-list-item :title="tt('Billed')" :key="'billed-' + total.currency" v-for="total in billedTotals">
                                <template #append>{{ formatAmount(total.amount, total.currency) }}</template>
                            </v-list-item>
                            <v-list-item :title="tt('Paid')" :key="'paid-' + total.currency" v-for="total in paidTotals">
                                <template #append>{{ formatAmount(total.amount, total.currency) }}</template>
                            </v-list-item>
                            <v-list-item :title="tt('Outstanding')" :key="'due-' + total.currency" v-for="total in outstandingTotals">
                                <template #append><span class="font-weight-bold">{{ formatAmount(total.amount, total.currency) }}</span></template>
                            </v-list-item>
                        </v-list>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Products Bought') }}</div>
                        <v-list density="compact" v-if="overview.products.length">
                            <v-list-item :key="product.productId" v-for="product in overview.products" :title="product.name"
                                         :subtitle="tt('format.misc.timesBought', { count: product.timesBought, date: formatDate(product.lastBoughtDate) })" />
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No products bought yet.') }}</div>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Renewals') }}</div>
                        <div class="d-flex align-center px-4 pb-2" v-if="invoices.length">
                            <v-select density="compact" hide-details persistent-placeholder
                                      item-title="number" item-value="id"
                                      :label="tt('Invoice')" :placeholder="tt('Invoice')"
                                      :items="invoices" v-model="invoiceIdToRepeat" />
                            <v-btn class="ml-2" variant="tonal" :prepend-icon="mdiPlus"
                                   :disabled="!invoiceIdToRepeat" @click="addRecurring">{{ tt('Add') }}</v-btn>
                        </div>
                        <v-list density="compact" v-if="recurrings.length">
                            <v-list-item :key="recurring.id" v-for="recurring in recurrings" link
                                         :title="getScheduleText(recurring)"
                                         :subtitle="`${tt('Next Date')}: ${formatDate(recurring.nextDate)}`"
                                         @click="editRecurring(recurring)">
                                <template #append>
                                    <v-chip size="x-small" variant="tonal" v-if="!recurring.enabled">{{ tt('Paused') }}</v-chip>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No renewals yet.') }}</div>

                        <template v-if="customer.notes">
                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Internal Notes') }}</div>
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line">{{ customer.notes }}</div>
                        </template>
                    </v-col>

                    <v-col cols="12" md="7">
                        <div class="text-title-small px-4 py-2">{{ tt('Timeline') }}</div>
                        <v-list density="compact" v-if="overview.timeline.length">
                            <v-list-item :key="`${item.kind}-${item.invoiceId}-${item.unixTime}`" v-for="item in overview.timeline" link
                                         :title="getTimelineTitle(item)" :subtitle="formatTime(item.unixTime)"
                                         @click="viewInvoice(item.invoiceId)">
                                <template #append v-if="item.kind !== 'email'">
                                    <span :class="{ 'text-income': item.kind === 'payment' }">{{ formatAmount(item.amount, item.currency) }}</span>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('Nothing has happened yet.') }}</div>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <invoice-detail-dialog ref="invoiceDetailDialog" />
    <recurring-invoice-edit-dialog ref="recurringInvoiceEditDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';
import { mdiPlus } from '@mdi/js';

import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceDetailDialog from './InvoiceDetailDialog.vue';
import RecurringInvoiceEditDialog from './RecurringInvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCustomerDetailPageBase } from '@/views/base/invoices/CustomerDetailPageBase.ts';
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';

import type { RecurringInvoice } from '@/models/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceDetailDialogType = InstanceType<typeof InvoiceDetailDialog>;
type RecurringInvoiceEditDialogType = InstanceType<typeof RecurringInvoiceEditDialog>;

const { tt } = useI18n();
const { loading, customer, overview, invoices, recurrings, billedTotals, paidTotals, outstandingTotals, load, formatAmount, formatDate, formatTime, getTimelineTitle } = useCustomerDetailPageBase();
const { getScheduleText } = useRecurringInvoiceListPageBase();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceDetailDialog = useTemplateRef<InvoiceDetailDialogType>('invoiceDetailDialog');
const recurringInvoiceEditDialog = useTemplateRef<RecurringInvoiceEditDialogType>('recurringInvoiceEditDialog');

const showState = ref<boolean>(false);
const invoiceIdToRepeat = ref<string | null>(null);
let customerId = '';

function open(options: { id: string }): void {
    customerId = options.id;
    invoiceIdToRepeat.value = null;
    showState.value = true;
    reload();
}

function reload(): void {
    load(customerId).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function viewInvoice(invoiceId: string): void {
    invoiceDetailDialog.value?.open({ id: invoiceId }).then(result => {
        if (result.changed) {
            reload();
        }
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function editRecurring(recurring: RecurringInvoice): void {
    recurringInvoiceEditDialog.value?.open({ recurring }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

// a renewal repeats one of this customer's invoices
function addRecurring(): void {
    const invoice = invoices.value.find(item => item.id === invoiceIdToRepeat.value);

    if (!invoice) {
        return;
    }

    recurringInvoiceEditDialog.value?.open({ invoice }).then(result => {
        invoiceIdToRepeat.value = null;
        snackbar.value?.showMessage(result.message);
        reload();
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

defineExpose({
    open
});
</script>
