<script setup lang="ts">
// Disclosure list built on native <details>: keyboard accessible without JS.
// `toggle` lets a consumer lazy-load a panel's content the first time it opens.
export interface AccordionItem { key: string; title: string; subtitle?: string }
defineProps<{ items: AccordionItem[]; open?: string[] | undefined }>()
const emit = defineEmits<{ (e: 'toggle', key: string, open: boolean): void }>()
</script>

<template>
  <div class="flex flex-col gap-2">
    <details v-for="it in items" :key="it.key" class="collapse collapse-arrow border border-base-300 bg-base-100" :open="open?.includes(it.key) || undefined" @toggle="emit('toggle', it.key, ($event.target as HTMLDetailsElement).open)">
      <summary class="collapse-title cursor-pointer text-sm font-medium">
        {{ it.title }}<span v-if="it.subtitle" class="ms-2 text-base-content/70">{{ it.subtitle }}</span>
      </summary>
      <div class="collapse-content text-sm"><slot :name="it.key" :item="it" /></div>
    </details>
  </div>
</template>
