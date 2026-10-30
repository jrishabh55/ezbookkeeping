<template>
    <v-dialog width="1000" :persistent="isInvoiceModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="6">
                            <v-autocomplete item-title="name" item-value="id" persistent-placeholder
                                            :disabled="loading || submitting"
                                            :label="tt('Customer')" :placeholder="tt('Customer')"
                                            :items="allCustomers" :no-data-text="tt('No results')"
                                            v-model="invoice.customerId">
                                <template #append>
                                    <v-btn density="compact" color="default" variant="text" :icon="true"
                                           :aria-label="tt('Add Customer')" :disabled="loading || submitting" @click="addCustomer">
                                        <v-icon :icon="mdiAccountPlusOutline" />
                                        <v-tooltip activator="parent">{{ tt('Add Customer') }}</v-tooltip>
                                    </v-btn>
                                </template>
                            </v-autocomplete>
                        </v-col>
                        <v-col cols="12" md="3">
                            <v-text-field type="text" autocomplete="off" persistent-placeholder
                                          :disabled="loading || submitting"
                                          :label="tt('Invoice Number')" :placeholder="tt('Invoice Number')"
                                          v-model="invoice.number" />
                        </v-col>
                        <v-col cols="12" md="3">
                            <!-- lines and payments are in this currency, so it is fixed once the invoice exists -->
                            <currency-select :disabled="loading || submitting || !!editInvoiceId"
                                             :label="tt('Currency')" :placeholder="tt('Currency')"
                                             v-model="invoice.currency" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <date-select :disabled="loading || submitting"
                                         :label="tt('Invoice Date')"
                                         :model-value="invoice.invoiceDate"
                                         @update:model-value="onInvoiceDateChanged" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <date-select :disabled="loading || submitting"
                                         :label="tt('Payment Due')"
                                         v-model="invoice.dueDate" />
                        </v-col>

                        <!-- one grid for the whole form: every gap comes from the column gutters, never from per-row margins -->
                        <template :key="index" v-for="(line, index) in invoice.lines">
                            <v-col cols="12" md="3">
                                <v-combobox autocomplete="off" persistent-placeholder no-filter hide-no-data
                                            :disabled="loading || submitting"
                                            :label="tt('Item')" :placeholder="tt('Item')"
                                            :items="getProductSuggestions(line.itemName)" item-title="name"
                                            :model-value="line.itemName"
                                            @update:model-value="onItemChanged(line, $event)">
                                    <!-- the product link sits inside the field so every line keeps the same height -->
                                    <template #append-inner v-if="getLinkedProductName(line)">
                                        <v-chip size="x-small" label closable :prepend-icon="mdiPackageVariantClosed"
                                                :disabled="loading || submitting"
                                                @click:close="line.productId = ''">
                                            <v-tooltip activator="parent">{{ tt('Product') + ': ' + getLinkedProductName(line) }}</v-tooltip>
                                        </v-chip>
                                    </template>
                                </v-combobox>
                            </v-col>
                            <v-col cols="12" md="3">
                                <v-textarea type="text" autocomplete="off" persistent-placeholder rows="1" auto-grow
                                            :disabled="loading || submitting"
                                            :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                            v-model="line.description" />
                            </v-col>
                            <v-col cols="4" md="2">
                                <v-text-field type="text" autocomplete="off" persistent-placeholder
                                              :disabled="loading || submitting"
                                              :label="tt('SAC')" :placeholder="tt('SAC')"
                                              v-model="line.sac" />
                            </v-col>
                            <v-col cols="4" md="1">
                                <number-input :disabled="loading || submitting" :persistent-placeholder="true"
                                              :label="tt('Qty')" :min-value="0.01" :max-value="10000" :max-decimal-count="2"
                                              :model-value="line.quantity / INVOICE_QUANTITY_SCALE"
                                              @update:model-value="line.quantity = Math.round($event * INVOICE_QUANTITY_SCALE)" />
                            </v-col>
                            <v-col cols="3" md="2">
                                <amount-input :disabled="loading || submitting" :persistent-placeholder="true"
                                              :currency="invoice.currency" :show-currency="true"
                                              :label="tt('Rate')" :placeholder="tt('Rate')"
                                              v-model="line.rate" />
                            </v-col>
                            <v-col cols="1" md="1" class="d-flex align-center justify-center">
                                <v-btn color="error" variant="text" density="comfortable" :icon="mdiDeleteOutline" :aria-label="tt('Remove')"
                                       :disabled="loading || submitting || invoice.lines.length <= 1"
                                       @click="removeLine(index)"></v-btn>
                            </v-col>
                        </template>

                        <v-col cols="12">
                            <v-btn color="primary" variant="text" density="comfortable" :prepend-icon="mdiPlus"
                                   :disabled="loading || submitting" @click="addLine">{{ tt('Add Line') }}</v-btn>
                        </v-col>

                        <v-col cols="6" md="3">
                            <v-select item-title="displayName" item-value="type" persistent-placeholder
                                      :disabled="loading || submitting"
                                      :label="tt('Tax')" :placeholder="tt('Tax')"
                                      :items="allTaxTypes"
                                      v-model="invoice.taxType" />
                        </v-col>
                        <v-col cols="6" md="3">
                            <number-input :disabled="loading || submitting || invoice.taxType === 'none'" :persistent-placeholder="true"
                                          :label="tt('Tax Rate (%)')" :placeholder="tt('Tax Rate (%)')"
                                          :min-value="0" :max-value="100" :max-decimal-count="2"
                                          v-model="taxRatePercent" />
                        </v-col>
                        <v-col cols="12" md="6" class="d-flex flex-column align-end justify-center">
                            <template v-if="invoice.taxType !== 'none'">
                                <div>
                                    <span class="text-body-large me-4">{{ tt('Subtotal') }}</span>
                                    <span class="text-body-large">{{ formatAmount(totals.subtotal) }}</span>
                                </div>
                                <div :key="row.label" v-for="row in taxRows">
                                    <span class="text-body-large me-4">{{ row.label }}</span>
                                    <span class="text-body-large">{{ formatAmount(row.amount) }}</span>
                                </div>
                            </template>
                            <div>
                                <span class="text-body-large me-4">{{ tt('Total') }}</span>
                                <span class="text-title-medium">{{ formatAmount(totals.total) }}</span>
                            </div>
                        </v-col>

                        <v-col cols="12" md="12">
                            <v-textarea type="text" autocomplete="off" persistent-placeholder rows="6"
                                        :disabled="loading || submitting"
                                        :label="tt('Notes / Terms')" :placeholder="tt('Notes / Terms')"
                                        v-model="invoice.notes" />
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

    <customer-edit-dialog ref="customerEditDialog" />
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef } from 'vue';

