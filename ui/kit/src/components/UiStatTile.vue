<script setup lang="ts">
import UiIcon from './UiIcon.vue'
withDefaults(defineProps<{ title: string; value: string | number; icon?: string | undefined; color?: 'primary' | 'secondary' | 'accent' | 'info' | 'success' | 'warning' | 'error' | 'neutral' | undefined; subtitle?: string | undefined }>(), { color: 'neutral' })
const bg = { primary: 'bg-primary/10 text-primary', secondary: 'bg-secondary/10 text-secondary', accent: 'bg-accent/10 text-accent', info: 'bg-info/10 text-info', success: 'bg-success/10 text-success', warning: 'bg-warning/10 text-warning', error: 'bg-error/10 text-error', neutral: 'bg-base-200 text-base-content' }
</script>

<template>
  <div class="stat-tile card shadow-base-300/10 shadow-md">
    <div class="card-body flex-row justify-between gap-3">
      <div class="min-w-0 space-y-1">
        <p class="text-base-content/70 truncate">{{ title }}</p>
        <h4 class="text-base-content truncate text-2xl font-medium">{{ value }}</h4>
        <small v-if="subtitle" class="text-base-content/70 block truncate">{{ subtitle }}</small>
      </div>
      <div v-if="icon" class="avatar avatar-placeholder shrink-0">
        <div class="rounded-field size-9.5 flex items-center justify-center" :class="bg[color]"><UiIcon :name="icon" size="lg" /></div>
      </div>
    </div>
  </div>
</template>
