<script setup lang="ts">
// Application frame: app bar, navigation drawer (overlay below lg, persistent
// rail above), main content, toast + confirm hosts. The shell and the console
// use it; module remotes render inside its default slot.
import { onBeforeUnmount, ref, watch } from 'vue'
import { useBreakpoint } from '@/composables/useBreakpoint'
import UiToast from './UiToast.vue'
import UiConfirm from './UiConfirm.vue'
import UiIcon from './UiIcon.vue'

defineProps<{ title: string; hideNav?: boolean | undefined }>()
const lg = useBreakpoint('lg')
const open = ref(false)
watch(lg, (v) => { if (v) open.value = false })
// The overlay drawer closes on Escape like any other modal surface.
const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && open.value) open.value = false }
watch(open, (v) => (v ? document.addEventListener('keydown', onKey) : document.removeEventListener('keydown', onKey)))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
defineExpose({ closeNav: () => (open.value = false) })
</script>

<template>
  <div class="bg-base-200 flex min-h-screen flex-col">
    <a href="#main" class="focus:rounded-field focus:bg-base-100 sr-only focus:not-sr-only focus:absolute focus:z-[70] focus:m-2 focus:px-3 focus:py-2">Skip to content</a>
    <!-- Navigation: an overlay drawer below lg, a persistent rail above it. -->
    <template v-if="!hideNav">
      <div v-if="open && !lg" class="bg-base-content/40 fixed inset-0 z-40" @click="open = false" />
      <!-- Sized here, not with FlyonUI's .drawer (width:100%/max-width:24rem would
           not match the ps-65 content offset and would overlap the page). -->
      <aside
        id="ui-nav"
        aria-label="Main"
        class="bg-base-100 border-base-300 fixed inset-y-0 start-0 z-50 w-65 max-w-[85vw] border-e shadow-sm"
        :class="lg ? 'translate-x-0 transition-none' : 'transition-transform ' + (open ? 'translate-x-0' : '-translate-x-full rtl:translate-x-full')"
      >
        <div class="flex h-full flex-col">
          <div class="flex items-center gap-3 px-4 py-4">
            <slot name="brand"><span class="text-base-content truncate text-lg font-semibold tracking-tight">{{ title }}</span></slot>
            <div class="grow" />
            <button v-if="!lg" type="button" class="btn btn-text btn-circle btn-sm" aria-label="Close navigation" @click="open = false"><UiIcon name="mdi-close" size="sm" /></button>
          </div>
          <div class="h-full overflow-y-auto"><slot name="nav" :close="() => (open = false)" /></div>
        </div>
      </aside>
    </template>

    <div class="flex grow flex-col" :class="!hideNav && lg ? 'ps-65' : ''">
      <header class="sticky top-0 z-30 flex">
        <div class="mx-auto mt-4 w-full max-w-[1388px] px-4 sm:px-6">
          <nav class="navbar rounded-field shadow-base-300/10 gap-2 py-2.5 shadow-sm">
            <button v-if="!hideNav && !lg" type="button" class="btn btn-soft btn-square btn-sm" aria-label="Open navigation" :aria-expanded="open" aria-controls="ui-nav" @click="open = !open"><UiIcon name="mdi-menu" size="sm" /></button>
            <span class="text-base-content truncate font-medium">{{ title }}</span>
            <div class="grow" />
            <div class="flex items-center gap-1"><slot name="app-bar" /></div>
          </nav>
        </div>
      </header>
      <main id="main" class="mx-auto w-full max-w-[1388px] grow p-4 sm:p-6" tabindex="-1"><slot /></main>
    </div>
    <UiToast />
    <UiConfirm />
  </div>
</template>
