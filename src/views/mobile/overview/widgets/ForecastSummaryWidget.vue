<template>
    <f7-list strong inset dividers class="overview-widget-list no-margin-top margin-bottom" :class="{ 'skeleton-text': loading }">
        <f7-list-item group-title v-if="showTitle">
            <small>{{ title }}</small>
        </f7-list-item>

        <f7-list-item :title="errorMessage" v-if="errorMessage && !loading"></f7-list-item>

        <f7-list-item :title="tt('No forecast items yet')" link="/forecast/list" v-else-if="isEmpty"></f7-list-item>

        <template v-else>
            <f7-list-item :title="tt('Month-end cash')" :after="summary ? formatAmount(summary.closingCash) : '-'"></f7-list-item>
            <f7-list-item :title="tt('Without expected invoice income')" :after="summary ? formatAmount(summary.closingCashWithoutUncertain) : '-'" v-if="summary"></f7-list-item>

            <f7-list-item :key="'shortfall-' + shortfall.accountId + shortfall.date" v-for="shortfall in summary?.shortfalls ?? []"
                          :footer="tt('Lowest {amount} on {date}', { amount: formatAmount(shortfall.lowestBalance), date: formatItemDate(shortfall.lowestDate) })">
                <template #title>
                    <span class="text-color-orange">{{ tt('{account} goes below zero on {date}', { account: getAccountName(shortfall.accountId), date: formatItemDate(shortfall.date) }) }}</span>
                </template>
            </f7-list-item>

            <f7-list-item :key="'item-' + item.date + item.source + item.label" v-for="item in summary?.nextItems ?? []"
                          :title="item.label" :after="formatAmount(item.amount)" :footer="formatItemDate(item.date)"></f7-list-item>

            <f7-list-item :footer="tt('Cards without a payment due day or pay-from account are not billed')" v-if="summary && summary.skippedCards.length"></f7-list-item>
        </template>
    </f7-list>
</template>

<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';

import {
    type CommonForecastSummaryWidgetProps,
    useForecastSummaryWidgetBase
} from '@/views/base/overview/ForecastSummaryWidgetBase.ts';

interface MobileForecastSummaryWidgetProps extends CommonForecastSummaryWidgetProps {
    editing?: boolean;
    showTitle: boolean;
}

const props = defineProps<MobileForecastSummaryWidgetProps>();

const { tt } = useI18n();

const {
    loading,
    errorMessage,
    summary,
    isEmpty,
    title,
    getAccountName,
    formatAmount,
    formatItemDate
} = useForecastSummaryWidgetBase(props);
</script>
