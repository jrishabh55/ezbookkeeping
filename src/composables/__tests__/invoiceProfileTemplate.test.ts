import { describe, expect, test } from 'vitest';

import { InvoiceProfile } from '@/models/invoice.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';

describe('invoice profile template validation', () => {
    test('blocks saving until the current template preview succeeds', () => {
        const state = useInvoiceProfilePageBase();
        state.setProfile(InvoiceProfile.createNewProfile());
        expect(state.inputIsInvalid.value).toBe(true);
        state.emailTemplateValid.value = true;
        expect(state.inputIsInvalid.value).toBe(false);
    });

    test('validates subject and serialized body length before saving', () => {
        const state = useInvoiceProfilePageBase();
        state.emailTemplateValid.value = true;
        state.newProfile.value.emailSubjectTemplate = 'a'.repeat(257);
        expect(state.inputIsInvalid.value).toBe(true);
        state.newProfile.value.emailSubjectTemplate = 'a'.repeat(256);
        state.newProfile.value.emailBodyTemplate = 'a'.repeat(32001);
        expect(state.inputIsInvalid.value).toBe(true);
        state.newProfile.value.emailBodyTemplate = 'a'.repeat(32000);
        expect(state.inputIsInvalid.value).toBe(false);
    });

    test('accepts an astral Unicode character at the subject limit and rejects one code point beyond it', () => {
        const state = useInvoiceProfilePageBase();
        state.emailTemplateValid.value = true;
        state.newProfile.value.emailSubjectTemplate = 'a'.repeat(255) + '😀';
        expect(state.inputIsInvalid.value).toBe(false);
        state.newProfile.value.emailSubjectTemplate += 'b';
        expect(state.inputIsInvalid.value).toBe(true);
    });

    test('counts serialized body markup and astral Unicode as code points at the body limit', () => {
        const state = useInvoiceProfilePageBase();
        state.emailTemplateValid.value = true;
        // Seven markup characters, 31992 ASCII characters and one emoji = 32000 code points.
        state.newProfile.value.emailBodyTemplate = '<p>' + 'a'.repeat(31992) + '😀</p>';
        expect(state.inputIsInvalid.value).toBe(false);
        state.newProfile.value.emailBodyTemplate += 'b';
        expect(state.inputIsInvalid.value).toBe(true);
    });

    test('reset restores unresolved template keys and participates in dirty state', () => {
        const profile = InvoiceProfile.createNewProfile();
        profile.emailSubjectTemplate = '{{invoice_number}}';
        profile.emailBodyTemplate = '<p>{{customer_name}}</p>';
        const state = useInvoiceProfilePageBase();
        state.setProfile(profile);
        state.newProfile.value.emailBodyTemplate = '';
        expect(state.inputIsNotChanged.value).toBe(false);
        state.reset();
        expect(state.newProfile.value.emailBodyTemplate).toBe('<p>{{customer_name}}</p>');
        expect(state.inputIsNotChanged.value).toBe(true);
    });
});
