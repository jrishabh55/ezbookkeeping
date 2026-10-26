import { describe, expect, test } from 'vitest';

import { Invoice } from '@/models/invoice.ts';

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
