import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { expectA11y, mountBody } from '../helpers'
import { UiPage, UiToolbar, UiGrid, UiCard, UiSection, UiAlert, UiBadge, UiStatusChip, UiEmptyState, UiErrorState, UiSkeleton, UiAvatar, UiCopyButton, UiIcon, UiButton, UiLiveIndicator } from '@/index'

describe('primitives', () => {
  it('render, pass class through and use no inline styles', async () => {
    const w = mountBody({
      components: { UiPage, UiToolbar, UiGrid, UiCard, UiSection, UiAlert, UiBadge, UiStatusChip, UiEmptyState, UiSkeleton, UiAvatar, UiIcon, UiButton, UiLiveIndicator },
      template: `
        <UiPage title="Assets" subtitle="sub" class="extra"><template #actions><UiButton label="New" icon="mdi-plus" /></template><template #filters><span>f</span></template>
          <UiToolbar justify="between"><UiBadge color="success">ok</UiBadge><UiStatusChip status="assigned" /><UiStatusChip status="weird-thing" /><UiLiveIndicator :connected="true" /></UiToolbar>
          <UiGrid :cols="3" :lg-cols="4"><UiCard title="Card" subtitle="s"><p>body</p><template #footer><UiButton variant="soft" size="sm">x</UiButton></template></UiCard><UiSection title="Sec" description="d">content</UiSection></UiGrid>
          <UiAlert kind="error" title="Oops" dismissible>text</UiAlert>
          <UiAlert kind="success">fine</UiAlert>
          <UiEmptyState title="Nothing" text="t" />
          <UiSkeleton :lines="2" /><UiSkeleton kind="card" />
          <UiAvatar name="Ann Example" /><UiAvatar name="Ann" src="data:image/png;base64,iVBORw0KGgo=" />
          <UiIcon name="mdi-laptop" label="Laptop" /><UiIcon name="laptop" />
          <UiButton icon="mdi-plus" icon-only label="Add" /><UiButton loading>Saving</UiButton>
        </UiPage>`,
    })
    expect(w.text()).toContain('Assets')
    expect(w.find('section').classes()).toContain('extra')
    expect(w.find('.badge-success').exists()).toBe(true)
    expect(w.findAll('.badge').some((b) => b.text() === 'weird-thing')).toBe(true)
    expect(w.find('[role=alert]').text()).toContain('Oops')
    expect(w.find('[role=status]').text()).toContain('fine')
    expect(w.find('.avatar-placeholder').text()).toBe('AE')
    expect(w.find('span[role=img]').attributes('aria-label')).toBe('Laptop')
    expect(w.find('span.icon-\\[mdi--laptop\\]').exists()).toBe(true)
    expect(w.find('button[aria-label=Add]').exists()).toBe(true)
    expect(w.find('button[aria-busy=true]').exists()).toBe(true)
    await expectA11y(w.element)
    w.unmount()
  })
  it('alert dismiss emits, error state retries, copy button copies', async () => {
    const a = mount(UiAlert, { props: { dismissible: true }, slots: { default: 'x' } })
    await a.find('button[aria-label=Dismiss]').trigger('click')
    expect(a.emitted('dismiss')).toHaveLength(1)
    const e = mount(UiErrorState, { props: { text: 'boom' } })
    await e.find('button').trigger('click')
    expect(e.emitted('retry')).toHaveLength(1)
    let copied = ''
    Object.assign(navigator, { clipboard: { writeText: async (s: string) => { copied = s } } })
    const c = mount(UiCopyButton, { props: { value: 'SECRET' } })
    await c.find('button').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(copied).toBe('SECRET')
    expect(c.text()).toContain('Copied')
    expect(c.html()).not.toContain('SECRET')
    Object.assign(navigator, { clipboard: { writeText: async () => { throw new Error('denied') } } })
    await c.find('button').trigger('click')
    const g = mount(UiGrid, { props: { cols: 6 }, slots: { default: '<i/>' } })
    expect(g.classes()).toContain('md:grid-cols-6')
  })
})

describe('UiCard padding', () => {
  it('pads the body by default and only drops it with padded=false', async () => {
    const { mount } = await import('@vue/test-utils')
    const { default: UiCard } = await import('@/components/UiCard.vue')
    const padded = mount(UiCard, { slots: { default: 'x' } })
    expect(padded.find('.card-body').exists()).toBe(true)
    const bare = mount(UiCard, { props: { padded: false }, slots: { default: 'x' } })
    expect(bare.find('.card-body').exists()).toBe(false)
  })
})
