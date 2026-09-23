#!/usr/bin/env node
// For every front-end marked `migrated` in ui/MIGRATION.md: no Vuetify / @mdi/font
// imports, no `:rules` bindings, <v-form> or inline validator arrays outside
// `src/schemas/`, and no style attributes (CSP).
//
//   node ui/scripts/check-no-legacy.mjs [--frontends a,b] [--root DIR] [--status JSON]
//
// A status of `migrated (…transitional vuetify…)` (the host while legacy remotes
// remain) keeps its Vuetify dependency and `src/legacy/**` out of the check.
import { join } from 'node:path'
import { readFileSync } from 'node:fs'
import { ROOT, walk, migrationStatus } from './lib.mjs'

const args = process.argv.slice(2)
const rootDir = args.includes('--root') ? args[args.indexOf('--root') + 1] : ROOT
const status = args.includes('--status') ? JSON.parse(args[args.indexOf('--status') + 1]) : migrationStatus(rootDir)
const only = args.includes('--frontends') ? args[args.indexOf('--frontends') + 1].split(',') : null

const RULES = [
  { re: /from\s+['"](vuetify|vite-plugin-vuetify|@mdi\/font)(\/[^'"]*)?['"]/, why: 'Vuetify/@mdi/font import' },
  { re: /import\s+['"](vuetify|@mdi\/font)(\/[^'"]*)?['"]/, why: 'Vuetify/@mdi/font import' },
  { re: /<v-[a-z]/, why: 'Vuetify component' },
  { re: /(^|\s):rules=/, why: 'hand-written :rules validation (use a Zod schema with useZodForm)' },
  { re: /(^|\s)(:style|v-bind:style|style)="/, why: 'inline style attribute (blocked by CSP)' },
]
// Validation logic belongs in src/schemas/*.ts (Zod); anywhere else it is a legacy inline validator.
const OUTSIDE_SCHEMAS = [
  { re: /\b(rules|validators?)\s*[:=]\s*\[/, why: 'inline validator array (move the rule into a Zod schema under src/schemas/)' },
  { re: /\bv\.\s*length\s*(<=|>=|<|>)\s*\d+\s*\|\|\s*['"`]/, why: 'inline validator (move the rule into a Zod schema under src/schemas/)' },
]

let failed = 0
for (const [fe, st] of Object.entries(status)) {
  if (!st.startsWith('migrated')) continue
  if (only && !only.includes(fe)) continue
  const transitional = /transitional/.test(st)
  for (const f of walk(join(rootDir, fe, 'src'), ['.vue', '.ts'])) {
    if (transitional && f.startsWith('legacy/')) continue
    if (f.endsWith('.d.ts')) continue
    const text = readFileSync(join(rootDir, fe, 'src', f), 'utf8')
    const rules = f.startsWith('schemas/') ? RULES : [...RULES, ...OUTSIDE_SCHEMAS]
    text.split('\n').forEach((line, i) => {
      for (const r of rules) {
        if (r.re.test(line)) {
          failed++
          console.error(`${fe}/src/${f}:${i + 1}: ${r.why}`)
        }
      }
    })
  }
  if (transitional) continue
  const pkg = JSON.parse(readFileSync(join(rootDir, fe, 'package.json'), 'utf8'))
  for (const dep of ['vuetify', 'vite-plugin-vuetify', '@mdi/font']) {
    if (pkg.dependencies?.[dep] || pkg.devDependencies?.[dep]) {
      failed++
      console.error(`${fe}/package.json: legacy dependency ${dep}`)
    }
  }
}
if (failed) {
  console.error(`check-no-legacy: ${failed} finding(s)`)
  process.exit(1)
}
console.log('check-no-legacy: ok')
