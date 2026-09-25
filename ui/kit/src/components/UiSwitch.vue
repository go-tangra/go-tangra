<script setup lang="ts">
// FlyonUI 2 has no row layout for label.label (and .label-text is a block):
// the kit lays the control and its text out on one centred line itself.
defineProps<{ modelValue?: unknown | undefined; id: string; label: string; hint?: string | undefined; disabled?: boolean | undefined }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
</script>

<template>
  <div class="form-control">
    <label class="label flex cursor-pointer items-center justify-start gap-2" :for="id">
      <input :id="id" :data-field="id" type="checkbox" role="switch" class="switch switch-primary switch-sm" :checked="!!modelValue" :aria-checked="!!modelValue" :disabled="disabled" @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked)">
      <span class="label-text text-sm">{{ label }}</span>
    </label>
    <p v-if="hint" class="helper-text text-xs text-base-content/70">{{ hint }}</p>
  </div>
</template>
