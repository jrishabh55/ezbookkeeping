import { computed, ref, watch } from 'vue';

import type { InvoiceEmailTemplateUpdateRequest, InvoiceProfile } from '@/models/invoice.ts';

export function useInvoiceEmailTemplatePageBase() {
    const loading = ref(true);
    const saving = ref(false);
    const newTemplate = ref<InvoiceEmailTemplateUpdateRequest>({ subject: '', bodyHtml: '' });
    const oldTemplate = ref<InvoiceEmailTemplateUpdateRequest>({ subject: '', bodyHtml: '' });
    const emailTemplateValid = ref(false);

    watch([() => newTemplate.value.subject, () => newTemplate.value.bodyHtml], () => {
        emailTemplateValid.value = false;
    }, { flush: 'sync' });

    const inputIsNotChanged = computed(() => newTemplate.value.subject === oldTemplate.value.subject && newTemplate.value.bodyHtml === oldTemplate.value.bodyHtml);
    const inputInvalidProblemMessage = computed<string | null>(() => {
        if (!newTemplate.value.subject.trim()) {
            return 'Email subject cannot be blank';
        } else if (Array.from(newTemplate.value.subject).length > 256) {
            return 'Email subject must be at most 256 characters';
        } else if (Array.from(newTemplate.value.bodyHtml).length > 32000) {
            return 'Email body must be at most 32000 characters';
        } else if (!emailTemplateValid.value) {
            return 'Preview the current email before saving or sending';
        }

        return null;
    });
    const inputIsInvalid = computed(() => !!inputInvalidProblemMessage.value);

    function setProfile(profile: InvoiceProfile): void {
        const template = { subject: profile.emailSubjectTemplate, bodyHtml: profile.emailBodyTemplate };
        newTemplate.value = { ...template };
        oldTemplate.value = { ...template };
    }

    function reset(): void {
        newTemplate.value = { ...oldTemplate.value };
    }

    return { loading, saving, newTemplate, emailTemplateValid, inputIsNotChanged, inputInvalidProblemMessage, inputIsInvalid, setProfile, reset };
}
