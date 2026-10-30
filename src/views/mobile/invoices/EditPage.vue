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
                              :class="{ 'disabled': !!invoice.estimateId }"
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
                               :label="tt(numberLabel)" :placeholder="tt(numberLabel)"
                               v-model:value="invoice.number"></f7-list-input>
                <!-- lines and payments are in this currency, so it is fixed once the invoice exists -->
                <f7-list-item class="list-item-with-header-and-title list-item-no-item-after" link="#"
                              :class="{ 'disabled': !!editInvoiceId || !!invoice.estimateId }"
                              :header="tt('Currency')"
                              :no-chevron="!!editInvoiceId || !!invoice.estimateId"
                              @click="showCurrencyPopup = true">
                    <template #title>
                        <div class="no-padding no-margin">
                            <span>{{ getCurrencyName(invoice.currency) }}&nbsp;</span>
                            <small class="smaller">{{ invoice.currency }}</small>
                        </div>
                    </template>
                    <list-item-selection-popup value-type="item"
                                               key-field="currencyCode" value-field="currencyCode"
                                               title-field="displayName" after-field="currencyCode"
                                               :title="tt('Currency Name')"
                                               :enable-filter="true"
                                               :filter-placeholder="tt('Currency')"
                                               :filter-no-items-text="tt('No results')"
                                               :items="allCurrencies"
                                               v-model:show="showCurrencyPopup"
                                               v-model="invoice.currency">
                    </list-item-selection-popup>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt(dateLabel)"
                              :title="formatDate(invoice.invoiceDate)"
                              @click="showInvoiceDateSheet = true">
                    <date-selection-sheet v-model:show="showInvoiceDateSheet"
                                          :model-value="invoice.invoiceDate"
                                          @update:model-value="onInvoiceDateChanged">
                    </date-selection-sheet>
                </f7-list-item>
                <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                              :header="tt(dueDateLabel)"
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
                                       :class="{ 'disabled': isMilestoneInvoice || invoice.lines.length <= 1 }"
                                       @click="removeLine(index)"></f7-button>
                        </f7-list-item>
                        <f7-list-input type="text" autocomplete="off" clear-button :disabled="isMilestoneInvoice"
                                       :label="tt('Item')" :placeholder="tt('Item')"
                                       v-model:value="line.itemName"></f7-list-input>
                        <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                                      :class="{ 'disabled': isMilestoneInvoice }"
                                      :header="tt('Product')"
                                      :title="getLinkedProductName(line) || tt('None')"
                                      @click="productPopupLineIndex = index"
                                      v-if="allProducts.length">
                        </f7-list-item>
                        <f7-list-input type="textarea" style="height: auto" autocomplete="off" :disabled="isMilestoneInvoice"
                                       :label="tt('Description')" :placeholder="tt('Description (optional)')"
                                       v-textarea-auto-size v-model:value="line.description"></f7-list-input>
                        <f7-list-input type="text" autocomplete="off" clear-button :disabled="isMilestoneInvoice"
                                       :label="tt('SAC')" :placeholder="tt('SAC')"
                                       v-model:value="line.sac"></f7-list-input>
                        <list-number-input :label="tt('Qty')" :placeholder="tt('Qty')" :disabled="isMilestoneInvoice"
                                           :min-value="0.01" :max-value="10000" :max-decimal-count="2"
                                           :model-value="line.quantity / INVOICE_QUANTITY_SCALE"
                                           @update:model-value="line.quantity = Math.round($event * INVOICE_QUANTITY_SCALE)"></list-number-input>
                        <f7-list-item link="#" no-chevron class="list-item-with-header-and-title"
                                      :class="{ 'disabled': isMilestoneInvoice }"
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
                <f7-block class="text-align-center" v-if="isMilestoneInvoice">
                    <small>{{ tt('The amount of a milestone invoice is set from the estimate.') }}</small>
                </f7-block>
            </f7-block>

            <f7-list strong inset dividers class="margin-vertical" v-if="isEstimate">
                <!-- list-number-input must go in #list, so the whole block goes there to keep its order -->
                <template #list>
                    <f7-list-item group-title>
                        <small>{{ tt('Milestones') }}</small>
                        <small v-if="invoice.milestones.length">{{ milestonesProblem ? tt(milestonesProblem) : formatMilestonePercent(milestonePercentTotal) + ' ✓' }}</small>
                    </f7-list-item>
                    <template :key="index" v-for="(milestone, index) in invoice.milestones">
                        <f7-list-input type="text" autocomplete="off" clear-button
                                       :label="tt('Milestone') + ' #' + (index + 1)" :placeholder="tt('Milestone')"
                                       v-model:value="milestone.label"></f7-list-input>
                        <list-number-input :label="tt('Percent (%)')" :placeholder="tt('Percent (%)')"
                                           :min-value="0.01" :max-value="100" :max-decimal-count="2"
                                           :model-value="milestone.percent / 100"
                                           @update:model-value="milestone.percent = Math.round($event * 100)"></list-number-input>
                        <f7-list-item :title="tt('Amount')" :after="formatAmount(milestoneAmounts[index] ?? 0)"></f7-list-item>
                        <f7-list-button color="red" :title="tt('Remove')" @click="removeMilestone(index)"></f7-list-button>
                    </template>
                    <f7-list-button :class="{ 'disabled': invoice.milestones.length >= ESTIMATE_MAX_MILESTONES }"
                                    :title="tt('Add Milestone')" @click="addMilestone"></f7-list-button>
                </template>
            </f7-list>

            <list-item-selection-popup value-type="item" key-field="id" value-field="id" title-field="name"
                                       :title="tt('Product')" :enable-filter="true"
                                       :filter-placeholder="tt('Product')" :filter-no-items-text="tt('No results')"
                                       :items="productChoices"
                                       :show="productPopupLineIndex >= 0"
                                       @update:show="productPopupLineIndex = $event ? productPopupLineIndex : -1"
                                       :model-value="productPopupLineIndex >= 0 ? invoice.lines[productPopupLineIndex]?.productId ?? '' : ''"
                                       @update:model-value="onProductPicked">
            </list-item-selection-popup>

            <f7-list strong inset dividers class="margin-vertical">
                <template #list>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :class="{ 'disabled': isMilestoneInvoice }"
                                  :header="tt('Tax')"
                                  :title="selectedTaxTypeName"
                                  @click="showTaxTypeSheet = true">
                        <list-item-selection-sheet value-type="item"
                                                   key-field="type" value-field="type" title-field="displayName"
                                                   :items="allTaxTypes"
                                                   v-model:show="showTaxTypeSheet"
                                                   v-model="invoice.taxType">
                        </list-item-selection-sheet>
                    </f7-list-item>
                    <list-number-input :label="tt('Tax Rate (%)')" :placeholder="tt('Tax Rate (%)')" :disabled="isMilestoneInvoice"
                                       :min-value="0" :max-value="100" :max-decimal-count="2"
                                       v-model="taxRatePercent" v-if="invoice.taxType !== 'none'"></list-number-input>
                    <template v-if="invoice.taxType !== 'none'">
                        <f7-list-item :title="tt('Subtotal')" :after="formatAmount(totals.subtotal)"></f7-list-item>
                        <f7-list-item :title="row.label" :after="formatAmount(row.amount)" :key="row.label" v-for="row in taxRows"></f7-list-item>
                    </template>
                    <f7-list-item :title="tt('Total')" :after="formatAmount(totals.total)"></f7-list-item>
                    <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                                   :label="tt('Notes / Terms')" :placeholder="tt('Notes / Terms')"
                                   v-textarea-auto-size v-model:value="invoice.notes"></f7-list-input>
                </template>
            </f7-list>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!isMilestoneInvoice" @click="addLine">{{ tt('Add Line') }}</f7-actions-button>
                <f7-actions-button @click="addCustomer">{{ tt('Add Customer') }}</f7-actions-button>
                <f7-actions-button v-if="isEstimate" @click="addMilestone">{{ tt('Add Milestone') }}</f7-actions-button>
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
import type { LocalizedCurrencyInfo } from '@/core/currency.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
import type { InvoiceProfile } from '@/models/invoice.ts';
import { INVOICE_QUANTITY_SCALE } from '@/lib/invoice.ts';
import { ESTIMATE_MAX_MILESTONES, formatMilestonePercent } from '@/lib/estimate.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getAllCurrencies, getCurrencyName } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editInvoiceId,
    loading,
    submitting,
    invoice,
    allCustomers,
    allProducts,
    isEstimate,
    isMilestoneInvoice,
    title,
    numberLabel,
    dateLabel,
    dueDateLabel,
    inputEmptyProblemMessage,
    inputIsEmpty,
    totals,
    taxRows,
    allTaxTypes,
    selectedTaxTypeName,
    taxRatePercent,
    milestoneAmounts,
    milestonePercentTotal,
    milestonesProblem,
    addLine,
    selectProduct,
    getLinkedProductName,
    removeLine,
    addMilestone,
    removeMilestone,
    setInvoice,
    prepareNewInvoice,
    prepareConvertedInvoice,
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
const showCurrencyPopup = ref<boolean>(false);
const showInvoiceDateSheet = ref<boolean>(false);
const showDueDateSheet = ref<boolean>(false);
const showTaxTypeSheet = ref<boolean>(false);
const rateSheetLineIndex = ref<number>(-1);
const productPopupLineIndex = ref<number>(-1);
const productChoices = computed<{ id: string, name: string }[]>(() => [{ id: '', name: tt('None') }, ...allProducts.value]);
const customerIdsBeforeAdding = ref<Set<string> | null>(null);

