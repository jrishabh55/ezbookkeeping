<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading || sending }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Send Invoice')"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="paperplane" :class="{ 'disabled': !sendRequest.to || sending || !emailTemplateValid }" :aria-label="tt('Send')" @click="send"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list form strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-input type="text" label="To" placeholder="Email"></f7-list-input>
            <f7-list-input type="text" label="Subject" placeholder="Subject"></f7-list-input>
            <f7-list-input type="textarea" label="Message" placeholder="Message"></f7-list-input>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else>
            <f7-list-input type="email" autocomplete="off" clear-button
                           :disabled="sending" :label="tt('To')" :placeholder="tt('Email')"
                           v-model:value="sendRequest.to"></f7-list-input>
        </f7-list>
        <f7-block strong inset v-if="!loading && renewalOption && renewalOption.mixedPeriods">
            <small>{{ tt('These products renew on different periods. Split them into separate invoices to track their renewals.') }}</small>
        </f7-block>
        <f7-list strong inset dividers v-if="!loading && renewalOption && !renewalOption.mixedPeriods">
            <f7-list-item checkbox :title="renewalOptionText" :disabled="sending"
                          :checked="!!sendRequest.createRenewal" @change="sendRequest.createRenewal = $event.target.checked"></f7-list-item>
        </f7-list>
        <f7-block strong inset v-if="!loading">
            <invoice-email-template-form v-model:subject="sendRequest.subject" v-model:body="sendRequest.bodyHtml"
                                         :invoice-id="sendRequest.id" :disabled="sending"
                                         @validity-change="emailTemplateValid = $event" />
        </f7-block>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import InvoiceEmailTemplateForm from '@/components/common/InvoiceEmailTemplateForm.vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { InvoiceSendForm } from '@/models/invoice.ts';

// The shared rich compose form starts from saved templates for this send only.
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
    renewalOption,
    renewalOptionText,
    prepareSendRequest,
    sendInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();

const loadingError = ref<unknown | null>(null);
const invoiceId = ref<string>(props.f7route.query['id'] || '');
const sending = ref<boolean>(false);
const emailTemplateValid = ref<boolean>(false);
const sendRequest = ref<InvoiceSendForm & { bodyHtml: string }>({ id: '', to: '', subject: '', bodyHtml: '', message: '' });

function init(): void {
    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: true })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        return prepareSendRequest(); // preserves public-link creation before the actual-invoice preview
    }).then(request => {
        emailTemplateValid.value = false;
        sendRequest.value = { ...request, bodyHtml: request.bodyHtml ?? '' };
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
    if (sending.value || !sendRequest.value.to || !emailTemplateValid.value) {
        return;
    }

    const router = props.f7router;

    sending.value = true;
    showLoading(() => sending.value);

    sendInvoice(sendRequest.value).then(() => {
        sending.value = false;
        hideLoading();
        invoicesStore.loadAllRecurringInvoices({ force: false }).catch(() => []);
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
