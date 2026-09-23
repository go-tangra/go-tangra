<script setup lang="ts">
withDefaults(defineProps<{ cols?: 2 | 3 | 4 | 6 | undefined }>(), { cols: 4 })
const md = { 2: 'md:grid-cols-2', 3: 'md:grid-cols-3', 4: 'md:grid-cols-4', 6: 'md:grid-cols-3 lg:grid-cols-6' }
</script>

<template>
  <div class="grid grid-cols-2 gap-3" :class="md[cols]"><slot /></div>
</template>
