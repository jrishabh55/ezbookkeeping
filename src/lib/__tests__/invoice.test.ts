import { describe, expect, test } from 'vitest';

import {
    computeInvoiceTotals,
    formatInvoiceQuantity,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    isInvoiceOverdue,
    getInvoiceDisplayStatus,
    getInvoiceShortfall,
    getInvoiceDisplayBalanceDue,
    formatInvoiceDocumentAmount,
    formatInvoiceDocumentDate,
    getInvoicePdfFileName,
    addDaysToDate,
    getDaysBetweenDates
} from '@/lib/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const invoice = (total: number, received: number, closedManually = false, dueDate: TextualYearMonthDay = '2026-04-30') =>
    ({ total, received, closedManually, dueDate });

describe('computeInvoiceTotals', () => {
    test('sums quantity (hundredths) × rate', () => {
        expect(computeInvoiceTotals([{ quantity: 100, rate: 37000000 }, { quantity: 300, rate: 150000 }])).toEqual({ subtotal: 37450000, total: 37450000 });
        expect(computeInvoiceTotals([])).toEqual({ subtotal: 0, total: 0 });
    });

    test('rounds a fractional quantity to the paisa', () => {
        expect(computeInvoiceTotals([{ quantity: 750, rate: 33333 }])).toEqual({ subtotal: 249998, total: 249998 });
        expect(computeInvoiceTotals([{ quantity: 1, rate: 1 }])).toEqual({ subtotal: 0, total: 0 });
    });
});

describe('formatInvoiceQuantity', () => {
    test('trims trailing zeros', () => {
        expect(formatInvoiceQuantity(100)).toBe('1');
        expect(formatInvoiceQuantity(750)).toBe('7.5');
        expect(formatInvoiceQuantity(225)).toBe('2.25');
        expect(formatInvoiceQuantity(755)).toBe('7.55');
        expect(formatInvoiceQuantity(1000000)).toBe('10000');
    });
});

describe('getInvoiceStatus', () => {
    test('unpaid with nothing received', () => {
        expect(getInvoiceStatus(invoice(37000000, 0))).toBe('unpaid');
    });

    test('paid when exactly 3% short', () => {
        expect(getInvoiceStatus(invoice(37000000, 35890000))).toBe('paid');
        expect(getInvoiceShortfall(invoice(37000000, 35890000))).toEqual({ amount: 1110000, kind: 'bankCharges' });
    });

    test('partial when one paisa more than 3% short', () => {
        expect(getInvoiceStatus(invoice(37000000, 35889999))).toBe('partial');
        expect(getInvoiceShortfall(invoice(37000000, 35889999))).toEqual({ amount: 0, kind: null });
    });

    test('paid when closed manually even with a big shortfall', () => {
        expect(getInvoiceStatus(invoice(37000000, 10000000, true))).toBe('paid');
        expect(getInvoiceShortfall(invoice(37000000, 10000000, true))).toEqual({ amount: 27000000, kind: 'writtenOff' });
    });

    test('balance due never negative after editing the total below what was received', () => {
        expect(getInvoiceBalanceDue(invoice(30000000, 37000000))).toBe(0);
        expect(getInvoiceStatus(invoice(30000000, 37000000))).toBe('paid');
        expect(getInvoiceShortfall(invoice(30000000, 37000000))).toEqual({ amount: 0, kind: null });
    });
});

describe('getInvoiceShortfall', () => {
    test('bank charges when a manual close is within the tolerance', () => {
        expect(getInvoiceShortfall(invoice(37000000, 36850000, true))).toEqual({ amount: 150000, kind: 'bankCharges' });
    });

    test('none when unpaid', () => {
        expect(getInvoiceShortfall(invoice(37000000, 0))).toEqual({ amount: 0, kind: null });
    });
});

describe('getInvoiceDisplayBalanceDue', () => {
    test('zero once paid, the gap otherwise', () => {
        expect(getInvoiceDisplayBalanceDue(invoice(37000000, 36850000))).toBe(0);
        expect(getInvoiceDisplayBalanceDue(invoice(37000000, 10000000, true))).toBe(0);
        expect(getInvoiceDisplayBalanceDue(invoice(37000000, 10000000))).toBe(27000000);
    });
});

describe('overdue', () => {
    test('only unpaid or partial invoices past the due date', () => {
        expect(isInvoiceOverdue(invoice(100, 0), '2026-05-01')).toBe(true);
        expect(isInvoiceOverdue(invoice(100, 0), '2026-04-30')).toBe(false);
        expect(isInvoiceOverdue(invoice(100, 100), '2026-05-01')).toBe(false);
        expect(getInvoiceDisplayStatus(invoice(100, 50), '2026-05-01')).toBe('overdue');
        expect(getInvoiceDisplayStatus(invoice(100, 50), '2026-04-01')).toBe('partial');
    });
});

describe('document formatting', () => {
    test('amount like the Wave sample', () => {
        expect(formatInvoiceDocumentAmount(37000000)).toBe('₹370,000.00');
    });

    test('date like the Wave sample', () => {
        expect(formatInvoiceDocumentDate('2026-04-23')).toBe('April 23, 2026');
    });

    test('pdf file name like the Wave sample', () => {
        expect(getInvoicePdfFileName('CA/2026-27/01', '2026-04-30')).toBe('Invoice_CA_2026-27_01_2026-04-30');
    });

    test('add days across month end', () => {
        expect(addDaysToDate('2026-04-23', 7)).toBe('2026-04-30');
        expect(addDaysToDate('2026-04-28', 7)).toBe('2026-05-05');
    });

    test('days between dates across month end', () => {
        expect(getDaysBetweenDates('2026-04-28', '2026-05-05')).toBe(7);
        expect(getDaysBetweenDates('2026-04-23', '2026-04-23')).toBe(0);
    });
});
