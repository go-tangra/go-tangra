import { reactive, readonly } from 'vue'

export interface ConfirmOptions {
  title: string
  text?: string
  confirmLabel?: string
  cancelLabel?: string
  /** Renders the confirm button in the error colour. */
  danger?: boolean
}

interface Pending extends Required<ConfirmOptions> {
  resolve: (ok: boolean) => void
}

// One shared confirm host (UiConfirm in the shell/console). `ask()` returns a
// promise resolved by the user's choice.
const state = reactive<{ pending: Pending | null }>({ pending: null })

export function useConfirm() {
  function ask(o: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      state.pending?.resolve(false)
      state.pending = {
        title: o.title,
        text: o.text ?? '',
        confirmLabel: o.confirmLabel ?? 'Confirm',
        cancelLabel: o.cancelLabel ?? 'Cancel',
        danger: o.danger ?? false,
        resolve,
      }
    })
  }
  function answer(ok: boolean) {
    const p = state.pending
    state.pending = null
    p?.resolve(ok)
  }
  return { ask, answer, state: readonly(state) }
}
