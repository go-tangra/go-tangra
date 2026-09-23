<script setup lang="ts">
// Page frame: title row (title, optional live indicator + actions slot), an
// optional filters row and the content. Wraps at phone widths.
defineProps<{ title: string; subtitle?: string | undefined }>()
</script>

<template>
  <section class="space-y-6">
    <header class="flex flex-wrap items-center gap-3">
      <slot name="before-title" />
      <div class="min-w-0">
        <h1 class="text-base-content truncate text-xl font-semibold leading-tight">{{ title }}</h1>
        <p v-if="subtitle" class="text-base-content/70 text-sm">{{ subtitle }}</p>
      </div>
      <slot name="badges" />
      <div class="grow" />
      <div v-if="$slots.actions" class="flex flex-wrap items-center gap-3">
        <slot name="actions" />
      </div>
    </header>
    <div v-if="$slots.filters" class="flex flex-wrap items-end gap-3">
      <slot name="filters" />
    </div>
    <slot />
  </section>
</template>
