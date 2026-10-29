import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import { type Invoice, type RecurringInvoiceMode, RecurringInvoice, RECURRING_INVOICE_MODES, RECURRING_INVOICE_PERIODS } from '@/models/invoice.ts';

import { getTodayTextualDate } from '@/lib/invoice.ts';
import { RECURRING_INVOICE_MAX_DAY_OF_MONTH, getDefaultRecurringDayOfMonth, getNextRecurringDate } from '@/lib/recurringInvoice.ts';

// the "Repeat" form: day of month, first date, period and mode; adds a schedule for an invoice or edits an existing one
export function useRecurringInvoiceEditPageBase() {
    const { tt, formatGregorianTextualYearMonthDayToLongDate } = useI18n();

    const submitting = ref<boolean>(false);
    const editRecurringId = ref<string | null>(null);
    const recurring = ref<RecurringInvoice>(new RecurringInvoice());

    const title = computed<string>(() => editRecurringId.value ? 'Edit Recurring Invoice' : 'Repeat');
    const saveButtonTitle = computed<string>(() => editRecurringId.value ? 'Save' : 'Add');

    const allModes = computed<{ mode: RecurringInvoiceMode, displayName: string }[]>(() => RECURRING_INVOICE_MODES.map(item => ({ mode: item.mode, displayName: tt(item.name) })));
    const allPeriods = computed<{ months: number, displayName: string }[]>(() => RECURRING_INVOICE_PERIODS.map(item => ({ months: item.months, displayName: tt(item.name) })));
    const selectedModeName = computed<string>(() => allModes.value.find(item => item.mode === recurring.value.mode)?.displayName ?? '');
    const selectedPeriodName = computed<string>(() => allPeriods.value.find(item => item.months === recurring.value.periodMonths)?.displayName
        ?? tt('format.misc.everyNMonths', { count: recurring.value.periodMonths }));

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!recurring.value.dayOfMonth || recurring.value.dayOfMonth < 1 || recurring.value.dayOfMonth > RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            return 'Day of month must be between 1 and 28';
        }

        if (!recurring.value.periodMonths || recurring.value.periodMonths < 1 || recurring.value.periodMonths > 120) {
            return 'Period must be between 1 and 120 months';
        }

        if (!recurring.value.nextDate || recurring.value.nextDate === '0-0-0') {
            return 'First invoice date cannot be blank';
        }

        return null;
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function initFromInvoice(invoice: Invoice): void {
        const dayOfMonth = getDefaultRecurringDayOfMonth(invoice.invoiceDate);
        editRecurringId.value = null;
        recurring.value = RecurringInvoice.createNewRecurringInvoice(invoice.id, dayOfMonth, getNextRecurringDate(dayOfMonth, getTodayTextualDate()));
    }

    function initFromRecurring(existing: RecurringInvoice): void {
        editRecurringId.value = existing.id;
        recurring.value = existing.clone();
    }

    // changing the day moves a new schedule's first date to that day's next occurrence; an existing schedule keeps its date
    function setDayOfMonth(dayOfMonth: number): void {
        recurring.value.dayOfMonth = dayOfMonth;

        if (!editRecurringId.value && dayOfMonth >= 1 && dayOfMonth <= RECURRING_INVOICE_MAX_DAY_OF_MONTH) {
            recurring.value.nextDate = getNextRecurringDate(dayOfMonth, getTodayTextualDate());
        }
    }

    function formatDate(date: TextualYearMonthDay): string {
        return formatGregorianTextualYearMonthDayToLongDate(date);
    }

    return {
        // states
        submitting,
        editRecurringId,
        recurring,
        // computed states
        title,
        saveButtonTitle,
        allModes,
        allPeriods,
        selectedModeName,
        selectedPeriodName,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        initFromInvoice,
        initFromRecurring,
        setDayOfMonth,
        formatDate
    };
}
