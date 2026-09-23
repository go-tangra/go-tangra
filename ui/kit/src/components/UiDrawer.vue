<script setup lang="ts">
// Side sheet (detail/edit panels): right-anchored above md, full-screen below.
import { computed, ref } from 'vue'

// Attributes (data-test, aria-*) land on the dialog panel: the root is a Teleport.
defineOptions({ inheritAttrs: false })
import { useFocusTrap } from '@/composables/useFocusTrap'
import UiIcon from './UiIcon.vue'

const props = withDefaults(defineProps<{ modelValue: boolean; title?: string | undefined; size?: 'md' | 'lg' | 'xl' | undefined; persistent?: boolean | undefined }>(), { size: 'md' })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'close'): void }>()
const panel = ref<HTMLElement | null>(null)
const open = computed(() => props.modelValue)
const widths = { md: 'md:max-w-md', lg: 'md:max-w-2xl', xl: 'md:max-w-4xl' }
let uid = 0
const titleId = 'ui-drawer-title-' + ++uid
function close() {
  if (props.persistent) return
  emit('update:modelValue', false)
  emit('close')
}
useFocusTrap(panel, open, close)
</script>

<template>
  <Teleport to="body">
    <!-- No dimming backdrop: the page stays visible beside the panel. The
         transparent layer still closes the drawer on an outside click. -->
    <div v-if="open" class="fixed inset-0 z-50 flex justify-end" @mousedown.self="close">
      <aside ref="panel" v-bind="$attrs" role="dialog" aria-modal="true" :aria-labelledby="title ? titleId : undefined" tabindex="-1" class="flex h-full w-full flex-col border-s border-base-300 bg-base-100 shadow-2xl" :class="widths[props.size]">
        <header class="border-base-300 flex items-center gap-2 border-b px-6 py-4">
          <h2 v-if="title" :id="titleId" class="text-lg font-semibold truncate">{{ title }}</h2>
          <div class="grow" />
          <slot name="header-actions" />
          <button type="button" class="btn btn-text btn-circle btn-sm" aria-label="Close" data-focus-skip @click="close"><UiIcon name="mdi-close" /></button>
        </header>
        <div class="grow overflow-y-auto p-6"><slot /></div>
        <footer v-if="$slots.actions" class="border-base-300 flex flex-wrap justify-end gap-3 border-t px-6 py-4"><slot name="actions" /></footer>
      </aside>
    </div>
  </Teleport>
</template>
