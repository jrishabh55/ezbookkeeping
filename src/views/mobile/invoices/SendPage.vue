<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading || sending }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Send Invoice')"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="paperplane" :class="{ 'disabled': !sendRequest.to || sending }" :aria-label="tt('Send')" @click="send"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list form strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-input type="text" label="To" placeholder="Email"></f7-list-input>
            <f7-list-input type="text" label="Subject" placeholder="Subject"></f7-list-input>
            <f7-list-input type="textarea" label="Message" placeholder="Message"></f7-list-input>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else>
            <f7-list-input type="email" autocomplete="off" clear-button
                           :label="tt('To')" :placeholder="tt('Email')"
                           v-model:value="sendRequest.to"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Subject')" :placeholder="tt('Subject')"
                           v-model:value="sendRequest.subject"></f7-list-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Message')" :placeholder="tt('Message')"
                           v-textarea-auto-size v-model:value="sendRequest.message"></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceSendForm } from '@/models/invoice.ts';

// the mobile "Send" form: the same three fields as the desktop dialog, prefilled by the shared detail base
const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    invoice,
    profile,
    prepareSendRequest,
    sendInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const invoiceId = ref<string>(props.f7route.query['id'] || '');
const sending = ref<boolean>(false);
const sendRequest = ref<InvoiceSendForm>({ id: '', to: '', subject: '', message: '' });

function init(): void {
    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        return prepareSendRequest(); // creates the public link when the invoice has none, so the message can carry it
    }).then(request => {
        sendRequest.value = { ...request };
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

function send(): void {
    const router = props.f7router;

    sending.value = true;
    showLoading(() => sending.value);

    sendInvoice(sendRequest.value).then(() => {
        sending.value = false;
        hideLoading();
        showToast('Invoice has been sent');
        router.back();
    }).catch(error => {
        sending.value = false;
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
