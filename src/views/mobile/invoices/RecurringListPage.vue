<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Recurring')"></f7-nav-title>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Customer" footer="CA/2026-27/01 · November 15, 2026" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableRecurringInvoice">
            <f7-list-item :title="tt('No available recurring invoice')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !noAvailableRecurringInvoice">
            <f7-list-item swipeout
                          :id="getRecurringDomId(recurring)"
                          :title="recurring.customerName || getSourceNumber(recurring)"
                          :key="recurring.id"
                          v-for="recurring in allRecurringInvoices">
                <template #footer>
                    <div>{{ getSourceNumber(recurring) }} · {{ tt('Next Date') }}: {{ formatDate(recurring.nextDate) }}</div>
                </template>
                <template #after>
                    <f7-toggle :checked="recurring.enabled" :disabled="updating" @toggle:change="setEnabled(recurring, $event)"></f7-toggle>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(recurring, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this recurring invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(recurringToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { RecurringInvoice } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    allRecurringInvoices,
    noAvailableRecurringInvoice,
    formatDate,
    getSourceNumber
} = useRecurringInvoiceListPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const updating = ref<boolean>(false);
const recurringToDelete = ref<RecurringInvoice | null>(null);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getRecurringDomId(recurring: RecurringInvoice): string {
    return 'recurring_invoice_' + recurring.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllRecurringInvoices({ force: false }).then(() => {
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

    invoicesStore.loadAllRecurringInvoices({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Recurring invoice list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Recurring invoice list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function setEnabled(recurring: RecurringInvoice, enabled: boolean): void {
    if (recurring.enabled === enabled) {
        return;
    }

    const modified = recurring.clone();
    modified.enabled = enabled;
    updating.value = true;

    invoicesStore.modifyRecurringInvoice({ recurring: modified }).then(() => {
        updating.value = false;
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function remove(recurring: RecurringInvoice | null, confirm: boolean): void {
    if (!recurring) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        recurringToDelete.value = recurring;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    recurringToDelete.value = null;
    showLoading();

    invoicesStore.deleteRecurringInvoice({
        recurring: recurring,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getRecurringDomId(recurring), done);
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
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
