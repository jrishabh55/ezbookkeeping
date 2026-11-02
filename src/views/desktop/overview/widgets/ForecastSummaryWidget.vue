<template>
    <v-card class="overview-widget forecast-summary-widget h-100" :class="{ disabled: loading }">
        <template #title>
            <overview-widget-header :title="title" :icon="mdiCashClock">
                <v-btn density="compact" color="default" variant="text" :aria-label="tt('Refresh')" :icon="true" :loading="loading" @click="refresh" v-if="!editing">
                    <template #loader>
                        <v-progress-circular indeterminate size="20" />
                    </template>
                    <v-icon :icon="mdiRefresh" size="24" />
                    <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                </v-btn>
            </overview-widget-header>
        </template>

        <v-card-text class="overview-widget__body forecast-summary-widget__body" v-if="errorMessage && !loading">
            <div class="overview-widget__empty">
                <v-icon :icon="mdiAlertCircleOutline" size="32" />
                <span>{{ errorMessage }}</span>
            </div>
        </v-card-text>

        <v-card-text class="overview-widget__body forecast-summary-widget__body" v-else-if="isEmpty">
            <div class="overview-widget__empty">
                <v-icon :icon="mdiCashClock" size="32" />
                <span>{{ tt('No forecast items yet') }}</span>
                <v-btn variant="tonal" color="primary" to="/forecast/list">{{ tt('Forecast Items') }}</v-btn>
            </div>
        </v-card-text>

        <v-card-text class="overview-widget__body forecast-summary-widget__body" v-else>
            <div class="overview-widget__stat">
                <span class="overview-widget__caption">{{ tt('Month-end cash') }}</span>
                <div class="overview-widget__amount text-headline-small text-truncate" v-if="!loading || summary">{{ summary ? formatAmount(summary.closingCash) : '-' }}</div>
                <v-skeleton-loader class="skeleton-no-margin mt-h1 pt-2 mb-2" type="text" width="140px" :loading="true" v-else></v-skeleton-loader>
            </div>

            <div class="overview-widget__detail-row overview-widget__caption mt-2" v-if="!loading || summary">
                <span class="text-truncate">{{ tt('Without expected invoice income') }}</span>
                <span class="overview-widget__amount text-truncate" v-if="summary">{{ formatAmount(summary.closingCashWithoutUncertain) }}</span>
            </div>
            <v-skeleton-loader class="skeleton-no-margin mt-2" type="text" width="110px" :loading="true" v-else></v-skeleton-loader>

            <div class="forecast-summary-widget__shortfalls mt-3" v-if="summary && summary.shortfalls.length">
                <div class="forecast-summary-widget__shortfall text-warning" :key="shortfall.accountId + shortfall.date" v-for="shortfall in summary.shortfalls">
                    <v-icon class="forecast-summary-widget__shortfall-icon" :icon="mdiAlertOutline" size="16" />
                    <span class="forecast-summary-widget__shortfall-text">
                        <span class="text-truncate">{{ tt('{account} goes below zero on {date}', { account: getAccountName(shortfall.accountId), date: formatItemDate(shortfall.date) }) }}</span>
                        <span class="overview-widget__caption text-truncate">{{ tt('Lowest {amount} on {date}', { amount: formatAmount(shortfall.lowestBalance), date: formatItemDate(shortfall.lowestDate) }) }}</span>
                    </span>
                </div>
            </div>

            <div class="forecast-summary-widget__items mt-3" v-if="summary && summary.nextItems.length">
                <div class="overview-widget__detail-row forecast-summary-widget__item" :key="item.date + item.source + item.label" v-for="item in summary.nextItems">
                    <span class="text-truncate">{{ item.label }}</span>
                    <span class="overview-widget__amount text-truncate">{{ formatItemDate(item.date) }} · {{ formatAmount(item.amount) }}</span>
                </div>
            </div>

            <div class="overview-widget__footer" v-if="summary && summary.skippedCards.length">{{ tt('Cards without a payment due day or pay-from account are not billed') }}</div>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import OverviewWidgetHeader from './OverviewWidgetHeader.vue';

import { useI18n } from '@/locales/helpers.ts';

import {
    type CommonForecastSummaryWidgetProps,
    useForecastSummaryWidgetBase
} from '@/views/base/overview/ForecastSummaryWidgetBase.ts';

import {
    mdiCashClock,
    mdiRefresh,
    mdiAlertOutline,
    mdiAlertCircleOutline
} from '@mdi/js';

interface DesktopForecastSummaryWidgetProps extends CommonForecastSummaryWidgetProps {
    editing?: boolean;
}

const props = defineProps<DesktopForecastSummaryWidgetProps>();

const { tt } = useI18n();

const {
    loading,
    errorMessage,
    summary,
    isEmpty,
    title,
    load,
    getAccountName,
    formatAmount,
    formatItemDate
} = useForecastSummaryWidgetBase(props);

function refresh(): void {
    load();
}
</script>
