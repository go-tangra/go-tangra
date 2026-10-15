<script setup lang="ts">
// "?" link from a module screen to its help page in the console help center
// (/help/<module>/<slug>[#anchor]). Inside the host router it navigates in
// place; without one (or with a modifier key) it is a plain link.
import { computed, inject } from 'vue'
import { routerKey } from 'vue-router'
import UiIcon from './UiIcon.vue'

const props = withDefaults(defineProps<{
  module: string
  slug: string
  anchor?: string | undefined
  /** Visible text; also the accessible name of the icon-only variant. */
  label?: string | undefined
  iconOnly?: boolean | undefined
  size?: 'xs' | 'sm' | 'md' | undefined
}>(), { label: 'Help', size: 'sm' })

const router = inject(routerKey, null)
const href = computed(() => {
  const slug = props.slug.split('/').filter(Boolean).map(encodeURIComponent).join('/')
  const anchor = props.anchor ? '#' + encodeURIComponent(props.anchor.replace(/^#/, '')) : ''
  return '/help/' + encodeURIComponent(props.module) + '/' + slug + anchor
})
const sizes = { xs: 'btn-xs', sm: 'btn-sm', md: '' }

function go(e: MouseEvent): void {
  if (!router || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
  e.preventDefault()
  void router.push(href.value)
}
</script>

<template>
  <a
    :href="href"
    class="btn btn-text"
    :class="[sizes[size], iconOnly ? 'btn-circle' : '']"
    :aria-label="iconOnly ? label : undefined"
    :title="label"
    data-help-link
    @click="go"
  >
    <UiIcon name="mdi-help-circle-outline" :size="size === 'md' ? 'md' : 'sm'" />
    <span v-if="!iconOnly">{{ label }}</span>
  </a>
</template>