import { mdiPlus, mdiDeleteOutline, mdiAccountPlusOutline, mdiPackageVariantClosed } from '@mdi/js';

import SnackBar from '@/components/desktop/SnackBar.vue';
import CustomerEditDialog from './CustomerEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceEditPageBase } from '@/views/base/invoices/InvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type InvoiceProfile, type InvoiceLine, Invoice } from '@/models/invoice.ts';
import type { Product } from '@/models/product.ts';

import { isEquals } from '@/lib/common.ts';
import { INVOICE_QUANTITY_SCALE } from '@/lib/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;
type CustomerEditDialogType = InstanceType<typeof CustomerEditDialog>;

interface InvoiceEditResponse {
    message: string;
    invoice: Invoice;
}

const { tt } = useI18n();
const {
    editInvoiceId,
    loading,
    submitting,
    invoice,
    allCustomers,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    totals,
    taxRows,
    allTaxTypes,
    taxRatePercent,
    addLine,
    getProductSuggestions,
    selectProduct,
    getLinkedProductName,
    removeLine,
    setInvoice,
    prepareNewInvoice,
    setInvoiceDate,
    suggestNumber: loadSuggestedNumber,
    formatAmount
} = useInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const customerEditDialog = useTemplateRef<CustomerEditDialogType>('customerEditDialog');

let resolveFunc: ((value: InvoiceEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const profile = ref<InvoiceProfile | null>(null);
const initInvoice = ref<Invoice>(Invoice.createNewInvoice());

const isInvoiceModified = computed<boolean>(() => !isEquals(invoice.value.toCreateRequest(), initInvoice.value.toCreateRequest()));

function suggestNumber(): void {
    loadSuggestedNumber().then(applied => {
        if (applied) {
            initInvoice.value.number = invoice.value.number;
        }
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function open(options: { id?: string, currentInvoice?: Invoice, duplicateFrom?: Invoice }): Promise<InvoiceEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    editInvoiceId.value = options.id ?? null;

    if (options.currentInvoice) {
        setInvoice(options.currentInvoice);
    }

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        invoicesStore.loadAllProducts({ force: false }).catch(() => []),
        options.id ? invoicesStore.getInvoice({ invoiceId: options.id }) : Promise.resolve(null)
    ]).then(([, loadedProfile, , loadedInvoice]) => {
        profile.value = loadedProfile;

        if (loadedInvoice) {
            setInvoice(loadedInvoice, loadedProfile);
        } else {
            prepareNewInvoice(loadedProfile, options.duplicateFrom);
            suggestNumber();
        }

        initInvoice.value = invoice.value.clone();
        loading.value = false;
    }).catch(error => {
        loading.value = false;
        showState.value = false;

        if (!error.processed) {
            rejectFunc?.(error);
        }
    });

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function onInvoiceDateChanged(date: TextualYearMonthDay | ''): void {
    if (!date) {
        return;
    }

    setInvoiceDate(date, profile.value?.defaultTermsDays ?? 7);

    if (!editInvoiceId.value) {
        suggestNumber();
    }
}

// a picked suggestion fills the line from the product; typing keeps any product link (the text may be edited freely)
function onItemChanged(line: InvoiceLine, value: string | Product | null): void {
    if (value && typeof value === 'object') {
        selectProduct(line, value);
    } else {
        line.itemName = value ?? '';
    }
}

function addCustomer(): void {
    customerEditDialog.value?.open({}).then(result => {
        invoice.value.customerId = result.customer.id;
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    invoicesStore.saveInvoice({ invoice: invoice.value, isEdit: !!editInvoiceId.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice', invoice: saved });
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
