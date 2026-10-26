import { afterEach, describe, expect, test } from 'vitest';
import { effectScope, ref, type EffectScope } from 'vue';

import { useInvoiceEmailPreview } from '@/composables/invoiceEmailPreview.ts';
import { createInvoiceEmailSendRequest, getInvoiceEmailPreviewErrorMessage } from '@/lib/invoiceEmail.ts';
import type { InvoiceEmailPreviewResponse } from '@/models/invoice.ts';

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason: unknown) => void;
    const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
    return { promise, resolve, reject };
}

const response = (subject: string): InvoiceEmailPreviewResponse => ({
    subject, bodyHtml: '<p>Resolved</p>', emailHtml: '<html><body>Resolved</body></html>',
    variables: [], defaultSubjectTemplate: '{{invoice_number}}', defaultBodyTemplate: '<p>Default</p>', sample: true
});
let scope: EffectScope;
afterEach(() => scope?.stop());

describe('invoice email preview readiness', () => {
    test('new edits invalidate immediately and out-of-order results cannot enable an older email', async () => {
        const subject = ref('first');
        const first = deferred<InvoiceEmailPreviewResponse>();
        const second = deferred<InvoiceEmailPreviewResponse>();
        scope = effectScope();
        const state = scope.run(() => useInvoiceEmailPreview(() => ({ subject: subject.value, bodyHtml: '' }), req => req.subject === 'first' ? first.promise : second.promise, 0))!;
        expect(state.pending.value).toBe(true);
        subject.value = 'second';
        expect(state.valid.value).toBe(false);
        first.resolve(response('first'));
        await first.promise;
        await Promise.resolve();
        expect(state.valid.value).toBe(false);
        second.resolve(response('second'));
        await second.promise;
        await Promise.resolve();
        expect(state.preview.value?.subject).toBe('second');
        expect(state.valid.value).toBe(true);
        expect(state.pending.value).toBe(false);
        subject.value = 'third';
        expect(state.valid.value).toBe(false);
    });

    test('an older successful response cannot overwrite the newest completed preview', async () => {
        const subject = ref('older');
        const older = deferred<InvoiceEmailPreviewResponse>();
        const newer = deferred<InvoiceEmailPreviewResponse>();
        scope = effectScope();
        const state = scope.run(() => useInvoiceEmailPreview(() => ({ subject: subject.value }), req => req.subject === 'older' ? older.promise : newer.promise, 0))!;
        subject.value = 'newer';
        newer.resolve(response('newer'));
        await newer.promise;
        await Promise.resolve();
        older.resolve(response('older'));
        await older.promise;
        await Promise.resolve();
        expect(state.preview.value?.subject).toBe('newer');
        expect(state.valid.value).toBe(true);
    });

    test('latest errors clear readiness and stale errors cannot replace a successful preview', async () => {
        const subject = ref('first');
        const first = deferred<InvoiceEmailPreviewResponse>();
        const second = deferred<InvoiceEmailPreviewResponse>();
        scope = effectScope();
        const state = scope.run(() => useInvoiceEmailPreview(() => ({ subject: subject.value }), req => req.subject === 'first' ? first.promise : second.promise, 0))!;
        subject.value = 'second';
        second.resolve(response('second'));
        await second.promise;
        await Promise.resolve();
        first.reject({ message: 'Stale error' });
        await first.promise.catch(() => undefined);
        await Promise.resolve();
        expect(state.error.value).toBeNull();
        expect(state.valid.value).toBe(true);
        subject.value = 'first';
        await Promise.resolve();
        await Promise.resolve();
        expect(state.error.value).toEqual({ message: 'Stale error' });
        expect(state.valid.value).toBe(false);
        expect(state.pending.value).toBe(false);
    });

    test('disposing while a preview is pending ignores its result', async () => {
        const result = deferred<InvoiceEmailPreviewResponse>();
        scope = effectScope();
        const state = scope.run(() => useInvoiceEmailPreview(() => ({ subject: 'invoice' }), () => result.promise, 0))!;
        expect(state.pending.value).toBe(true);
        scope.stop();
        result.resolve(response('invoice'));
        await result.promise;
        await Promise.resolve();
        expect(state.preview.value).toBeNull();
        expect(state.valid.value).toBe(false);
    });
});

describe('invoice email compose request', () => {
    test('keeps saved variables unresolved and preserves an intentionally empty rich body', () => {
        const profile = { emailSubjectTemplate: '{{invoice_number}} for {{customer_name}}', emailBodyTemplate: '' };
        const request = createInvoiceEmailSendRequest({ id: '42', billToEmail: 'customer@example.com' }, profile);
        expect(request).toEqual({ id: '42', to: 'customer@example.com', subject: '{{invoice_number}} for {{customer_name}}', bodyHtml: '', message: '' });
        const edited = { ...request, subject: 'One send only', bodyHtml: '<p>Edited</p>' };
        expect(edited.bodyHtml).toBe('<p>Edited</p>');
        expect(profile).toEqual({ emailSubjectTemplate: '{{invoice_number}} for {{customer_name}}', emailBodyTemplate: '' });
    });
});

describe('invoice email renderer errors', () => {
    test('shows the renderer explanation from the API error envelope', () => {
        expect(getInvoiceEmailPreviewErrorMessage({ error: { errorCode: 400, errorMessage: 'Unknown invoice email variable: customer_naem' } })).toBe('Unknown invoice email variable: customer_naem');
        expect(getInvoiceEmailPreviewErrorMessage({ message: 'Network Error' })).toBe('Network Error');
        expect(getInvoiceEmailPreviewErrorMessage('Preview failed')).toBe('Preview failed');
        expect(getInvoiceEmailPreviewErrorMessage({ processed: true })).toBe('Unable to preview invoice email');
    });
});
