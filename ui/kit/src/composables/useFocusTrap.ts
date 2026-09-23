import { nextTick, onBeforeUnmount, watch, type Ref } from 'vue'

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]):not([type=hidden]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

/**
 * Traps Tab focus inside `el` while `active` is true, focuses the first
 * focusable element (or the element itself) on activation, and restores focus
 * to the previously focused element on deactivation. Used by dialogs/drawers.
 */
export function useFocusTrap(el: Ref<HTMLElement | null>, active: Ref<boolean>, onEscape?: () => void) {
  let previous: HTMLElement | null = null

  function focusables(): HTMLElement[] {
    return el.value ? Array.from(el.value.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((e) => e.offsetParent !== null || e === document.activeElement || true) : []
  }

  function onKey(e: KeyboardEvent) {
    if (!active.value || !el.value) return
    if (e.key === 'Escape' && onEscape) {
      e.stopPropagation()
      onEscape()
      return
    }
    if (e.key !== 'Tab') return
    const items = focusables()
    if (items.length === 0) {
      e.preventDefault()
      el.value.focus()
      return
    }
    const first = items[0]!
    const last = items[items.length - 1]!
    const activeEl = document.activeElement as HTMLElement | null
    if (e.shiftKey && (activeEl === first || !el.value.contains(activeEl))) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && activeEl === last) {
      e.preventDefault()
      first.focus()
    }
  }

  watch(
    active,
    async (on) => {
      if (on) {
        previous = document.activeElement as HTMLElement | null
        document.addEventListener('keydown', onKey, true)
        await nextTick()
        const items = focusables()
        // Initial focus: an [autofocus] element, else the first focusable that is
        // not marked data-focus-skip (dialog close buttons), else the container.
        ;(items.find((i) => i.hasAttribute('autofocus')) ?? items.find((i) => !i.hasAttribute('data-focus-skip')) ?? items[0] ?? el.value)?.focus()
      } else {
        document.removeEventListener('keydown', onKey, true)
        previous?.focus?.()
        previous = null
      }
    },
    { immediate: true },
  )
  onBeforeUnmount(() => document.removeEventListener('keydown', onKey, true))
}
