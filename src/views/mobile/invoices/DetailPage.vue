<template>
    <f7-page><f7-navbar :back-link="tt('Back')"></f7-navbar></f7-page>
</template>
<script setup lang="ts">
import { useI18n } from '@/locales/helpers.ts';
const { tt } = useI18n();
</script>
