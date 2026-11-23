<template>
    <f7-list strong inset dividers class="overview-transaction-list no-margin-top margin-bottom" :class="{ 'skeleton-text': loading }">
        <f7-list-item chevron-center :key="period.dateRange.type"
                      :link="getDetailsUrl(period.dateRange.type)"
                      v-for="period in displayedPeriods">
            <template #media>
                <f7-icon :f7="period.icon"></f7-icon>
            </template>
            <template #title>
                <div class="padding-top-half">
                    <span v-if="loading">{{ period.skeletonTitle }}</span>
                    <span v-else-if="!loading">{{ tt(period.dateRange.name) }}</span>
                </div>
            </template>
            <template #footer>
                <div class="overview-transaction-footer padding-bottom-half">
                    <template v-if="loading">{{ period.skeletonTime }}</template>
                    <template v-else-if="!loading">{{ getDisplayDateTime(period.dateRange.type) }}</template>
                </div>
            </template>
            <template #after>
                <div class="overview-transaction-amount">
                    <div class="text-income text-align-right">
                        <small v-if="loading">0.00 USD</small>
                        <small v-else-if="getOverviewItem(period.dateRange.type)?.valid">{{ getDisplayIncomeAmount(getOverviewItem(period.dateRange.type)!) }}</small>
                    </div>
                    <div class="text-expense text-align-right">
                        <small v-if="loading">0.00 USD</small>
                        <small v-else-if="getOverviewItem(period.dateRange.type)?.valid">{{ getDisplayExpenseAmount(getOverviewItem(period.dateRange.type)!) }}</small>
                    </div>
                </div>
            </template>
        </f7-list-item>
    </f7-list>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { usePeriodStatisticsWidgetBase } from '@/views/base/overview/PeriodStatisticsWidgetBase.ts';

import { DateRange } from '@/core/datetime.ts';
import type { OverviewWidgetFilter } from '@/core/overview_layout.ts';

const props = defineProps<{
    loading: boolean;
    dateRanges: number[];
    widgetId?: string;
    filter?: OverviewWidgetFilter;
}>();

const { tt } = useI18n();

const {
    getOverviewItem,
    getDisplayDateTime,
    getDetailsUrl,
    getDisplayIncomeAmount,
    getDisplayExpenseAmount
} = usePeriodStatisticsWidgetBase(props);

const periods = [
    { dateRange: DateRange.Today, icon: 'calendar_today', skeletonTitle: 'Today', skeletonTime: 'MM/DD/YYYY' },
    { dateRange: DateRange.ThisWeek, icon: 'calendar', skeletonTitle: 'This Week', skeletonTime: 'MM/DD - MM/DD' },
    { dateRange: DateRange.ThisMonth, icon: 'calendar', skeletonTitle: 'This Month', skeletonTime: 'MM/DD - MM/DD' },
    { dateRange: DateRange.LastMonth, icon: 'calendar', skeletonTitle: 'Last Month', skeletonTime: 'MM/DD/YYYY - MM/DD/YYYY' },
    { dateRange: DateRange.ThisYear, icon: 'square_stack_3d_up', skeletonTitle: 'This Year', skeletonTime: 'YYYY' },
    { dateRange: DateRange.ThisFiscalYear, icon: 'square_stack_3d_up', skeletonTitle: 'This Fiscal Year', skeletonTime: 'MM/DD/YYYY - MM/DD/YYYY' },
    { dateRange: DateRange.LastFiscalYear, icon: 'square_stack_3d_up', skeletonTitle: 'Last Fiscal Year', skeletonTime: 'MM/DD/YYYY - MM/DD/YYYY' }
];

// with no date range selected the widget shows the four periods it has always shown
const defaultPeriodTypes: number[] = [DateRange.Today.type, DateRange.ThisWeek.type, DateRange.ThisMonth.type, DateRange.ThisYear.type];

const displayedPeriods = computed(() => periods.filter(period => props.dateRanges && props.dateRanges.length ? props.dateRanges.includes(period.dateRange.type) : defaultPeriodTypes.includes(period.dateRange.type)));
</script>
