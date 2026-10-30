<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Invoices')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half" :class="{ 'skeleton-text': loading }">
            <f7-list-item :title="index === 0 ? tt('Outstanding') : ''" :after="loading ? '₹0.00' : formatAmount(total.amount, total.currency)"
                          :key="total.currency" v-for="(total, index) in outstandingTotals"></f7-list-item>
            <f7-list-item :title="index === 0 ? tt('Overdue') : ''" :after="loading ? '₹0.00' : formatAmount(total.amount, total.currency)"
                          :key="total.currency" v-for="(total, index) in overdueTotals"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="CA/2026-27/01" after="₹000,000.00" footer="Customer · Unpaid" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableInvoice">
            <f7-list-item :title="tt('No available invoice')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !noAvailableInvoice">
            <f7-list-item swipeout
                          :id="getInvoiceDomId(invoice)"
                          :title="invoice.number"
                          :after="formatAmount(getInvoiceDisplayBalanceDue(invoice), invoice.currency)"
                          :link="'/invoice/detail?id=' + invoice.id"
                          :key="invoice.id"
                          v-for="invoice in displayInvoices">
                <template #footer>
                    <div>{{ invoice.billToName }} · {{ tt(getDisplayStatusName(invoice)) }} · {{ formatDate(invoice.dueDate) }}</div>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(invoice)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(invoice, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showPaid" @click="showPaid = true">{{ tt('Show Paid Invoices') }}</f7-actions-button>
                <f7-actions-button v-if="showPaid" @click="showPaid = false">{{ tt('Hide Paid Invoices') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/customer/list')">{{ tt('Customers') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/product/list')">{{ tt('Products') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/profile')">{{ tt('Invoice Profile') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/email_template')">{{ tt('Invoice Email Template') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/recurring/list')">{{ tt('Recurring') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate('/invoice/estimate/list')">{{ tt('Estimates') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(invoiceToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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
import { useInvoiceListPageBase } from '@/views/base/invoices/InvoiceListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Invoice } from '@/models/invoice.ts';

import { getInvoiceDisplayBalanceDue } from '@/lib/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showPaid,
    displayInvoices,
    noAvailableInvoice,
    outstandingTotals,
    overdueTotals,
    formatAmount,
    formatDate,
    getDisplayStatusName
} = useInvoiceListPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const invoiceToDelete = ref<Invoice | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getInvoiceDomId(invoice: Invoice): string {
    return 'invoice_' + invoice.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllInvoices({ force: false }).then(() => {
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

    invoicesStore.loadAllInvoices({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Invoice list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Invoice list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(invoice: Invoice): void {
    props.f7router.navigate(`/invoice/edit?id=${invoice.id}`);
}

function remove(invoice: Invoice | null, confirm: boolean): void {
    if (!invoice) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        invoiceToDelete.value = invoice;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    invoiceToDelete.value = null;
    showLoading();

    invoicesStore.deleteInvoice({
        invoice: invoice,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getInvoiceDomId(invoice), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        reload();
    }

    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
