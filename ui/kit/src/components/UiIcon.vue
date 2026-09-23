<script setup lang="ts">
import { computed } from 'vue'
import { iconClass } from '@/icons'

const props = withDefaults(defineProps<{ name: string; size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl' | undefined; label?: string | undefined }>(), { size: 'md' })
const sizes = { xs: 'size-3.5', sm: 'size-4', md: 'size-5', lg: 'size-6', xl: 'size-8' }
const cls = computed(() => [iconClass(props.name), sizes[props.size], 'inline-block shrink-0 align-middle'])
</script>

<template>
  <span :class="cls" :role="label ? 'img' : undefined" :aria-label="label" :aria-hidden="label ? undefined : 'true'" />
</template>
