<script setup lang="ts">
import { ref } from 'vue'
import { UiPage, UiSection, UiToolbar, UiButton, UiDialog, UiDrawer, UiDropdownMenu, UiTooltip, UiTabs, UiAccordion, useToast, useConfirm, type MenuItem, type TabItem, type AccordionItem } from '@/index'

const dialog = ref(false)
const persistent = ref(false)
const drawer = ref(false)
const tab = ref('one')
const toast = useToast()
const confirm = useConfirm()
const items: MenuItem[] = [{ key: 'edit', label: 'Edit', icon: 'mdi-pencil' }, { key: 'dup', label: 'Duplicate', disabled: true }, { key: 'del', label: 'Delete', icon: 'mdi-delete-outline', danger: true }]
const tabs: TabItem[] = [{ key: 'one', label: 'Overview', icon: 'mdi-view-dashboard-outline' }, { key: 'two', label: 'History', count: 12 }, { key: 'three', label: 'Settings' }]
const acc: AccordionItem[] = [{ key: 'a', title: 'First', subtitle: 'open by default' }, { key: 'b', title: 'Second' }]
const last = ref('')
async function ask() {
  last.value = (await confirm.ask({ title: 'Delete record?', text: 'This cannot be undone.', danger: true, confirmLabel: 'Delete' })) ? 'confirmed' : 'cancelled'
}
</script>

<template>
  <UiPage title="Overlays" subtitle="Dialog, drawer, menu, tooltip, toast, confirm, tabs, accordion">
    <UiSection title="Dialog and drawer">
      <UiToolbar>
        <UiButton data-testid="open-dialog" @click="dialog = true">Open dialog</UiButton>
        <UiButton variant="soft" @click="persistent = true">Persistent dialog</UiButton>
        <UiButton variant="outline" data-testid="open-drawer" @click="drawer = true">Open drawer</UiButton>
      </UiToolbar>
      <UiDialog v-model="dialog" title="A dialog" size="md">
        <p>Focus is trapped; Esc and the backdrop close it.</p>
        <template #actions><UiButton variant="text" @click="dialog = false">Cancel</UiButton><UiButton @click="dialog = false">OK</UiButton></template>
      </UiDialog>
      <UiDialog v-model="persistent" title="Persistent" persistent hide-close>
        <p>Only the button closes this one.</p>
        <template #actions><UiButton @click="persistent = false">Close</UiButton></template>
      </UiDialog>
      <UiDrawer v-model="drawer" title="Side sheet" size="lg">
        <template #header-actions><UiButton size="sm" variant="text" icon="mdi-pencil" icon-only label="Edit" /></template>
        <p>Detail content.</p>
        <template #actions><UiButton @click="drawer = false">Done</UiButton></template>
      </UiDrawer>
    </UiSection>

    <UiSection title="Menu, tooltip, toast, confirm">
      <UiToolbar>
        <UiDropdownMenu :items="items" @select="toast.info('Selected ' + $event)" />
        <UiDropdownMenu :items="items" label="Left menu" icon="mdi-menu" align="start" size="md" />
        <UiTooltip text="Helpful hint"><UiButton variant="soft">Hover me</UiButton></UiTooltip>
        <UiTooltip text="Below" position="bottom"><span class="link">bottom tooltip</span></UiTooltip>
        <UiButton color="success" @click="toast.success('Saved', 'The record was stored.')">Toast success</UiButton>
        <UiButton color="error" @click="toast.error('Failed')">Toast error</UiButton>
        <UiButton color="warning" @click="toast.show({ kind: 'warning', title: 'Sticky', timeout: 0 })">Sticky toast</UiButton>
        <UiButton color="neutral" data-testid="ask" @click="ask">Confirm… <span v-if="last">({{ last }})</span></UiButton>
      </UiToolbar>
    </UiSection>

    <UiSection title="Tabs and accordion">
      <UiTabs v-model="tab" :tabs="tabs" />
      <p class="mt-2 text-sm">Selected: {{ tab }}</p>
      <UiAccordion class="mt-4" :items="acc" :open="['a']">
        <template #a><p>First body.</p></template>
        <template #b><p>Second body.</p></template>
      </UiAccordion>
    </UiSection>
  </UiPage>
</template>
