<template>
    <invoice-email-editor :model-value="modelValue" :variables="variables" :disabled="disabled" :aria-label="ariaLabel"
                          @update:model-value="$emit('update:modelValue', $event)" />
</template>
<script setup lang="ts">
import InvoiceEmailEditor from './InvoiceEmailEditor.vue';
import type { InvoiceEmailEditorVariable } from '@/lib/invoice_email_editor.ts';
defineProps<{ modelValue: string; variables?: readonly InvoiceEmailEditorVariable[]; disabled?: boolean; ariaLabel?: string }>();
defineEmits<{ (e: 'update:modelValue', value: string): void }>();
</script>
