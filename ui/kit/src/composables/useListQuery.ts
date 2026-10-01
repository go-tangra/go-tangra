// Page, page size and sort of one server-side table, kept in the route query
// under a per-table key (`?hosts.page=3&hosts.size=100&hosts.sort=last_seen&hosts.order=desc`)
// so a reload or a shared link restores the view. Invalid values fall back to
// the defaults; defaults are kept out of the URL. Without a router the state
// lives in memory.
import { computed, inject, ref, watch, type ComputedRef, type Ref } from 'vue'
import { routerKey, type LocationQuery } from 'vue-router'
import type { TableSort } from '@/components/UiDataTable.vue'

export interface ListQueryOptions {
  /** Field names the server allows for `sort`. */
  sortable: string[]
  defaultSort: TableSort
  defaultSize?: number | undefined
  pageSizes?: number[] | undefined
}

/** The parameters to send to a list endpoint. */
export interface ListParams { page: number; page_size: number; sort: string; order: 'asc' | 'desc' }

export interface ListQuery {
  page: Ref<number>
  pageSize: Ref<number>
  sort: Ref<TableSort>
  query: ComputedRef<ListParams>
  setPage(p: number): void
  /** Keeps the first visible record on screen at the new size. */
  setPageSize(s: number): void
  /** A new sort starts at page 1. Fields outside `sortable` are ignored. */
  setSort(s: TableSort): void
  /** Call when filters or search change. */
  resetPage(): void
  /** Resolves with the value of the latest tracked request only; superseded ones resolve with null. */
  track<T>(p: Promise<T>): Promise<T | null>
  /** Adopt the page the server returned (it clamps pages beyond the end). */
  clampTo(page: number): void
}

const SIZES = [10, 25, 50, 100, 200]

export function useListQuery(key: string, opts: ListQueryOptions): ListQuery {
  const sizes = opts.pageSizes ?? SIZES
  const defSize = opts.defaultSize ?? 25
  const k = { page: key + '.page', size: key + '.size', sort: key + '.sort', order: key + '.order' }
  const router = inject(routerKey, null)

  const one = (q: LocationQuery, name: string) => {
    const v = q[name]
    return typeof v === 'string' ? v : undefined
  }
  function read(q: LocationQuery) {
    const p = Number(one(q, k.page))
    const s = Number(one(q, k.size))
    const field = one(q, k.sort)
    const dir = one(q, k.order)
    const sortKey = field && opts.sortable.includes(field) ? field : opts.defaultSort.key
    return {
      page: Number.isInteger(p) && p >= 1 ? p : 1,
      size: sizes.includes(s) ? s : defSize,
      sort: { key: sortKey, dir: dir === 'asc' || dir === 'desc' ? dir : sortKey === opts.defaultSort.key ? opts.defaultSort.dir : 'asc' } as TableSort,
    }
  }

  const init = read(router?.currentRoute.value.query ?? {})
  const page = ref(init.page)
  const pageSize = ref(init.size)
  const sort = ref<TableSort>(init.sort)

  function write() {
    if (!router) return
    const cur = router.currentRoute.value.query
    const next: LocationQuery = { ...cur }
    const set = (name: string, v: string | undefined) => {
      if (v === undefined) delete next[name]
      else next[name] = v
    }
    set(k.page, page.value === 1 ? undefined : String(page.value))
    set(k.size, pageSize.value === defSize ? undefined : String(pageSize.value))
    const isDefault = sort.value.key === opts.defaultSort.key && sort.value.dir === opts.defaultSort.dir
    set(k.sort, isDefault ? undefined : sort.value.key)
    set(k.order, isDefault ? undefined : sort.value.dir)
    if ([k.page, k.size, k.sort, k.order].some((n) => next[n] !== cur[n])) void router.replace({ query: next })
  }
  watch([page, pageSize, sort], write, { deep: true })
  if (router) {
    // Back/forward or a link changing the query updates the state.
    watch(() => router.currentRoute.value.query, (q) => {
      const r = read(q)
      if (r.page !== page.value) page.value = r.page
      if (r.size !== pageSize.value) pageSize.value = r.size
      if (r.sort.key !== sort.value.key || r.sort.dir !== sort.value.dir) sort.value = r.sort
    })
  }

  let seq = 0
  return {
    page,
    pageSize,
    sort,
    query: computed(() => ({ page: page.value, page_size: pageSize.value, sort: sort.value.key, order: sort.value.dir })),
    setPage: (p) => {
      if (Number.isInteger(p) && p >= 1) page.value = p
    },
    setPageSize: (s) => {
      if (!sizes.includes(s)) return
      const first = (page.value - 1) * pageSize.value
      pageSize.value = s
      page.value = Math.floor(first / s) + 1
    },
    setSort: (s) => {
      if (!opts.sortable.includes(s.key)) return
      sort.value = { key: s.key, dir: s.dir }
      page.value = 1
    },
    resetPage: () => {
      page.value = 1
    },
    track: async <T>(p: Promise<T>) => {
      const id = ++seq
      const v = await p
      return id === seq ? v : null
    },
    clampTo: (p) => {
      if (Number.isInteger(p) && p >= 1) page.value = p
    },
  }
}
