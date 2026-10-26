<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': saving }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Invoice Email Template')"></f7-nav-title>
            <f7-nav-right class="navbar-compact-icons">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': loading || saving || inputIsNotChanged || inputIsInvalid }"
                         :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-block strong inset class="margin-vertical skeleton-text" v-if="loading">
            <p>{{ tt('Subject') }}</p>
            <p>{{ tt('Message') }}</p>
            <p>{{ tt('Email Preview') }}</p>
        </f7-block>

        <f7-block strong inset class="margin-vertical" v-else>
            <invoice-email-template-form v-model:subject="newTemplate.subject" v-model:body="newTemplate.bodyHtml"
                                         :disabled="saving" allow-restore
                                         @validity-change="emailTemplateValid = $event" />
            <f7-button class="margin-top" :disabled="saving || inputIsNotChanged" @click="reset">{{ tt('Reset') }}</f7-button>
        </f7-block>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import InvoiceEmailTemplateForm from '@/components/common/InvoiceEmailTemplateForm.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceEmailTemplatePageBase } from '@/views/base/invoices/InvoiceEmailTemplatePageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    saving,
    newTemplate,
    emailTemplateValid,
    inputIsNotChanged,
    inputInvalidProblemMessage,
    inputIsInvalid,
    setProfile,
    reset
} = useInvoiceEmailTemplatePageBase();
const invoicesStore = useInvoicesStore();
const loadingError = ref<unknown | null>(null);

function init(): void {
    loading.value = true;

    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
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
    if (loading.value || saving.value || inputIsNotChanged.value) {
        return;
    }

    const problemMessage = inputInvalidProblemMessage.value;
    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    saving.value = true;
    showLoading(() => saving.value);

    invoicesStore.updateInvoiceEmailTemplate(newTemplate.value).then(profile => {
        setProfile(profile);
        saving.value = false;
        hideLoading();
        showToast('Invoice email template has been saved');
    }).catch(error => {
        saving.value = false;
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
