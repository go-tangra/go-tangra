<script setup lang="ts">
import UiField from './UiField.vue'
withDefaults(defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; rows?: number | undefined; disabled?: boolean | undefined; placeholder?: string | undefined }>(), { rows: 3 })
const emit = defineEmits<{ (e: 'update:modelValue', v: unknown): void; (e: 'blur'): void }>()
</script>

<template>
  <UiField :id="id" :label="label" :hint="hint" :error="error" :required="required">
    <template #default="{ describedBy, invalid }">
      <textarea :id="id" :data-field="id" class="textarea w-full" :class="invalid ? 'is-invalid' : ''" :rows="rows" :value="(modelValue as string) ?? ''" :disabled="disabled" :placeholder="placeholder" :aria-invalid="invalid || undefined" :aria-describedby="describedBy" @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)" @blur="emit('blur')" />
    </template>
  </UiField>
</template>
