<script setup lang="ts">
// Documents attached to an entity: list, upload (with description), download,
// delete. Talks to any API exposing GET <base>, POST <base> (multipart),
// DELETE <base>/{id}, GET <base>/{id}/download — the asset/paperless shape.
import { onMounted, ref, watch } from 'vue'
import type { Api } from '@/api/client'
import { describe } from '@/api/client'
import { useConfirm } from '@/composables/useConfirm'
import UiCard from './UiCard.vue'
import UiAlert from './UiAlert.vue'
import UiFilePicker from './UiFilePicker.vue'
import UiInput from './UiInput.vue'
import UiButton from './UiButton.vue'
import UiDataTable, { type Column } from './UiDataTable.vue'

export interface DocumentRow extends Record<string, unknown> { id: string; file_name: string; file_size: number; mime_type?: string; description?: string; created_at: string }
const props = withDefaults(defineProps<{ api: Api; base: string; title?: string | undefined; canManage?: boolean | undefined; maxBytes?: number | undefined; accept?: string | undefined }>(), { title: 'Documents', canManage: true })
const items = ref<DocumentRow[]>([])
const error = ref('')
const busy = ref(false)
const file = ref<File | null>(null)
const description = ref('')
const confirm = useConfirm()

async function reload() {
  error.value = ''
  try {
    items.value = (await props.api<{ items: DocumentRow[] }>('GET', props.base)).items ?? []
  } catch (e) {
    error.value = describe(e)
  }
}
onMounted(reload)
watch(() => props.base, reload)
async function send() {
  if (!file.value) return
  busy.value = true
  error.value = ''
  try {
    await props.api.upload(props.base, file.value, { description: description.value })
    file.value = null
    description.value = ''
    await reload()
  } catch (e) {
    error.value = describe(e)
  } finally {
    busy.value = false
  }
}
async function remove(d: DocumentRow) {
  if (!(await confirm.ask({ title: 'Delete document?', text: d.file_name, danger: true, confirmLabel: 'Delete' }))) return
  try {
    await props.api('DELETE', props.base + '/' + d.id)
    await reload()
  } catch (e) {
    error.value = describe(e)
  }
}
const size = (n: number) => (n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB')
const columns: Column<DocumentRow>[] = [
  { key: 'file_name', label: 'Name' },
  { key: 'mime_type', label: 'Type', hideOnStack: true },
  { key: 'file_size', label: 'Size', format: (r) => size(r.file_size), align: 'end' },
  { key: 'description', label: 'Description' },
  { key: 'created_at', label: 'Uploaded', format: (r) => new Date(r.created_at).toLocaleString(), hideOnStack: true },
]
defineExpose({ reload })
</script>

<template>
  <UiCard :title="title" flat>
    <UiAlert v-if="error" kind="error" dismissible @dismiss="error = ''">{{ error }}</UiAlert>
    <div v-if="canManage" class="grid grid-cols-1 gap-2 md:grid-cols-12 md:items-end">
      <div class="md:col-span-6"><UiFilePicker id="doc-file" v-model="file" label="File" :accept="accept" :max-bytes="maxBytes" @too-large="error = 'The file exceeds the upload limit.'" /></div>
      <div class="md:col-span-4"><UiInput id="doc-description" v-model="description" label="Description" /></div>
      <div class="md:col-span-2"><UiButton block :disabled="!file" :loading="busy" icon="mdi-upload" @click="send">Upload</UiButton></div>
    </div>
    <UiDataTable :items="items" :columns="columns" empty-title="No documents" class="mt-3">
      <template #cell-file_name="{ row }"><a class="link link-primary" :href="api.fileUrl(base + '/' + row.id + '/download')" target="_blank" rel="noopener">{{ row.file_name }}</a></template>
      <template v-if="canManage" #actions="{ row }"><UiButton variant="text" color="error" size="xs" icon="mdi-delete-outline" icon-only label="Delete" @click="remove(row)" /></template>
    </UiDataTable>
  </UiCard>
</template>
