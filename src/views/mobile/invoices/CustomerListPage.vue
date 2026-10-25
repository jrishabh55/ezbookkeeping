<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Customers')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/invoice/customer/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Customer Name" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && !customers.length">
            <f7-list-item :title="tt('No available customer')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && customers.length">
            <f7-list-item swipeout
                          :id="getCustomerDomId(customer)"
                          :title="customer.name"
                          :footer="customer.contactName"
                          :key="customer.id"
                          v-for="customer in customers">
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(customer)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(customer, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this customer?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(customerToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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

import { useInvoicesStore } from '@/stores/invoice.ts';

import { TextDirection } from '@/core/text.ts';
import type { Customer } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const invoicesStore = useInvoicesStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const customerToDelete = ref<Customer | null>(null);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());
const customers = computed<Customer[]>(() => invoicesStore.allCustomers);

function getCustomerDomId(customer: Customer): string {
    return 'customer_' + customer.id;
}

function init(): void {
    loading.value = true;

    invoicesStore.loadAllCustomers({ force: false }).then(() => {
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

    invoicesStore.loadAllCustomers({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Customer list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Customer list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(customer: Customer): void {
    props.f7router.navigate(`/invoice/customer/edit?id=${customer.id}`);
}

function remove(customer: Customer | null, confirm: boolean): void {
    if (!customer) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        customerToDelete.value = customer;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    customerToDelete.value = null;
    showLoading();

    invoicesStore.deleteCustomer({
        customer: customer,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getCustomerDomId(customer), done);
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
