<script setup lang="ts">
// Copies a value to the clipboard with a transient confirmation. Secrets are
// passed as a value only; nothing is stored.
import { ref } from 'vue'
import UiIcon from './UiIcon.vue'
const props = withDefaults(defineProps<{ value: string; label?: string | undefined; size?: 'xs' | 'sm' | undefined }>(), { label: 'Copy', size: 'sm' })
const done = ref(false)
async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
    done.value = true
    setTimeout(() => (done.value = false), 1500)
  } catch {
    /* clipboard unavailable: the value is still visible */
  }
}
</script>

<template>
  <button type="button" class="btn btn-soft btn-secondary" :class="size === 'xs' ? 'btn-xs' : 'btn-sm'" :aria-label="done ? 'Copied' : label" @click="copy">
    <UiIcon :name="done ? 'mdi-check' : 'mdi-content-copy'" size="sm" /><span v-if="label !== ''">{{ done ? 'Copied' : label }}</span>
  </button>
</template>
