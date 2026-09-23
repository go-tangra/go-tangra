<script setup lang="ts">
import { ref } from 'vue'
import { UiPage, UiSection, UiCard, UiGrid, UiDataTable, UiKeyValueTable, UiStatTile, UiStatGrid, UiBarList, UiPagination, UiTree, UiStatusChip, UiButton, useToast, type Column, type KeyValue, type TreeNode } from '@/index'

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
        <UiCard title="Pagination"><UiPagination has-prev has-next label="Page 2" /><UiPagination class="mt-2" :has-prev="false" :has-next="false" label="Only page" /></UiCard>
        <UiCard title="Tree"><UiTree v-model:selected="treeSel" :items="tree" /><p class="mt-2 text-sm">Selected: {{ treeSel }}</p></UiCard>
      </UiGrid>
    </UiSection>
  </UiPage>
</template>
