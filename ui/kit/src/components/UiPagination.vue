<script setup lang="ts">
// Cursor-style pager: previous/next with an optional page label.
import UiIcon from './UiIcon.vue'
defineProps<{ hasPrev?: boolean | undefined; hasNext?: boolean | undefined; label?: string | undefined }>()
const emit = defineEmits<{ (e: 'prev'): void; (e: 'next'): void }>()
</script>

<template>
  <nav class="join" aria-label="Pagination">
    <button type="button" class="btn btn-soft btn-sm join-item" :disabled="!hasPrev" aria-label="Previous page" @click="emit('prev')"><UiIcon name="mdi-chevron-left" size="sm" /></button>
    <span v-if="label" class="btn btn-soft btn-sm join-item pointer-events-none">{{ label }}</span>
    <button type="button" class="btn btn-soft btn-sm join-item" :disabled="!hasNext" aria-label="Next page" @click="emit('next')"><UiIcon name="mdi-chevron-right" size="sm" /></button>
  </nav>
</template>
