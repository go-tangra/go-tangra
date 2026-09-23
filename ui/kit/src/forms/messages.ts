// The shared vocabulary: what a refused request and what an invalid field say
// to the user. One place for the whole platform; never echoes server detail.
import type { z } from 'zod'

export const API_REASONS = [
  'unauthenticated', 'forbidden', 'not_found', 'conflict', 'validation_failed', 'malformed_body',
  'body_too_large', 'unsupported_media_type', 'rate_limited', 'temporarily_unavailable',
  'not_implemented', 'invalid_credentials', 'locked', 'csrf', 'network', 'internal', 'error',
] as const
export type ApiReason = (typeof API_REASONS)[number]

const reason: Record<ApiReason | 'generic', string> = {
  unauthenticated: 'Your session has ended. Sign in again.',
  forbidden: 'You are not allowed to do that.',
  not_found: 'Not found.',
  conflict: 'That operation is not possible in the current state.',
  validation_failed: 'Please check the highlighted fields.',
  malformed_body: 'The request could not be read.',
  body_too_large: 'The upload is too large.',
  unsupported_media_type: 'That file type is not supported.',
  rate_limited: 'Too many requests; try again shortly.',
  temporarily_unavailable: 'The service is temporarily unavailable.',
  not_implemented: 'This feature is not available yet.',
  invalid_credentials: 'Those credentials were not accepted.',
  locked: 'This account is temporarily locked.',
  csrf: 'The page is out of date. Reload and try again.',
  network: 'The service could not be reached.',
  internal: 'Something went wrong.',
  error: 'Something went wrong.',
  generic: 'Something went wrong.',
}

export const messages = {
  reason,
  required: 'This field is required.',
  invalid: 'This value is not valid.',
  tooShort: (n: number) => `Must be at least ${n} characters.`,
  tooLong: (n: number) => `Must be at most ${n} characters.`,
  tooSmall: (n: number) => `Must be at least ${n}.`,
  tooBig: (n: number) => `Must be at most ${n}.`,
  tooFew: (n: number) => `Choose at least ${n}.`,
  tooMany: (n: number) => `Choose at most ${n}.`,
  format: {
    email: 'Enter a valid e-mail address.',
    uuid: 'Enter a valid identifier.',
    cidrv4: 'Enter a valid network in CIDR notation (e.g. 10.0.0.0/24).',
    cidrv6: 'Enter a valid IPv6 network in CIDR notation.',
    ipv4: 'Enter a valid IPv4 address.',
    ipv6: 'Enter a valid IPv6 address.',
    url: 'Enter a valid URL.',
    regex: 'This value has the wrong format.',
    date: 'Enter a valid date.',
    datetime: 'Enter a valid date and time.',
  } as Record<string, string>,
  notMultiple: (n: number) => `Must be a multiple of ${n}.`,
  invalidValue: 'Choose one of the listed values.',
  unrecognized: 'Unexpected field.',
}

const knownReasons: ReadonlySet<string> = new Set(API_REASONS)
// Module-specific reasons (e.g. warden's vault_unavailable) registered at module load.
const extraReasons = new Map<string, string>()

/**
 * Lets a module add wording for its own closed-vocabulary reasons. Only the
 * reason key is looked up; server detail is still never echoed.
 */
export function registerReasons(map: Record<string, string>): void {
  for (const [k, v] of Object.entries(map)) extraReasons.set(k, v)
}

/** Text for an API reason; unknown reasons never reach the user verbatim. */
export function describeReason(r: string): string {
  const extra = extraReasons.get(r)
  if (extra) return extra
  if (knownReasons.has(r)) return reason[r as ApiReason]
  if (typeof console !== 'undefined') console.warn('[freya/ui] unknown api reason', r)
  return reason.generic
}

/** Text for a Zod issue (zod 4 issue codes). */
// Zod's own English defaults all start with one of these; anything else was
// written by the schema author and is shown verbatim.
const ZOD_DEFAULT = /^(Invalid|Too (small|big)|Unrecognized key|Expected |Input not instance|Number must|String must|Array must)/

export function describeIssue(issue: z.core.$ZodIssue): string {
  const i = issue as z.core.$ZodIssue & { expected?: string; input?: unknown; minimum?: number | bigint; maximum?: number | bigint; origin?: string; format?: string; divisor?: number; values?: unknown[] }
  if (i.message && i.code !== 'custom' && !ZOD_DEFAULT.test(i.message)) return i.message
  switch (i.code) {
    case 'invalid_type':
      return i.input === undefined || i.input === null || i.input === '' ? messages.required : messages.invalid
    case 'too_small': {
      const n = Number(i.minimum)
      if (i.origin === 'string') return n <= 1 ? messages.required : messages.tooShort(n)
      if (i.origin === 'array' || i.origin === 'set') return messages.tooFew(n)
      return messages.tooSmall(n)
    }
    case 'too_big': {
      const n = Number(i.maximum)
      if (i.origin === 'string') return messages.tooLong(n)
      if (i.origin === 'array' || i.origin === 'set') return messages.tooMany(n)
      return messages.tooBig(n)
    }
    case 'invalid_format':
      return messages.format[i.format ?? ''] ?? messages.invalid
    case 'not_multiple_of':
      return messages.notMultiple(Number(i.divisor))
    case 'invalid_value':
      return messages.invalidValue
    case 'unrecognized_keys':
      return messages.unrecognized
    case 'custom':
      return i.message || messages.invalid
    case 'invalid_union':
    case 'invalid_key':
    case 'invalid_element':
      return messages.invalid
    default:
      return messages.invalid
  }
}
