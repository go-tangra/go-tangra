<script setup lang="ts">
const props = withDefaults(defineProps<{ color?: 'neutral' | 'primary' | 'secondary' | 'accent' | 'info' | 'success' | 'warning' | 'error' | undefined; size?: 'xs' | 'sm' | 'md' | undefined; soft?: boolean | undefined; outline?: boolean | undefined }>(), { color: 'neutral', size: 'sm', soft: true })
const colors = { neutral: 'badge-neutral', primary: 'badge-primary', secondary: 'badge-secondary', accent: 'badge-accent', info: 'badge-info', success: 'badge-success', warning: 'badge-warning', error: 'badge-error' }
const sizes = { xs: 'badge-xs', sm: 'badge-sm', md: '' }
</script>

<template>
  <span class="badge whitespace-nowrap" :class="[colors[props.color], sizes[props.size], soft ? 'badge-soft' : '', outline ? 'badge-outline' : '']"><slot /></span>
</template>
