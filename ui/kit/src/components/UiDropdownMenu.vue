<script setup lang="ts">
// Menu button with keyboard navigation (Arrow keys, Home/End, Esc) and
// outside-click close. Items: { key, label, icon?, danger?, disabled? }.
// The list opens in the browser top layer (Popover API) and is placed next to
// the button from script, so scrolling/overflow containers such as a data
// table's wrapper cannot clip it. Coordinates are set through the CSSOM, which
// the CSP allows (only inline style attributes are blocked).
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import UiIcon from './UiIcon.vue'

export interface MenuItem { key: string; label: string; icon?: string; danger?: boolean; disabled?: boolean }
const props = withDefaults(defineProps<{ items: MenuItem[]; label?: string | undefined; icon?: string | undefined; align?: 'start' | 'end' | undefined; size?: 'xs' | 'sm' | 'md' | undefined }>(), { label: 'Actions', icon: 'mdi-dots-vertical', align: 'end', size: 'sm' })
const emit = defineEmits<{ (e: 'select', key: string): void }>()
const open = ref(false)
const root = ref<HTMLElement | null>(null)
const list = ref<HTMLElement | null>(null)
let uid = 0
const id = 'ui-menu-' + ++uid

function menuItems(): HTMLElement[] {
  return list.value ? Array.from(list.value.querySelectorAll<HTMLElement>('[role=menuitem]:not([aria-disabled=true])')) : []
}
function place() {
  const el = list.value
  const btn = root.value?.querySelector<HTMLElement>('button')
  if (!el || !btn) return
  const r = btn.getBoundingClientRect()
  const gap = 4
  const w = el.offsetWidth
  const h = el.offsetHeight
  let left = props.align === 'end' ? r.right - w : r.left
  left = Math.max(gap, Math.min(left, window.innerWidth - w - gap))
  // Open upwards when there is no room below the button.
  const top = r.bottom + gap + h > window.innerHeight && r.top - gap - h > 0 ? r.top - gap - h : r.bottom + gap
  Object.assign(el.style, { position: 'fixed', inset: 'auto', margin: '0', top: top + 'px', left: left + 'px' })
}
async function show() {
  await nextTick()
  const el = list.value as (HTMLElement & { showPopover?: () => void }) | null
  if (!el) return
  el.showPopover?.()
  place()
}
async function toggle(focusFirst = false) {
  open.value = !open.value
  if (!open.value) return
  await show()
  if (focusFirst) menuItems()[0]?.focus()
}
function close() {
  if (!open.value) return
  open.value = false
  root.value?.querySelector<HTMLElement>('button')?.focus()
}
// A fixed-position menu would drift away from its button on scroll/resize.
function dismiss() {
  open.value = false
}
function onKey(e: KeyboardEvent) {
  const it = menuItems()
  const i = it.indexOf(document.activeElement as HTMLElement)
  if (e.key === 'Escape') { e.preventDefault(); close() }
  else if (e.key === 'ArrowDown') { e.preventDefault(); (it[(i + 1) % it.length] ?? it[0])?.focus() }
  else if (e.key === 'ArrowUp') { e.preventDefault(); (it[(i - 1 + it.length) % it.length] ?? it[it.length - 1])?.focus() }
  else if (e.key === 'Home') { e.preventDefault(); it[0]?.focus() }
  else if (e.key === 'End') { e.preventDefault(); it[it.length - 1]?.focus() }
}
function select(item: MenuItem) {
  if (item.disabled) return
  open.value = false
  emit('select', item.key)
}
function onDoc(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => {
  document.addEventListener('mousedown', onDoc)
  window.addEventListener('scroll', dismiss, true)
  window.addEventListener('resize', dismiss)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDoc)
  window.removeEventListener('scroll', dismiss, true)
  window.removeEventListener('resize', dismiss)
})
</script>

<template>
  <div ref="root" class="relative inline-block" @keydown="onKey">
    <button type="button" class="btn btn-text btn-circle" :class="size === 'xs' ? 'btn-xs' : size === 'sm' ? 'btn-sm' : ''" :aria-label="label" aria-haspopup="menu" :aria-expanded="open" :aria-controls="id" @click="toggle(false)" @keydown.down.prevent="toggle(true)">
      <UiIcon :name="icon" size="sm" />
    </button>
    <ul v-if="open" :id="id" ref="list" role="menu" popover="manual" class="menu z-40 min-w-44 rounded-box border border-base-300 bg-base-100 p-1 text-base-content shadow-lg">
      <li v-for="it in props.items" :key="it.key">
        <button type="button" role="menuitem" class="flex items-center gap-2" :class="{ 'text-error': it.danger, 'opacity-50': it.disabled }" :aria-disabled="it.disabled || undefined" tabindex="-1" @click="select(it)">
          <UiIcon v-if="it.icon" :name="it.icon" size="sm" />{{ it.label }}
        </button>
      </li>
    </ul>
  </div>
</template>
