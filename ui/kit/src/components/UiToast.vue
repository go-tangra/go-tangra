<script setup lang="ts">
// The single toast host (mounted by the shell/console). Reads the shared store.
// Positioned with utilities: FlyonUI has no daisyUI-style `toast` component.
import { useToast } from '@/composables/useToast'
import UiIcon from './UiIcon.vue'
const { items, dismiss } = useToast()
const cls = { info: 'alert-info', success: 'alert-success', warning: 'alert-warning', error: 'alert-error' }
const icon = { info: 'mdi-information-outline', success: 'mdi-check-circle-outline', warning: 'mdi-alert-outline', error: 'mdi-alert-circle-outline' }
</script>

<template>
  <div class="fixed end-4 bottom-4 z-[60] flex w-[calc(100%-2rem)] max-w-sm flex-col gap-2" aria-live="polite" aria-atomic="false">
    <div v-for="t in items" :key="t.id" class="alert alert-soft flex items-start gap-2 shadow-lg" :class="cls[t.kind]" :role="t.kind === 'error' ? 'alert' : 'status'">
      <UiIcon :name="icon[t.kind]" class="mt-0.5" />
      <div class="min-w-0 grow">
        <div class="font-semibold">{{ t.title }}</div>
        <div v-if="t.text" class="text-sm">{{ t.text }}</div>
      </div>
      <button type="button" class="btn btn-text btn-circle btn-xs" aria-label="Dismiss" @click="dismiss(t.id)"><UiIcon name="mdi-close" size="sm" /></button>
    </div>
  </div>
</template>
