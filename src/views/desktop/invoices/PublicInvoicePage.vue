<template>
    <div class="public-invoice-page">
        <div class="public-invoice-toolbar" v-if="invoice">
            <span class="public-invoice-status" :class="{ 'public-invoice-status-paid': isPaid }">
                {{ isPaid ? tt('Paid') : tt('Amount Due') + ': ' + formatInvoiceDocumentAmount(balanceDue) }}
            </span>
            <v-spacer />
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiPrinterOutline" @click="print">{{ tt('Print') }}</v-btn>
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiDownloadOutline" :href="pdfUrl" target="_blank">{{ tt('Download PDF') }}</v-btn>
        </div>

        <div class="public-invoice-paper" v-if="invoice && profile">
            <invoice-document :invoice="invoice" :profile="profile" />
        </div>
        <div class="public-invoice-message" v-else-if="!loading">{{ tt('This invoice is not available.') }}</div>
        <div class="public-invoice-message" v-else>
            <v-progress-circular indeterminate color="primary" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { mdiPrinterOutline, mdiDownloadOutline } from '@mdi/js';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';

import { useI18n } from '@/locales/helpers.ts';

import { Invoice, InvoiceProfile } from '@/models/invoice.ts';

import services from '@/lib/services.ts';
import { formatInvoiceDocumentAmount, getInvoiceDisplayBalanceDue, getInvoiceDisplayStatus, getTodayTextualDate, printInvoiceDocument } from '@/lib/invoice.ts';

// A recipient's view of one invoice: no app chrome, no stores, no login. The data comes from the
// unauthenticated public endpoint, which returns the document fields only (no ids, payments or token).
const props = defineProps<{
    token: string;
}>();

const { tt } = useI18n();

const loading = ref<boolean>(true);
const invoice = ref<Invoice | null>(null);
const profile = ref<InvoiceProfile | null>(null);

const isPaid = computed<boolean>(() => !!invoice.value && getInvoiceDisplayStatus(invoice.value, getTodayTextualDate()) === 'paid');
const balanceDue = computed<number>(() => invoice.value ? getInvoiceDisplayBalanceDue(invoice.value) : 0);
// the server-rendered PDF by the same token; the response is an attachment, so the new tab turns into a download
const pdfUrl = computed<string>(() => services.getPublicInvoicePdfUrl(props.token));

function print(): void {
    if (invoice.value) {
        printInvoiceDocument(invoice.value);
    }
}

services.getPublicInvoice({ token: props.token }).then(response => {
    const data = response.data;

    if (data && data.success && data.result) {
        invoice.value = Invoice.ofPublic(data.result);
        profile.value = InvoiceProfile.ofPublic(data.result.profile);
        document.title = data.result.number;
    }
}).catch(() => {
    // unknown or disabled link: the page shows "not available" below
}).finally(() => {
    loading.value = false;
});
</script>

<style scoped>
.public-invoice-page {
    min-height: 100vh;
    background: #e5e7eb;
    color: #111827;
    padding: 24px 16px 48px;
}

.public-invoice-toolbar, .public-invoice-paper, .public-invoice-message {
    max-width: 864px;
    margin: 0 auto;
}

.public-invoice-toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
}

.public-invoice-status {
    font-weight: 700;
}

.public-invoice-status-paid {
    color: #15803d;
}

.public-invoice-paper {
    background: #fff;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
    overflow-x: auto; /* the document keeps its 640px minimum width on phones */
}

.public-invoice-message {
    text-align: center;
    padding: 48px 0;
}
</style>
