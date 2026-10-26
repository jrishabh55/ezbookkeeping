import { describe, expect, test } from 'vitest';

import { Invoice, InvoiceProfile } from '@/models/invoice.ts';

describe('InvoiceProfile email templates', () => {
    test('cloning and saving preserves template variables rather than sample values', () => {
        const profile = InvoiceProfile.of({
            ...InvoiceProfile.createNewProfile().toUpdateRequest(),
            emailSubjectTemplate: 'Invoice {{invoice_number}} for {{customer_name}}',
            emailBodyTemplate: '<p>Hello <span data-invoice-variable="contact_name">{{contact_name}}</span></p>'
        });
        const saved = profile.clone().toUpdateRequest();
        expect(saved.emailSubjectTemplate).toBe('Invoice {{invoice_number}} for {{customer_name}}');
        expect(saved.emailBodyTemplate).toBe('<p>Hello <span data-invoice-variable="contact_name">{{contact_name}}</span></p>');
    });

    test('an intentionally empty body survives clone and update serialization', () => {
        const profile = InvoiceProfile.of({
            ...InvoiceProfile.createNewProfile().toUpdateRequest(),
            emailSubjectTemplate: 'Invoice {{invoice_number}}',
            emailBodyTemplate: ''
        });
        expect(profile.clone().toUpdateRequest().emailBodyTemplate).toBe('');
        expect(profile.clone().toUpdateRequest().emailSubjectTemplate).toBe('Invoice {{invoice_number}}');
    });

    test('responses predating email templates still load', () => {
        const legacy = InvoiceProfile.createNewProfile().toUpdateRequest();
        const profile = InvoiceProfile.of({ ...legacy, emailSubjectTemplate: undefined, emailBodyTemplate: undefined } as unknown as InvoiceProfile);
        expect(profile.emailSubjectTemplate).toBe('');
        expect(profile.emailBodyTemplate).toBe('');
    });
});

describe('InvoiceProfile logo visibility', () => {
    test('cloning and saving a hidden logo preserves its uploaded image', () => {
        const profile = InvoiceProfile.createNewProfile();
        profile.logoDataUrl = 'data:image/png;base64,stored-upload';
        profile.hideLogo = true;
        const saved = profile.clone().toUpdateRequest();
        expect(saved.hideLogo).toBe(true);
        expect(saved.logoDataUrl).toBe('data:image/png;base64,stored-upload');
        const shown = InvoiceProfile.of(saved);
        shown.hideLogo = false;
        expect(shown.toUpdateRequest().hideLogo).toBe(false);
        expect(shown.logoDataUrl).toBe('data:image/png;base64,stored-upload');
    });

    test('old profiles show their uploaded logo by default', () => {
        const profile = InvoiceProfile.createNewProfile();
        expect(profile.hideLogo).toBe(false);
        const oldResponse = { ...profile.toUpdateRequest(), hideLogo: undefined };
        expect(InvoiceProfile.of(oldResponse as unknown as InvoiceProfile).hideLogo).toBe(false);
    });
});

describe('Invoice', () => {
    test('a new line has quantity 1 stored as 100 hundredths', () => {
        expect(Invoice.createNewLine().quantity).toBe(100);
        expect(Invoice.createNewInvoice().lines[0]!.quantity).toBe(100);
    });

    test('duplicate keeps hundredths as they are', () => {
        const source = Invoice.createNewInvoice();
        source.lines[0]!.quantity = 750;
        expect(source.duplicate().lines[0]!.quantity).toBe(750);
    });
});
