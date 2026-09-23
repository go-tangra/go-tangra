<script setup lang="ts">
// Label + control + hint/error wrapper. Wires aria-describedby / aria-invalid
// onto the slotted control through the `describedBy`/`invalid` slot props.
import { computed } from 'vue'
const props = defineProps<{ id: string; label: string; hint?: string | undefined; error?: string | undefined; required?: boolean | undefined; srOnlyLabel?: boolean | undefined }>()
const hintId = computed(() => props.id + '-hint')
const errorId = computed(() => props.id + '-error')
const describedBy = computed(() => (props.error ? errorId.value : props.hint ? hintId.value : undefined))
</script>

<template>
  <div class="form-control w-full">
    <label :for="id" class="label-text" :class="{ 'sr-only': srOnlyLabel }">
      {{ label }}<span v-if="required" class="text-error ms-0.5" aria-hidden="true">*</span>
    </label>
    <slot :described-by="describedBy" :invalid="!!error" />
    <p v-if="error" :id="errorId" class="helper-text text-error mt-1 text-sm" role="alert">{{ error }}</p>
    <p v-else-if="hint" :id="hintId" class="helper-text text-base-content/70 mt-1 text-sm">{{ hint }}</p>
  </div>
</template>
