<script setup lang="ts">
import { ref } from 'vue'
import { z } from 'zod'
import { UiPage, UiSection, UiGrid, UiCard, UiButton, UiToolbar, UiRecordDialog, UiRecordDrawer, UiDocumentList, UiAuditTable, UiPermissionDrawer, UiRemoteBoundary, useToast, type Grant, type DocumentRow, type AuditRow, type SelectOption } from '@/index'
import { nonEmpty, money, isoDate, optionalString } from '@/forms'
import { ApiError, type Api } from '@/api'

const toast = useToast()
const Schema = z.object({ name: nonEmpty(40), cost: money, status: z.enum(['active', 'retired']), purchased_at: isoDate, notes: optionalString(200) })
const dialog = ref(false)
const drawer = ref(false)
const perms = ref(false)

// In-memory fake API so the catalogue never touches a live system (SR-005).
const docs = ref<DocumentRow[]>([{ id: 'd1', file_name: 'invoice.pdf', file_size: 48213, mime_type: 'application/pdf', description: 'Synthetic invoice', created_at: '2026-01-04T10:00:00Z' }])
const audit: AuditRow[] = Array.from({ length: 60 }, (_, i) => ({ id: 'e' + i, at: new Date(Date.UTC(2026, 0, 1 + (i % 28), i % 24)).toISOString(), actor_kind: 'user', actor_id: 'u' + (i % 4), action: ['asset.created', 'asset.updated', 'asset.assigned'][i % 3]!, subject_kind: 'asset', subject_id: 'a' + i, outcome: i % 7 === 0 ? 'refused' : 'ok', detail: { n: i } }))
const api = (async (method: string, path: string, _body?: unknown, opts?: { query?: Record<string, unknown> }) => {
  await new Promise((r) => setTimeout(r, 150))
  if (path === 'documents') return { items: docs.value }
  if (path === 'audit') {
    const limit = Number(opts?.query?.limit ?? 50)
    const start = opts?.query?.cursor ? audit.findIndex((a) => a.id === opts.query!.cursor) + 1 : 0
    const items = audit.slice(start, start + limit)
    return { items, next_cursor: items[items.length - 1]?.id }
  }
  if (method === 'DELETE') {
    docs.value = docs.value.filter((d) => 'documents/' + d.id !== path)
    return undefined
  }
  throw new ApiError(404, 'not_found')
}) as unknown as Api
api.upload = (async (_path: string, file: File, fields?: Record<string, string>) => {
  const row: DocumentRow = { id: 'd' + Date.now(), file_name: file.name, file_size: file.size, mime_type: file.type, created_at: new Date().toISOString(), ...(fields?.description ? { description: fields.description } : {}) }
  docs.value = [...docs.value, row]
  return row
}) as Api['upload']
api.fileUrl = (p: string) => '#' + p

const grants = ref<Grant[]>([{ id: 'g1', subject_kind: 'group', subject_id: 'grp-ops', subject_name: 'Operations', level: 'edit' }, { id: 'g2', subject_kind: 'user', subject_id: 'u1', subject_name: 'Ada', level: 'view' }])
const subjects: SelectOption[] = [{ title: 'Grace (user)', value: 'user:u2' }, { title: 'Developers (group)', value: 'group:grp-dev' }]
const levels: SelectOption[] = [{ title: 'View', value: 'view' }, { title: 'Edit', value: 'edit' }, { title: 'Admin', value: 'admin' }]
const remoteState = ref<'ok' | 'loading' | 'error' | 'mismatch'>('ok')
const remoteError = () => (remoteState.value === 'error' ? new Error('Remote threw during mount') : remoteState.value === 'mismatch' ? new Error('Unsatisfied version 3.9.0 from asset of shared singleton module @go-tangra/ui (required ^4.0.0)') : undefined)
</script>

<template>
  <UiPage title="Composite" subtitle="Record dialog/drawer, documents, audit, permissions, remote boundary">
    <UiSection title="Record dialog and drawer">
      <UiToolbar>
        <UiButton data-testid="open-record" @click="dialog = true">New record (dialog)</UiButton>
        <UiButton variant="soft" @click="drawer = true">Edit record (drawer)</UiButton>
        <UiButton variant="outline" @click="perms = true">Permissions</UiButton>
      </UiToolbar>
      <UiRecordDialog v-model="dialog" title="New record" :schema="Schema" :submit="async (v) => v" @saved="toast.success('Saved', JSON.stringify($event))" />
      <UiRecordDrawer v-model="drawer" title="Edit record" :schema="Schema" :initial="{ name: 'Existing', cost: 12.5, status: 'active' }" :submit="async (v) => v" size="lg" @saved="toast.success('Saved')" />
      <UiPermissionDrawer v-model="perms" title="Who can access" :grants="grants" :subjects="subjects" :levels="levels" @grant="grants.push({ id: 'g' + Date.now(), subject_kind: $event.subject.split(':')[0]!, subject_id: $event.subject.split(':')[1]!, level: $event.level })" @revoke="grants = grants.filter((g) => g.id !== $event.id)" @change-level="(g, l) => (g.level = l)" />
    </UiSection>
    <UiSection title="Documents and audit">
      <UiGrid :cols="1" :lg-cols="2">
        <UiCard><UiDocumentList :api="api" base="documents" :max-bytes="1048576" /></UiCard>
        <UiCard title="Audit trail"><UiAuditTable :api="api" :page-size="20" /></UiCard>
      </UiGrid>
    </UiSection>
    <UiSection title="Remote boundary">
      <UiToolbar>
        <UiButton v-for="s in ['ok', 'loading', 'error', 'mismatch'] as const" :key="s" size="sm" :variant="remoteState === s ? 'solid' : 'soft'" @click="remoteState = s">{{ s }}</UiButton>
      </UiToolbar>
      <UiCard class="mt-3">
        <UiRemoteBoundary module="asset" :loading="remoteState === 'loading'" :error="remoteError()" @retry="remoteState = 'ok'">
          <p>The remote module rendered here.</p>
        </UiRemoteBoundary>
      </UiCard>
    </UiSection>
  </UiPage>
</template>
