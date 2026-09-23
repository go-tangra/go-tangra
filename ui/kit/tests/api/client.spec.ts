import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createApi, ApiError, csrfToken, describe as describeErr, CSRF_HEADER } from '@/api/client'

function mockFetch(status: number, body: unknown, headers: Record<string, string> = {}) {
  const res = {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers(headers),
    json: async () => body,
    text: async () => JSON.stringify(body),
  } as unknown as Response
  const fn = vi.fn(async () => res)
  globalThis.fetch = fn as unknown as typeof fetch
  return fn
}

describe('api client', () => {
  beforeEach(() => {
    document.cookie = '__Host-csrf=tok123; path=/; secure'
  })
  it('reads the CSRF cookie', () => {
    expect(csrfToken()).toBe('tok123')
  })
  it('GET without body or csrf header; query serialised; base applied', async () => {
    const fetchMock = mockFetch(200, { items: [] })
    const api = createApi({ base: '/api/asset/v1' })
    const out = await api<{ items: unknown[] }>('GET', 'assets', undefined, { query: { q: 'x', limit: 5, skip: undefined, empty: '' } })
    expect(out.items).toEqual([])
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/asset/v1/assets?q=x&limit=5')
    expect((init.headers as Record<string, string>)[CSRF_HEADER]).toBeUndefined()
    expect(init.body).toBeNull()
  })
  it('POST sends JSON, csrf header, request id passthrough, absolute paths untouched', async () => {
    const fetchMock = mockFetch(201, { id: '1' })
    const api = createApi({ base: '/api/x/v1', requestId: () => 'rid-1' })
    await api('POST', '/gateway/v1/me', { a: 1 })
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/gateway/v1/me')
    const h = init.headers as Record<string, string>
    expect(h[CSRF_HEADER]).toBe('tok123')
    expect(h['Content-Type']).toBe('application/json')
    expect(h['X-Request-Id']).toBe('rid-1')
    expect(init.body).toBe('{"a":1}')
  })
  it('204 resolves undefined; non-2xx throws ApiError with reason + detail', async () => {
    mockFetch(204, {})
    const api = createApi({ base: '/b' })
    expect(await api('DELETE', 'x/1')).toBeUndefined()
    mockFetch(422, { reason: 'validation_failed', detail: { fields: { name: 'bad' } } })
    await expect(api('POST', 'x', {})).rejects.toMatchObject({ status: 422, reason: 'validation_failed', detail: { fields: { name: 'bad' } } })
    mockFetch(500, 'not json')
    const err = (await api('GET', 'x').catch((e: unknown) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.reason).toBe('error')
  })
  it('network failure → ApiError(0, network); abort propagates', async () => {
    globalThis.fetch = vi.fn(async () => { throw new TypeError('fail') }) as unknown as typeof fetch
    const api = createApi({ base: '/b' })
    await expect(api('GET', 'x')).rejects.toMatchObject({ status: 0, reason: 'network' })
    globalThis.fetch = vi.fn(async () => { throw new DOMException('aborted', 'AbortError') }) as unknown as typeof fetch
    await expect(api('GET', 'x', undefined, { signal: new AbortController().signal })).rejects.toBeInstanceOf(DOMException)
  })
  it('upload sends multipart with csrf and extra fields; fileUrl resolves', async () => {
    const fetchMock = mockFetch(201, { id: 'd1' })
    const api = createApi({ base: '/api/asset/v1' })
    const file = new File(['abc'], 'a.txt', { type: 'text/plain' })
    const out = await api.upload<{ id: string }>('assets/1/documents', file, { description: 'inv' })
    expect(out.id).toBe('d1')
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/asset/v1/assets/1/documents')
    expect((init.headers as Record<string, string>)[CSRF_HEADER]).toBe('tok123')
    const fd = init.body as FormData
    expect((fd.get('file') as File).name).toBe('a.txt')
    expect(fd.get('description')).toBe('inv')
    expect(api.fileUrl('assets/1/photo')).toBe('/api/asset/v1/assets/1/photo')
    expect(api.fileUrl('/m/x')).toBe('/m/x')
    mockFetch(413, { reason: 'body_too_large' })
    await expect(api.upload('x', file)).rejects.toMatchObject({ status: 413, reason: 'body_too_large' })
    globalThis.fetch = vi.fn(async () => { throw new TypeError('fail') }) as unknown as typeof fetch
    await expect(api.upload('x', file)).rejects.toMatchObject({ reason: 'network' })
  })
  it('describe maps reasons', () => {
    expect(describeErr(new ApiError(404, 'not_found'))).toBe('Not found.')
    expect(describeErr(new Error('x'))).toBe('Something went wrong.')
    expect(describeErr(new ApiError(500, 'weird'))).toBe('Something went wrong.') // unknown reasons are never echoed
  })
})

describe('api client (branches)', () => {
  it('no csrf cookie → empty token; query on a path that already has a query; upload with request id', async () => {
    document.cookie = '__Host-csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/; secure'
    expect(csrfToken()).toBe('')
    const fetchMock = mockFetch(200, {})
    const api = createApi({ base: '/b', requestId: () => 'r2' })
    await api('GET', 'x?a=1', undefined, { query: { b: 2 } })
    expect((fetchMock.mock.calls[0] as unknown as [string])[0]).toBe('/b/x?a=1&b=2')
    await api.upload('u', new File(['x'], 'f'))
    const init = (fetchMock.mock.calls[1] as unknown as [string, RequestInit])[1]
    expect((init.headers as Record<string, string>)['X-Request-Id']).toBe('r2')
    mockFetch(400, { reason: 'malformed_body', detail: 'not-an-object' })
    await expect(api('POST', 'x', {})).rejects.toMatchObject({ reason: 'malformed_body', detail: undefined })
  })
})

describe('api client (query edge)', () => {
  it('malformed JSON bodies are treated as empty objects (request + upload)', async () => {
    const broken = { ok: false, status: 500, json: async () => { throw new SyntaxError('bad') } } as unknown as Response
    globalThis.fetch = vi.fn(async () => broken) as unknown as typeof fetch
    const api = createApi({ base: '/b' })
    await expect(api('GET', 'x')).rejects.toMatchObject({ status: 500, reason: 'error' })
    await expect(api.upload('up', new File(['a'], 'a.txt'))).rejects.toMatchObject({ status: 500, reason: 'error' })
    const okBroken = { ok: true, status: 200, json: async () => { throw new SyntaxError('bad') } } as unknown as Response
    globalThis.fetch = vi.fn(async () => okBroken) as unknown as typeof fetch
    expect(await api('GET', 'x')).toEqual({})
  })
  it('an all-empty query adds nothing', async () => {
    const fetchMock = mockFetch(200, {})
    const api = createApi({ base: '/b' })
    await api('GET', 'x', undefined, { query: { a: undefined, b: '' } })
    expect((fetchMock.mock.calls[0] as unknown as [string])[0]).toBe('/b/x')
  })
})
