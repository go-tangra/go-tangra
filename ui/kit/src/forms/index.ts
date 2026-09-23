import { z } from 'zod'

// The edge CSP has no 'unsafe-eval': keep Zod on its interpreter. Without this
// Zod probes `new Function` once per page, which the browser reports as a
// securitypolicyviolation even though Zod swallows the throw.
z.config({ jitless: true })

export { useZodForm } from './useZodForm'
export type { UseZodFormOptions, ZodForm, FieldBinding } from './useZodForm'
export { messages, describeReason, describeIssue, registerReasons, API_REASONS } from './messages'
export type { ApiReason } from './messages'
export { zodToFields } from './zodToFields'
export type { FieldDef, FieldOption, FieldType } from './zodToFields'
export * from './schemas'
