import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { InvoiceTaxType } from '@/models/invoice.ts';

// mirrors models.InvoicePaidTolerance in pkg/models/invoice.go
export const INVOICE_PAID_TOLERANCE = 0.03;

export type InvoiceStatus = 'unpaid' | 'partial' | 'paid';
export type InvoiceDisplayStatus = InvoiceStatus | 'overdue';

interface InvoiceAmounts {
    readonly total: number;
    readonly received: number;
    readonly closedManually: boolean;
}

// quantity is stored in hundredths (100 = 1, 750 = 7.5), mirroring models.InvoiceLine.Quantity
export const INVOICE_QUANTITY_SCALE = 100;

// tax rate is stored in basis points (1800 = 18%), mirroring models.Customer.TaxRate / models.Invoice.TaxRate
export const INVOICE_TAX_RATE_SCALE = 10000;

export interface InvoiceTotals {
    readonly subtotal: number;
    readonly taxAmount: number;
    readonly cgst: number;
    readonly sgst: number;
    readonly total: number;
}

export function isInvoiceTaxable(taxType: InvoiceTaxType | string): boolean {
    return taxType === 'igst' || taxType === 'cgst_sgst';
}

// mirrors computeInvoiceTotals in pkg/services/invoices.go: tax rounded once, half up; an odd tax paisa goes to SGST
export function computeInvoiceTotals(lines: { quantity: number, rate: number }[], taxType: InvoiceTaxType | string, taxRate: number): InvoiceTotals {
    const subtotal = lines.reduce((sum, line) => sum + Math.round(line.quantity * line.rate / INVOICE_QUANTITY_SCALE), 0);
    const taxAmount = isInvoiceTaxable(taxType) && taxRate > 0 ? Math.round(subtotal * taxRate / INVOICE_TAX_RATE_SCALE) : 0;
    const cgst = Math.floor(taxAmount / 2);
    return { subtotal, taxAmount, cgst, sgst: taxAmount - cgst, total: subtotal + taxAmount };
}

// basis points as a percentage with trailing zeros trimmed: 1800 → "18%", 925 → "9.25%"
export function formatInvoiceTaxRate(basisPoints: number): string {
    return (basisPoints / 100).toString() + '%';
}

export interface InvoiceTaxRow {
    readonly name: 'IGST' | 'CGST' | 'SGST';
    readonly rate: number; // basis points
    readonly amount: number;
}

// the tax lines printed between subtotal and total: one for IGST, two halves for CGST + SGST, none otherwise
export function getInvoiceTaxRows(taxType: InvoiceTaxType | string, taxRate: number, taxAmount: number): InvoiceTaxRow[] {
    if (taxType === 'igst') {
        return [{ name: 'IGST', rate: taxRate, amount: taxAmount }];
    } else if (taxType === 'cgst_sgst') {
        const cgst = Math.floor(taxAmount / 2);
        return [{ name: 'CGST', rate: taxRate / 2, amount: cgst }, { name: 'SGST', rate: taxRate / 2, amount: taxAmount - cgst }];
    }

    return [];
}

export function formatInvoiceQuantity(hundredths: number): string {
    return (hundredths / INVOICE_QUANTITY_SCALE).toString();
}

export function getInvoiceBalanceDue(invoice: InvoiceAmounts): number {
    return Math.max(invoice.total - invoice.received, 0);
}

export function getInvoiceStatus(invoice: InvoiceAmounts): InvoiceStatus {
    if (invoice.closedManually || getInvoiceBalanceDue(invoice) <= invoice.total * INVOICE_PAID_TOLERANCE) {
        return 'paid';
    }

    return invoice.received > 0 ? 'partial' : 'unpaid';
}

export function isInvoiceOverdue(invoice: InvoiceAmounts & { readonly dueDate: string }, today: string): boolean {
    return getInvoiceStatus(invoice) !== 'paid' && invoice.dueDate < today;
}

export function getInvoiceDisplayStatus(invoice: InvoiceAmounts & { readonly dueDate: string }, today: string): InvoiceDisplayStatus {
    return isInvoiceOverdue(invoice, today) ? 'overdue' : getInvoiceStatus(invoice);
}

export type InvoiceShortfallKind = 'bankCharges' | 'writtenOff';

// the gap left on a paid invoice: bank / forex charges within the tolerance, otherwise written off by a manual close
export function getInvoiceShortfall(invoice: InvoiceAmounts): { amount: number, kind: InvoiceShortfallKind | null } {
    const balanceDue = getInvoiceBalanceDue(invoice);

    if (getInvoiceStatus(invoice) !== 'paid' || balanceDue <= 0) {
        return { amount: 0, kind: null };
    }

    return { amount: balanceDue, kind: balanceDue <= invoice.total * INVOICE_PAID_TOLERANCE ? 'bankCharges' : 'writtenOff' };
}

// what is still owed: nothing once the invoice is paid, even if a shortfall remains
export function getInvoiceDisplayBalanceDue(invoice: InvoiceAmounts): number {
    return getInvoiceStatus(invoice) === 'paid' ? 0 : getInvoiceBalanceDue(invoice);
}

// ponytail: the printed document is always English / en-US style like the Wave original; app screens use the i18n helpers instead
const invoiceDocumentAmountFormat = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'INR' });

export function formatInvoiceDocumentAmount(paise: number): string {
    return invoiceDocumentAmountFormat.format(paise / 100);
}

export function formatInvoiceDocumentDate(date: string): string {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day!)).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' });
}

export function getInvoicePdfFileName(number: string, dueDate: string): string {
    return `Invoice_${number.replace(/[\\/:*?"<>|\s]+/g, '_')}_${dueDate}`;
}

export function addDaysToDate(date: string, days: number): TextualYearMonthDay {
    const [year, month, day] = date.split('-').map(Number);
    return new Date(Date.UTC(year!, month! - 1, day! + days)).toISOString().substring(0, 10) as TextualYearMonthDay;
}

export function getDaysBetweenDates(from: string, to: string): number {
    const toUtcTime = (date: string) => {
        const [year, month, day] = date.split('-').map(Number);
        return Date.UTC(year!, month! - 1, day!);
    };

    return Math.round((toUtcTime(to) - toUtcTime(from)) / 86400000);
}

export function getTodayTextualDate(): TextualYearMonthDay {
    const now = new Date();
    return new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())).toISOString().substring(0, 10) as TextualYearMonthDay;
}
