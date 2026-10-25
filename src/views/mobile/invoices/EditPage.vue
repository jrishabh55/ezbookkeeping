<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title" header="Customer" title="Customer Name"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Number" title="CA/2026-27/01"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Date" title="April 23, 2026"></f7-list-item>
        </f7-list>

        <template v-else>
            <f7-list form strong inset dividers class="margin-vertical">
                <f7-list-item class="list-item-with-header-and-title" link="#"
                              :header="tt('Customer')"
                              :title="selectedCustomerName"
                              @click="showCustomerPopup = true">
                    <list-item-selection-popup value-type="item"
                                               key-field="id" value-field="id"
                                               title-field="name"
                                               :title="tt('Customer')"
                                               :enable-filter="true"
                                               :filter-placeholder="tt('Customer')"
                                               :filter-no-items-text="tt('No results')"
                                               :items="allCustomers"
                                               v-model:show="showCustomerPopup"
                                               v-model="invoice.customerId">
                    </list-item-selection-popup>
                </f7-list-item>
                <f7-list-input type="text" autocomplete="off" clear-button
                               :label="tt('Invoice Number')" :placeholder="tt('Invoice Number')"
                               v-model:value="invoice.number"></f7-list-input>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Invoice Date')"
                              :title="formatDate(invoice.invoiceDate)"
                              @click="showInvoiceDateSheet = true">
                    <date-selection-sheet v-model:show="showInvoiceDateSheet"
                                          :model-value="invoice.invoiceDate"
                                          @update:model-value="onInvoiceDateChanged">
                    </date-selection-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt('Payment Due')"
                              :title="formatDate(invoice.dueDate)"
                              @click="showDueDateSheet = true">
                    <date-selection-sheet v-model:show="showDueDateSheet"
                                          :model-value="invoice.dueDate"
                                          @update:model-value="onDueDateChanged">
                    </date-selection-sheet>
                </f7-list-item>
            </f7-list>

            <f7-block class="no-padding no-margin">
                <f7-list strong inset dividers class="margin-vertical"
                         :key="index" v-for="(line, index) in invoice.lines">
                    <!-- list-number-input must go in #list (f7-list cannot scan it in the default slot), so the whole line goes there to keep its order -->
                    <template #list>
                        <f7-list-item group-title>
                            <small>{{ tt('Line') + ' #' + (index + 1) }}</small>
                            <f7-button rasied fill color="red" icon-f7="trash" icon-size="16px" :aria-label="tt('Remove')"
                                       :class="{ 'disabled': invoice.lines.length <= 1 }"
                                       @click="removeLine(index)"></f7-button>
                        </f7-list-item>
                        <f7-list-input type="text" autocomplete="off" clear-button
                                       :label="tt('Item')" :placeholder="tt('Item')"
                                       v-model:value="line.itemName"></f7-list-input>
                        <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                                       :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                       v-textarea-auto-size v-model:value="line.description"></f7-list-input>
                        <list-number-input :label="tt('Qty')" :placeholder="tt('Qty')"
                                           :min-value="1" :max-value="10000" :max-decimal-count="0"
                                           v-model="line.quantity"></list-number-input>
                        <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                                      :header="tt('Rate')"
                                      :title="formatAmount(line.rate)"
                                      @click="rateSheetLineIndex = index">
                            <number-pad-sheet :min-value="0"
                                              :max-value="TRANSACTION_MAX_AMOUNT"
                                              :currency="invoice.currency"
                                              :show="rateSheetLineIndex === index"
                                              @update:show="rateSheetLineIndex = $event ? index : -1"
                                              v-model="line.rate"></number-pad-sheet>
                        </f7-list-item>
                    </template>
                </f7-list>
            </f7-block>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item :title="tt('Total')" :after="formatAmount(totals.total)"></f7-list-item>
                <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                               :label="tt('Notes / Terms')" :placeholder="tt('Notes / Terms')"
                               v-textarea-auto-size v-model:value="invoice.notes"></f7-list-input>
            </f7-list>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="addLine">{{ tt('Add Line') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/customer/add')">{{ tt('Add Customer') }}</f7-actions-button>
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
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceEditPageBase } from '@/views/base/invoices/InvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
import type { InvoiceProfile } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editInvoiceId,
    loading,
    submitting,
    invoice,
    allCustomers,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    totals,
    addLine,
    removeLine,
    setInvoice,
    prepareNewInvoice,
    setInvoiceDate,
    suggestNumber: loadSuggestedNumber,
    formatAmount,
    formatDate
} = useInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const profile = ref<InvoiceProfile | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showCustomerPopup = ref<boolean>(false);
const showInvoiceDateSheet = ref<boolean>(false);
const showDueDateSheet = ref<boolean>(false);
const rateSheetLineIndex = ref<number>(-1);

const selectedCustomerName = computed<string>(() => invoicesStore.allCustomersMap[invoice.value.customerId]?.name ?? tt('None'));

function suggestNumber(): void {
    loadSuggestedNumber().catch(error => {
        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function init(): void {
    const query = props.f7route.query;
    loading.value = true;
    editInvoiceId.value = query['id'] || null;

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        query['id'] ? invoicesStore.getInvoice({ invoiceId: query['id'] }) : Promise.resolve(null),
        query['duplicateFrom'] ? invoicesStore.getInvoice({ invoiceId: query['duplicateFrom'] }) : Promise.resolve(null)
    ]).then(([, loadedProfile, loadedInvoice, duplicateFrom]) => {
        profile.value = loadedProfile;

        if (loadedInvoice) {
            setInvoice(loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, duplicateFrom);
            suggestNumber();
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

function onInvoiceDateChanged(date: TextualYearMonthDay | ''): void {
    if (!date) {
        return;
    }

    setInvoiceDate(date, profile.value?.defaultTermsDays ?? 7);

    if (!editInvoiceId.value) {
        suggestNumber();
    }
}

function onDueDateChanged(date: TextualYearMonthDay | ''): void {
    if (!date) {
        return;
    }

    invoice.value.dueDate = date;
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

    invoicesStore.saveInvoice({ invoice: invoice.value, isEdit: !!editInvoiceId.value }).then(() => {
        submitting.value = false;
        hideLoading();
        showToast(editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice');
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
