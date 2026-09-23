<script setup lang="ts">
import { UiPage, UiToolbar, UiGrid, UiCard, UiSection, UiAlert, UiBadge, UiStatusChip, UiEmptyState, UiErrorState, UiSkeleton, UiAvatar, UiCopyButton, UiIcon, UiLiveIndicator, UiButton } from '@/index'

const colors = ['primary', 'secondary', 'accent', 'neutral', 'info', 'success', 'warning', 'error'] as const
const variants = ['solid', 'soft', 'outline', 'text'] as const
const statuses = ['active', 'assigned', 'pending', 'broken', 'retired', 'custom']
</script>

<template>
  <UiPage title="Primitives" subtitle="Layout, feedback and small display components">
    <template #badges><UiBadge color="primary">kit</UiBadge></template>
    <template #actions><UiButton icon="mdi-plus" size="sm">Action</UiButton></template>

    <UiSection title="Buttons" description="Every colour × variant, sizes, loading and icon-only">
      <div v-for="v in variants" :key="v" class="mb-2 flex flex-wrap gap-2">
        <UiButton v-for="c in colors" :key="c" :color="c" :variant="v" size="sm">{{ c }}</UiButton>
      </div>
      <UiToolbar>
        <UiButton size="xs">xs</UiButton><UiButton size="sm">sm</UiButton><UiButton>md</UiButton><UiButton size="lg">lg</UiButton>
        <UiButton loading>Saving</UiButton><UiButton disabled>Disabled</UiButton>
        <UiButton icon="mdi-pencil" icon-only label="Edit" /><UiButton icon="mdi-download" variant="soft">With icon</UiButton>
      </UiToolbar>
    </UiSection>

    <UiSection title="Alerts">
      <div class="flex flex-col gap-2">
        <UiAlert kind="info" title="Info">Something worth knowing.</UiAlert>
        <UiAlert kind="success">Saved.</UiAlert>
        <UiAlert kind="warning" title="Careful" dismissible>Dismissible warning.</UiAlert>
        <UiAlert kind="error" title="Failed">The request was refused.</UiAlert>
      </div>
    </UiSection>

    <UiSection title="Badges and status chips">
      <UiToolbar>
        <UiBadge v-for="c in colors" :key="c" :color="c">{{ c }}</UiBadge>
        <UiBadge color="primary" :soft="false">solid</UiBadge><UiBadge color="primary" outline>outline</UiBadge><UiBadge size="xs">xs</UiBadge><UiBadge size="md">md</UiBadge>
      </UiToolbar>
      <UiToolbar class="mt-2">
        <UiStatusChip v-for="s in statuses" :key="s" :status="s" />
        <UiStatusChip status="x" label="Labelled" :colors="{ x: 'accent' }" />
      </UiToolbar>
    </UiSection>

    <UiSection title="Cards and grid">
      <UiGrid :cols="1" :lg-cols="3">
        <UiCard title="Card" subtitle="With subtitle"><p>Body text.</p><template #footer><UiButton size="sm" variant="text">Footer</UiButton></template></UiCard>
        <UiCard flat title="Flat"><p>No shadow.</p></UiCard>
        <UiCard :padded="false"><template #header><span class="px-4 pt-4 font-medium">Custom header</span></template><p class="p-4">Unpadded body.</p></UiCard>
      </UiGrid>
    </UiSection>

    <UiSection title="States">
      <UiGrid :cols="1" :lg-cols="3">
        <UiCard><UiEmptyState title="No records" text="Create the first one." icon="mdi-package-variant"><UiButton size="sm">Create</UiButton></UiEmptyState></UiCard>
        <UiCard><UiErrorState text="The service did not answer." /></UiCard>
        <UiCard><UiSkeleton :lines="4" /><UiSkeleton kind="card" /><UiSkeleton kind="table" :lines="2" /></UiCard>
      </UiGrid>
    </UiSection>

    <UiSection title="Avatar, copy, icon, live indicator">
      <UiToolbar>
        <UiAvatar name="Ada Lovelace" size="sm" /><UiAvatar name="Grace Hopper" /><UiAvatar name="Linus" size="lg" /><UiAvatar />
        <UiCopyButton value="example-value" /><UiCopyButton value="x" label="Copy id" size="xs" />
        <UiIcon name="mdi-laptop" size="xs" /><UiIcon name="mdi-laptop" size="sm" /><UiIcon name="mdi-laptop" /><UiIcon name="mdi-laptop" size="lg" /><UiIcon name="mdi-laptop" size="xl" label="Laptop" />
        <UiLiveIndicator :connected="true" /><UiLiveIndicator :connected="false" />
      </UiToolbar>
    </UiSection>
  </UiPage>
</template>
