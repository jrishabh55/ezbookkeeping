<template>
    <invoice-document-content :invoice="invoice" :profile="profile" />
    <!-- a print-only copy outside the app's transformed dialogs / pages, so it prints from the top and flows across pages -->
    <teleport to="body">
        <div class="invoice-print-root">
            <invoice-document-content :invoice="invoice" :profile="profile" />
        </div>
    </teleport>
</template>

<script setup lang="ts">
import InvoiceDocumentContent from './InvoiceDocumentContent.vue';

import type { Invoice, InvoiceProfile } from '@/models/invoice.ts';

defineProps<{
    invoice: Invoice;
    profile: InvoiceProfile;
}>();
</script>

<style>
/* Not scoped: the print root lives directly under body. */
.invoice-print-root {
    display: none;
}

@media print {
    /* undo the scroll lock an open dialog puts on html (position: fixed), which would clip printing to one page */
    html, body {
        position: static !important;
        top: auto !important;
        height: auto !important;
        overflow: visible !important;
        background: #fff !important;
    }

    body > *:not(.invoice-print-root) {
        display: none !important;
    }

    body > .invoice-print-root {
        display: block;
    }

    /* if a hidden page below still holds an older document, print only the newest one */
    body > .invoice-print-root:has(~ .invoice-print-root) {
        display: none;
    }

    body > .invoice-print-root .invoice-document {
        min-width: 0;
        min-height: 0;
        max-width: none;
        padding: 0;
    }

    @page {
        margin: 12mm;
    }
}
</style>
