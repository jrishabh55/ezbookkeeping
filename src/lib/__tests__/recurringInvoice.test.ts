import { describe, expect, test } from 'vitest';

import { getDefaultRecurringDayOfMonth, getNextRecurringDate } from '@/lib/recurringInvoice.ts';

describe('getDefaultRecurringDayOfMonth', () => {
    test('uses the invoice day of month capped at 28', () => {
        expect(getDefaultRecurringDayOfMonth('2026-04-23')).toBe(23);
        expect(getDefaultRecurringDayOfMonth('2026-01-31')).toBe(28);
        expect(getDefaultRecurringDayOfMonth('2026-02-01')).toBe(1);
    });

    test('falls back to 1 for an unparsable date', () => {
        expect(getDefaultRecurringDayOfMonth('')).toBe(1);
    });
});

describe('getNextRecurringDate', () => {
    test('is the next occurrence of the day strictly after today', () => {
        expect(getNextRecurringDate(15, '2026-10-26')).toBe('2026-11-15');
        expect(getNextRecurringDate(28, '2026-10-26')).toBe('2026-10-28');
        expect(getNextRecurringDate(26, '2026-10-26')).toBe('2026-11-26');
        expect(getNextRecurringDate(5, '2026-12-26')).toBe('2027-01-05');
    });
});
