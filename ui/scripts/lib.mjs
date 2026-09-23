import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

export const ROOT = fileURLToPath(new URL('../../', import.meta.url))

export const FRONTENDS = [
  'services/gateway/shell',
  'services/auth/console',
  'services/asset/ui',
  'services/inventory/ui',
  'services/ipam/ui',
  'services/paperless/ui',
  'services/deployer/ui',
  'services/lcm/ui',
  'services/notification/ui',
  'services/warden/ui',
]

/** Recursively lists files under dir (relative paths), skipping node_modules/dist. */
export function walk(dir, exts, out = [], base = dir) {
  if (!existsSync(dir)) return out
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'dist' || name === 'coverage') continue
    const p = join(dir, name)
    if (statSync(p).isDirectory()) walk(p, exts, out, base)
    else if (exts.some((e) => name.endsWith(e))) out.push(relative(base, p))
  }
  return out
}

/** Parses ui/MIGRATION.md into { path: status }. */
export function migrationStatus(rootDir = ROOT) {
  const md = readFileSync(join(rootDir, 'ui/MIGRATION.md'), 'utf8')
  const out = {}
  for (const line of md.split('\n')) {
    // Status is the whole cell ("legacy", "migrated", "migrated (host, transitional vuetify)").
    const m = line.match(/^\|\s*[^|]+\|\s*(services\/[^|\s]+)\s*\|\s*((?:legacy|migrated)[^|]*?)\s*\|/)
    if (m) out[m[1]] = m[2]
  }
  return out
}

export function read(p) {
  return readFileSync(join(ROOT, p), 'utf8')
}
