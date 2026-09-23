import { describe, it, expect } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { useBreakpoint, prefersReducedMotion, useTheme, useFocusTrap, THEME_KEY, iconClass } from '@/index'
import { setViewport } from './helpers'

describe('composables', () => {
  it('useBreakpoint reflects matchMedia', () => {
    setViewport(360)
    const C = defineComponent({ setup: () => ({ md: useBreakpoint('md'), lg: useBreakpoint('lg') }), template: '<i>{{ md }}-{{ lg }}</i>' })
    expect(mount(C).text()).toBe('false-false')
    setViewport(1280)
    expect(mount(C).text()).toBe('true-true')
    expect(prefersReducedMotion()).toBe(false)
  })
  it('useTheme applies data-theme, persists and ignores bad stored values', async () => {
    localStorage.setItem(THEME_KEY, 'garbage')
    const t = useTheme()
    expect(['freya-light', 'freya-dark']).toContain(t.theme.value)
    t.set('freya-dark')
    await nextTick()
    expect(document.documentElement.getAttribute('data-theme')).toBe('freya-dark')
    expect(localStorage.getItem(THEME_KEY)).toBe('freya-dark')
    expect(t.themes).toHaveLength(2)
    t.set('freya-light')
  })
  it('useFocusTrap focuses the container when nothing is focusable and restores on deactivate', async () => {
    const prev = document.createElement('button')
    document.body.appendChild(prev)
    prev.focus()
    const C = defineComponent({
      setup() {
        const el = ref<HTMLElement | null>(null)
        const active = ref(false)
        useFocusTrap(el, active)
        return { el, active }
      },
      template: '<div ref="el" tabindex="-1"><span>no focusables</span></div>',
    })
    const w = mount(C, { attachTo: document.body })
    w.vm.active = true
    await nextTick()
    await new Promise((r) => setTimeout(r, 0))
    expect(document.activeElement).toBe(w.element)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(w.element)
    w.vm.active = false
    await nextTick()
    expect(document.activeElement).toBe(prev)
    w.unmount()
  })
  it('iconClass maps names and tolerates missing prefix', () => {
    expect(iconClass('mdi-laptop')).toBe('icon-[mdi--laptop]')
    expect(iconClass('laptop')).toBe('icon-[mdi--laptop]')
    expect(iconClass(undefined)).toBe('icon-[mdi--circle-outline]')
    expect(iconClass('mdi-not-listed-anywhere')).toBe('icon-[mdi--not-listed-anywhere]')
  })
})

describe('usePermissionGrants', () => {
  it('maps store grants to drawer rows, limits levels to the holder, parses subject keys', async () => {
    const { usePermissionGrants } = await import('@/index')
    const granted: unknown[] = []
    const revoked: string[] = []
    const g = usePermissionGrants({
      grants: () => [{ id: 'g1', subject_type: 'user', subject_id: 'u1', relation: 'viewer', expires_at: '2027-01-01T00:00:00Z' }, { id: 'g2', subject_type: 'tenant', relation: 'viewer' }],
      effective: () => ({ relation: 'sharer', canShare: true }),
      grant: async (r) => { granted.push(r) },
      revoke: async (id) => { revoked.push(id) },
      directory: { roles: () => [{ slug: 'admin', display_name: 'Admin' }], searchUsers: async (q) => (q ? [{ id: 'u2', display_name: 'Bo' }] : []), resolveUsers: async () => {}, userName: (id) => (id === 'u1' ? 'Ana' : id ?? ''), roleName: (s) => s ?? '' },
      grantable: (held) => (held === 'sharer' ? ['viewer', 'sharer'] : []),
    })
    expect(g.grants.value.map((r) => [r.subject_name, r.level, r.expires_at])).toEqual([['Ana', 'viewer', '2027-01-01T00:00:00Z'], ['Everyone in the tenant', 'viewer', undefined]])
    expect(g.levels.value.map((l) => l.value)).toEqual(['viewer', 'sharer'])
    expect(g.subjects.value.map((s) => s.value)).toEqual(['tenant', 'role:admin'])
    await g.search('b')
    expect(g.subjects.value.map((s) => s.value)).toContain('user:u2')
    await g.onGrant({ subject: 'user:u2', level: 'viewer', expires_at: '2027-01-01T00:00:00.000Z' })
    await g.onGrant({ subject: 'tenant', level: 'viewer' })
    expect(granted).toEqual([{ subject_type: 'user', subject_id: 'u2', relation: 'viewer', expires_at: '2027-01-01T00:00:00.000Z' }, { subject_type: 'tenant', subject_id: undefined, relation: 'viewer' }])
    await g.onRevoke(g.grants.value[0]!)
    expect(revoked).toEqual(['g1'])
    await g.onChangeLevel(g.grants.value[1]!, 'sharer')
    expect(granted.at(-1)).toEqual({ subject_type: 'tenant', subject_id: undefined, relation: 'sharer' })
    expect(g.hint.value).toBe('Your relation: sharer')
  })
})
