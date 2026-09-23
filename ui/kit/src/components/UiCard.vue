<script setup lang="ts">
// `padded` must default to true explicitly: Vue casts an absent Boolean prop to false.
withDefaults(defineProps<{ title?: string | undefined; subtitle?: string | undefined; flat?: boolean | undefined; padded?: boolean | undefined }>(), { padded: true })
</script>

<template>
  <div class="card" :class="flat ? 'card-border shadow-none' : 'shadow-base-300/10 shadow-md'">
    <div v-if="title || $slots.header" class="card-header flex flex-wrap items-center gap-2 pb-0">
      <div v-if="title" class="min-w-0">
        <h2 class="card-title truncate text-lg">{{ title }}</h2>
        <p v-if="subtitle" class="text-base-content/70 text-sm">{{ subtitle }}</p>
      </div>
      <div class="grow" />
      <slot name="header" />
    </div>
    <div :class="padded ? 'card-body gap-4' : ''">
      <slot />
    </div>
    <div v-if="$slots.footer" class="card-footer flex flex-wrap justify-end gap-3 pt-0">
      <slot name="footer" />
    </div>
  </div>
</template>
