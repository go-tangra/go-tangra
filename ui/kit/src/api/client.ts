// The one fetch client every Tangra front-end uses (previously copied into each
// module). Same-origin calls through the gateway with the double-submit CSRF
// header on mutations, a closed reason vocabulary on refusals, multipart
// uploads and binary-route URL building.
import { messages, describeReason } from '@/forms/messages'

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

export const CSRF_COOKIE = '__Host-csrf'
export const CSRF_HEADER = 'X-CSRF-Token'
export const REQUEST_ID_HEADER = 'X-Request-Id'

/** A refused request: the closed-vocabulary reason plus the server's optional detail object. */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly reason: string,
    public readonly detail?: Record<string, unknown>,
  ) {
    super(reason)
    this.name = 'ApiError'
  }
}

/** Reads the double-submit CSRF cookie issued by the gateway edge. */
export function csrfToken(): string {
  const prefix = CSRF_COOKIE + '='
  const hit = document.cookie.split('; ').find((c) => c.startsWith(prefix))
  return hit ? decodeURIComponent(hit.slice(prefix.length)) : ''
}

export interface RequestOptions {
  signal?: AbortSignal
  query?: Record<string, string | number | boolean | undefined>
}

export interface ApiConfig {
  /** Module API base, e.g. "/api/asset/v1". Paths starting with "/" bypass it. */
  base: string
  /** Optional correlation id provider; sent as X-Request-Id. */
  requestId?: () => string
}

export interface Api {
  <T = unknown>(method: Method, path: string, body?: unknown, opts?: RequestOptions): Promise<T>
  upload<T = unknown>(path: string, file: File, fields?: Record<string, string>): Promise<T>
  fileUrl(path: string): string
  readonly base: string
}

function reasonOf(data: unknown): { reason: string; detail?: Record<string, unknown> } {
  if (typeof data === 'object' && data !== null && 'reason' in data) {
    const d = data as { reason: unknown; detail?: unknown }
    const detail = typeof d.detail === 'object' && d.detail !== null ? (d.detail as Record<string, unknown>) : undefined
    const flat = Object.fromEntries(Object.entries(data).filter(([key]) => key !== 'reason' && key !== 'detail'))
    const extra = Object.keys(flat).length ? flat : undefined
    const resolved = detail ?? extra
    return resolved ? { reason: String(d.reason), detail: resolved } : { reason: String(d.reason) }
  }
  return { reason: 'error' }
}

/** Builds a client bound to a module base path. */
export function createApi(cfg: ApiConfig): Api {
  const url = (path: string) => (path.startsWith('/') ? path : cfg.base + '/' + path)
  const api = (async <T,>(method: Method, path: string, body?: unknown, opts: RequestOptions = {}): Promise<T> => {
    const headers: Record<string, string> = { Accept: 'application/json' }
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    if (method !== 'GET') headers[CSRF_HEADER] = csrfToken()
    if (cfg.requestId) headers[REQUEST_ID_HEADER] = cfg.requestId()
    let target = url(path)
    if (opts.query) {
      const q = new URLSearchParams()
      for (const [k, v] of Object.entries(opts.query)) if (v !== undefined && v !== '') q.set(k, String(v))
      const s = q.toString()
      if (s) target += (target.includes('?') ? '&' : '?') + s
    }
    let res: Response
    try {
      res = await fetch(target, {
        method,
        headers,
        credentials: 'same-origin',
        body: body === undefined ? null : JSON.stringify(body),
        ...(opts.signal ? { signal: opts.signal } : {}),
      })
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') throw err
      throw new ApiError(0, 'network')
    }
    if (res.status === 204) return undefined as T
    const data: unknown = await res.json().catch(() => ({}))
    if (!res.ok) {
      const { reason, detail } = reasonOf(data)
      throw new ApiError(res.status, reason, detail)
    }
    return data as T
  }) as Api
  Object.defineProperty(api, 'base', { value: cfg.base })
  api.upload = async <T,>(path: string, file: File, fields: Record<string, string> = {}): Promise<T> => {
    const form = new FormData()
    form.append('file', file, file.name)
    for (const [k, v] of Object.entries(fields)) form.append(k, v)
    const headers: Record<string, string> = { Accept: 'application/json', [CSRF_HEADER]: csrfToken() }
    if (cfg.requestId) headers[REQUEST_ID_HEADER] = cfg.requestId()
    let res: Response
    try {
      res = await fetch(url(path), { method: 'POST', headers, credentials: 'same-origin', body: form })
    } catch {
      throw new ApiError(0, 'network')
    }
    const data: unknown = await res.json().catch(() => ({}))
    if (!res.ok) {
      const { reason, detail } = reasonOf(data)
      throw new ApiError(res.status, reason, detail)
    }
    return data as T
  }
  api.fileUrl = url
  return api
}

/** Human wording for an error (closed reason vocabulary; generic otherwise). */
export function describe(err: unknown): string {
  if (!(err instanceof ApiError)) return messages.reason.generic
  return describeReason(err.reason)
}
