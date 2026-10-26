import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, RecurringInvoice } from '@/models/invoice.ts';

import { getTodayTextualDate } from '@/lib/invoice.ts';
import { RECURRING_INVOICE_MAX_DAY_OF_MONTH, getDefaultRecurringDayOfMonth, getNextRecurringDate } from '@/lib/recurringInvoice.ts';

// the "Repeat Monthly" form: day of month (default: the source invoice's day, capped at 28) and first date (default: its next occurrence)
export function useRecurringInvoiceEditPageBase() {
    const { formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const submitting = ref<boolean>(false);
    const recurring = ref<RecurringInvoice>(new RecurringInvoice());

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!recurring.value.dayOfMonth || recurring.value.dayOfMonth < 1 || recurring.value.dayOfMonth > RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            return 'Day of month must be between 1 and 28';
        }

        if (!recurring.value.nextDate || recurring.value.nextDate === '0-0-0') {
            return 'First invoice date cannot be blank';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function initFromInvoice(invoice: Invoice): void {
        const dayOfMonth = getDefaultRecurringDayOfMonth(invoice.invoiceDate);
        recurring.value = RecurringInvoice.createNewRecurringInvoice(invoice.id, dayOfMonth, getNextRecurringDate(dayOfMonth, getTodayTextualDate()));
    }

    // changing the day moves the first date to that day's next occurrence
    function setDayOfMonth(dayOfMonth: number): void {
        recurring.value.dayOfMonth = dayOfMonth;

        if (dayOfMonth >= 1 && dayOfMonth <= RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            recurring.value.nextDate = getNextRecurringDate(dayOfMonth, getTodayTextualDate());
        }
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        submitting,
        recurring,
        // computed states
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        initFromInvoice,
        setDayOfMonth,
        formatDate
    };
}
