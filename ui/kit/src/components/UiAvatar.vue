<script setup lang="ts">
import { computed } from 'vue'
const props = withDefaults(defineProps<{ name?: string | undefined; src?: string | undefined; size?: 'sm' | 'md' | 'lg' | undefined }>(), { size: 'md' })
const initials = computed(() => (props.name ?? '?').split(/\s+/).map((p) => p[0] ?? '').join('').slice(0, 2).toUpperCase() || '?')
const sizes = { sm: 'size-8 text-xs', md: 'size-10 text-sm', lg: 'size-14 text-lg' }
</script>

<template>
  <div class="avatar" :class="src ? '' : 'avatar-placeholder'">
    <div class="rounded-full bg-neutral text-neutral-content flex items-center justify-center overflow-hidden" :class="sizes[props.size]">
      <img v-if="src" :src="src" :alt="name ?? ''">
      <span v-else aria-hidden="true">{{ initials }}</span>
    </div>
  </div>
</template>
