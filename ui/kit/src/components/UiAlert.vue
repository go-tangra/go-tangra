<script setup lang="ts">
import UiIcon from './UiIcon.vue'
const props = withDefaults(defineProps<{ kind?: 'info' | 'success' | 'warning' | 'error' | undefined; title?: string | undefined; dismissible?: boolean | undefined }>(), { kind: 'info' })
const emit = defineEmits<{ (e: 'dismiss'): void }>()
const cls = { info: 'alert-info', success: 'alert-success', warning: 'alert-warning', error: 'alert-error' }
const icon = { info: 'mdi-information-outline', success: 'mdi-check-circle-outline', warning: 'mdi-alert-outline', error: 'mdi-alert-circle-outline' }
</script>

<template>
  <div class="alert alert-soft flex items-start gap-2" :class="cls[props.kind]" :role="kind === 'error' || kind === 'warning' ? 'alert' : 'status'">
    <UiIcon :name="icon[props.kind]" class="mt-0.5" />
    <div class="min-w-0 grow">
      <div v-if="title" class="font-semibold">{{ title }}</div>
      <div class="text-sm"><slot /></div>
    </div>
    <button v-if="dismissible" type="button" class="btn btn-text btn-circle btn-xs" aria-label="Dismiss" @click="emit('dismiss')"><UiIcon name="mdi-close" size="sm" /></button>
  </div>
</template>
