<script setup lang="ts">
import UiIcon from './UiIcon.vue'
withDefaults(defineProps<{ title?: string | undefined; text?: string | undefined; retryLabel?: string | undefined }>(), { title: 'Something went wrong', retryLabel: 'Retry' })
const emit = defineEmits<{ (e: 'retry'): void }>()
</script>

<template>
  <div role="alert" class="card border border-error/40 bg-base-100">
    <div class="card-body items-center gap-2 py-8 text-center">
      <UiIcon name="mdi-alert-circle-outline" size="xl" class="text-error" />
      <div class="font-semibold">{{ title }}</div>
      <p v-if="text" class="max-w-md text-sm text-base-content/70">{{ text }}</p>
      <button type="button" class="btn btn-primary btn-sm mt-2" @click="emit('retry')"><UiIcon name="mdi-reload" size="sm" />{{ retryLabel }}</button>
    </div>
  </div>
</template>
