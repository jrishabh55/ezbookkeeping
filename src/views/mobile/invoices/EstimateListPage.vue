<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Estimates')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/add?kind=estimate"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="EST/2026-27/01" after="₹000,000.00" footer="Customer · Sent" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableEstimate">
            <f7-list-item :title="tt('No available estimate')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !noAvailableEstimate">
            <f7-list-item swipeout
                          :id="getEstimateDomId(estimate)"
                          :title="estimate.number"
                          :after="formatAmount(estimate.total, estimate.currency)"
                          :link="'/invoice/detail?id=' + estimate.id"
                          :key="estimate.id"
                          v-for="estimate in displayEstimates">
                <template #footer>
                    <div>{{ estimate.billToName }} · {{ tt(getStatusName(estimate)) }} · {{ formatDate(estimate.dueDate) }}</div>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" v-if="!isEstimateLocked(estimate)" @click="edit(estimate)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(estimate, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showClosed" @click="showClosed = true">{{ tt('Show Closed Estimates') }}</f7-actions-button>
                <f7-actions-button v-if="showClosed" @click="showClosed = false">{{ tt('Hide Closed Estimates') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this estimate?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(estimateToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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
import { useEstimateListPageBase } from '@/views/base/invoices/EstimateListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Invoice } from '@/models/invoice.ts';

import { isEstimateLocked } from '@/lib/estimate.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showClosed,
    displayEstimates,
    noAvailableEstimate,
    getStatusName,
    formatAmount,
    formatDate
} = useEstimateListPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const estimateToDelete = ref<Invoice | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getEstimateDomId(estimate: Invoice): string {
    return 'estimate_' + estimate.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllEstimates({ force: false }).then(() => {
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

    invoicesStore.loadAllEstimates({ force: force }).then(() => {
        done?.();
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Estimate list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(estimate: Invoice): void {
    props.f7router.navigate(`/invoice/edit?id=${estimate.id}`);
}

function remove(estimate: Invoice | null, confirm: boolean): void {
    if (!estimate) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        estimateToDelete.value = estimate;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    estimateToDelete.value = null;
    showLoading();

    invoicesStore.deleteInvoice({
        invoice: estimate,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getEstimateDomId(estimate), done);
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
