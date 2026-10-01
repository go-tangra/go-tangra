<script setup lang="ts">
// Numbered pager for server-side lists: "Showing a–b of N", first/previous,
// page numbers with elision, next/last, and a page-size choice. Below lg only
// the current page number is shown between the arrows so the bar fits a phone.
// Controlled: it only emits; the owner fetches the page and passes the new
// props back.
import { computed } from 'vue'
import UiIcon from './UiIcon.vue'

const props = withDefaults(defineProps<{
  page: number
  pageSize: number
  total: number
  pageSizes?: number[] | undefined
  label?: string | undefined
}>(), { pageSizes: () => [10, 25, 50, 100, 200], label: 'Pagination' })
const emit = defineEmits<{ (e: 'update:page', v: number): void; (e: 'update:pageSize', v: number): void }>()

const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const from = computed(() => (props.page - 1) * props.pageSize + 1)
const to = computed(() => Math.min(props.page * props.pageSize, props.total))
const fmt = (n: number) => n.toLocaleString('en-US')

// Page numbers to show: all when there are few, otherwise the first, the last
// and two on each side of the current page; a gap of exactly one page shows
// that page instead of an ellipsis.
const items = computed<(number | 'gap')[]>(() => {
  const n = pages.value
  if (n <= 7) return Array.from({ length: n }, (_, i) => i + 1)
  const want = new Set([1, n])
  for (let p = props.page - 2; p <= props.page + 2; p++) if (p >= 1 && p <= n) want.add(p)
  const sorted = [...want].sort((a, b) => a - b)
  const out: (number | 'gap')[] = []
  sorted.forEach((p, i) => {
    const prev = sorted[i - 1]
    if (prev !== undefined && p - prev === 2) out.push(p - 1)
    else if (prev !== undefined && p - prev > 2) out.push('gap')
    out.push(p)
  })
  return out
})

const go = (p: number) => {
  if (p >= 1 && p <= pages.value && p !== props.page) emit('update:page', p)
}
const onSize = (e: Event) => emit('update:pageSize', Number((e.target as HTMLSelectElement).value))
</script>

<template>
  <div v-if="total > 0" class="flex flex-wrap items-center justify-between gap-2 text-sm">
    <span class="text-base-content/70">Showing {{ fmt(from) }}–{{ fmt(to) }} of {{ fmt(total) }}</span>
    <nav v-if="pages > 1" class="join max-w-full flex-wrap" :aria-label="label">
      <button type="button" class="btn btn-soft btn-sm join-item" :disabled="page <= 1" aria-label="First page" @click="go(1)"><UiIcon name="mdi-chevron-double-left" size="sm" /></button>
      <button type="button" class="btn btn-soft btn-sm join-item" :disabled="page <= 1" aria-label="Previous page" @click="go(page - 1)"><UiIcon name="mdi-chevron-left" size="sm" /></button>
      <template v-for="(it, i) in items" :key="i">
        <span v-if="it === 'gap'" class="btn btn-soft btn-sm join-item pointer-events-none max-lg:hidden" data-page aria-hidden="true">…</span>
        <button v-else type="button" class="btn btn-sm join-item" :class="it === page ? 'btn-primary' : 'btn-soft max-lg:hidden'" data-page :aria-label="'Page ' + it" :aria-current="it === page ? 'page' : undefined" @click="go(it)">{{ it }}</button>
      </template>
      <button type="button" class="btn btn-soft btn-sm join-item" :disabled="page >= pages" aria-label="Next page" @click="go(page + 1)"><UiIcon name="mdi-chevron-right" size="sm" /></button>
      <button type="button" class="btn btn-soft btn-sm join-item" :disabled="page >= pages" aria-label="Last page" @click="go(pages)"><UiIcon name="mdi-chevron-double-right" size="sm" /></button>
    </nav>
    <label class="flex items-center gap-2">
      <span class="text-base-content/70">Rows per page</span>
      <select class="select select-sm w-auto" :value="pageSize" @change="onSize">
        <option v-for="s in pageSizes" :key="s" :value="s">{{ s }}</option>
      </select>
    </label>
  </div>
</template>
