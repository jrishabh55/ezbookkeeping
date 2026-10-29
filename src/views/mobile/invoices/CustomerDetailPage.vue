<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="customer ? customer.name : tt('Customer Details')"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading || !customer }">
                <f7-link :text="tt('Edit')" @click="f7router.navigate('/invoice/customer/edit?id=' + customerId)"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Billed" after="₹ 0.00" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <template v-if="!loading && customer">
            <f7-list strong inset dividers class="margin-top-half" v-if="customer.contactName || customer.email || customer.phone || customer.gstin">
                <f7-list-item :title="tt('Contact Name')" :after="customer.contactName" v-if="customer.contactName"></f7-list-item>
                <f7-list-item :title="tt('Email')" :after="customer.email" v-if="customer.email"></f7-list-item>
                <f7-list-item :title="tt('Phone')" :after="customer.phone" v-if="customer.phone"></f7-list-item>
                <f7-list-item :title="tt('GSTIN')" :after="customer.gstin" v-if="customer.gstin"></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-top-half">
                <f7-list-item :title="tt('Billed')" :after="formatAmount(total.amount, total.currency)" :key="'billed-' + total.currency" v-for="total in billedTotals"></f7-list-item>
                <f7-list-item :title="tt('Paid')" :after="formatAmount(total.amount, total.currency)" :key="'paid-' + total.currency" v-for="total in paidTotals"></f7-list-item>
                <f7-list-item :title="tt('Outstanding')" :after="formatAmount(total.amount, total.currency)" :key="'due-' + total.currency" v-for="total in outstandingTotals"></f7-list-item>
            </f7-list>

            <f7-block-title>{{ tt('Products Bought') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('No products bought yet.')" v-if="!overview.products.length"></f7-list-item>
                <f7-list-item :key="product.productId" v-for="product in overview.products" :title="product.name"
                              :footer="tt('format.misc.timesBought', { count: product.timesBought, date: formatDate(product.lastBoughtDate) })"></f7-list-item>
            </f7-list>

            <f7-block-title>{{ tt('Renewals') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('No renewals yet.')" v-if="!recurrings.length"></f7-list-item>
                <f7-list-item link="#" :key="recurring.id" v-for="recurring in recurrings"
                              :title="getScheduleText(recurring)" :footer="`${tt('Next Date')}: ${formatDate(recurring.nextDate)}`"
                              :after="recurring.enabled ? '' : tt('Paused')"
                              @click="editRecurring(recurring)"></f7-list-item>
                <f7-list-item link="#" no-chevron :title="tt('Add')" @click="showInvoicePopup = true" v-if="invoices.length">
                    <list-item-selection-popup value-type="item" key-field="id" value-field="id" title-field="number"
                                               :title="tt('Invoice')" :enable-filter="true"
                                               :filter-placeholder="tt('Invoice')" :filter-no-items-text="tt('No results')"
                                               :items="invoices"
                                               v-model:show="showInvoicePopup"
                                               :model-value="''"
                                               @update:model-value="addRecurring"></list-item-selection-popup>
                </f7-list-item>
            </f7-list>

            <template v-if="customer.notes">
                <f7-block-title>{{ tt('Internal Notes') }}</f7-block-title>
                <f7-block strong inset style="white-space: pre-line">{{ customer.notes }}</f7-block>
            </template>

            <f7-block-title>{{ tt('Timeline') }}</f7-block-title>
            <f7-list strong inset dividers>
                <f7-list-item :title="tt('Nothing has happened yet.')" v-if="!overview.timeline.length"></f7-list-item>
                <f7-list-item :key="`${item.kind}-${item.invoiceId}-${item.unixTime}`" v-for="item in overview.timeline"
                              :link="'/invoice/detail?id=' + item.invoiceId"
                              :title="getTimelineTitle(item)" :footer="formatTime(item.unixTime)"
                              :after="item.kind === 'email' ? '' : formatAmount(item.amount, item.currency)"></f7-list-item>
            </f7-list>
        </template>

        <recurring-invoice-add-popup :invoice="invoiceToRepeat" :recurring="recurringToEdit" v-model:show="showEditPopup" @added="reload()" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import RecurringInvoiceAddPopup from './RecurringInvoiceAddPopup.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { useCustomerDetailPageBase } from '@/views/base/invoices/CustomerDetailPageBase.ts';
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';

import type { Invoice, RecurringInvoice } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();
const { loading, customer, overview, invoices, recurrings, billedTotals, paidTotals, outstandingTotals, load, formatAmount, formatDate, formatTime, getTimelineTitle } = useCustomerDetailPageBase();
const { getScheduleText } = useRecurringInvoiceListPageBase();

const customerId = props.f7route.query['id'] ?? '';
const loadingError = ref<unknown | null>(null);
const recurringToEdit = ref<RecurringInvoice | null>(null);
const invoiceToRepeat = ref<Invoice | null>(null);
const showEditPopup = ref<boolean>(false);
const showInvoicePopup = ref<boolean>(false);

function reload(done?: () => void): void {
    load(customerId).then(() => done?.()).catch(error => {
        done?.();

        if (!done) {
            loadingError.value = error;
        }

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function editRecurring(recurring: RecurringInvoice): void {
    invoiceToRepeat.value = null;
    recurringToEdit.value = recurring;
    showEditPopup.value = true;
}

// a renewal repeats one of this customer's invoices
function addRecurring(invoiceId: unknown): void {
    const invoice = invoices.value.find(item => item.id === invoiceId);

    if (!invoice) {
        return;
    }

    recurringToEdit.value = null;
    invoiceToRepeat.value = invoice;
    showEditPopup.value = true;
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

reload();
</script>
