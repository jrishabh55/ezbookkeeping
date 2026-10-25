<template>
    <div class="invoice-document">
        <div class="invoice-header">
            <img class="invoice-logo" :src="profile.logoDataUrl" alt="" v-if="profile.logoDataUrl" />
            <div v-else></div>
            <div class="invoice-business">
                <div class="invoice-strong">{{ profile.businessName }}</div>
                <div class="invoice-pre">{{ profile.addressLines }}</div>
                <div class="invoice-gap" v-if="profile.phone || profile.website"></div>
                <div v-if="profile.phone">Mobile: {{ profile.phone }}</div>
                <div v-if="profile.website">{{ profile.website }}</div>
            </div>
        </div>

        <div class="invoice-banner">
            <div class="invoice-rule"></div>
            <div class="invoice-title">Invoice</div>
            <div class="invoice-rule"></div>
        </div>

        <div class="invoice-parties">
            <div>
                <div class="invoice-muted">BILL TO</div>
                <div class="invoice-strong">{{ invoice.billToName }}</div>
                <div v-if="invoice.billToContactName">{{ invoice.billToContactName }}</div>
                <div class="invoice-pre">{{ invoice.billToAddressLines }}</div>
                <div class="invoice-gap" v-if="invoice.billToEmail"></div>
                <div v-if="invoice.billToEmail">{{ invoice.billToEmail }}</div>
            </div>
            <table class="invoice-meta">
                <tbody>
                <tr><th>Invoice Number:</th><td>{{ invoice.number }}</td></tr>
                <tr><th>Invoice Date:</th><td>{{ formatInvoiceDocumentDate(invoice.invoiceDate) }}</td></tr>
                <tr><th>Payment Due:</th><td>{{ formatInvoiceDocumentDate(invoice.dueDate) }}</td></tr>
                <tr class="invoice-meta-highlight"><th>Amount Due ({{ invoice.currency }}):</th><td class="invoice-strong">{{ formatInvoiceDocumentAmount(balanceDue) }}</td></tr>
                </tbody>
            </table>
        </div>

        <table class="invoice-lines">
            <thead>
            <tr>
                <th class="invoice-left">Services</th>
                <th class="invoice-right" v-if="showQuantity">Quantity</th>
                <th class="invoice-right">Rate</th>
                <th class="invoice-right">Amount</th>
            </tr>
            </thead>
            <tbody>
            <tr :key="index" v-for="(line, index) in invoice.lines">
                <td>
                    <div class="invoice-strong">{{ line.itemName }}</div>
                    <div class="invoice-muted invoice-pre">{{ line.description }}</div>
                </td>
                <td class="invoice-right" v-if="showQuantity">{{ line.quantity }}</td>
                <td class="invoice-right">{{ formatInvoiceDocumentAmount(line.rate) }}</td>
                <td class="invoice-right">{{ formatInvoiceDocumentAmount(line.amount) }}</td>
            </tr>
            </tbody>
        </table>

        <table class="invoice-totals">
            <tbody>
            <tr><th>Total:</th><td>{{ formatInvoiceDocumentAmount(invoice.total) }}</td></tr>
            <tr v-if="invoice.received > 0"><th>Payments received:</th><td>−{{ formatInvoiceDocumentAmount(Math.min(invoice.received, invoice.total)) }}</td></tr>
            <tr class="invoice-totals-due"><th>Amount Due ({{ invoice.currency }}):</th><td>{{ formatInvoiceDocumentAmount(balanceDue) }}</td></tr>
            </tbody>
        </table>

        <div class="invoice-notes" v-if="invoice.notes">
            <div class="invoice-muted invoice-strong">Notes / Terms</div>
            <div class="invoice-muted invoice-pre">{{ invoice.notes }}</div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import type { Invoice, InvoiceProfile } from '@/models/invoice.ts';
import { formatInvoiceDocumentAmount, formatInvoiceDocumentDate, getInvoiceDisplayBalanceDue } from '@/lib/invoice.ts';

const props = defineProps<{
    invoice: Invoice;
    profile: InvoiceProfile;
}>();

// a paid invoice owes nothing, even when a shortfall was written off
const balanceDue = computed<number>(() => getInvoiceDisplayBalanceDue(props.invoice));
const showQuantity = computed<boolean>(() => props.invoice.lines.some(line => line.quantity !== 1));
</script>

<style scoped>
/* A document for the customer: always white paper and black text, whatever the app theme or language. */
.invoice-document {
    --invoice-accent: #3d86f5;
    background: #fff;
    color: #000;
    font-family: Georgia, 'Noto Serif', 'Times New Roman', serif; /* these fonts carry the ₹ glyph */
    font-size: 13px;
    line-height: 1.35;
    max-width: 800px;
    min-width: 640px;
    margin: 0 auto;
    padding: 32px;
}

.invoice-header, .invoice-parties {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
}

.invoice-logo {
    max-height: 48px;
    max-width: 320px;
}

.invoice-business {
    text-align: right;
}

.invoice-strong {
    font-weight: 700;
}

.invoice-muted {
    color: #6b7280;
}

.invoice-pre {
    white-space: pre-line;
}

.invoice-gap {
    height: 12px;
}

.invoice-left {
    text-align: left;
}

.invoice-right {
    text-align: right;
}

.invoice-banner {
    display: flex;
    align-items: center;
    gap: 16px;
    margin: 40px 0 56px;
}

.invoice-rule {
    flex: 1;
    border-top: 2px solid var(--invoice-accent);
    border-bottom: 2px solid var(--invoice-accent);
    height: 6px;
}

.invoice-title {
    color: var(--invoice-accent);
    font-size: 30px;
    border: 2px solid var(--invoice-accent);
    border-radius: 6px;
    outline: 2px solid var(--invoice-accent);
    outline-offset: 3px;
    padding: 6px 34px;
}

.invoice-meta th {
    text-align: right;
    padding: 2px 12px 2px 16px;
}

.invoice-meta td {
    padding: 2px 16px 2px 0;
}

.invoice-meta-highlight {
    background: #f3f4f6;
}

.invoice-lines {
    width: 100%;
    border-collapse: collapse;
    margin-top: 40px;
}

.invoice-lines th {
    padding: 8px 0;
    border-bottom: 1px dotted #000;
}

.invoice-lines td {
    padding: 12px 0;
    border-bottom: 1px dotted #000;
    vertical-align: top;
}

.invoice-totals {
    margin: 16px 0 0 auto;
    border-collapse: collapse;
}

.invoice-totals th {
    text-align: right;
    padding: 8px 48px 8px 0;
}

.invoice-totals td {
    text-align: right;
    padding: 8px 0;
}

.invoice-totals-due {
    border-top: 3px solid #e5e7eb;
    font-weight: 700;
}

.invoice-notes {
    margin-top: 56px;
}
</style>

<style>
/* Not scoped: when printing, hide the whole app (desktop dialog overlay or mobile page) and show only the document. */
@media print {
    body * {
        visibility: hidden;
    }

    .invoice-document, .invoice-document * {
        visibility: visible;
    }

    .invoice-document {
        position: fixed;
        inset: 0;
        min-width: 0;
        max-width: none;
    }

    @page {
        margin: 12mm;
    }
}
</style>
