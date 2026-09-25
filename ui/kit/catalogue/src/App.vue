<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { UiAppShell, UiNavDrawer, UiButton, useTheme, type NavGroup } from '@/index'
import { pages } from './pages'

const theme = useTheme()
const groups: NavGroup[] = [{ title: 'Catalogue', items: pages.map((p) => ({ title: p.title, path: '/' + p.key, icon: 'mdi-view-module-outline' })) }]
</script>

<template>
  <UiAppShell title="Tangra UI kit">
    <template #app-bar>
      <UiButton variant="text" size="sm" :icon="theme.theme.value === 'freya-dark' ? 'mdi-weather-sunny' : 'mdi-weather-night'" icon-only label="Toggle theme" data-testid="theme-toggle" @click="theme.toggle()" />
    </template>
    <template #nav="{ close }">
      <UiNavDrawer :groups="groups" @navigate="close" />
    </template>
    <RouterView />
    <p class="mt-8 text-xs text-base-content/70">Synthetic data only — nothing here comes from a live system.</p>
    <RouterLink to="/" class="sr-only">home</RouterLink>
  </UiAppShell>
</template>
