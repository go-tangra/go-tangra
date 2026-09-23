<script setup lang="ts">
import UiBadge from './UiBadge.vue'
defineProps<{ connected: boolean }>()
</script>

<template>
  <UiBadge :color="connected ? 'success' : 'neutral'" size="xs"><span class="sr-only">Live updates </span>{{ connected ? 'live' : 'offline' }}</UiBadge>
</template>
