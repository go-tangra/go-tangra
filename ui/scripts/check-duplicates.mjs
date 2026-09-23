#!/usr/bin/env node
// Fails when a component exists in more than one place: in the kit AND a
// front-end, or in two or more front-ends. Components are compared by basename
// (case-insensitive, "Ui" prefix ignored) so `StatsCard.vue` in a module clashes
// with `UiStatsCard.vue` in the kit.
import { join } from 'node:path'
import { FRONTENDS, ROOT, walk } from './lib.mjs'

const args = process.argv.slice(2)
const rootDir = args.includes('--root') ? args[args.indexOf('--root') + 1] : ROOT
const frontends = args.includes('--frontends') ? args[args.indexOf('--frontends') + 1].split(',') : FRONTENDS

const norm = (f) => f.replace(/^.*\//, '').replace(/\.vue$/, '').replace(/^Ui/, '').toLowerCase()
const owners = new Map() // norm → [{ path }]
const add = (owner, files) => {
  for (const f of files) {
    const k = norm(f)
    if (!owners.has(k)) owners.set(k, [])
    owners.get(k).push(join(owner, f))
  }
}
add('ui/kit/src/components', walk(join(rootDir, 'ui/kit/src/components'), ['.vue']))
for (const fe of frontends) add(join(fe, 'src/components'), walk(join(rootDir, fe, 'src/components'), ['.vue']))

let failed = 0
for (const [, paths] of owners) {
  if (paths.length > 1) {
    failed++
    console.error(`duplicate component: ${paths.join('  <->  ')}\n  keep the kit copy (ui/kit/src/components) and import it from @freya/ui`)
  }
}
if (failed) {
  console.error(`check-duplicates: ${failed} duplicated component(s)`)
  process.exit(1)
}
console.log('check-duplicates: ok')
