<template>
    <div class="public-invoice-page">
        <div class="public-invoice-banner" :class="`public-invoice-banner-${status}`" role="status" v-if="invoice && !isEstimate">
            <v-icon :icon="statusIcon" />
            <span class="public-invoice-banner-title">{{ statusTitle }}</span>
            <span class="public-invoice-banner-detail" v-if="statusDetail">{{ statusDetail }}</span>
        </div>
        <div class="public-invoice-toolbar" v-if="invoice">
            <span class="public-invoice-status" v-if="isEstimate">
                {{ estimateStatusText || tt('Total') + ': ' + formatInvoiceDocumentAmount(invoice.total, invoice.currency) }}
            </span>
            <a class="public-invoice-latest-link" :href="latestVersionUrl" v-if="latestVersionUrl">{{ tt('View the latest version') }}</a>
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

import { mdiPrinterOutline, mdiDownloadOutline, mdiCheckCircle, mdiAlertCircle, mdiClockOutline, mdiProgressCheck } from '@mdi/js';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';

import { useI18n } from '@/locales/helpers.ts';

import { Invoice, InvoiceProfile } from '@/models/invoice.ts';
import type { InvoiceDisplayStatus } from '@/lib/invoice.ts';

import services from '@/lib/services.ts';
import { readInvoicePdf } from '@/lib/invoicePdf.ts';
import { startDownloadFile } from '@/lib/ui/common.ts';
import {
    formatInvoiceDocumentAmount,
    formatInvoiceDocumentDate,
    getInvoicePdfFileName,
    getInvoiceDisplayBalanceDue,
    getInvoiceDisplayStatus,
    getDaysBetweenDates,
    getInvoicePublicLinkUrl,
    getTodayTextualDate,
    printInvoiceDocument
} from '@/lib/invoice.ts';
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
// a superseded version: the public token of the latest version that was sent, empty when there is none to show
const supersededBy = ref<string>('');
const latestVersionUrl = computed<string>(() => supersededBy.value ? getInvoicePublicLinkUrl(supersededBy.value) : '');

const status = computed<InvoiceDisplayStatus>(() => invoice.value ? getInvoiceDisplayStatus(invoice.value, today) : 'unpaid');
const balanceDue = computed<number>(() => invoice.value ? getInvoiceDisplayBalanceDue(invoice.value) : 0);

const statusIcon = computed<string>(() => ({ paid: mdiCheckCircle, partial: mdiProgressCheck, overdue: mdiAlertCircle, unpaid: mdiClockOutline })[status.value]);
const statusTitle = computed<string>(() => tt(({ paid: 'Paid', partial: 'Partially Paid', overdue: 'Overdue', unpaid: 'Amount Due' })[status.value]));

// the banner's second half: when it was paid, or what is still owed and by when
const statusDetail = computed<string>(() => {
    const current = invoice.value;

    if (!current) {
        return '';
    }

    const due = formatInvoiceDocumentAmount(balanceDue.value, current.currency);
    const date = formatInvoiceDocumentDate(current.dueDate);

    switch (status.value) {
        case 'paid':
            return current.paidDate ? tt('format.misc.invoicePaidOn', { date: formatInvoiceDocumentDate(current.paidDate) }) : '';
        case 'overdue':
            return tt('format.misc.invoiceOverdueBy', { days: getDaysBetweenDates(current.dueDate, today), due });
        case 'partial':
            return tt('format.misc.invoicePartlyPaid', { received: formatInvoiceDocumentAmount(current.received, current.currency), due, date });
        default:
            return tt('format.misc.invoiceDueBy', { due, date });
    }
});

const isEstimate = computed<boolean>(() => invoice.value?.kind === 'estimate');
// the server checks the same: sent and still valid
const canAnswer = computed<boolean>(() => !!invoice.value && canAnswerEstimate(invoice.value, today));

const estimateStatusText = computed<string>(() => {
    const current = invoice.value;

    if (!current || !isEstimate.value) {
        return '';
    }

    if (current.estimateStatus === 'superseded') {
        return tt('This estimate has been revised.');
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
            supersededBy.value = data.result.supersededBy ?? '';
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

.public-invoice-banner {
    max-width: 864px;
    margin: 0 auto 16px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    padding: 14px 20px;
    border-radius: 8px;
    font-size: 16px;
    background: #f3f4f6;
    color: #111827;
    border: 1px solid #d1d5db;
}

.public-invoice-banner-title {
    font-size: 20px;
    font-weight: 700;
}

.public-invoice-banner-paid {
    background: #dcfce7;
    color: #14532d;
    border-color: #86efac;
}

.public-invoice-banner-partial {
    background: #fef3c7;
    color: #78350f;
    border-color: #fcd34d;
}

.public-invoice-banner-overdue {
    background: #fee2e2;
    color: #7f1d1d;
    border-color: #fca5a5;
}

.public-invoice-latest-link {
    color: #1d4ed8;
    font-weight: 600;
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
