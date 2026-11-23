import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useOverviewStore } from '@/stores/overview.ts';

import type { BigDecimal } from '@/core/numeral.ts';
import { DateRange } from '@/core/datetime.ts';
import type { OverviewWidgetFilter, OverviewNamedRange } from '@/core/overview_layout.ts';

import {
    DISPLAY_HIDDEN_AMOUNT,
    DISPLAY_HIDDEN_PERCENT,
    INCOMPLETE_AMOUNT_SUFFIX
} from '@/consts/numeral.ts';

import { isDefined, isInteger } from '@/lib/common.ts';
import { BIG_DECIMAL_ZERO } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import type {
    TransactionOverviewData,
    TransactionOverviewDisplayTime,
    TransactionOverviewDataItem
} from '@/models/transaction.ts';

export interface CommonPeriodStatisticsWidgetProps {
    dateType?: number;
    widgetId?: string;
    filter?: OverviewWidgetFilter;
}

// the date ranges the shared overview data covers; other ranges only come from the widget's own data
const SHARED_OVERVIEW_DATE_RANGE_TYPES: number[] = [
    DateRange.Today.type,
    DateRange.ThisWeek.type,
    DateRange.ThisMonth.type,
    DateRange.ThisYear.type
];

export function usePeriodStatisticsWidgetBase(props: CommonPeriodStatisticsWidgetProps) {
    const {
        tt,
        formatRange,
        formatDateTimeToLongDate,
        formatDateTimeToLongMonthDay,
        formatDateTimeToGregorianLikeLongYear,
        formatDateTimeToGregorianLikeLongMonth,
        formatAmountToLocalizedNumeralsWithCurrency,
        formatPercentToLocalizedNumerals
    } = useI18n();

    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const overviewStore = useOverviewStore();

    const showAmountInHomePage = computed<boolean>({
        get: () => settingsStore.appSettings.showAmountInHomePage,
        set: (value) => settingsStore.setShowAmountInHomePage(value)
    });

    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);

    const transactionOverview = computed<TransactionOverviewData>(() => overviewStore.transactionOverview);
    const widgetAmounts = computed<Record<string, TransactionOverviewDataItem> | undefined>(() => props.widgetId ? overviewStore.getWidgetAmounts(props.widgetId) : undefined);

    const displayDateRange = computed<TransactionOverviewDisplayTime>(() => {
        return {
            today: {
                displayTime: formatDateTimeToLongDate(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.today.startTime)),
            },
            thisWeek: {
                startTime: formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisWeek.startTime)),
                endTime: formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisWeek.endTime))
            },
            thisMonth: {
                displayTime: formatDateTimeToGregorianLikeLongMonth(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisMonth.startTime)),
                startTime: formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisMonth.startTime)),
                endTime: formatDateTimeToLongMonthDay(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisMonth.endTime))
            },
            thisYear: {
                displayTime: formatDateTimeToGregorianLikeLongYear(parseDateTimeFromUnixTime(overviewStore.transactionDataRange.thisYear.startTime))
            }
        };
    });

    const currentOverviewItem = computed<TransactionOverviewDataItem | undefined>(() => getOverviewItem(props.dateType));
    const currentPeriodTitle = computed<string>(() => tt(isInteger(props.dateType) ? (DateRange.valueOf(props.dateType)?.name ?? 'Unknown') : 'Unknown'));
    const currentDisplayDateTime = computed<string>(() => getDisplayDateTime(props.dateType));

    const currentIncomeAmount = computed<BigDecimal>(() => currentOverviewItem.value?.incomeAmount ?? BIG_DECIMAL_ZERO);
    const currentExpenseAmount = computed<BigDecimal>(() => currentOverviewItem.value?.expenseAmount ?? BIG_DECIMAL_ZERO);
    const currentNetIncomeAmount = computed<BigDecimal>(() => currentIncomeAmount.value.subtract(currentExpenseAmount.value));

    const currentDisplayIncomeAmount = computed<string>(() => currentOverviewItem.value?.valid ? getDisplayIncomeAmount(currentOverviewItem.value) : '');
    const currentDisplayExpenseAmount = computed<string>(() => currentOverviewItem.value?.valid ? getDisplayExpenseAmount(currentOverviewItem.value) : '');
    const currentDisplayNetIncomeAmount = computed<string>(() => {
        if (!currentOverviewItem.value?.valid) {
            return '';
        }

        const netIncomeAmount: BigDecimal = currentNetIncomeAmount.value;
        const incomplete: boolean = currentOverviewItem.value.incompleteIncomeAmount || currentOverviewItem.value.incompleteExpenseAmount;

        return getDisplayAmount(netIncomeAmount, incomplete);
    });

    const currentDisplaySavingsRate = computed<string>(() => {
        if (!showAmountInHomePage.value) {
            return DISPLAY_HIDDEN_PERCENT;
        }

        if (!currentOverviewItem.value?.valid) {
            return '';
        }

        const incomeAmount: BigDecimal = currentIncomeAmount.value;
        const expenseAmount: BigDecimal = currentExpenseAmount.value;

        if (incomeAmount.isZero()) {
            return expenseAmount.isZero() ? formatPercentToLocalizedNumerals(0, 2, '<0.01') : '-';
        }

        const rate = incomeAmount.subtract(expenseAmount).divide(incomeAmount).multiply(100).toDoubleNumber();
        return formatPercentToLocalizedNumerals(rate, 2, '<0.01');
    });

    const currentDetailsUrl = computed<string>(() => getDetailsUrl(props.dateType));

    // a widget with one date range (dateType prop) reads the range named "current" from its own data,
    // the mobile widget with several date ranges reads one range per date type named "r<dateType>"
    function getWidgetRangeName(dateType: number | undefined): string {
        return isDefined(props.dateType) ? 'current' : `r${dateType}`;
    }

    function getWidgetRange(dateType: number | undefined): OverviewNamedRange | undefined {
        const query = props.widgetId ? overviewStore.getWidgetAmountsQuery(props.widgetId) : undefined;
        const rangeName = getWidgetRangeName(dateType);
        return query?.ranges.find(range => range.name === rangeName);
    }

    function getOverviewItem(dateType: number | undefined): TransactionOverviewDataItem | undefined {
        if (widgetAmounts.value) {
            return widgetAmounts.value[getWidgetRangeName(dateType)];
        }

        if (dateType === DateRange.Today.type) {
            return transactionOverview.value.today;
        } else if (dateType === DateRange.ThisWeek.type) {
            return transactionOverview.value.thisWeek;
        } else if (dateType === DateRange.ThisMonth.type) {
            return transactionOverview.value.thisMonth;
        } else if (dateType === DateRange.ThisYear.type) {
            return transactionOverview.value.thisYear;
        } else {
            return undefined;
        }
    }

    function getDisplayDateTime(dateType: number | undefined): string {
        if (isInteger(dateType) && !SHARED_OVERVIEW_DATE_RANGE_TYPES.includes(dateType)) {
            const range = getWidgetRange(dateType);

            if (!range) {
                return '';
            }

            return formatRange(formatDateTimeToLongDate(parseDateTimeFromUnixTime(range.startTime)), formatDateTimeToLongDate(parseDateTimeFromUnixTime(range.endTime)));
        }

        if (dateType === DateRange.Today.type) {
            return displayDateRange.value.today?.displayTime || '';
        } else if (dateType === DateRange.ThisWeek.type) {
            return formatRange(displayDateRange.value.thisWeek?.startTime ?? '', displayDateRange.value.thisWeek?.endTime ?? '');
        } else if (dateType === DateRange.ThisMonth.type) {
            return formatRange(displayDateRange.value.thisMonth?.startTime ?? '', displayDateRange.value.thisMonth?.endTime ?? '');
        } else if (dateType === DateRange.ThisYear.type) {
            return displayDateRange.value.thisYear?.displayTime || '';
        } else {
            return '';
        }
    }

    function getDetailsUrl(dateType: number | undefined): string {
        const filter = props.filter ? { ...props.filter, ignoreHomeFilters: !overviewStore.activeDashboardIsHome } : undefined;
        return `/transaction/list?${overviewStore.getTransactionListPageParams({ dateType: dateType, filter: filter })}`;
    }

    function getDisplayAmount(amount: BigDecimal, incomplete: boolean): string {
        if (!showAmountInHomePage.value) {
            return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, defaultCurrency.value);
        }

        return formatAmountToLocalizedNumeralsWithCurrency(amount, defaultCurrency.value) + (incomplete ? INCOMPLETE_AMOUNT_SUFFIX : '');
    }

    function getDisplayIncomeAmount(category: TransactionOverviewDataItem): string {
        return getDisplayAmount(category.incomeAmount, category.incompleteIncomeAmount);
    }

    function getDisplayExpenseAmount(category: TransactionOverviewDataItem): string {
        return getDisplayAmount(category.expenseAmount, category.incompleteExpenseAmount);
    }

    return {
        // computed states
        showAmountInHomePage,
        defaultCurrency,
        transactionOverview,
        displayDateRange,
        currentOverviewItem,
        currentPeriodTitle,
        currentDisplayDateTime,
        currentIncomeAmount,
        currentExpenseAmount,
        currentNetIncomeAmount,
        currentDisplayIncomeAmount,
        currentDisplayExpenseAmount,
        currentDisplayNetIncomeAmount,
        currentDisplaySavingsRate,
        currentDetailsUrl,
        // functions
        getOverviewItem,
        getDisplayDateTime,
        getDetailsUrl,
        getDisplayAmount,
        getDisplayIncomeAmount,
        getDisplayExpenseAmount
    };
}
