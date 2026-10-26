<template>
    <v-row>
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="d-flex flex-wrap align-center ga-3">
                        <span>{{ tt('Invoice Email Template') }}</span>
                        <v-progress-circular indeterminate size="20" v-if="loading"></v-progress-circular>
                        <v-spacer />
                        <v-btn color="default" variant="text" :disabled="saving" @click="back">{{ tt('Back') }}</v-btn>
                        <v-btn color="default" variant="tonal" :disabled="loading || saving || !loaded || inputIsNotChanged" @click="reset">{{ tt('Reset') }}</v-btn>
                        <v-tooltip :disabled="!inputIsInvalid" :text="inputInvalidProblemMessage ? tt(inputInvalidProblemMessage) : ''">
                            <template #activator="{ props }">
                                <div v-bind="props" class="d-inline-block">
                                    <v-btn :disabled="loading || saving || !loaded || inputIsNotChanged || inputIsInvalid" @click="save">
                                        {{ tt('Save Changes') }}
                                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"></v-progress-circular>
                                    </v-btn>
                                </div>
                            </template>
                        </v-tooltip>
                    </div>
                </template>

                <v-card-text class="pt-2">
                    <span class="text-body-large">{{ tt('Customize the subject and complete message used when sending invoices.') }}</span>
                </v-card-text>
                <v-divider />
                <v-card-text v-if="loading">
                    <v-skeleton-loader type="article, paragraph" :loading="true" />
                </v-card-text>
                <v-card-text v-else-if="!loaded">
                    <v-alert type="error" variant="tonal" class="mb-4">{{ tt('Unable to load invoice email template') }}</v-alert>
                    <v-btn variant="tonal" @click="init">{{ tt('Retry') }}</v-btn>
                </v-card-text>
                <v-card-text v-else>
                    <invoice-email-template-form v-model:subject="newTemplate.subject" v-model:body="newTemplate.bodyHtml"
                                                 :disabled="saving" split allow-restore
                                                 @validity-change="emailTemplateValid = $event" />
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';
import { useRouter } from 'vue-router';

import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceEmailTemplateForm from '@/components/common/InvoiceEmailTemplateForm.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceEmailTemplatePageBase } from '@/views/base/invoices/InvoiceEmailTemplatePageBase.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const router = useRouter();
const invoicesStore = useInvoicesStore();
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
const loaded = ref<boolean>(false);
const snackbar = useTemplateRef<SnackBarType>('snackbar');

function init(): void {
    if (saving.value) {
        return;
    }

    loading.value = true;
    loaded.value = false;
    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
        loaded.value = true;
        loading.value = false;
    }).catch(error => {
        loading.value = false;
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function save(): void {
    if (loading.value || saving.value || !loaded.value || inputIsNotChanged.value || inputIsInvalid.value) {
        return;
    }

    saving.value = true;
    invoicesStore.updateInvoiceEmailTemplate(newTemplate.value).then(profile => {
        setProfile(profile);
        saving.value = false;
        snackbar.value?.showMessage('Invoice email template has been saved');
    }).catch(error => {
        saving.value = false;
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function back(): void {
    if (saving.value) {
        return;
    }

    if (router.options.history.state['back']) {
        router.back();
    } else {
        void router.push('/settings/invoice_profile');
    }
}

init();
</script>
