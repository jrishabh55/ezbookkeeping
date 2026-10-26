import { describe, expect, test } from 'vitest';

import { readInvoicePdf } from '@/lib/invoicePdf.ts';

describe('invoice PDF download', () => {
    test('returns the actual PDF bytes for saving as a file', async () => {
        const pdf = new Blob(['%PDF-1.7\ninvoice document'], { type: 'application/pdf' });
        expect(await (await readInvoicePdf(pdf)).text()).toBe('%PDF-1.7\ninvoice document');
    });

    test.each([
        ['HTML login page', '<html>Sign in</html>', 'text/html'],
        ['JSON error', '{"success":false}', 'application/json'],
        ['malformed PDF response', 'Operation failed', 'application/pdf'],
        ['HTML served with PDF-looking text', '%PDF- is not a document here', 'text/html']
    ])('rejects %s instead of downloading a fake PDF', async (_name, content, type) => {
        await expect(readInvoicePdf(new Blob([content], { type }))).rejects.toMatchObject({ message: 'Unable to download invoice PDF' });
    });
});
