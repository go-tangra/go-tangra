<script setup lang="ts">
// Searchable single select (ARIA 1.2 combobox with listbox popup): type to
// filter, Arrow keys to move, Enter to choose, Esc to close; Vue-owned state.
import { computed, ref, watch } from 'vue'
import UiField from './UiField.vue'
import type { SelectOption } from './UiSelect.vue'

const props = defineProps<{ modelValue?: unknown | undefined; id: string; label: string; options: SelectOption[]; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; placeholder?: string | undefined; disabled?: boolean | undefined }>()
// `search` lets a parent fetch options asynchronously (directory lookups) as the person types.
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void; (e: 'search', q: string): void }>()
const query = ref('')
const open = ref(false)
const active = ref(0)
const listId = computed(() => props.id + '-listbox')
const selected = computed(() => props.options.find((o) => o.value === props.modelValue))
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? props.options.filter((o) => o.title.toLowerCase().includes(q)) : props.options
})
watch(selected, (s) => { if (!open.value) query.value = s?.title ?? '' }, { immediate: true })
function choose(o: SelectOption) {
  emit('update:modelValue', o.value)
  query.value = o.title
  open.value = false
}
function clear() {
  emit('update:modelValue', '')
  query.value = ''
}
function onInput(e: Event) {
  query.value = (e.target as HTMLInputElement).value
  open.value = true
  active.value = 0
  emit('search', query.value)
  if (query.value === '') emit('update:modelValue', '')
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') { e.preventDefault(); open.value = true; active.value = Math.min(active.value + 1, filtered.value.length - 1) }
  else if (e.key === 'ArrowUp') { e.preventDefault(); active.value = Math.max(active.value - 1, 0) }
  else if (e.key === 'Enter' && open.value) { e.preventDefault(); const o = filtered.value[active.value]; if (o) choose(o) }
  else if (e.key === 'Escape') { open.value = false; query.value = selected.value?.title ?? '' }
}
function onBlur() {
  setTimeout(() => { open.value = false; if (!selected.value) query.value = ''; else query.value = selected.value.title; emit('blur') }, 120)
}
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <template #default="{ describedBy, invalid }">
      <div class="relative">
        <input
          :id="id"
          :data-field="id"
          class="input w-full pe-8"
          :class="invalid ? 'is-invalid' : ''"
          role="combobox"
          :aria-expanded="open"
          :aria-controls="listId"
          :aria-activedescendant="open && filtered[active] ? listId + '-' + active : undefined"
          aria-autocomplete="list"
          :aria-invalid="invalid || undefined"
          :aria-describedby="describedBy"
          :value="query"
          :placeholder="placeholder"
          :disabled="disabled"
          autocomplete="off"
          @input="onInput"
          @focus="open = true"
          @keydown="onKey"
          @blur="onBlur"
        >
        <button v-if="modelValue && !disabled" type="button" class="btn btn-text btn-circle btn-xs absolute end-1 top-1/2 -translate-y-1/2" aria-label="Clear" @mousedown.prevent="clear">×</button>
        <ul v-if="open" :id="listId" role="listbox" class="menu absolute z-40 mt-1 max-h-60 w-full flex-nowrap overflow-y-auto rounded-box border border-base-300 bg-base-100 p-1 shadow-lg">
          <li v-if="filtered.length === 0" class="px-3 py-2 text-sm text-base-content/70" role="option" aria-selected="false">No matches</li>
          <li v-for="(o, i) in filtered" :id="listId + '-' + i" :key="o.value" role="option" :aria-selected="o.value === modelValue">
            <button type="button" :class="{ 'active': i === active }" tabindex="-1" @mousedown.prevent="choose(o)">{{ o.title }}</button>
          </li>
        </ul>
      </div>
    </template>
  </UiField>
</template>
