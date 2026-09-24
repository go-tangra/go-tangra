// T065: the three static checks refuse what they must and accept what they must.
// Fixtures are synthetic trees under a temp dir; run with `node --test ui/scripts/tests`.
// check-no-legacy ships in the kit (ui/kit/bin, bin name go-tangra-ui-check-no-legacy).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { join, dirname } from 'node:path'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'

const scripts = join(dirname(fileURLToPath(import.meta.url)), '..')
const kitBin = join(scripts, '..', 'kit', 'bin')
const run = (script, args) => spawnSync(process.execPath, [join(script === 'check-no-legacy.mjs' ? kitBin : scripts, script), ...args], { encoding: 'utf8' })

function tree(files) {
  const root = mkdtempSync(join(tmpdir(), 'freya-checks-'))
  for (const [p, body] of Object.entries(files)) {
    mkdirSync(join(root, dirname(p)), { recursive: true })
    writeFileSync(join(root, p), body)
  }
  return root
}
const migration = (rows) => `# status\n\n| Front-end | Path | Status |\n|---|---|---|\n${rows.map(([p, s]) => `| x | ${p} | ${s} |`).join('\n')}\n`
const pkg = (deps = {}) => JSON.stringify({ name: 'x', dependencies: deps })

test('check-duplicates: kit/module and module/module basename clashes fail; distinct names pass', () => {
  const clash = tree({
    'ui/kit/src/components/UiStatsCard.vue': '<template><div/></template>',
    'services/a/ui/src/components/StatsCard.vue': '<template><div/></template>',
    'services/b/ui/src/components/Tree.vue': '<template><div/></template>',
    'services/c/ui/src/components/Tree.vue': '<template><div/></template>',
  })
  const r = run('check-duplicates.mjs', ['--root', clash, '--frontends', 'services/a/ui,services/b/ui,services/c/ui'])
  assert.equal(r.status, 1)
  assert.match(r.stderr, /UiStatsCard\.vue.*StatsCard\.vue/)
  assert.match(r.stderr, /services\/b\/ui.*Tree\.vue.*services\/c\/ui.*Tree\.vue/)
  assert.match(r.stderr, /2 duplicated/)
  const ok = tree({ 'ui/kit/src/components/UiTree.vue': '', 'services/a/ui/src/components/OnlyHere.vue': '' })
  assert.equal(run('check-duplicates.mjs', ['--root', ok, '--frontends', 'services/a/ui']).status, 0)
  rmSync(clash, { recursive: true, force: true })
  rmSync(ok, { recursive: true, force: true })
})

test('check-no-legacy: vuetify / @mdi/font / :rules= / style= fail in a migrated front-end and are ignored in a legacy one', () => {
  const src = {
    'src/views/A.vue': `<script setup>\nimport { createVuetify } from 'vuetify'\nimport '@mdi/font/css/materialdesignicons.css'\n</script>\n<template><v-btn :rules="[r]" style="color:red" /></template>`,
    'src/views/B.vue': `<script setup>\nconst rules = [(v) => v.length < 3 || 'short']\n</script>`,
    'src/schemas/ok.ts': `export const rules = [1]\n`,
  }
  const files = { 'ui/MIGRATION.md': migration([['services/m/ui', 'migrated'], ['services/l/ui', 'legacy']]) }
  for (const fe of ['services/m/ui', 'services/l/ui']) {
    files[`${fe}/package.json`] = pkg({ vuetify: '^3' })
    for (const [p, b] of Object.entries(src)) files[`${fe}/${p}`] = b
  }
  const root = tree(files)
  const r = run('check-no-legacy.mjs', ['--root', root])
  assert.equal(r.status, 1)
  assert.match(r.stderr, /services\/m\/ui\/src\/views\/A\.vue:2: Vuetify\/@mdi\/font import/)
  assert.match(r.stderr, /A\.vue:3: Vuetify\/@mdi\/font import/)
  assert.match(r.stderr, /A\.vue:5: Vuetify component/)
  assert.match(r.stderr, /A\.vue:5: hand-written :rules/)
  assert.match(r.stderr, /A\.vue:5: inline style/)
  assert.match(r.stderr, /B\.vue:2: inline validator/)
  assert.match(r.stderr, /services\/m\/ui\/package\.json: legacy dependency vuetify/)
  assert.doesNotMatch(r.stderr, /services\/l\/ui/)
  assert.doesNotMatch(r.stderr, /schemas\/ok\.ts/)
  // A transitional host may keep src/legacy/** and the dependency until the last remote lands.
  const host = tree({
    'ui/MIGRATION.md': migration([['services/h/shell', 'migrated (host, transitional vuetify)']]),
    'services/h/shell/package.json': pkg({ vuetify: '^3' }),
    'services/h/shell/src/legacy/vuetify.ts': `import { createVuetify } from 'vuetify'\n`,
    'services/h/shell/src/views/X.vue': '<template><div/></template>',
  })
  assert.equal(run('check-no-legacy.mjs', ['--root', host]).status, 0)
  const clean = tree({ 'ui/MIGRATION.md': migration([['services/m/ui', 'migrated']]), 'services/m/ui/package.json': pkg(), 'services/m/ui/src/views/A.vue': '<template><UiButton /></template>' })
  assert.equal(run('check-no-legacy.mjs', ['--root', clean]).status, 0)
  for (const d of [root, host, clean]) rmSync(d, { recursive: true, force: true })
})

test('check-no-legacy --dir: checks one front-end without ui/MIGRATION.md (standalone repo)', () => {
  const bad = tree({ 'package.json': pkg({ vuetify: '^3' }), 'src/views/A.vue': `<template><v-btn style="x" /></template>` })
  const r = run('check-no-legacy.mjs', ['--dir', bad])
  assert.equal(r.status, 1)
  assert.match(r.stderr, /^src\/views\/A\.vue:1: Vuetify component/m)
  assert.match(r.stderr, /^package\.json: legacy dependency vuetify/m)
  const ok = tree({ 'package.json': pkg(), 'src/views/A.vue': '<template><UiButton /></template>' })
  assert.equal(run('check-no-legacy.mjs', ['--dir', ok]).status, 0)
  const cwd = spawnSync(process.execPath, [join(kitBin, 'check-no-legacy.mjs')], { cwd: ok, encoding: 'utf8' })
  assert.equal(cwd.status, 0, cwd.stderr)
  for (const d of [bad, ok]) rmSync(d, { recursive: true, force: true })
})

test('check-bundle-size: fails above 75% of the recorded baseline, passes below', () => {
  const big = 'x'.repeat(200_000) + Math.random().toString(36).repeat(50_000) // ~gz 100k+
  const md = (baseline) => `# b\n\n| Build | Bytes |\n|---|---|\n| shell + asset | ${baseline} |\n`
  const files = { 'services/gateway/shell/dist/a.js': big, 'services/asset/ui/dist/b.css': 'body{}' }
  const fail = tree({ ...files, 'ui/MIGRATION.md': md(1000) })
  const r = run('check-bundle-size.mjs', ['--root', fail])
  assert.equal(r.status, 1)
  assert.match(r.stderr, /> limit 750 \(75% of baseline 1000\)/)
  const pass = tree({ ...files, 'ui/MIGRATION.md': md(100_000_000) })
  assert.equal(run('check-bundle-size.mjs', ['--root', pass]).status, 0)
  const missing = tree({ 'ui/MIGRATION.md': md(1) })
  assert.notEqual(run('check-bundle-size.mjs', ['--root', missing]).status, 0)
  for (const d of [fail, pass, missing]) rmSync(d, { recursive: true, force: true })
})
