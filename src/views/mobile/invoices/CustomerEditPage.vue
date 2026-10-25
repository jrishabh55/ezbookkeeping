<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list form strong inset dividers class="margin-vertical">
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Customer Name')" :placeholder="tt('Customer Name')"
                           v-model:value="customer.name"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Contact Person')" :placeholder="tt('Contact Person (optional)')"
                           v-model:value="customer.contactName"></f7-list-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Address')" :placeholder="tt('Address')"
                           v-textarea-auto-size v-model:value="customer.addressLines"></f7-list-input>
            <f7-list-input type="email" autocomplete="off" clear-button
                           :label="tt('Email')" :placeholder="tt('Email (optional)')"
                           v-model:value="customer.email"></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useCustomerEditPageBase } from '@/views/base/invoices/CustomerEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editCustomerId,
    loading,
    submitting,
    customer,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty
} = useCustomerEditPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);

function init(): void {
    const customerId = props.f7route.query['id'];

    if (!customerId) {
        return;
    }

    loading.value = true;
    editCustomerId.value = customerId;

    invoicesStore.loadAllCustomers({ force: false }).then(() => {
        const existing = invoicesStore.allCustomersMap[customerId];

        if (!existing) {
            loadingError.value = 'Unable to retrieve customer';
            showToast('Unable to retrieve customer');
            return;
        }

        customer.value = existing.clone();
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

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    invoicesStore.saveCustomer({ customer: customer.value, isEdit: !!editCustomerId.value }).then(() => {
        submitting.value = false;
        hideLoading();
        showToast(editCustomerId.value ? 'You have saved this customer' : 'You have added a new customer');
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
