<template>
    <div class="public-invoice-page">
        <div class="public-invoice-toolbar" v-if="invoice">
            <span class="public-invoice-status" v-if="isEstimate">
                {{ estimateStatusText || tt('Total') + ': ' + formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}
            </span>
            <span class="public-invoice-status" :class="{ 'public-invoice-status-paid': isPaid }" v-else>
                {{ isPaid ? tt('Paid') : tt('Amount Due') + ': ' + formatInvoiceDocumentAmount(balanceDue, invoice.currency) }}
            </span>
            <v-spacer />
            <template v-if="canAnswer">
                <v-btn color="success" :loading="answering" :disabled="answering" @click="answer(true)">{{ tt('Accept') }}</v-btn>
                <v-btn color="error" variant="tonal" :disabled="answering" @click="showDeclineDialog = true">{{ tt('Decline') }}</v-btn>
            </template>
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiPrinterOutline" @click="print">{{ tt('Print') }}</v-btn>
            <v-btn color="primary" variant="tonal" :prepend-icon="mdiDownloadOutline" :loading="downloadingPdf" @click="downloadPdf">{{ tt('Download PDF') }}</v-btn>
        </div>

        <div class="public-invoice-message" role="alert" v-if="answerError">{{ tt('This estimate can no longer be answered.') }}</div>
        <div class="public-invoice-message" role="alert" v-if="downloadError">{{ tt('Unable to download invoice PDF') }}</div>
        <div class="public-invoice-paper" v-if="invoice && profile">
            <invoice-document :invoice="invoice" :profile="profile" />
        </div>
        <div class="public-invoice-message" v-else-if="!loading">{{ tt('This invoice is not available.') }}</div>
        <div class="public-invoice-message" v-else>
            <v-progress-circular indeterminate color="primary" />
        </div>

        <v-dialog width="480" v-model="showDeclineDialog">
            <v-card>
                <v-card-title>{{ tt('Decline Estimate') }}</v-card-title>
                <v-card-text>
                    <v-textarea rows="3" auto-grow counter="500" maxlength="500" :disabled="answering"
                                :label="tt('Note (optional)')" v-model="declineNote" />
                </v-card-text>
                <v-card-actions>
                    <v-spacer />
                    <v-btn :disabled="answering" @click="showDeclineDialog = false">{{ tt('Cancel') }}</v-btn>
                    <v-btn color="error" :loading="answering" @click="answer(false)">{{ tt('Decline') }}</v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { mdiPrinterOutline, mdiDownloadOutline } from '@mdi/js';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';

import { useI18n } from '@/locales/helpers.ts';

import { Invoice, InvoiceProfile } from '@/models/invoice.ts';

import services from '@/lib/services.ts';
import { readInvoicePdf } from '@/lib/invoicePdf.ts';
import { startDownloadFile } from '@/lib/ui/common.ts';
import { formatInvoiceDocumentAmount, formatInvoiceDocumentDate, getInvoicePdfFileName, getInvoiceDisplayBalanceDue, getInvoiceDisplayStatus, getTodayTextualDate, printInvoiceDocument } from '@/lib/invoice.ts';
import { canAnswerEstimate, getEstimateDisplayStatus, unixTimeToTextualDate } from '@/lib/estimate.ts';

// A recipient's view of one invoice: no app chrome, no stores, no login. The data comes from the
// unauthenticated public endpoint, which returns the document fields only (no ids, payments or token).
const props = defineProps<{
    token: string;
}>();

const { tt } = useI18n();

const loading = ref<boolean>(true);
const downloadingPdf = ref<boolean>(false);
const downloadError = ref<boolean>(false);
const invoice = ref<Invoice | null>(null);
const profile = ref<InvoiceProfile | null>(null);

const today = getTodayTextualDate();
const answering = ref<boolean>(false);
const answerError = ref<boolean>(false);
const showDeclineDialog = ref<boolean>(false);
const declineNote = ref<string>('');

const isPaid = computed<boolean>(() => !!invoice.value && getInvoiceDisplayStatus(invoice.value, getTodayTextualDate()) === 'paid');
const balanceDue = computed<number>(() => invoice.value ? getInvoiceDisplayBalanceDue(invoice.value) : 0);

const isEstimate = computed<boolean>(() => invoice.value?.kind === 'estimate');
// the server checks the same: sent and still valid
const canAnswer = computed<boolean>(() => !!invoice.value && canAnswerEstimate(invoice.value, today));

const estimateStatusText = computed<string>(() => {
    const current = invoice.value;

    if (!current || !isEstimate.value) {
        return '';
    }

    if ((current.estimateStatus === 'accepted' || current.estimateStatus === 'declined') && current.respondedUnixTime) {
        const date = formatInvoiceDocumentDate(unixTimeToTextualDate(current.respondedUnixTime));
        return tt(current.estimateStatus === 'accepted' ? 'format.misc.estimateAcceptedOn' : 'format.misc.estimateDeclinedOn', { date });
    }

    if (getEstimateDisplayStatus(current, today) === 'expired') {
        return tt('format.misc.estimateExpiredOn', { date: formatInvoiceDocumentDate(current.dueDate) });
    }

    return '';
});

async function downloadPdf(): Promise<void> {
    const current = invoice.value;
    if (!current || downloadingPdf.value) {
        return;
    }

    downloadingPdf.value = true;
    downloadError.value = false;
    try {
        const response = await services.getPublicInvoicePdf(props.token);
        const pdf = await readInvoicePdf(response.data);
        startDownloadFile(`${getInvoicePdfFileName(current.number, current.dueDate, current.kind)}.pdf`, pdf);
    } catch {
        downloadError.value = true;
    } finally {
        downloadingPdf.value = false;
    }
}

function print(): void {
    if (invoice.value) {
        printInvoiceDocument(invoice.value);
    }
}

function load(): Promise<void> {
    return services.getPublicInvoice({ token: props.token }).then(response => {
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
}

// the answer is recorded once; whatever happened, the page reloads the estimate to show its real state
function answer(accept: boolean): void {
    answering.value = true;
    answerError.value = false;

    services.respondToPublicEstimate({ token: props.token, accept, note: accept ? '' : declineNote.value.trim() }).then(response => {
        answerError.value = !response.data?.success;
    }).catch(() => {
        answerError.value = true;
    }).finally(() => {
        answering.value = false;
        showDeclineDialog.value = false;
        load();
    });
}

load();
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
