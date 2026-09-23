<script setup lang="ts">
// A row of controls that wraps on narrow screens.
defineProps<{ justify?: 'start' | 'end' | 'between' | undefined }>()
</script>

<template>
  <div class="flex flex-wrap items-center gap-2" :class="{ 'justify-end': justify === 'end', 'justify-between': justify === 'between' }">
    <slot />
  </div>
</template>
