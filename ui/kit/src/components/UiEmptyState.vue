<script setup lang="ts">
import UiIcon from './UiIcon.vue'
withDefaults(defineProps<{ title?: string | undefined; text?: string | undefined; icon?: string | undefined }>(), { title: 'Nothing here yet', icon: 'mdi-inbox-outline' })
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 py-10 text-center text-base-content/70">
    <UiIcon :name="icon" size="xl" />
    <div class="font-medium text-base-content">{{ title }}</div>
    <p v-if="text" class="max-w-md text-sm">{{ text }}</p>
    <div v-if="$slots.default" class="mt-2"><slot /></div>
  </div>
</template>
