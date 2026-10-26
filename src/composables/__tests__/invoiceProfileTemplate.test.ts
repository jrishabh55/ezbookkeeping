import { describe, expect, test } from 'vitest';

import { InvoiceProfile } from '@/models/invoice.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';
import { useInvoiceEmailTemplatePageBase } from '@/views/base/invoices/InvoiceEmailTemplatePageBase.ts';

describe('dedicated invoice email template draft', () => {
    function ready() {
        const profile = InvoiceProfile.createNewProfile();
        profile.emailSubjectTemplate = 'Invoice {{invoice_number}}';
        profile.emailBodyTemplate = '<p>Hello {{customer_name}}</p>';
        const state = useInvoiceEmailTemplatePageBase();
        state.setProfile(profile);
        return state;
    }

    test('blocks template save until its current preview succeeds', () => {
        const state = ready();
        expect(state.inputIsInvalid.value).toBe(true);
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(false);
        state.newTemplate.value.subject = 'Updated {{invoice_number}}';
        expect(state.emailTemplateValid.value).toBe(false);
        expect(state.inputIsInvalid.value).toBe(true);
    });

    test('validates a nonblank subject and Unicode character limits independently of readiness', () => {
        const state = ready();
        state.newTemplate.value.subject = ' ';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(true);
        state.newTemplate.value.subject = 'a'.repeat(255) + '😀';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(false);
        state.newTemplate.value.subject += 'b';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(true);
        state.newTemplate.value.subject = 'Invoice';
        state.newTemplate.value.bodyHtml = '<p>' + 'a'.repeat(31992) + '😀</p>';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(false);
        state.newTemplate.value.bodyHtml += 'b';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(true);
    });

    test('reset restores the saved unresolved template without changing profile details', () => {
        const profile = InvoiceProfile.createNewProfile();
        profile.businessName = 'Business';
        profile.emailSubjectTemplate = '{{invoice_number}}';
        profile.emailBodyTemplate = '<p>{{customer_name}}</p>';
        const state = useInvoiceEmailTemplatePageBase();
        state.setProfile(profile);
        state.newTemplate.value.bodyHtml = '';
        expect(state.inputIsNotChanged.value).toBe(false);
        expect(profile.emailBodyTemplate).toBe('<p>{{customer_name}}</p>');
        state.reset();
        expect(state.newTemplate.value.bodyHtml).toBe('<p>{{customer_name}}</p>');
        expect(state.inputIsNotChanged.value).toBe(true);
        expect(Object.keys(state.newTemplate.value).sort()).toEqual(['bodyHtml', 'subject']);
        expect(profile.businessName).toBe('Business');
    });

    test('an intentionally empty rich body can be saved after preview', () => {
        const state = ready();
        state.newTemplate.value.bodyHtml = '';
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(false);
        expect(state.newTemplate.value.bodyHtml).toBe('');
    });
});

describe('business invoice profile independence', () => {
    test('business details are saveable without an email preview or template editor', () => {
        const state = useInvoiceProfilePageBase();
        state.setProfile(InvoiceProfile.createNewProfile());
        state.newProfile.value.businessName = 'Updated business';
        expect(state.inputIsInvalid.value).toBe(false);
        expect(state.inputIsNotChanged.value).toBe(false);
    });
});
