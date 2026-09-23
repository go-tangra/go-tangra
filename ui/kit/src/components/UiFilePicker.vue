<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UiField from './UiField.vue'
const props = defineProps<{ modelValue?: unknown | undefined; id: string; label: string; accept?: string | undefined; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; disabled?: boolean | undefined; maxBytes?: number | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: File | null): void; (e: 'blur'): void; (e: 'too-large', f: File): void }>()
const input = ref<HTMLInputElement | null>(null)
const file = computed(() => (props.modelValue instanceof File ? props.modelValue : null))
// A form reset (modelValue → null) clears the native control too.
watch(file, (f) => { if (!f && input.value) input.value.value = '' })
function onChange(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  if (f && props.maxBytes && f.size > props.maxBytes) {
    emit('too-large', f)
    if (input.value) input.value.value = ''
    emit('update:modelValue', null)
    return
  }
  emit('update:modelValue', f)
}
function human(n: number) {
  return n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB'
}
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <template #default="{ describedBy, invalid }">
      <input :id="id" ref="input" :data-field="id" type="file" class="input w-full" :class="invalid ? 'is-invalid' : ''" :accept="accept" :disabled="disabled" :aria-invalid="invalid || undefined" :aria-describedby="describedBy" @change="onChange" @blur="emit('blur')">
      <p v-if="file" class="mt-1 text-xs text-base-content/70">{{ file.name }} · {{ human(file.size) }}</p>
    </template>
  </UiField>
</template>
