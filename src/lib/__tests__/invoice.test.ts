import { describe, expect, test } from 'vitest';

import {
    computeInvoiceTotals,
    formatInvoiceQuantity,
    formatInvoiceTaxRate,
    getInvoiceTaxRows,
    getInvoiceBalanceDue,
    getInvoiceStatus,
    isInvoiceOverdue,
    getInvoiceDisplayStatus,
    getInvoiceShortfall,
    getInvoiceDisplayBalanceDue,
    getInvoiceOutstandingByCurrency,
    getInvoiceOverdueByCurrency,
    formatInvoiceDocumentAmount,
    formatInvoiceDocumentDate,
    getInvoicePdfFileName,
    addDaysToDate,
    getDaysBetweenDates
} from '@/lib/invoice.ts';
import type { TextualYearMonthDay } from '@/core/datetime.ts';

const invoice = (total: number, received: number, closedManually = false, dueDate: TextualYearMonthDay = '2026-04-30') =>
    ({ total, received, closedManually, dueDate });

const noTax = { subtotal: 0, taxAmount: 0, cgst: 0, sgst: 0, total: 0 };

describe('computeInvoiceTotals', () => {
    test('sums quantity (hundredths) × rate', () => {
        expect(computeInvoiceTotals([{ quantity: 100, rate: 37000000 }, { quantity: 300, rate: 150000 }], 'none', 0)).toEqual({ ...noTax, subtotal: 37450000, total: 37450000 });
        expect(computeInvoiceTotals([], 'none', 0)).toEqual(noTax);
    });

    test('rounds a fractional quantity to the paisa', () => {
        expect(computeInvoiceTotals([{ quantity: 750, rate: 33333 }], 'none', 0)).toEqual({ ...noTax, subtotal: 249998, total: 249998 });
        expect(computeInvoiceTotals([{ quantity: 1, rate: 1 }], 'none', 0)).toEqual(noTax);
    });

    test('18% on ₹1,00,000.01 rounds once and splits evenly', () => {
        expect(computeInvoiceTotals([{ quantity: 100, rate: 10000001 }], 'cgst_sgst', 1800))
            .toEqual({ subtotal: 10000001, taxAmount: 1800000, cgst: 900000, sgst: 900000, total: 11800001 });
        expect(computeInvoiceTotals([{ quantity: 100, rate: 10000001 }], 'igst', 1800).total).toBe(11800001);
    });

    test('an odd tax paisa goes to SGST', () => {
        expect(computeInvoiceTotals([{ quantity: 100, rate: 5 }], 'cgst_sgst', 1800)).toEqual({ subtotal: 5, taxAmount: 1, cgst: 0, sgst: 1, total: 6 });
    });

    test('none yields zero tax whatever the rate', () => {
        expect(computeInvoiceTotals([{ quantity: 100, rate: 10000001 }], 'none', 1800)).toEqual({ ...noTax, subtotal: 10000001, total: 10000001 });
    });
});

describe('getInvoiceTaxRows', () => {
    test('one IGST row, two CGST + SGST halves with the odd paisa on SGST, none without tax', () => {
        expect(getInvoiceTaxRows('igst', 1800, 1800000)).toEqual([{ name: 'IGST', rate: 1800, amount: 1800000 }]);
        expect(getInvoiceTaxRows('cgst_sgst', 1800, 1)).toEqual([{ name: 'CGST', rate: 900, amount: 0 }, { name: 'SGST', rate: 900, amount: 1 }]);
        expect(getInvoiceTaxRows('none', 1800, 0)).toEqual([]);
    });
});

describe('formatInvoiceTaxRate', () => {
    test('basis points as a trimmed percentage', () => {
        expect(formatInvoiceTaxRate(1800)).toBe('18%');
        expect(formatInvoiceTaxRate(900)).toBe('9%');
        expect(formatInvoiceTaxRate(1250)).toBe('12.5%');
        expect(formatInvoiceTaxRate(925)).toBe('9.25%');
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

describe('per-currency summaries', () => {
    const row = (currency: string, total: number, received: number, dueDate: string, closedManually: boolean = false) => ({ currency, total, received, dueDate, closedManually });

    test('outstanding and overdue sum the balance due per currency, INR first', () => {
        const invoices = [
            row('USD', 100000, 0, '2026-04-30'),
            row('INR', 37000000, 10000000, '2026-04-30'),
            row('INR', 500000, 0, '2026-05-10'),
            row('INR', 200000, 200000, '2026-04-01'), // paid
            row('EUR', 50000, 0, '2026-05-10')
        ];

        expect(getInvoiceOutstandingByCurrency(invoices)).toEqual([
            { currency: 'INR', amount: 27500000 },
            { currency: 'EUR', amount: 50000 },
            { currency: 'USD', amount: 100000 }
        ]);
        expect(getInvoiceOverdueByCurrency(invoices, '2026-05-01')).toEqual([
            { currency: 'INR', amount: 27000000 },
            { currency: 'EUR', amount: 0 },
            { currency: 'USD', amount: 100000 }
        ]);
    });

    test('no invoices still gives one INR line', () => {
        expect(getInvoiceOutstandingByCurrency([])).toEqual([{ currency: 'INR', amount: 0 }]);
        expect(getInvoiceOverdueByCurrency([], '2026-05-01')).toEqual([{ currency: 'INR', amount: 0 }]);
    });
});

describe('document formatting', () => {
    test('amount like the Wave sample, with Indian grouping for INR', () => {
        expect(formatInvoiceDocumentAmount(37000000, 'INR')).toBe('₹3,70,000.00');
        expect(formatInvoiceDocumentAmount(100000, 'USD')).toBe('$1,000.00');
        expect(formatInvoiceDocumentAmount(100000, 'EUR')).toBe('€1,000.00');
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
