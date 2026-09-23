<script setup lang="ts">
import UiInput from './UiInput.vue'
defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; min?: number | undefined; max?: number | undefined; step?: number | undefined; disabled?: boolean | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void }>()
</script>

<template>
  <UiInput :id="id" type="number" :label="label" :hint="hint" :error="error" :required="required" :min="min" :max="max" :step="step ?? 'any'" :disabled="disabled" :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" @blur="emit('blur')" />
</template>
