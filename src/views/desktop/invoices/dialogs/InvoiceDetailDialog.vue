<template><div/></template>
<script setup lang="ts">
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function open(options: { id: string }): Promise<{ changed: boolean }> { return Promise.resolve({ changed: false }); }
defineExpose({ open });
</script>
