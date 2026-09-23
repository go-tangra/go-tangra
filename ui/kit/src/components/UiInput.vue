<script setup lang="ts">
import { computed } from 'vue'
import UiField from './UiField.vue'
const props = withDefaults(defineProps<{
  modelValue?: unknown | undefined
  id: string
  label: string
  type?: 'text' | 'email' | 'url' | 'search' | 'tel' | 'number' | 'date' | 'datetime-local' | 'password' | undefined
  hint?: string | undefined
  error?: string | undefined
  required?: boolean | undefined
  placeholder?: string | undefined
  disabled?: boolean | undefined
  readonly?: boolean | undefined
  autocomplete?: string | undefined
  inputmode?: 'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url' | undefined
  min?: number | string | undefined
  max?: number | string | undefined
  step?: number | string | undefined
  size?: 'sm' | 'md' | undefined
  srOnlyLabel?: boolean | undefined
}>(), { type: 'text', size: 'md' })
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void; (e: 'enter'): void }>()
const display = computed(() => (props.modelValue === undefined || props.modelValue === null ? '' : String(props.modelValue)))
function onInput(e: Event) {
  const v = (e.target as HTMLInputElement).value
  emit('update:modelValue', props.type === 'number' ? (v === '' ? '' : Number(v)) : v)
}
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required" :sr-only-label="srOnlyLabel">
    <template #default="{ describedBy, invalid }">
      <input
        :id="id"
        :data-field="id"
        class="input w-full"
        :class="[size === 'sm' ? 'input-sm' : '', invalid ? 'is-invalid' : '']"
        :type="type"
        :value="display"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        :required="required"
        :autocomplete="autocomplete"
        :inputmode="inputmode"
        :min="min"
        :max="max"
        :step="step"
        :aria-invalid="invalid || undefined"
        :aria-describedby="describedBy"
        @input="onInput"
        @blur="emit('blur')"
        @keyup.enter="emit('enter')"
      >
    </template>
  </UiField>
</template>
