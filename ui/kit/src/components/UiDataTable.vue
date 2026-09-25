<script setup lang="ts" generic="T extends Record<string, unknown>">
// Responsive data table. Above md: a real <table> that scrolls horizontally
// inside its own wrapper (never the page); below md: stacked cards, one per row.
// Client-side sort on sortable columns; cursor "load more"; empty/loading
// states; per-row actions slot; row click. Virtualises past `virtualAt` rows
// by rendering a window (keeps huge lists usable without a dependency).
import { computed, ref } from 'vue'
import { useBreakpoint } from '@/composables/useBreakpoint'
import UiEmptyState from './UiEmptyState.vue'
import UiSkeleton from './UiSkeleton.vue'
import UiIcon from './UiIcon.vue'

export interface Column<Row = Record<string, unknown>> {
  key: string
  label: string
  sortable?: boolean
  align?: 'start' | 'end'
  width?: 'sm' | 'md' | 'lg'
  /** Formats the cell text when no cell slot is provided. */
  format?: (row: Row) => string
  /** Hidden on the stacked (phone) layout. */
  hideOnStack?: boolean
}

const props = withDefaults(defineProps<{
  items: T[]
  columns: Column<T>[]
  rowKey?: string | undefined
  loading?: boolean | undefined
  emptyTitle?: string | undefined
  emptyText?: string | undefined
  clickable?: boolean | undefined
  hasMore?: boolean | undefined
  responsive?: 'stack' | 'scroll' | undefined
  virtualAt?: number | undefined
  caption?: string | undefined
  selectable?: boolean | undefined
  selected?: string[] | undefined
  /** With `selectable`: rows it rejects get a disabled checkbox and are left out of "select all". */
  rowSelectable?: ((row: T) => boolean) | undefined
  /** Extra attributes per row (e.g. data-test ids for end-to-end tests). */
  rowAttrs?: ((row: T) => Record<string, string>) | undefined
}>(), { rowKey: 'id', responsive: 'stack', virtualAt: 200, emptyTitle: 'No records' })
const emit = defineEmits<{ (e: 'row-click', row: T): void; (e: 'load-more'): void; (e: 'update:selected', v: string[]): void }>()

const md = useBreakpoint('md')
const sortKey = ref('')
const sortDir = ref<'asc' | 'desc'>('asc')
const windowEnd = ref(props.virtualAt)

const key = (row: T) => String(row[props.rowKey] ?? '')
const text = (col: Column<T>, row: T) => (col.format ? col.format(row) : row[col.key] === undefined || row[col.key] === null ? '' : String(row[col.key]))

const sorted = computed(() => {
  if (!sortKey.value) return props.items
  const col = props.columns.find((c) => c.key === sortKey.value)
  const dir = sortDir.value === 'asc' ? 1 : -1
  return [...props.items].sort((a, b) => {
    const av = col?.format ? col.format(a) : (a[sortKey.value] as string | number | undefined) ?? ''
    const bv = col?.format ? col.format(b) : (b[sortKey.value] as string | number | undefined) ?? ''
    return (av < bv ? -1 : av > bv ? 1 : 0) * dir
  })
})
const visible = computed(() => (sorted.value.length > props.virtualAt ? sorted.value.slice(0, windowEnd.value) : sorted.value))
const truncated = computed(() => sorted.value.length > visible.value.length)

function sortBy(col: Column<T>) {
  if (!col.sortable) return
  if (sortKey.value === col.key) sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    sortKey.value = col.key
    sortDir.value = 'asc'
  }
}
const stacked = computed(() => props.responsive === 'stack' && !md.value)
const widths = { sm: 'w-24', md: 'w-40', lg: 'w-64' }

const canSelect = (row: T) => props.rowSelectable?.(row) ?? true
const isSelected = (row: T) => (props.selected ?? []).includes(key(row))
const selectableRows = computed(() => visible.value.filter(canSelect))
const allSelected = computed(() => selectableRows.value.length > 0 && selectableRows.value.every(isSelected))

function toggle(row: T) {
  if (!canSelect(row)) return
  const k = key(row)
  const cur = props.selected ?? []
  emit('update:selected', cur.includes(k) ? cur.filter((x) => x !== k) : [...cur, k])
}
// "Select all" toggles the selectable visible rows only; locked rows keep their state.
function toggleAll() {
  const locked = new Set(visible.value.filter((r) => !canSelect(r)).map(key))
  const keep = (props.selected ?? []).filter((k) => locked.has(k))
  emit('update:selected', allSelected.value ? keep : [...keep, ...selectableRows.value.map(key)])
}
</script>

