<script setup lang="ts">
import UiIcon from './UiIcon.vue'
const props = withDefaults(defineProps<{
  color?: 'primary' | 'secondary' | 'accent' | 'neutral' | 'info' | 'success' | 'warning' | 'error' | undefined
  variant?: 'solid' | 'soft' | 'outline' | 'text' | undefined
  size?: 'xs' | 'sm' | 'md' | 'lg' | undefined
  icon?: string | undefined
  iconOnly?: boolean | undefined
  loading?: boolean | undefined
  disabled?: boolean | undefined
  type?: 'button' | 'submit' | 'reset' | undefined
  block?: boolean | undefined
  label?: string | undefined
}>(), { color: 'primary', variant: 'solid', size: 'md', type: 'button' })
const colors = { primary: 'btn-primary', secondary: 'btn-secondary', accent: 'btn-accent', neutral: 'btn-neutral', info: 'btn-info', success: 'btn-success', warning: 'btn-warning', error: 'btn-error' }
const variants = { solid: '', soft: 'btn-soft', outline: 'btn-outline', text: 'btn-text' }
const sizes = { xs: 'btn-xs', sm: 'btn-sm', md: '', lg: 'btn-lg' }
</script>

<template>
  <button
    :type="type"
    class="btn"
    :class="[colors[props.color], variants[props.variant], sizes[props.size], iconOnly ? 'btn-circle' : '', block ? 'btn-block' : '']"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
    :aria-label="iconOnly ? label : undefined"
    :title="iconOnly ? label : undefined"
  >
    <span v-if="loading" class="loading loading-spinner loading-xs" aria-hidden="true" />
    <UiIcon v-else-if="icon" :name="icon" :size="size === 'xs' ? 'xs' : 'sm'" />
    <span v-if="!iconOnly"><slot>{{ label }}</slot></span>
  </button>
</template>
