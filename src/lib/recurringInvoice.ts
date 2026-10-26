import type { TextualYearMonthDay } from '@/core/datetime.ts';

// the last day of month a recurring invoice may repeat on, so every month has it (mirrors models.RecurringInvoiceMaxDayOfMonth)
export const RECURRING_INVOICE_MAX_DAY_OF_MONTH: number = 28;

function toTextualDate(year: number, monthIndex: number, day: number): TextualYearMonthDay {
    return new Date(Date.UTC(year, monthIndex, day)).toISOString().substring(0, 10) as TextualYearMonthDay;
}

// the source invoice's day of month, capped at 28
export function getDefaultRecurringDayOfMonth(invoiceDate: string): number {
    const day = Number(invoiceDate.split('-')[2]);

    if (!day || day < 1) {
        return 1;
    }

    return Math.min(day, RECURRING_INVOICE_MAX_DAY_OF_MONTH);
}

// the next occurrence of dayOfMonth strictly after today
export function getNextRecurringDate(dayOfMonth: number, today: string): TextualYearMonthDay {
    const [year, month, day] = today.split('-').map(Number);
    const monthIndex = month! - 1;

    if (dayOfMonth > day!) {
        return toTextualDate(year!, monthIndex, dayOfMonth);
    }

    return toTextualDate(year!, monthIndex + 1, dayOfMonth);
}
