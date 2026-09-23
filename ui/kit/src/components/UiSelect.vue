<script setup lang="ts">
import UiField from './UiField.vue'
export interface SelectOption { title: string; value: string }
withDefaults(defineProps<{ modelValue?: unknown | undefined; id: string; label: string; options: SelectOption[]; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; clearable?: boolean | undefined; disabled?: boolean | undefined; placeholder?: string | undefined; size?: 'sm' | 'md' | undefined; srOnlyLabel?: boolean | undefined }>(), { placeholder: '—', size: 'md', clearable: true })
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void; (e: 'change', v: unknown): void }>()
function onChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value
  emit('update:modelValue', v)
  emit('change', v)
}
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required" :sr-only-label="srOnlyLabel">
    <template #default="{ describedBy, invalid }">
      <select :id="id" :data-field="id" class="select w-full" :class="[size === 'sm' ? 'select-sm' : '', invalid ? 'is-invalid' : '']" :value="(modelValue as string) ?? ''" :disabled="disabled" :required="required" :aria-invalid="invalid || undefined" :aria-describedby="describedBy" @change="onChange" @blur="emit('blur')">
        <option v-if="clearable || !required" value="">{{ placeholder }}</option>
        <option v-for="o in options" :key="o.value" :value="o.value">{{ o.title }}</option>
      </select>
    </template>
  </UiField>
</template>
