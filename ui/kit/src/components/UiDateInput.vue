<script setup lang="ts">
// Native date input; the v-model carries "YYYY-MM-DD" (isoDate schemas
// normalise it to ISO on submit). ISO values coming in are cut to the date.
import { computed } from 'vue'
import UiInput from './UiInput.vue'
const props = defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; disabled?: boolean | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void }>()
const day = computed(() => (typeof props.modelValue === 'string' ? props.modelValue.slice(0, 10) : ''))
</script>

<template>
  <UiInput :id="id" type="date" :label="label" :hint="hint" :error="error" :required="required" :disabled="disabled" :model-value="day" @update:model-value="emit('update:modelValue', $event)" @blur="emit('blur')" />
</template>
