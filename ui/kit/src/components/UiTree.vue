<script setup lang="ts">
// Keyboard-navigable tree (ARIA tree/treeitem): Arrow Up/Down move, Right
// expands / moves into children, Left collapses / moves to parent, Enter or
// Space selects. Node: { id, label, children?, meta? }.
import { computed, ref } from 'vue'
import UiIcon from './UiIcon.vue'

export interface TreeNode { id: string; label: string; children?: TreeNode[]; meta?: Record<string, unknown>; icon?: string; badge?: string }
const props = withDefaults(defineProps<{ items: TreeNode[]; expanded?: string[] | undefined; selected?: string | undefined; defaultExpanded?: boolean | undefined }>(), { defaultExpanded: true })
const emit = defineEmits<{ (e: 'update:expanded', v: string[]): void; (e: 'update:selected', v: string): void; (e: 'select', n: TreeNode): void }>()

const internal = ref<string[] | null>(null)
const exp = computed(() => props.expanded ?? internal.value ?? (props.defaultExpanded ? allIds(props.items) : []))
function allIds(n: TreeNode[], out: string[] = []): string[] {
  for (const x of n) {
    out.push(x.id)
    if (x.children) allIds(x.children, out)
  }
  return out
}
function setExpanded(v: string[]) {
  internal.value = v
  emit('update:expanded', v)
}
function isOpen(n: TreeNode) {
  return exp.value.includes(n.id)
}
function toggle(n: TreeNode) {
  if (!n.children?.length) return
  setExpanded(isOpen(n) ? exp.value.filter((i) => i !== n.id) : [...exp.value, n.id])
}
function select(n: TreeNode) {
  emit('update:selected', n.id)
  emit('select', n)
}
interface Flat { node: TreeNode; depth: number; parent?: TreeNode }
const flat = computed<Flat[]>(() => {
  const out: Flat[] = []
  const walk = (nodes: TreeNode[], depth: number, parent?: TreeNode) => {
    for (const n of nodes) {
      out.push(parent ? { node: n, depth, parent } : { node: n, depth })
      if (n.children?.length && isOpen(n)) walk(n.children, depth + 1, n)
    }
  }
  walk(props.items, 0)
  return out
})
const root = ref<HTMLElement | null>(null)
function focusAt(i: number) {
  root.value?.querySelectorAll<HTMLElement>('[role=treeitem]')[i]?.focus()
}
function onKey(e: KeyboardEvent, f: Flat, i: number) {
  switch (e.key) {
    case 'ArrowDown': e.preventDefault(); focusAt(Math.min(i + 1, flat.value.length - 1)); break
    case 'ArrowUp': e.preventDefault(); focusAt(Math.max(i - 1, 0)); break
    case 'ArrowRight': e.preventDefault(); if (f.node.children?.length) { if (!isOpen(f.node)) toggle(f.node); else focusAt(i + 1) } break
    case 'ArrowLeft': e.preventDefault(); if (f.node.children?.length && isOpen(f.node)) toggle(f.node); else if (f.parent) focusAt(flat.value.findIndex((x) => x.node === f.parent)); break
    case 'Home': e.preventDefault(); focusAt(0); break
    case 'End': e.preventDefault(); focusAt(flat.value.length - 1); break
    case 'Enter': case ' ': e.preventDefault(); select(f.node); break
  }
}
const pad = ['ps-0', 'ps-5', 'ps-10', 'ps-14', 'ps-20', 'ps-24', 'ps-28', 'ps-32']
</script>

<template>
  <ul ref="root" role="tree" class="flex flex-col text-sm">
    <li
      v-for="(f, i) in flat"
      :key="f.node.id"
      role="treeitem"
      :aria-level="f.depth + 1"
      :aria-expanded="f.node.children?.length ? isOpen(f.node) : undefined"
      :aria-selected="selected === f.node.id"
      :tabindex="i === 0 ? 0 : -1"
      class="flex items-center gap-1 rounded-field py-1 pe-1 outline-none focus-visible:ring-2 focus-visible:ring-primary"
      :class="[pad[Math.min(f.depth, 7)], selected === f.node.id ? 'bg-primary/10' : 'hover:bg-base-200']"
      @keydown="onKey($event, f, i)"
      @click="select(f.node)"
    >
      <button v-if="f.node.children?.length" type="button" class="btn btn-text btn-circle btn-xs" :aria-label="isOpen(f.node) ? 'Collapse' : 'Expand'" tabindex="-1" @click.stop="toggle(f.node)"><UiIcon :name="isOpen(f.node) ? 'mdi-chevron-down' : 'mdi-chevron-right'" size="sm" /></button>
      <span v-else class="inline-block w-6" aria-hidden="true" />
      <UiIcon :name="f.node.icon ?? (f.node.children?.length ? 'mdi-folder-outline' : 'mdi-subdirectory-arrow-right')" size="sm" class="text-base-content/70" />
      <button type="button" class="min-w-0 grow truncate text-start" tabindex="-1"><slot name="label" :node="f.node">{{ f.node.label }}</slot></button>
      <span v-if="f.node.badge" class="badge badge-xs badge-soft">{{ f.node.badge }}</span>
      <span v-if="$slots.actions" class="shrink-0" @click.stop><slot name="actions" :node="f.node" /></span>
    </li>
    <li v-if="flat.length === 0" class="py-2 text-base-content/70">Nothing here yet.</li>
  </ul>
</template>
