#!/usr/bin/env node
// Gzipped JS+CSS of the shell and asset builds must be ≤ 75% of the pre-migration
// baseline recorded in ui/MIGRATION.md (SC-007). `--record` prints current sizes.
import { join } from 'node:path'
import { readFileSync, existsSync } from 'node:fs'
import { gzipSync } from 'node:zlib'
import { ROOT, walk } from './lib.mjs'

const args = process.argv.slice(2)
const rootDir = args.includes('--root') ? args[args.indexOf('--root') + 1] : ROOT
const ratio = args.includes('--ratio') ? Number(args[args.indexOf('--ratio') + 1]) : 0.75
const BUILDS = ['services/gateway/shell', 'services/asset/ui']

function baseline() {
  const md = readFileSync(join(rootDir, 'ui/MIGRATION.md'), 'utf8')
  const m = md.match(/\|\s*shell \+ asset\s*\|\s*(\d+)\s*\|/)
  if (!m) throw new Error('baseline "shell + asset" row missing in ui/MIGRATION.md')
  return Number(m[1])
}

function size(build) {
  const dist = join(rootDir, build, 'dist')
  if (!existsSync(dist)) throw new Error(`${build}/dist missing — build it first`)
  let total = 0
  for (const f of walk(dist, ['.js', '.css'], [], dist)) total += gzipSync(readFileSync(join(dist, f)), { level: 9 }).length
  return total
}

let sum = 0
for (const b of BUILDS) {
  const s = size(b)
  sum += s
  console.log(`${b}: ${s} bytes gz`)
}
console.log(`shell + asset: ${sum} bytes gz`)
if (args.includes('--record')) process.exit(0)
const limit = Math.floor(baseline() * ratio)
if (sum > limit) {
  console.error(`check-bundle-size: ${sum} > limit ${limit} (${ratio * 100}% of baseline ${baseline()})`)
  process.exit(1)
}
console.log(`check-bundle-size: ok (limit ${limit})`)
