<template>
    <div class="overview-widget__header">
        <v-icon class="overview-widget__icon" color="primary" :icon="icon" size="20" />
        <span class="overview-widget__title text-title-small" :title="title">{{ title }}</span>
        <span class="overview-widget__icon d-inline-flex" :aria-label="tt('Filtered')" v-if="filtered">
            <v-icon class="text-medium-emphasis" :icon="mdiFilterOutline" size="16" />
            <v-tooltip activator="parent">{{ tt('Filtered') }}</v-tooltip>
        </span>
        <slot />
    </div>
</template>

<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';

import {
    mdiFilterOutline
} from '@mdi/js';

defineProps<{
    title: string;
    icon: string;
    filtered?: boolean;
}>();

const { tt } = useI18n();
</script>