const selectedCustomerName = computed<string>(() => invoicesStore.allCustomersMap[invoice.value.customerId]?.name ?? tt('None'));
const allCurrencies = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies());

function suggestNumber(): void {
    loadSuggestedNumber().catch(error => {
        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onProductPicked(productId: string): void {
    const line = invoice.value.lines[productPopupLineIndex.value];
    const product = invoicesStore.allProductsMap[productId];

    if (line && !productId) {
        line.productId = ''; // "None" removes the link and keeps the line as typed
    } else if (line && product) {
        selectProduct(line, product);
    }

    productPopupLineIndex.value = -1;
}

function init(): void {
    const query = props.f7route.query;
    loading.value = true;
    editInvoiceId.value = query['id'] || null;

    const fromEstimate = query['fromEstimate'] || '';

    Promise.all([
        invoicesStore.loadAllCustomers({ force: false }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        invoicesStore.loadAllProducts({ force: false }).catch(() => []),
        query['id'] ? invoicesStore.getInvoice({ invoiceId: query['id'] })
            : fromEstimate ? invoicesStore.getEstimateInvoiceDraft({ estimateId: fromEstimate, milestoneId: query['milestoneId'] || '' }) : Promise.resolve(null),
        query['duplicateFrom'] ? invoicesStore.getInvoice({ invoiceId: query['duplicateFrom'] }) : Promise.resolve(null)
    ]).then(([, loadedProfile, , loadedInvoice, duplicateFrom]) => {
        profile.value = loadedProfile;

        if (loadedInvoice && query['id']) {
            setInvoice(loadedInvoice, loadedProfile);
        } else if (loadedInvoice) {
            prepareConvertedInvoice(loadedProfile, loadedInvoice);
        } else {
            prepareNewInvoice(loadedProfile, duplicateFrom, query['kind'] === 'estimate' ? 'estimate' : 'invoice');
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

function addCustomer(): void {
    customerIdsBeforeAdding.value = new Set(allCustomers.value.map(customer => customer.id));
    props.f7router.navigate('/invoice/customer/add');
}

// back from Add Customer: select the customer just added when none is selected yet
function selectAddedCustomer(): void {
    const knownIds = customerIdsBeforeAdding.value;
    customerIdsBeforeAdding.value = null;

    if (!knownIds || invoice.value.customerId) {
        return;
    }

    // ids grow over time, so the longest / greatest id is the newest
    const addedIds = allCustomers.value.map(customer => customer.id).filter(id => !knownIds.has(id))
        .sort((a, b) => a.length - b.length || a.localeCompare(b));

    if (addedIds.length) {
        invoice.value.customerId = addedIds[addedIds.length - 1]!;
    }
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

    invoicesStore.saveInvoice({ invoice: invoice.value, isEdit: !!editInvoiceId.value }).then(saved => {
        submitting.value = false;
        hideLoading();
        showToast(isEstimate.value ?
            (editInvoiceId.value ? 'You have saved this estimate' : 'You have added a new estimate') :
            (editInvoiceId.value ? 'You have saved this invoice' : 'You have added a new invoice'));

        if (props.f7route.query['duplicateFrom'] || props.f7route.query['fromEstimate']) {
            router.navigate(`/invoice/detail?id=${saved.id}`, { reloadCurrent: true }); // open the copy in place of this page
        } else {
            router.back();
        }
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    selectAddedCustomer();
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
