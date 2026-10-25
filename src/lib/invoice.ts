import type { TextualYearMonthDay } from '@/core/datetime.ts';

// mirrors models.InvoicePaidTolerance in pkg/models/invoice.go
export const INVOICE_PAID_TOLERANCE = 0.03;

export type InvoiceStatus = 'unpaid' | 'partial' | 'paid';
export type InvoiceDisplayStatus = InvoiceStatus | 'overdue';

interface InvoiceAmounts {
    readonly total: number;
    readonly received: number;
    readonly closedManually: boolean;
}

export function computeInvoiceTotals(lines: { quantity: number, rate: number }[]): { subtotal: number, total: number } {
    const subtotal = lines.reduce((sum, line) => sum + line.quantity * line.rate, 0);
    // GST would add tax lines between subtotal and total here
    return { subtotal, total: subtotal };
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

// the shortfall written off on a paid invoice (bank / forex charges, or a manual close)
export function getInvoiceBankCharges(invoice: InvoiceAmounts): number {
    return getInvoiceStatus(invoice) === 'paid' ? getInvoiceBalanceDue(invoice) : 0;
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

export function getTodayTextualDate(): TextualYearMonthDay {
    const now = new Date();
    return new Date(Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())).toISOString().substring(0, 10) as TextualYearMonthDay;
}
