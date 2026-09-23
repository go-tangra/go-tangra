<script setup lang="ts">
// Label/value rows for detail views; wraps values, two columns from md.
export interface KeyValue { label: string; value?: unknown; copyable?: boolean }
defineProps<{ items: KeyValue[]; columns?: 1 | 2 | undefined }>()
function show(v: unknown): string {
  if (v === undefined || v === null || v === '') return '—'
  if (Array.isArray(v)) return v.length ? v.join(', ') : '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
</script>

<template>
  <dl class="grid gap-x-4 gap-y-2 text-sm" :class="columns === 2 ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1'">
    <div v-for="(it, i) in items" :key="i" class="grid grid-cols-[minmax(0,40%)_1fr] gap-2 border-b border-base-300/60 py-1 last:border-0">
      <dt class="truncate text-base-content/70">{{ it.label }}</dt>
      <dd class="min-w-0 break-words"><slot :name="'value-' + i" :item="it">{{ show(it.value) }}</slot></dd>
    </div>
  </dl>
</template>
