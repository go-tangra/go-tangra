import { reactive, readonly } from 'vue'

export type ToastKind = 'info' | 'success' | 'warning' | 'error'
export interface ToastOptions {
  kind?: ToastKind
  title: string
  text?: string
  /** ms; 0 = sticky. Default 5000 (errors 8000). */
  timeout?: number
}
export interface Toast extends Required<Omit<ToastOptions, 'text'>> {
  id: number
  text: string
}

// One shared store: the shell (or console) mounts a single <UiToast/> host;
// remotes call useToast().show() and never mount their own.
const state = reactive<{ items: Toast[]; seq: number }>({ items: [], seq: 0 })

export function useToast() {
  function show(o: ToastOptions): number {
    const id = ++state.seq
    const kind = o.kind ?? 'info'
    const timeout = o.timeout ?? (kind === 'error' ? 8000 : 5000)
    state.items.push({ id, kind, title: o.title, text: o.text ?? '', timeout })
    if (timeout > 0) setTimeout(() => dismiss(id), timeout)
    return id
  }
  function dismiss(id: number) {
    const i = state.items.findIndex((t) => t.id === id)
    if (i >= 0) state.items.splice(i, 1)
  }
  function clear() {
    state.items.splice(0)
  }
  return {
    show,
    dismiss,
    clear,
    items: readonly(state).items,
    success: (title: string, text?: string) => show({ kind: 'success', title, ...(text !== undefined ? { text } : {}) }),
    error: (title: string, text?: string) => show({ kind: 'error', title, ...(text !== undefined ? { text } : {}) }),
    info: (title: string, text?: string) => show({ kind: 'info', title, ...(text !== undefined ? { text } : {}) }),
    warning: (title: string, text?: string) => show({ kind: 'warning', title, ...(text !== undefined ? { text } : {}) }),
  }
}
