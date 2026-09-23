<script setup lang="ts">
// Navigation entries for UiAppShell's nav slot. A group with a title renders
// as a collapsible menu (open when it owns the current route); a group without
// one renders its items flat (e.g. "Home" and operator links).
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import UiIcon from './UiIcon.vue'
export interface NavItem { title: string; path: string; icon?: string; order?: number; exact?: boolean; testId?: string }
export interface NavGroup { key?: string; title?: string; icon?: string; items: NavItem[]; testId?: string }
const props = defineProps<{ groups: NavGroup[] }>()
const emit = defineEmits<{ (e: 'navigate'): void }>()
const route = useRoute()
const keyOf = (g: NavGroup) => g.key ?? g.title ?? ''
const owns = (g: NavGroup) => g.items.some((it) => (it.exact ? route?.path === it.path : route?.path === it.path || route?.path.startsWith(it.path + '/')))
const active = computed(() => props.groups.filter((g) => g.title && owns(g)).map(keyOf))
const manual = ref<Record<string, boolean>>({})
watch(active, () => (manual.value = {}))
const isOpen = (g: NavGroup) => manual.value[keyOf(g)] ?? active.value.includes(keyOf(g))
function toggle(g: NavGroup) {
  manual.value = { ...manual.value, [keyOf(g)]: !isOpen(g) }
}
const idOf = (g: NavGroup) => 'ui-nav-group-' + keyOf(g).replace(/[^\w-]/g, '-')
</script>

<template>
  <ul class="menu menu-sm w-full gap-1 p-3">
    <template v-for="g in groups" :key="keyOf(g)">
      <li v-if="g.title" class="accordion-item">
        <button
          type="button"
          class="accordion-toggle inline-flex items-center gap-2 p-2 text-start text-sm font-normal"
          :class="isOpen(g) ? 'bg-neutral/10' : ''"
          :aria-expanded="isOpen(g)"
          :aria-controls="idOf(g)"
          :data-test="g.testId"
          @click="toggle(g)"
        >
          <UiIcon v-if="g.icon" :name="g.icon" size="sm" /><span class="grow truncate">{{ g.title }}</span>
          <UiIcon name="mdi-chevron-right" size="sm" class="shrink-0 transition-transform duration-300" :class="isOpen(g) ? 'rotate-90' : ''" />
        </button>
        <!-- v-if, not v-show: a hidden element would carry an inline style, which the platform CSP forbids. -->
        <ul v-if="isOpen(g)" :id="idOf(g)" class="mt-1 w-full">
          <li v-for="it in g.items" :key="it.path">
            <RouterLink :to="it.path" class="flex items-center gap-2 px-2" :active-class="it.exact ? '' : 'menu-active'" exact-active-class="menu-active" :data-test="it.testId" @click="emit('navigate')">
              <UiIcon v-if="it.icon" :name="it.icon" size="sm" /><span class="truncate">{{ it.title }}</span>
            </RouterLink>
          </li>
        </ul>
      </li>
      <li v-for="it in g.title ? [] : g.items" v-else :key="it.path">
        <RouterLink :to="it.path" class="flex items-center gap-2 px-2" :active-class="it.exact ? '' : 'menu-active'" exact-active-class="menu-active" :data-test="it.testId" @click="emit('navigate')">
          <UiIcon v-if="it.icon" :name="it.icon" size="sm" /><span class="truncate">{{ it.title }}</span>
        </RouterLink>
      </li>
    </template>
  </ul>
</template>