<template>
  <div>
    <UiSkeleton v-if="loading && items.length === 0" kind="text" :lines="4" />
    <UiEmptyState v-else-if="items.length === 0" :title="emptyTitle" :text="emptyText"><slot name="empty" /></UiEmptyState>

    <!-- Stacked cards (phone) -->
    <ul v-else-if="stacked" class="flex flex-col gap-2" :aria-busy="loading || undefined">
      <li v-for="row in visible" :key="key(row)" class="card card-border bg-base-100 p-4 text-sm" :class="{ 'cursor-pointer active:bg-base-200': clickable }" v-bind="rowAttrs?.(row)" @click="clickable && emit('row-click', row)">
        <div class="flex items-start gap-2">
          <input v-if="selectable" type="checkbox" class="checkbox checkbox-sm mt-0.5" :checked="isSelected(row)" :disabled="!canSelect(row)" :aria-label="'Select ' + key(row)" @click.stop="toggle(row)">
          <dl class="grid min-w-0 grow grid-cols-[minmax(0,40%)_1fr] gap-x-2 gap-y-1">
            <template v-for="col in columns.filter((c) => !c.hideOnStack)" :key="col.key">
              <dt class="text-base-content/70 truncate text-sm">{{ col.label }}</dt>
              <dd class="min-w-0 break-words" :class="col.align === 'end' ? 'text-end' : ''"><slot :name="'cell-' + col.key" :row="row" :value="row[col.key]">{{ text(col, row) || '—' }}</slot></dd>
            </template>
          </dl>
          <div v-if="$slots.actions" class="shrink-0" @click.stop><slot name="actions" :row="row" /></div>
        </div>
      </li>
    </ul>

    <!-- Table (tablet/desktop): horizontal scroll contained in the wrapper -->
    <div v-else class="rounded-box bg-base-100 overflow-x-auto" :aria-busy="loading || undefined">
      <table class="table min-w-full">
        <caption v-if="caption" class="sr-only">{{ caption }}</caption>
        <thead>
          <tr>
            <th v-if="selectable" class="w-8"><input type="checkbox" class="checkbox checkbox-sm" aria-label="Select all" :checked="allSelected" :disabled="selectableRows.length === 0" @change="toggleAll"></th>
            <th v-for="col in columns" :key="col.key" :class="[col.align === 'end' ? 'text-end' : '', col.width ? widths[col.width] : '']" :aria-sort="sortKey === col.key ? (sortDir === 'asc' ? 'ascending' : 'descending') : undefined">
              <button v-if="col.sortable" type="button" class="inline-flex items-center gap-1 font-semibold" @click="sortBy(col)">
                {{ col.label }}<UiIcon v-if="sortKey === col.key" :name="sortDir === 'asc' ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="xs" />
              </button>
              <span v-else>{{ col.label }}</span>
            </th>
            <th v-if="$slots.actions" class="w-px"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in visible" :key="key(row)" :class="{ 'cursor-pointer hover:bg-base-200': clickable, 'bg-base-200': isSelected(row) }" v-bind="rowAttrs?.(row)" @click="clickable && emit('row-click', row)">
            <td v-if="selectable"><input type="checkbox" class="checkbox checkbox-sm" :checked="isSelected(row)" :disabled="!canSelect(row)" :aria-label="'Select ' + key(row)" @click.stop="toggle(row)"></td>
            <td v-for="col in columns" :key="col.key" :class="col.align === 'end' ? 'text-end' : ''"><slot :name="'cell-' + col.key" :row="row" :value="row[col.key]">{{ text(col, row) || '—' }}</slot></td>
            <td v-if="$slots.actions" class="text-end whitespace-nowrap" @click.stop><slot name="actions" :row="row" /></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="truncated || hasMore" class="mt-2 flex justify-center">
      <button v-if="truncated" type="button" class="btn btn-soft btn-sm" @click="windowEnd += virtualAt">Show more ({{ sorted.length - visible.length }} remaining)</button>
      <button v-else-if="hasMore" type="button" class="btn btn-soft btn-sm" :disabled="loading" @click="emit('load-more')">Load more</button>
    </div>
  </div>
</template>
