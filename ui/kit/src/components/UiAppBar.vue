<script setup lang="ts">
// Stand-alone top bar for pages outside UiAppShell (e.g. sign-in).
defineProps<{ title: string }>()
</script>

<template>
  <header class="navbar min-h-14 border-b border-base-300 bg-base-100 px-4">
    <span class="text-lg font-semibold">{{ title }}</span>
    <div class="grow" />
    <slot />
  </header>
</template>
