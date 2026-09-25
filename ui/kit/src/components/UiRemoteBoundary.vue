<script setup lang="ts">
// Error boundary for a federated remote's area: catches load/render errors
// (including shared-runtime version mismatches) and shows a retry card; the
// rest of the shell keeps working. Logs the failure without secrets.
import { onErrorCaptured, ref } from 'vue'

// Fallthrough attributes (e.g. data-test) land on the error card only; the slot content is the remote's own tree.
defineOptions({ inheritAttrs: false })
import UiErrorState from './UiErrorState.vue'
import UiSkeleton from './UiSkeleton.vue'

const props = defineProps<{ module: string; loading?: boolean | undefined; error?: unknown | undefined }>()
const emit = defineEmits<{ (e: 'retry'): void }>()
const caught = ref<unknown>(null)
onErrorCaptured((err) => {
  caught.value = err
  console.warn('[go-tangra/ui] module render failed', props.module, err instanceof Error ? err.message : String(err))
  return false
})
function text(e: unknown): string {
  const m = e instanceof Error ? e.message : String(e ?? '')
  if (/shared|singleton|version|requiredVersion|@(go-tangra|freya)\/ui/i.test(m)) return `The ${props.module} module was built for a different platform version. Reload after the deployment completes.`
  return `The ${props.module} module could not be loaded.`
}
function retry() {
  caught.value = null
  emit('retry')
}
defineExpose({ reset: () => (caught.value = null) })
</script>

<template>
  <UiSkeleton v-if="loading && !error && !caught" kind="card" />
  <UiErrorState v-else-if="error || caught" v-bind="$attrs" :title="'Module unavailable'" :text="text(error ?? caught)" @retry="retry" />
  <slot v-else />
</template>
