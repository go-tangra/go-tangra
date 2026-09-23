<script setup lang="ts">
// Horizontal bar list for dashboards: label, proportional bar, count.
import { computed } from 'vue'
import UiEmptyState from './UiEmptyState.vue'
export interface BarItem { label: string; value: number; color?: 'primary' | 'secondary' | 'accent' | 'info' | 'success' | 'warning' | 'error' | 'neutral' }
const props = withDefaults(defineProps<{ items: BarItem[]; emptyTitle?: string | undefined; color?: BarItem['color'] | undefined }>(), { emptyTitle: 'Nothing yet', color: 'primary' })
const max = computed(() => Math.max(1, ...props.items.map((i) => i.value)))
const cls: Record<NonNullable<BarItem['color']>, string> = { primary: 'progress-primary', secondary: 'progress-secondary', accent: 'progress-accent', info: 'progress-info', success: 'progress-success', warning: 'progress-warning', error: 'progress-error', neutral: 'progress-neutral' }
</script>

<template>
  <UiEmptyState v-if="items.length === 0" :title="emptyTitle" />
  <ul v-else class="flex flex-col gap-2">
    <li v-for="it in items" :key="it.label" class="text-sm">
      <div class="flex justify-between gap-2"><span class="truncate">{{ it.label }}</span><span class="tabular-nums">{{ it.value }}</span></div>
      <progress class="progress h-2 w-full" :class="cls[it.color ?? color ?? 'primary']" :value="it.value" :max="max" :aria-label="it.label" />
    </li>
  </ul>
</template>
