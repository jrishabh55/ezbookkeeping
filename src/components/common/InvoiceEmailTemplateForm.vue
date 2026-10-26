<template>
    <section class="invoice-email-template-form" :aria-label="tt('Invoice Email Template')">
        <div class="invoice-email-template-layout" :class="{ 'invoice-email-template-split': split }">
            <div class="invoice-email-draft">
                <label>{{ tt('Subject') }}</label>
                <invoice-email-subject-editor :model-value="subject" :variables="variables" :disabled="disabled"
                                              :aria-label="tt('Subject')" @update:model-value="updateSubject" />
                <label class="invoice-email-body-label">{{ tt('Message') }}</label>
                <invoice-email-body-editor :model-value="body" :variables="variables" :disabled="disabled"
                                           :aria-label="tt('Message')" @update:model-value="updateBody" />
                <button type="button" class="invoice-email-template-button" :disabled="disabled || !preview"
                        @click="restoreDefaults" v-if="allowRestore">{{ tt('Restore Default Template') }}</button>
            </div>
            <div class="invoice-email-preview" aria-live="polite" :aria-busy="pending">
                <h3>{{ tt('Email Preview') }}</h3>
                <p v-if="pending">{{ tt('Preparing email preview…') }}</p>
                <div role="alert" v-else-if="error">
                    <p>{{ errorMessage }}</p>
                    <button type="button" class="invoice-email-template-button" :disabled="disabled" @click="refresh">{{ tt('Retry') }}</button>
                </div>
                <template v-else-if="valid && preview">
                    <p v-if="preview.sample">{{ tt('Sample invoice preview') }}</p>
                    <p class="invoice-email-preview-subject"><strong>{{ tt('Subject') }}:</strong> {{ preview.subject }}</p>
                    <iframe class="invoice-email-preview-frame" sandbox="" referrerpolicy="no-referrer"
                            :title="tt('Email Preview')" :srcdoc="preview.emailHtml"></iframe>
                </template>
            </div>
        </div>
    </section>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, watch } from 'vue';

// The rich text editor is only needed on the send and template screens, so it loads on demand.
const InvoiceEmailBodyEditor = defineAsyncComponent(() => import('@/components/common/InvoiceEmailBodyEditor.vue'));
const InvoiceEmailSubjectEditor = defineAsyncComponent(() => import('@/components/common/InvoiceEmailSubjectEditor.vue'));

import { useI18n } from '@/locales/helpers.ts';
import { useInvoicesStore } from '@/stores/invoice.ts';
import { INVOICE_EMAIL_EDITOR_VARIABLES, getInvoiceEmailPreviewErrorMessage } from '@/lib/invoiceEmail.ts';
import { useInvoiceEmailPreview } from '@/composables/invoiceEmailPreview.ts';

const props = defineProps<{
    invoiceId?: string;
    disabled?: boolean;
    allowRestore?: boolean;
    split?: boolean;
}>();
const subject = defineModel<string>('subject', { required: true });
const body = defineModel<string>('body', { required: true });
const emit = defineEmits<{ validityChange: [valid: boolean] }>();
const { tt } = useI18n();
const invoicesStore = useInvoicesStore();
const { pending, valid, error, preview, refresh } = useInvoiceEmailPreview(
    () => ({ invoiceId: props.invoiceId, subject: subject.value, bodyHtml: body.value }),
    request => invoicesStore.previewInvoiceEmail(request)
);
const variables = computed(() => (preview.value?.variables ?? INVOICE_EMAIL_EDITOR_VARIABLES).map(variable => ({ key: variable.key, label: tt(variable.label) })));
const errorMessage = computed(() => tt(getInvoiceEmailPreviewErrorMessage(error.value)));

watch(valid, value => emit('validityChange', value), { immediate: true, flush: 'sync' });

function updateSubject(value: string): void {
    if (!props.disabled && value !== subject.value) {
        emit('validityChange', false);
        subject.value = value;
    }
}

function updateBody(value: string): void {
    if (!props.disabled && value !== body.value) {
        emit('validityChange', false);
        body.value = value;
    }
}

function restoreDefaults(): void {
    if (!props.disabled && preview.value) {
        updateSubject(preview.value.defaultSubjectTemplate);
        updateBody(preview.value.defaultBodyTemplate);
    }
}
</script>

<style scoped>
.invoice-email-template-form {
    min-width: 0;
    width: 100%;
    /* Split on the form's own width, so it works inside dialogs and the settings layout alike. */
    container-type: inline-size;
}
.invoice-email-draft {
    min-width: 0;
}
.invoice-email-draft > label {
    display: block;
    font-weight: 600;
    margin-bottom: 0.5rem;
}
.invoice-email-body-label {
    margin-top: 1rem;
}
.invoice-email-template-button {
    border: 1px solid currentColor;
    border-radius: 0.35rem;
    color: inherit;
    background: transparent;
    cursor: pointer;
    padding: 0.5rem 0.75rem;
    margin-top: 0.75rem;
    font: inherit;
}
.invoice-email-template-button:disabled {
    cursor: default;
    opacity: 0.5;
}
.invoice-email-preview {
    margin-top: 1.5rem;
}
.invoice-email-preview h3 {
    font-size: 1rem;
}
.invoice-email-preview-subject {
    overflow-wrap: anywhere;
}
.invoice-email-preview-frame {
    display: block;
    box-sizing: border-box;
    width: 100%;
    height: 520px;
    border: 1px solid #d0d0d0;
    border-radius: 0.35rem;
    background: white;
}
@container (min-width: 760px) {
    .invoice-email-template-split {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
        gap: 1.5rem;
        align-items: start;
    }
    .invoice-email-template-split .invoice-email-preview {
        min-width: 0;
        margin-top: 0;
    }
    .invoice-email-template-split .invoice-email-preview h3 {
        margin-bottom: 0.5rem;
    }
    .invoice-email-template-split .invoice-email-preview-frame {
        height: max(520px, calc(100vh - 280px));
    }
}
</style>
