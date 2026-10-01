import { describe, it, expect } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory, type Router } from 'vue-router'
import { useListQuery, type ListQuery, type ListQueryOptions } from '@/index'

const opts: ListQueryOptions = { sortable: ['hostname', 'last_seen'], defaultSort: { key: 'hostname', dir: 'asc' } }

async function setup(path = '/', o: ListQueryOptions = opts, key = 'hosts') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { render: () => null } }] })
  await router.push(path)
  await router.isReady()
  let lq!: ListQuery
  let other!: ListQuery
  const Host = defineComponent({
    setup() {
      lq = useListQuery(key, o)
      other = useListQuery('snap', { sortable: ['collected_at'], defaultSort: { key: 'collected_at', dir: 'desc' }, defaultSize: 50 })
      return () => h('div')
    },
  })
  mount(Host, { global: { plugins: [router] } })
  await flushPromises()
  return { router, lq: () => lq, other: () => other }
}

const q = (r: Router) => r.currentRoute.value.query

describe('useListQuery', () => {
  it('starts from defaults and exposes the API query', async () => {
    const { lq } = await setup()
    expect(lq().query.value).toEqual({ page: 1, page_size: 25, sort: 'hostname', order: 'asc' })
  })

  it('restores state from the URL and writes changes back under its own key', async () => {
    const { router, lq, other } = await setup('/?hosts.page=3&hosts.size=100&hosts.sort=last_seen&hosts.order=desc&keep=1')
    expect(lq().query.value).toEqual({ page: 3, page_size: 100, sort: 'last_seen', order: 'desc' })
    expect(other().query.value).toEqual({ page: 1, page_size: 50, sort: 'collected_at', order: 'desc' })
    lq().setPage(4)
    await flushPromises()
    expect(q(router)['hosts.page']).toBe('4')
    expect(q(router).keep).toBe('1') // unrelated params survive
    expect(q(router)['snap.page']).toBeUndefined() // the other table is untouched
    other().setPage(2)
    await flushPromises()
    expect(q(router)['snap.page']).toBe('2')
    expect(lq().page.value).toBe(4)
  })

  it('falls back to defaults for invalid URL values without errors', async () => {
    const { lq } = await setup('/?hosts.page=-2&hosts.size=999&hosts.sort=drop&hosts.order=up')
    expect(lq().query.value).toEqual({ page: 1, page_size: 25, sort: 'hostname', order: 'asc' })
  })

  it('keeps default values out of the URL', async () => {
    const { router, lq } = await setup('/?hosts.page=2')
    lq().setPage(1)
    await flushPromises()
    expect(q(router)['hosts.page']).toBeUndefined()
  })

  it('sort change and resetPage go to page 1; size change keeps the first visible record', async () => {
    const { lq } = await setup('/?hosts.page=7&hosts.size=50')
    lq().setPageSize(100) // record 301 → page 4
    expect(lq().page.value).toBe(4)
    expect(lq().pageSize.value).toBe(100)
    lq().setSort({ key: 'last_seen', dir: 'desc' })
    expect(lq().page.value).toBe(1)
    expect(lq().sort.value).toEqual({ key: 'last_seen', dir: 'desc' })
    lq().setPage(3)
    lq().resetPage()
    expect(lq().page.value).toBe(1)
    lq().setSort({ key: 'not-allowed', dir: 'asc' }) // ignored
    expect(lq().sort.value.key).toBe('last_seen')
    lq().setPageSize(7) // not an offered size → ignored
    expect(lq().pageSize.value).toBe(100)
  })

  it('follows navigation that changes the query (links, back/forward) and does not add history entries itself', async () => {
    const { router, lq } = await setup()
    await router.push('/?hosts.page=2&hosts.sort=last_seen&hosts.order=desc')
    await nextTick()
    expect(lq().query.value).toMatchObject({ page: 2, sort: 'last_seen', order: 'desc' })
    lq().setPage(5) // replace, not push
    await flushPromises()
    router.back()
    await flushPromises()
    await nextTick()
    expect(lq().query.value).toMatchObject({ page: 1, sort: 'hostname', order: 'asc' })
  })

  it('track drops superseded responses; clampTo adopts the server page', async () => {
    const { lq } = await setup('/?hosts.page=9')
    let resolveA!: (v: string) => void
    const a = lq().track(new Promise<string>((r) => (resolveA = r)))
    const b = lq().track(Promise.resolve('b'))
    resolveA('a')
    expect(await a).toBeNull()
    expect(await b).toBe('b')
    lq().clampTo(4)
    expect(lq().page.value).toBe(4)
  })

  it('works without a router (in-memory state)', () => {
    let lq!: ListQuery
    mount(defineComponent({ setup() { lq = useListQuery('t', opts); return () => h('div') } }))
    lq.setPage(3)
    expect(lq.query.value.page).toBe(3)
  })
})
