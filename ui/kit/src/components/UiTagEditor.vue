<script setup lang="ts">
// key=value tag map editor (assets, hosts, subnets): add with Enter, remove
// with the chip button; v-model is Record<string,string>.
import { computed, ref } from 'vue'
import UiField from './UiField.vue'
import UiIcon from './UiIcon.vue'
const props = defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; max?: number | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: Record<string, string>): void; (e: 'blur'): void }>()
const current = computed<Record<string, string>>(() => (props.modelValue && typeof props.modelValue === 'object' ? (props.modelValue as Record<string, string>) : {}))
const draft = ref('')
function add() {
  const s = draft.value.trim()
  if (!s) return
  const i = s.indexOf('=')
  const k = (i >= 0 ? s.slice(0, i) : s).trim()
  const v = i >= 0 ? s.slice(i + 1).trim() : ''
  if (!k) return
  const next = { ...current.value }
  if (props.max && !(k in next) && Object.keys(next).length >= props.max) return
  next[k] = v
  emit('update:modelValue', next)
  draft.value = ''
}
function remove(k: string) {
  const next = { ...current.value }
  delete next[k]
  emit('update:modelValue', next)
}
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint ?? 'key=value, Enter to add'" :error="error">
    <template #default="{ describedBy, invalid }">
      <div class="flex flex-wrap items-center gap-1 rounded-field border border-base-300 bg-base-100 p-1" :class="invalid ? 'border-error' : ''">
        <span v-for="(v, k) in current" :key="k" class="badge badge-soft badge-primary gap-1">
          {{ k }}<span v-if="v">={{ v }}</span>
          <button type="button" class="ms-0.5" :aria-label="'Remove tag ' + k" @click="remove(String(k))"><UiIcon name="mdi-close" size="xs" /></button>
        </span>
        <input :id="id" :data-field="id" class="min-w-32 grow border-0 bg-transparent px-1 py-1 text-sm outline-none" :value="draft" placeholder="key=value" :aria-invalid="invalid || undefined" :aria-describedby="describedBy" @input="draft = ($event.target as HTMLInputElement).value" @keydown.enter.prevent="add" @blur="add(); emit('blur')">
      </div>
    </template>
  </UiField>
</template>
