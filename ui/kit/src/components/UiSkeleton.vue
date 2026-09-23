<script setup lang="ts">
withDefaults(defineProps<{ lines?: number | undefined; kind?: 'text' | 'card' | 'table' | undefined }>(), { lines: 3, kind: 'text' })
</script>

<template>
  <div class="flex flex-col gap-2" aria-busy="true" aria-live="polite">
    <span class="sr-only">Loading…</span>
    <template v-if="kind === 'card'">
      <div class="skeleton h-24 w-full" />
    </template>
    <template v-else>
      <div v-for="i in lines" :key="i" class="skeleton h-4" :class="i % 3 === 0 ? 'w-2/3' : 'w-full'" />
    </template>
  </div>
</template>
