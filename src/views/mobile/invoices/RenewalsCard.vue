<template>
    <template v-if="renewals.length || estimates.length">
        <f7-block-title>{{ tt('Needs Attention') }}</f7-block-title>
        <f7-list strong inset dividers media-list>
            <f7-list-item swipeout :key="renewal.invoice.id" v-for="renewal in renewals"
                          :link="'/invoice/detail?id=' + renewal.invoice.id"
                          :title="renewal.invoice.billToName"
                          :subtitle="`${renewal.invoice.number} · ${formatAmount(renewal.invoice)}`"
                          :footer="`${getStateName(renewal)} · ${formatDate(renewal.invoice.invoiceDate)}`">
                <f7-swipeout-actions right v-if="canSend(renewal)">
                    <f7-swipeout-button color="blue" close :text="tt('Send')" @click="sendRenewal(renewal)"></f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
            <f7-list-item :key="estimate.id" v-for="estimate in estimates"
                          :link="'/invoice/detail?id=' + estimate.id"
                          :title="getEstimateTitle(estimate)"
                          :subtitle="`${estimate.number} · ${formatAmount(estimate)}`"
                          :footer="tt('Convert')">
            </f7-list-item>
        </f7-list>
    </template>
</template>

<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useRenewalsCardBase } from '@/views/base/invoices/RenewalsCardBase.ts';

import type { InvoiceRenewal } from '@/models/invoice.ts';

const { tt } = useI18n();
const { showToast } = useI18nUIComponents();
const { renewals, estimates, load, canSend, send, getStateName, getEstimateTitle, formatAmount, formatDate } = useRenewalsCardBase();

function sendRenewal(renewal: InvoiceRenewal): void {
    showLoading();

    send(renewal).then(() => {
        hideLoading();
        showToast('Invoice has been sent');
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

load();

defineExpose({
    load
});
</script>
