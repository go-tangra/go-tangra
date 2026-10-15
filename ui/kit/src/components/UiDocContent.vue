<script setup lang="ts">
// A module help page body. `html` is rendered and sanitized by the gateway
// (no scripts, styles or event attributes survive); it is shown as is in a
// prose container styled by doc-content.css (part of theme.css). After each
// render every code block gets a copy button and becomes keyboard-scrollable.
// Clicks on help links (/help/<module>/<slug>[#frag]) and on in-page anchors
// are not followed by the browser: the component emits `navigate` with the
// target so the host router handles it. `hash` scrolls to that heading.
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { iconClass } from '@/icons'

const props = withDefaults(defineProps<{
  html: string
  /** Fragment (with or without "#") to scroll to once the content is rendered. */
  hash?: string | undefined
  /** Path prefix of in-console help links the host router handles. */
  linkPrefix?: string | undefined
  label?: string | undefined
}>(), { linkPrefix: '/help/' })
const emit = defineEmits<{ (e: 'navigate', to: string): void }>()

const root = ref<HTMLElement | null>(null)
const timers = new Set<ReturnType<typeof setTimeout>>()
const COPY = iconClass('mdi-content-copy')
const DONE = iconClass('mdi-check')

function setIcon(btn: HTMLElement, cls: string, label: string): void {
  btn.setAttribute('aria-label', label)
  btn.setAttribute('title', label)
  const icon = btn.firstElementChild
  if (icon) icon.className = cls + ' size-4 inline-block'
}

/** Wraps each code block with a copy button (DOM only: no inline handlers, clicks are delegated). */
function enhance(): void {
  const el = root.value
  if (!el) return
  for (const pre of Array.from(el.querySelectorAll('pre'))) {
    if (pre.parentElement?.classList.contains('ui-doc-code')) continue
    pre.setAttribute('tabindex', '0') // scrollable region reachable by keyboard
    const wrap = document.createElement('div')
    wrap.className = 'ui-doc-code'
    pre.replaceWith(wrap)
    wrap.appendChild(pre)
    const btn = document.createElement('button')
    btn.type = 'button'
    btn.className = 'ui-doc-copy btn btn-soft btn-secondary btn-xs btn-square'
    const icon = document.createElement('span')
    icon.setAttribute('aria-hidden', 'true')
    btn.appendChild(icon)
    setIcon(btn, COPY, 'Copy code')
    wrap.appendChild(btn)
  }
}

async function copy(btn: HTMLElement): Promise<void> {
  const pre = btn.parentElement?.querySelector('pre')
  if (!pre) return
  try {
    await navigator.clipboard.writeText(pre.textContent ?? '')
  } catch {
    return // clipboard unavailable: the code is still selectable
  }
  setIcon(btn, DONE, 'Copied')
  const t = setTimeout(() => {
    timers.delete(t)
    setIcon(btn, COPY, 'Copy code')
  }, 1500)
  timers.add(t)
}

function scrollTo(hash: string | undefined): void {
  const id = decodeURIComponent((hash ?? '').replace(/^#/, ''))
  if (!id || !root.value) return
  const target = Array.from(root.value.querySelectorAll<HTMLElement>('[id]')).find((n) => n.id === id)
  target?.scrollIntoView?.({ block: 'start' })
}

function onClick(e: MouseEvent): void {
  const t = e.target as Element | null
  const btn = t?.closest?.('.ui-doc-copy')
  if (btn instanceof HTMLElement) {
    e.preventDefault()
    void copy(btn)
    return
  }
  const a = t?.closest?.('a')
  if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
  const href = a.getAttribute('href') ?? ''
  if (a.getAttribute('target') || !(href.startsWith(props.linkPrefix) || href.startsWith('#'))) return
  e.preventDefault()
  if (href.startsWith('#')) scrollTo(href)
  emit('navigate', href)
}

async function render(): Promise<void> {
  await nextTick()
  enhance()
  scrollTo(props.hash)
}
onMounted(render)
watch(() => props.html, render, { flush: 'post' })
watch(() => props.hash, (h) => scrollTo(h), { flush: 'post' })
onBeforeUnmount(() => {
  for (const t of timers) clearTimeout(t)
  timers.clear()
})
defineExpose({ scrollTo })
</script>

<template>
  <!-- eslint-disable-next-line vue/no-v-html -- gateway-sanitized help HTML (bluemonday allow-list) -->
  <article ref="root" class="ui-doc-content" :aria-label="label" @click="onClick" v-html="html" />
</template>
