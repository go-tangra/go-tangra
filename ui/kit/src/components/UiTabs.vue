<script setup lang="ts">
// Tab list with roving focus (Arrow keys). Panels are rendered by the parent
// (v-if on the selected key) to keep the component generic.
import { ref } from 'vue'
import UiIcon from './UiIcon.vue'
export interface TabItem { key: string; label: string; icon?: string; count?: number }
const props = defineProps<{ modelValue: string; tabs: TabItem[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()
const list = ref<HTMLElement | null>(null)
function select(k: string) {
  emit('update:modelValue', k)
}
function onKey(e: KeyboardEvent) {
  const i = props.tabs.findIndex((t) => t.key === props.modelValue)
  let n = i
  if (e.key === 'ArrowRight') n = (i + 1) % props.tabs.length
  else if (e.key === 'ArrowLeft') n = (i - 1 + props.tabs.length) % props.tabs.length
  else if (e.key === 'Home') n = 0
  else if (e.key === 'End') n = props.tabs.length - 1
  else return
  e.preventDefault()
  const t = props.tabs[n]
  if (t) {
    select(t.key)
    list.value?.querySelectorAll<HTMLElement>('[role=tab]')[n]?.focus()
  }
}
</script>

<template>
  <div ref="list" role="tablist" class="tabs tabs-bordered overflow-x-auto" @keydown="onKey">
    <button
      v-for="t in tabs"
      :id="'tab-' + t.key"
      :key="t.key"
      type="button"
      role="tab"
      class="tab whitespace-nowrap"
      :class="{ 'tab-active': t.key === modelValue }"
      :aria-selected="t.key === modelValue"
      :tabindex="t.key === modelValue ? 0 : -1"
      @click="select(t.key)"
    >
      <UiIcon v-if="t.icon" :name="t.icon" size="sm" class="me-1" />{{ t.label }}<span v-if="t.count !== undefined" class="badge badge-sm badge-soft ms-2">{{ t.count }}</span>
    </button>
  </div>
</template>
