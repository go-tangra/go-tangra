<script setup lang="ts">
// Tenant audit trail viewer used by warden/notification/lcm/auth: filters
// (actor, event type, from/to), JSON detail expander. Paging is the list
// contract (`paging="page"`: page/page_size + total, newest first) or the
// older cursor "load more" (default, kept for one release).
import { onMounted, ref } from 'vue'
import type { Api } from '@/api/client'
import { describe } from '@/api/client'
import UiDataTable, { type Column } from './UiDataTable.vue'
import UiInput from './UiInput.vue'
import UiButton from './UiButton.vue'
import UiAlert from './UiAlert.vue'
import UiStatusChip from './UiStatusChip.vue'

export interface AuditRow extends Record<string, unknown> { id: string; at: string; actor_id?: string; actor_kind?: string; action?: string; event_type?: string; subject_kind?: string; subject_id?: string; outcome?: string; reason?: string; detail?: unknown }
const props = withDefaults(defineProps<{ api: Api; path?: string | undefined; pageSize?: number | undefined; paging?: 'cursor' | 'page' | undefined }>(), { path: 'audit', pageSize: 50, paging: 'cursor' })
const items = ref<AuditRow[]>([])
const error = ref('')
const loading = ref(false)
const cursor = ref('')
const hasMore = ref(false)
const actor = ref('')
const type = ref('')
const from = ref('')
const to = ref('')
const open = ref<string | null>(null)
const page = ref(1)
const size = ref(props.pageSize)
const total = ref<number | undefined>(undefined)

async function load(reset = true) {
  if (props.paging === 'page') return loadPage(reset ? 1 : page.value)
  loading.value = true
  error.value = ''
  try {
    const res = await props.api<{ items: AuditRow[]; next_cursor?: string }>('GET', props.path, undefined, { query: { actor: actor.value || undefined, event_type: type.value || undefined, from: from.value || undefined, to: to.value || undefined, cursor: reset ? undefined : cursor.value || undefined, limit: props.pageSize } })
    const rows = res.items ?? []
    items.value = reset ? rows : [...items.value, ...rows]
    cursor.value = res.next_cursor ?? rows[rows.length - 1]?.id ?? ''
    hasMore.value = rows.length >= props.pageSize
  } catch (e) {
    error.value = describe(e)
  } finally {
    loading.value = false
  }
}
async function loadPage(p: number) {
  loading.value = true
  error.value = ''
  try {
    const res = await props.api<{ items: AuditRow[]; total: number; page: number; page_size: number }>('GET', props.path, undefined, { query: { actor: actor.value || undefined, event_type: type.value || undefined, from: from.value || undefined, to: to.value || undefined, page: p, page_size: size.value } })
    items.value = res.items ?? []
    total.value = res.total ?? 0
    page.value = res.page ?? p
  } catch (e) {
    error.value = describe(e)
  } finally {
    loading.value = false
  }
}
function onSize(s: number) {
  size.value = s
  void loadPage(1)
}
onMounted(() => load())
const columns: Column<AuditRow>[] = [
  { key: 'at', label: 'When', format: (r) => new Date(r.at).toLocaleString(), sortable: props.paging !== 'page' },
  { key: 'action', label: 'Event', format: (r) => String(r.action ?? r.event_type ?? '') },
  { key: 'actor_id', label: 'Actor', format: (r) => [r.actor_kind, r.actor_id].filter(Boolean).join(' ') , hideOnStack: true },
  { key: 'subject_id', label: 'Subject', format: (r) => [r.subject_kind, r.subject_id].filter(Boolean).join(' ') },
  { key: 'outcome', label: 'Outcome' },
]
</script>

<template>
  <div class="flex flex-col gap-3">
    <div class="grid grid-cols-2 gap-2 md:grid-cols-5 md:items-end">
      <UiInput id="audit-actor" v-model="actor" label="Actor" size="sm" @enter="load()" />
      <UiInput id="audit-type" v-model="type" label="Event type" size="sm" @enter="load()" />
      <UiInput id="audit-from" v-model="from" label="From" type="date" size="sm" />
      <UiInput id="audit-to" v-model="to" label="To" type="date" size="sm" />
      <UiButton size="sm" variant="soft" icon="mdi-filter-outline" @click="load()">Filter</UiButton>
    </div>
    <UiAlert v-if="error" kind="error">{{ error }}</UiAlert>
    <UiDataTable :items="items" :columns="columns" :loading="loading" :has-more="hasMore" :total="total" :page="page" :page-size="size" clickable empty-title="No audit events" @row-click="open = open === $event.id ? null : $event.id" @load-more="load(false)" @update:page="loadPage" @update:page-size="onSize">
      <template #cell-outcome="{ row }"><UiStatusChip :status="String(row.outcome ?? '')" :colors="{ ok: 'success', refused: 'warning', error: 'error' }" /></template>
    </UiDataTable>
    <pre v-if="open" class="max-h-64 overflow-auto rounded-box bg-base-200 p-3 text-xs">{{ JSON.stringify(items.find((r) => r.id === open)?.detail ?? {}, null, 2) }}</pre>
  </div>
</template>
