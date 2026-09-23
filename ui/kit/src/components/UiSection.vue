<script setup lang="ts">
defineProps<{ title: string; description?: string | undefined }>()
</script>

<template>
  <section class="space-y-4">
    <div>
      <h2 class="text-base-content text-base font-semibold">{{ title }}</h2>
      <p v-if="description" class="text-base-content/70 text-sm">{{ description }}</p>
    </div>
    <slot />
  </section>
</template>
