// Glue between a module's Zanzibar-style permission store (grants + effective
// relation over one resource), its auth directory (users/roles) and
// UiPermissionDrawer. lcm, notification and warden share this exact flow.
import { computed, ref, type Ref } from 'vue'
import type { Grant } from '@/components/UiPermissionDrawer.vue'
import type { SelectOption } from '@/components/UiSelect.vue'
import { describe } from '@/api/client'

export interface StoreGrant { id: string; subject_type: string; subject_id?: string | undefined; relation: string; expires_at?: string | null | undefined }
export interface GrantRequest { subject_type: string; subject_id?: string | undefined; relation: string; expires_at?: string | undefined }
export interface PermissionGrantsDeps {
  grants: () => StoreGrant[]
  /** The caller's own relation and whether it may share. */
  effective: () => { relation?: string | undefined; canShare: boolean }
  grant: (req: GrantRequest) => Promise<unknown>
  revoke: (id: string) => Promise<unknown>
  directory: {
    roles: () => { slug: string; display_name: string }[]
    searchUsers: (q: string) => Promise<{ id: string; display_name: string }[]>
    resolveUsers: (ids: Array<string | undefined>) => Promise<void>
    userName: (id: string | undefined) => string
    roleName: (slug: string | undefined) => string
  }
  /** Relations the holder of `held` may grant, lowest first. */
  grantable: (held: string) => string[]
}

export function usePermissionGrants(deps: PermissionGrantsDeps) {
  const error = ref('')
  const busy = ref(false)
  const userHits: Ref<SelectOption[]> = ref([])

  const subjectLabel = (g: { subject_type: string; subject_id?: string | undefined }) =>
    g.subject_type === 'tenant' ? 'Everyone in the tenant' : g.subject_type === 'role' ? 'Role: ' + deps.directory.roleName(g.subject_id) : deps.directory.userName(g.subject_id)
  const grants = computed<Grant[]>(() => deps.grants().map((g) => ({ id: g.id, subject_kind: g.subject_type, subject_id: g.subject_id ?? '', subject_name: subjectLabel(g), level: g.relation, ...(g.expires_at ? { expires_at: g.expires_at } : {}) })))
  const subjects = computed<SelectOption[]>(() => [
    { title: 'Everyone in the tenant', value: 'tenant' },
    ...deps.directory.roles().map((r) => ({ title: 'Role: ' + r.display_name, value: 'role:' + r.slug })),
    ...userHits.value,
  ])
  const canShare = computed(() => deps.effective().canShare)
  const levels = computed<SelectOption[]>(() => deps.grantable(deps.effective().relation ?? '').map((r) => ({ title: r, value: r })))
  const hint = computed(() => 'Your relation: ' + (deps.effective().relation || 'none'))

  async function search(q: string): Promise<void> {
    userHits.value = (await deps.directory.searchUsers(q)).map((u) => ({ title: u.display_name, value: 'user:' + u.id }))
  }
  async function run(fn: () => Promise<unknown>): Promise<void> {
    busy.value = true
    error.value = ''
    try {
      await fn()
    } catch (e) {
      error.value = describe(e)
    } finally {
      busy.value = false
    }
  }
  /** "tenant" | "role:<slug>" | "user:<id>" → subject_type/subject_id. */
  function parseSubject(key: string): { subject_type: string; subject_id: string | undefined } {
    if (key === 'tenant') return { subject_type: 'tenant', subject_id: undefined }
    const i = key.indexOf(':')
    return { subject_type: key.slice(0, i), subject_id: key.slice(i + 1) }
  }
  const onGrant = (v: { subject: string; level: string; expires_at?: string }) =>
    run(async () => {
      const s = parseSubject(v.subject)
      await deps.grant({ ...s, relation: v.level, ...(v.expires_at ? { expires_at: v.expires_at } : {}) })
      await deps.directory.resolveUsers([s.subject_id])
    })
  const onRevoke = (g: Grant) => run(() => deps.revoke(g.id))
  const onChangeLevel = (g: Grant, level: string) => run(() => deps.grant({ subject_type: g.subject_kind, subject_id: g.subject_id || undefined, relation: level }))
  /** Resolve display names for the currently listed grants. */
  const resolve = () => deps.directory.resolveUsers(deps.grants().map((g) => (g.subject_type === 'user' ? g.subject_id : undefined)))

  return { grants, subjects, levels, canShare, hint, error, busy, search, onGrant, onRevoke, onChangeLevel, resolve }
}
