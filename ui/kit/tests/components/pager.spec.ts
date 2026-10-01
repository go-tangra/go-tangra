import { describe, it, expect } from 'vitest'
import { expectA11y, mountBody } from '../helpers'
import { UiPager } from '@/index'

const labels = (w: ReturnType<typeof mountBody>) => w.findAll('[data-page]').map((b) => b.text())

describe('UiPager', () => {
  it('shows the range and total, numbered pages with elision, and emits page changes', async () => {
    const w = mountBody(UiPager, { props: { page: 6, pageSize: 50, total: 1234 } })
    expect(w.text()).toContain('Showing 251–300 of 1,234')
    expect(labels(w)).toEqual(['1', '…', '4', '5', '6', '7', '8', '…', '25'])
    expect(w.find('[aria-current="page"]').text()).toBe('6')
    await w.find('[aria-label="Next page"]').trigger('click')
    await w.find('[aria-label="Previous page"]').trigger('click')
    await w.find('[aria-label="First page"]').trigger('click')
    await w.find('[aria-label="Last page"]').trigger('click')
    await w.find('[aria-label="Page 8"]').trigger('click')
    expect(w.emitted('update:page')).toEqual([[7], [5], [1], [25], [8]])
    await expectA11y(w.element)
  })

  it('elides only real gaps and lists every page when there are few', () => {
    const near = mountBody(UiPager, { props: { page: 2, pageSize: 10, total: 250 } })
    expect(labels(near)).toEqual(['1', '2', '3', '4', '…', '25'])
    const end = mountBody(UiPager, { props: { page: 25, pageSize: 10, total: 250 } })
    expect(labels(end)).toEqual(['1', '…', '23', '24', '25'])
    const gapOfOne = mountBody(UiPager, { props: { page: 4, pageSize: 10, total: 250 } })
    expect(labels(gapOfOne)).toEqual(['1', '2', '3', '4', '5', '6', '…', '25'])
    const few = mountBody(UiPager, { props: { page: 1, pageSize: 10, total: 70 } })
    expect(labels(few)).toEqual(['1', '2', '3', '4', '5', '6', '7'])
    const huge = mountBody(UiPager, { props: { page: 500, pageSize: 10, total: 10000 } })
    expect(labels(huge)).toEqual(['1', '…', '498', '499', '500', '501', '502', '…', '1000'])
  })

  it('disables first/previous on page 1 and next/last on the last page', () => {
    const first = mountBody(UiPager, { props: { page: 1, pageSize: 25, total: 60 } })
    expect(first.find('[aria-label="First page"]').attributes('disabled')).toBeDefined()
    expect(first.find('[aria-label="Previous page"]').attributes('disabled')).toBeDefined()
    expect(first.find('[aria-label="Next page"]').attributes('disabled')).toBeUndefined()
    const last = mountBody(UiPager, { props: { page: 3, pageSize: 25, total: 60 } })
    expect(last.text()).toContain('Showing 51–60 of 60')
    expect(last.find('[aria-label="Last page"]').attributes('disabled')).toBeDefined()
    expect(last.find('[aria-label="Next page"]').attributes('disabled')).toBeDefined()
  })

  it('page-size select emits; a single page shows the range and size but no page navigation', async () => {
    const w = mountBody(UiPager, { props: { page: 1, pageSize: 25, total: 12 } })
    expect(w.text()).toContain('Showing 1–12 of 12')
    expect(w.find('nav').exists()).toBe(false)
    const sel = w.find('select')
    expect(sel.findAll('option').map((o) => o.text())).toEqual(['10', '25', '50', '100', '200'])
    await sel.setValue('100')
    expect(w.emitted('update:pageSize')).toEqual([[100]])
    const custom = mountBody(UiPager, { props: { page: 1, pageSize: 5, total: 12, pageSizes: [5, 15] } })
    expect(custom.findAll('option').map((o) => o.text())).toEqual(['5', '15'])
  })

  it('renders nothing for an empty list', () => {
    const w = mountBody(UiPager, { props: { page: 1, pageSize: 25, total: 0 } })
    expect(w.text()).toBe('')
    expect(w.find('select').exists()).toBe(false)
  })
})
