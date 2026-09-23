<script setup lang="ts">
// Modal dialog: Vue-owned state, focus trap, Esc/backdrop close (unless
// persistent), full-screen sheet below md, centred box above. Rendered via
// <Teleport> to body so it escapes any overflow container.
import { computed, ref, watch } from 'vue'

// Attributes (data-test, aria-*) land on the dialog panel: the root is a Teleport.
defineOptions({ inheritAttrs: false })
import { useFocusTrap } from '@/composables/useFocusTrap'
import { useBreakpoint } from '@/composables/useBreakpoint'
import UiIcon from './UiIcon.vue'

const props = withDefaults(defineProps<{ modelValue: boolean; title?: string | undefined; size?: 'sm' | 'md' | 'lg' | 'xl' | undefined; persistent?: boolean | undefined; hideClose?: boolean | undefined }>(), { size: 'md' })
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void; (e: 'close'): void }>()
const box = ref<HTMLElement | null>(null)
const open = computed(() => props.modelValue)
const md = useBreakpoint('md')
const widths = { sm: 'md:max-w-md', md: 'md:max-w-2xl', lg: 'md:max-w-4xl', xl: 'md:max-w-6xl' }
let uid = 0
const titleId = 'ui-dialog-title-' + ++uid

function close() {
  if (props.persistent) return
  emit('update:modelValue', false)
  emit('close')
}
useFocusTrap(box, open, close)
watch(open, (o) => {
  if (typeof document === 'undefined') return
  document.body.classList.toggle('overflow-hidden', o)
}, { immediate: true })
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-end justify-center bg-base-content/40 p-0 md:items-center md:p-4" @mousedown.self="close">
      <div
        ref="box"
        v-bind="$attrs"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="title ? titleId : undefined"
        tabindex="-1"
        class="flex max-h-full w-full flex-col overflow-hidden bg-base-100 shadow-xl md:rounded-box"
        :class="[widths[props.size], md ? '' : 'h-full rounded-none']"
      >
        <header v-if="title || !hideClose" class="border-base-300 flex items-center gap-2 border-b px-6 py-4">
          <h2 v-if="title" :id="titleId" class="text-lg font-semibold truncate">{{ title }}</h2>
          <div class="grow" />
          <button v-if="!hideClose" type="button" class="btn btn-text btn-circle btn-sm" aria-label="Close" data-focus-skip @click="close"><UiIcon name="mdi-close" /></button>
        </header>
        <div class="grow overflow-y-auto p-6"><slot /></div>
        <footer v-if="$slots.actions" class="border-base-300 flex flex-wrap justify-end gap-3 border-t px-6 py-4">
          <slot name="actions" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>
