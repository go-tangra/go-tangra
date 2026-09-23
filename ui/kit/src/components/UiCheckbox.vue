<script setup lang="ts">
defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; error?: string | undefined; disabled?: boolean | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'blur'): void }>()
</script>

<template>
  <div class="form-control">
    <label class="label cursor-pointer justify-start gap-2" :for="id">
      <input :id="id" :data-field="id" type="checkbox" class="checkbox checkbox-primary checkbox-sm" :checked="!!modelValue" :disabled="disabled" :aria-invalid="!!error || undefined" @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)" @blur="emit('blur')">
      <span class="label-text text-sm">{{ label }}</span>
    </label>
    <p v-if="error" class="helper-text text-error text-xs" role="alert">{{ error }}</p>
    <p v-else-if="hint" class="helper-text text-xs text-base-content/70">{{ hint }}</p>
  </div>
</template>
