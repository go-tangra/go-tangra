<script setup lang="ts">
// A status badge with a colour map; unknown statuses render neutral. Status is
// also conveyed by the text, never colour alone.
import { computed } from 'vue'
import UiBadge from './UiBadge.vue'

type Color = 'neutral' | 'primary' | 'secondary' | 'accent' | 'info' | 'success' | 'warning' | 'error'
const props = defineProps<{ status: string; colors?: Record<string, Color> | undefined; label?: string | undefined }>()
const DEFAULTS: Record<string, Color> = {
  active: 'success', online: 'success', deployable: 'success', ok: 'success', healthy: 'success', succeeded: 'success', issued: 'success', valid: 'success', enabled: 'success', delivered: 'success',
  assigned: 'info', running: 'info', pending: 'warning', planned: 'warning', stale: 'warning', expiring: 'warning', suspended: 'warning', draft: 'neutral', queued: 'info', sent: 'info',
  broken: 'error', error: 'error', failed: 'error', expired: 'error', revoked: 'error', offline: 'error', inactive: 'neutral', archived: 'neutral', retired: 'neutral', cancelled: 'neutral', decommissioned: 'neutral', disabled: 'neutral',
}
const color = computed<Color>(() => props.colors?.[props.status] ?? DEFAULTS[props.status] ?? 'neutral')
</script>

<template>
  <UiBadge :color="color">{{ label ?? status }}</UiBadge>
</template>
