<script setup lang="ts">
import { UiPage, UiSection, UiCard, UiAppBar, UiUserMenu, UiNavDrawer, UiToolbar, useToast, type NavGroup } from '@/index'

const toast = useToast()
const groups: NavGroup[] = [
  { title: 'Platform', items: [{ title: 'Home', path: '/shell', icon: 'mdi-home-outline' }, { title: 'Assets', path: '/shell', icon: 'mdi-laptop' }] },
  { title: 'Operations', items: [{ title: 'Audit', path: '/shell', icon: 'mdi-history' }] },
]
</script>

<template>
  <UiPage title="App shell" subtitle="The frame around this page is UiAppShell; the pieces below are shown standalone">
    <UiSection title="App bar and user menu">
      <UiCard :padded="false">
        <UiAppBar title="Module title">
          <UiToolbar>
            <UiUserMenu name="Ada Lovelace" email="ada@example.test" tenant="platform" :items="[{ key: 'profile', label: 'Profile', icon: 'mdi-account-outline' }, { key: 'logout', label: 'Sign out', icon: 'mdi-logout', danger: true }]" @select="toast.info($event)" />
          </UiToolbar>
        </UiAppBar>
      </UiCard>
    </UiSection>
    <UiSection title="Navigation drawer (standalone)">
      <UiCard :padded="false" class="max-w-xs"><UiNavDrawer :groups="groups" @navigate="toast.info('navigate')" /></UiCard>
    </UiSection>
  </UiPage>
</template>
