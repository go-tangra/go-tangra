<script setup lang="ts">
import { computed, ref } from 'vue'
import { UiPage, UiSection, UiCard, UiGrid, UiDataTable, UiKeyValueTable, UiStatTile, UiStatGrid, UiBarList, UiPagination, UiPager, UiTree, UiStatusChip, UiButton, useToast, type Column, type TableSort, type KeyValue, type TreeNode } from '@/index'

type Row = Record<string, unknown> & { id: string; name: string; status: string; count: number; owner: string }
const rows: Row[] = Array.from({ length: 12 }, (_, i) => ({ id: String(i + 1), name: 'Record ' + (i + 1), status: ['active', 'pending', 'broken'][i % 3]!, count: (i * 7) % 23, owner: ['ops', 'lab', 'dev'][i % 3]! }))
const columns: Column<Row>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'status', label: 'Status', width: 'sm' },
  { key: 'count', label: 'Count', align: 'end', sortable: true, hideOnStack: true },
  { key: 'owner', label: 'Owner', format: (r) => r.owner.toUpperCase(), width: 'sm' },
]
const many = Array.from({ length: 450 }, (_, i) => ({ id: String(i), name: 'Row ' + i, status: 'active', count: i, owner: 'gen' }))
const selected = ref<string[]>([])
// Server mode demo: a fake 1,234-row source sorted and sliced as a server would.
const source: Row[] = Array.from({ length: 1234 }, (_, i) => ({ id: String(i + 1), name: 'Host ' + String(i + 1).padStart(4, '0'), status: ['active', 'pending', 'broken'][i % 3]!, count: (i * 37) % 1000, owner: ['ops', 'lab', 'dev'][i % 3]! }))
const serverColumns: Column<Row>[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'status', label: 'Status', width: 'sm', sortable: true },
  { key: 'count', label: 'Count', align: 'end', sortable: true, defaultDir: 'desc' },
  { key: 'owner', label: 'Owner', width: 'sm' },
]
const sPage = ref(1)
const sSize = ref(25)
const sSort = ref<TableSort>({ key: 'name', dir: 'asc' })
const sRows = computed(() => {
  const k = sSort.value.key as keyof Row
  const dir = sSort.value.dir === 'asc' ? 1 : -1
  const all = [...source].sort((a, b) => (a[k]! < b[k]! ? -1 : a[k]! > b[k]! ? 1 : Number(a.id) - Number(b.id)) * dir)
  return all.slice((sPage.value - 1) * sSize.value, sPage.value * sSize.value)
})
function onSort(s: TableSort) { sSort.value = s; sPage.value = 1 }
function onSize(n: number) { sPage.value = Math.floor(((sPage.value - 1) * sSize.value) / n) + 1; sSize.value = n }
const kv: KeyValue[] = [{ label: 'Identifier', value: 'rec-000123', copyable: true }, { label: 'Tags', value: ['a', 'b'] }, { label: 'Empty' }, { label: 'Nested', value: { k: 1 } }]
const tree: TreeNode[] = [{ id: 'r', label: 'Root', icon: 'mdi-folder-outline', children: [{ id: 'a', label: 'Branch A', badge: '3', children: [{ id: 'a1', label: 'Leaf A1' }, { id: 'a2', label: 'Leaf A2' }] }, { id: 'b', label: 'Branch B' }] }]
const treeSel = ref('a1')
const toast = useToast()
</script>

<template>
  <UiPage title="Data" subtitle="Tables, key/value, stats, pagination, tree">
    <UiSection title="Stat tiles">
      <UiStatGrid :cols="4">
        <UiStatTile title="Assets" :value="1284" icon="mdi-laptop" color="primary" subtitle="+12 this week" />
        <UiStatTile title="Broken" :value="3" icon="mdi-alert-circle-outline" color="error" />
        <UiStatTile title="Pending" value="17" icon="mdi-clock-outline" color="warning" />
        <UiStatTile title="Healthy" value="99.2%" color="success" />
      </UiStatGrid>
    </UiSection>
    <UiSection title="Data table" description="Stacks below md; contained horizontal scroll above">
      <UiCard :padded="false">
        <UiDataTable v-model:selected="selected" :items="rows" :columns="columns" caption="Records" clickable selectable has-more @row-click="toast.info('Row ' + $event.id)" @load-more="toast.info('load more')">
          <template #cell-status="{ row }"><UiStatusChip :status="String(row.status)" /></template>
          <template #actions="{ row }"><UiButton size="xs" variant="text" icon="mdi-pencil" icon-only label="Edit" @click.stop="toast.info('edit ' + row.id)" /></template>
        </UiDataTable>
      </UiCard>
      <p class="mt-2 text-sm">Selected: {{ selected.join(', ') || '—' }}</p>
      <UiCard class="mt-4" title="Server mode (1,234 rows)" :padded="false" data-test="server-table">
        <UiDataTable :items="sRows" :columns="serverColumns" :total="source.length" :page="sPage" :page-size="sSize" :sort="sSort" caption="Server-paged hosts" @update:sort="onSort" @update:page="sPage = $event" @update:page-size="onSize">
          <template #cell-status="{ row }"><UiStatusChip :status="String(row.status)" /></template>
        </UiDataTable>
      </UiCard>
      <UiGrid class="mt-4" :cols="1" :lg-cols="3">
        <UiCard title="Empty"><UiDataTable :items="[]" :columns="columns" empty-title="No rows" empty-text="Nothing matches." /></UiCard>
        <UiCard title="Loading"><UiDataTable :items="[]" :columns="columns" loading /></UiCard>
        <UiCard title="Scroll mode"><UiDataTable :items="rows.slice(0, 3)" :columns="columns" responsive="scroll" /></UiCard>
      </UiGrid>
      <UiCard class="mt-4" title="Virtualised (450 rows, 200 at a time)" :padded="false"><UiDataTable :items="many" :columns="columns" :virtual-at="200" /></UiCard>
    </UiSection>
    <UiSection title="Key/value, pagination, tree">
      <UiGrid :cols="1" :lg-cols="3">
        <UiCard title="Key/value"><UiKeyValueTable :items="kv" /><UiKeyValueTable class="mt-4" :items="kv" :columns="2" /></UiCard>
        <UiCard title="Bar list"><UiBarList :items="[{ label: 'deployable', value: 42, color: 'success' }, { label: 'assigned', value: 17, color: 'info' }, { label: 'broken', value: 3, color: 'error' }]" /></UiCard>
        <UiCard title="Pagination"><UiPagination has-prev has-next label="Page 2" /><UiPagination class="mt-2" :has-prev="false" :has-next="false" label="Only page" /><UiPager class="mt-2" :page="6" :page-size="50" :total="1234" /></UiCard>
        <UiCard title="Tree"><UiTree v-model:selected="treeSel" :items="tree" /><p class="mt-2 text-sm">Selected: {{ treeSel }}</p></UiCard>
      </UiGrid>
    </UiSection>
  </UiPage>
</template>
