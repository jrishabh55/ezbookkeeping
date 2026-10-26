<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="invoice ? invoice.number : tt('Invoice')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading || updating }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item title="Status" after="Unpaid"></f7-list-item>
            <f7-list-item title="Total" after="₹000,000.00"></f7-list-item>
            <f7-list-item title="Balance Due" after="₹000,000.00"></f7-list-item>
        </f7-list>

        <template v-if="!loading && invoice && profile">
            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item :title="tt('Status')" :after="tt(displayStatusName)"></f7-list-item>
                <f7-list-item :title="tt('Customer')" :after="invoice.billToName"></f7-list-item>
                <f7-list-item :title="tt('Payment Due')" :after="formatDate(invoice.dueDate)"></f7-list-item>
                <f7-list-item :title="tt('Total')" :after="formatAmount(invoice.total)"></f7-list-item>
                <f7-list-item :title="tt('Received')" :after="formatAmount(invoice.received)"></f7-list-item>
                <f7-list-item :title="tt(shortfallName)" :after="formatAmount(shortfall)" v-if="shortfall > 0"></f7-list-item>
                <f7-list-item :title="tt('Balance Due')" :after="formatAmount(balanceDue)" v-else></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item group-title>
                    <small>{{ tt('Payments') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No payments linked yet.')" v-if="!invoice.payments.length"></f7-list-item>
                <f7-list-item swipeout
                              :title="formatAmount(payment.amount)"
                              :footer="formatPaymentDescription(payment)"
                              :key="payment.transactionId"
                              v-for="payment in invoice.payments">
                    <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                         :right="textDirection === TextDirection.LTR">
                        <f7-swipeout-button color="red" close :text="tt('Unlink')" @click="unlink(payment)"></f7-swipeout-button>
                    </f7-swipeout-actions>
                </f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical" v-if="displayStatus !== 'paid'">
                <f7-list-item group-title>
                    <small>{{ tt('Suggested Matches') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No matching income transactions found yet.')" v-if="!suggestions.length"></f7-list-item>
                <f7-list-item :title="formatAmount(suggestion.amount)"
                              :footer="formatPaymentDescription(suggestion)"
                              :key="suggestion.transactionId"
                              v-for="suggestion in suggestions">
                    <template #after>
                        <f7-button small fill :class="{ 'disabled': updating }" :text="tt('Link')" @click="link(suggestion)"></f7-button>
                    </template>
                </f7-list-item>
            </f7-list>

            <f7-block class="invoice-document-preview">
                <invoice-document :invoice="invoice" :profile="profile" />
            </f7-block>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/add?duplicateFrom=${invoiceId}`)">{{ tt('Duplicate') }}</f7-actions-button>
                <f7-actions-button @click="toggleClosed">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</f7-actions-button>
                <f7-actions-button @click="printInvoice">{{ tt('Print / Save PDF') }}</f7-actions-button>
                <f7-actions-button @click="showRepeatMonthlyPopup = true">{{ tt('Repeat Monthly') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button color="red" @click="showDeleteActionSheet = true">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <recurring-invoice-add-popup :invoice="invoice" v-model:show="showRepeatMonthlyPopup" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';
import RecurringInvoiceAddPopup from './RecurringInvoiceAddPopup.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { TextDirection } from '@/core/text.ts';
import type { InvoicePayment } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    updating,
    invoice,
    profile,
    suggestions,
    displayStatus,
    displayStatusName,
    balanceDue,
    shortfall,
    shortfallName,
    formatAmount,
    formatDate,
    formatPaymentDescription,
    printInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const loadingError = ref<unknown | null>(null);
const invoiceId = ref<string>(props.f7route.query['id'] || '');
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);
const showRepeatMonthlyPopup = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function load(): void {
    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        loading.value = false;

        if (displayStatus.value === 'paid') {
            suggestions.value = [];
            return;
        }

        invoicesStore.getInvoicePaymentSuggestions({ invoiceId: loadedInvoice.id }).then(result => {
            suggestions.value = result;
        }).catch(error => {
            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function runAndReload(action: () => Promise<unknown>): void {
    updating.value = true;
    showLoading(() => updating.value);

    action().then(() => {
        updating.value = false;
        hideLoading();
        load();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function link(suggestion: InvoicePayment): void {
    runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment({ invoiceId: invoiceId.value, transactionId: payment.transactionId }));
}

function toggleClosed(): void {
    if (invoice.value) {
        const current = invoice.value;
        runAndReload(() => invoicesStore.setInvoiceClosed({ invoice: current, closed: !current.closedManually }));
    }
}

function remove(): void {
    if (!invoice.value) {
        return;
    }

    const router = props.f7router;
    showDeleteActionSheet.value = false;
    updating.value = true;
    showLoading(() => updating.value);

    invoicesStore.deleteInvoice({ invoice: invoice.value }).then(() => {
        updating.value = false;
        hideLoading();
        router.back();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        load(); // back from the edit page
    }

    routeBackOnError(props.f7router, loadingError);
}

load();
</script>

<style>
.invoice-document-preview {
    overflow-x: auto;
}
</style>
