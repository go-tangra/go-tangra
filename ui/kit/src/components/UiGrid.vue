<script setup lang="ts">
// Responsive grid: 1 column on phones, `cols` from md (default 2), `lgCols` from lg.
const props = withDefaults(defineProps<{ cols?: 1 | 2 | 3 | 4 | 6 | undefined; lgCols?: 1 | 2 | 3 | 4 | 6 | undefined }>(), { cols: 2 })
const md = { 1: 'md:grid-cols-1', 2: 'md:grid-cols-2', 3: 'md:grid-cols-3', 4: 'md:grid-cols-4', 6: 'md:grid-cols-6' }
const lg = { 1: 'lg:grid-cols-1', 2: 'lg:grid-cols-2', 3: 'lg:grid-cols-3', 4: 'lg:grid-cols-4', 6: 'lg:grid-cols-6' }
</script>

<template>
  <div class="grid grid-cols-1 gap-3" :class="[md[props.cols], props.lgCols ? lg[props.lgCols] : '']">
    <slot />
  </div>
</template>
