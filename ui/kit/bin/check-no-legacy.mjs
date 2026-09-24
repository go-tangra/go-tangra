#!/usr/bin/env node
// Legacy-UI guard shipped with @go-tangra/ui (bin: go-tangra-ui-check-no-legacy).
// Fails on Vuetify / @mdi/font imports, `:rules` bindings, <v-*> components or
// inline validator arrays outside `src/schemas/`, and on style attributes (CSP).
//
// Single front-end (default; what a front-end's `npm run lint` calls):
//   go-tangra-ui-check-no-legacy [--dir DIR]        # DIR defaults to the cwd
//
// Monorepo (every front-end marked `migrated` in <root>/ui/MIGRATION.md):
//   go-tangra-ui-check-no-legacy --root DIR [--frontends a,b] [--status JSON]
//
// A status of `migrated (…transitional vuetify…)` (the host while legacy remotes
// remain) keeps its Vuetify dependency and `src/legacy/**` out of the check.
import { join, relative, resolve } from 'node:path'
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'

const args = process.argv.slice(2)
const opt = (name) => (args.includes(name) ? args[args.indexOf(name) + 1] : undefined)

/** Recursively lists files under dir (relative paths), skipping node_modules/dist/coverage. */
function walk(dir, exts, out = [], base = dir) {
  if (!existsSync(dir)) return out
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist' || name === 'coverage') continue
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, exts, out, base)
    else if (exts.some((e) => name.endsWith(e))) out.push(relative(base, p))
  }
  return out
}

/** Parses <root>/ui/MIGRATION.md into { path: status }. */
function migrationStatus(rootDir) {
  const md = readFileSync(join(rootDir, 'ui/MIGRATION.md'), 'utf8')
  const out = {}
  for (const line of md.split('\n')) {
    // Status is the whole cell ("legacy", "migrated", "migrated (host, transitional vuetify)").
    const m = line.match(/^\|\s*[^|]+\|\s*(services\/[^|\s]+)\s*\|\s*((?:legacy|migrated)[^|]*?)\s*\|/)
    if (m) out[m[1]] = m[2]
  }
  return out
}

let rootDir
let status
const rootArg = opt('--root')
if (rootArg !== undefined) {
  rootDir = resolve(rootArg)
  status = opt('--status') !== undefined ? JSON.parse(opt('--status')) : migrationStatus(rootDir)
} else {
  rootDir = resolve(opt('--dir') ?? process.cwd())
  status = { '.': 'migrated' }
}
const only = opt('--frontends')?.split(',') ?? null

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
  const label = fe === '.' ? '' : `${fe}/`
  for (const f of walk(join(rootDir, fe, 'src'), ['.vue', '.ts'])) {
    if (transitional && f.startsWith('legacy/')) continue
    if (f.endsWith('.d.ts')) continue
    const text = readFileSync(join(rootDir, fe, 'src', f), 'utf8')
    const rules = f.startsWith('schemas/') ? RULES : [...RULES, ...OUTSIDE_SCHEMAS]
    text.split('\n').forEach((line, i) => {
      for (const r of rules) {
        if (r.re.test(line)) {
          failed++
          console.error(`${label}src/${f}:${i + 1}: ${r.why}`)
        }
      }
    })
  }
  if (transitional) continue
  const pkgPath = join(rootDir, fe, 'package.json')
  if (!existsSync(pkgPath)) continue
  const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'))
  for (const dep of ['vuetify', 'vite-plugin-vuetify', '@mdi/font']) {
    if (pkg.dependencies?.[dep] || pkg.devDependencies?.[dep]) {
      failed++
      console.error(`${label}package.json: legacy dependency ${dep}`)
    }
  }
}
if (failed) {
  console.error(`check-no-legacy: ${failed} finding(s)`)
  process.exit(1)
}
console.log('check-no-legacy: ok')
