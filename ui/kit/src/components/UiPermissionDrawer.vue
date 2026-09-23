<script setup lang="ts">
// Per-record permission grants (warden folders/secrets, notification channels,
// lcm issuers/certificates): subject picker (users/roles from a caller-supplied
// list, optionally searched asynchronously), permission level select, optional
// expiry, grant/revoke rows. Rows carry an optional `expires_at`.
import { ref, watch } from 'vue'
import UiDrawer from './UiDrawer.vue'
import UiSelect, { type SelectOption } from './UiSelect.vue'
import UiCombobox from './UiCombobox.vue'
import UiInput from './UiInput.vue'
import UiButton from './UiButton.vue'
import UiAlert from './UiAlert.vue'
import UiDataTable, { type Column } from './UiDataTable.vue'

export interface Grant extends Record<string, unknown> { id: string; subject_kind: string; subject_id: string; subject_name?: string; level: string; expires_at?: string }
const props = withDefaults(defineProps<{
  modelValue: boolean
  title: string
  grants: Grant[]
  subjects: SelectOption[]
  levels: SelectOption[]
  canManage?: boolean | undefined
  /** Show an optional expiry date for new grants. */
  expires?: boolean | undefined
  /** Text above the grant row (e.g. the caller's own relation). */
  hint?: string | undefined
  /** Allow changing a row's level in place (default true). */
  editableLevel?: boolean | undefined
  error?: string | undefined
  busy?: boolean | undefined
}>(), { editableLevel: true })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'grant', v: { subject: string; level: string; expires_at?: string }): void; (e: 'revoke', g: Grant): void; (e: 'change-level', g: Grant, level: string): void; (e: 'search', q: string): void }>()
const subject = ref('')
const level = ref(props.levels[0]?.value ?? '')
const expiresAt = ref('')
watch(() => props.levels, (l) => { if (!level.value || !l.some((o) => o.value === level.value)) level.value = l[0]?.value ?? '' })
function grant() {
  const iso = expiresAt.value ? new Date(expiresAt.value + 'T00:00:00Z').toISOString() : undefined
  emit('grant', iso ? { subject: subject.value, level: level.value, expires_at: iso } : { subject: subject.value, level: level.value })
  subject.value = ''
  expiresAt.value = ''
}
const columns: Column<Grant>[] = [
  { key: 'subject_name', label: 'Subject', format: (g) => g.subject_name ?? g.subject_id },
  { key: 'subject_kind', label: 'Kind', hideOnStack: true },
  { key: 'level', label: 'Permission' },
  ...(props.expires ? [{ key: 'expires_at', label: 'Expires', format: (g: Grant) => (g.expires_at ? new Date(g.expires_at).toLocaleDateString() : ''), hideOnStack: true }] : []),
]
</script>

<template>
  <UiDrawer :model-value="modelValue" :title="title" size="lg" @update:model-value="emit('update:modelValue', $event)">
    <UiAlert v-if="error" kind="error" class="mb-3">{{ error }}</UiAlert>
    <p v-if="hint" class="mb-2 text-xs text-base-content/70">{{ hint }}</p>
    <div v-if="canManage" class="mb-4 grid grid-cols-1 gap-2 md:grid-cols-12 md:items-end">
      <div :class="expires ? 'md:col-span-4' : 'md:col-span-6'"><UiCombobox id="perm-subject" v-model="subject" label="User or role" :options="subjects" @search="emit('search', $event)" /></div>
      <div :class="expires ? 'md:col-span-3' : 'md:col-span-4'"><UiSelect id="perm-level" v-model="level" label="Permission" :options="levels" :clearable="false" required /></div>
      <div v-if="expires" class="md:col-span-3"><UiInput id="perm-expires" v-model="expiresAt" label="Expires (optional)" type="date" /></div>
      <div class="md:col-span-2"><UiButton block :disabled="!subject || !level" :loading="busy" @click="grant">Grant</UiButton></div>
    </div>
    <UiAlert v-else kind="info" class="mb-3">You need the share permission to grant access here.</UiAlert>
    <UiDataTable :items="grants" :columns="columns" empty-title="No grants">
      <template #cell-level="{ row }">
        <select v-if="canManage && editableLevel" class="select select-sm" :value="row.level" :aria-label="'Permission for ' + (row.subject_name ?? row.subject_id)" @change="emit('change-level', row, ($event.target as HTMLSelectElement).value)">
          <option v-for="l in levels" :key="l.value" :value="l.value">{{ l.title }}</option>
        </select>
        <span v-else>{{ row.level }}</span>
      </template>
      <template v-if="canManage" #actions="{ row }"><UiButton variant="text" color="error" size="xs" icon="mdi-close" icon-only label="Revoke" @click="emit('revoke', row)" /></template>
    </UiDataTable>
  </UiDrawer>
</template>
