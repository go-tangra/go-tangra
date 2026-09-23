<script setup lang="ts">
// CSS-only tooltip (daisyUI/FlyonUI `tooltip` + data-tip): keyboard focusable, no JS.
withDefaults(defineProps<{ text: string; position?: 'top' | 'bottom' | 'left' | 'right' | undefined }>(), { position: 'top' })
</script>

<template>
  <span class="tooltip" :class="'tooltip-' + position" :data-tip="text" tabindex="0" :aria-label="text"><slot /></span>
</template>
