<template>
    <v-card class="mb-4" v-if="renewals.length || estimates.length">
        <template #title>{{ tt('Needs Attention') }}</template>
        <v-list density="compact">
            <v-list-item :key="renewal.invoice.id" v-for="renewal in renewals"
                         :title="`${renewal.invoice.billToName} · ${renewal.invoice.number}`"
                         :subtitle="`${getStateName(renewal)} · ${formatDate(renewal.invoice.invoiceDate)} · ${formatAmount(renewal.invoice)}`">
                <template #append>
                    <v-btn class="me-2" color="primary" variant="tonal" size="small" v-if="canSend(renewal)"
                           :loading="sendingId === renewal.invoice.id" :disabled="sendingId !== ''"
                           @click="sendRenewal(renewal)">{{ tt('Send') }}</v-btn>
                    <v-btn variant="text" size="small" :to="'/invoice/list?invoiceId=' + renewal.invoice.id">{{ tt('Open') }}</v-btn>
                </template>
            </v-list-item>
            <v-list-item :key="estimate.id" v-for="estimate in estimates"
                         :title="`${getEstimateTitle(estimate)} · ${estimate.number}`"
                         :subtitle="formatAmount(estimate)">
                <template #append>
                    <v-btn color="primary" variant="tonal" size="small" :to="'/invoice/list?invoiceId=' + estimate.id">{{ tt('Convert') }}</v-btn>
                </template>
            </v-list-item>
        </v-list>
    </v-card>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useRenewalsCardBase } from '@/views/base/invoices/RenewalsCardBase.ts';

import type { InvoiceRenewal } from '@/models/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const { renewals, estimates, sendingId, load, canSend, send, getStateName, getEstimateTitle, formatAmount, formatDate } = useRenewalsCardBase();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

function sendRenewal(renewal: InvoiceRenewal): void {
    send(renewal).then(() => {
        snackbar.value?.showMessage('Invoice has been sent');
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

load();

defineExpose({
    load
});
</script>
