<script setup lang="ts">
// Profile menu in the app bar: name/tenant, theme toggle, account + sign out.
import { useTheme } from '@/composables/useTheme'
import UiDropdownMenu, { type MenuItem } from './UiDropdownMenu.vue'
import UiAvatar from './UiAvatar.vue'

const props = defineProps<{ name: string; email?: string | undefined; tenant?: string | undefined; avatar?: string | undefined; items?: MenuItem[] | undefined }>()
const emit = defineEmits<{ (e: 'select', key: string): void }>()
const theme = useTheme()
function onSelect(k: string) {
  if (k === 'theme') theme.toggle()
  else emit('select', k)
}
const menu = () => [{ key: 'theme', label: theme.isDark() ? 'Light theme' : 'Dark theme', icon: theme.isDark() ? 'mdi-white-balance-sunny' : 'mdi-weather-night' }, ...(props.items ?? []), { key: 'signout', label: 'Sign out', icon: 'mdi-logout' }]
</script>

<template>
  <div class="flex items-center gap-2">
    <div class="hidden text-end leading-tight md:block">
      <div class="text-sm font-medium">{{ name }}</div>
      <div v-if="tenant || email" class="text-xs text-base-content/70">{{ tenant ?? email }}</div>
    </div>
    <UiAvatar :name="name" :src="avatar" size="sm" />
    <UiDropdownMenu :items="menu()" label="Account menu" icon="mdi-chevron-down" @select="onSelect" />
  </div>
</template>
