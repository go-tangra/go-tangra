// Vite build plugin for module remotes: makes responsive utilities win
// regardless of stylesheet order.
//
// The shell loads every module remote's stylesheet into one page, and each
// carries its own copy of the Tailwind utilities (unlayered). A responsive rule
// such as `md:grid-cols-12` is a plain class inside a media query, so a later
// module's `.grid-cols-2` overrides it and the layout then depends on which
// remote happened to load last. Within a single Tailwind sheet the variant wins
// only because it comes later. This plugin makes that ordering explicit: rules
// under a min-width breakpoint get extra class-level specificity, ranked by the
// breakpoint (sm +1 … 2xl +5), so larger breakpoints beat smaller ones and every
// breakpoint beats the plain utility, whatever file declared them.
//
// `:not(._)` matches every element (nothing uses the class "_") and adds one
// class of specificity; it is inserted before any pseudo-element.
import postcss from 'postcss'

// Tailwind's default breakpoints in rem (sm, md, lg, xl, 2xl).
const BREAKPOINTS = [40, 48, 64, 80, 96]

/** Specificity rank for a media query, or 0 when it is not a min-width query. */
export function rankOf(params) {
  const m = /\(\s*(?:width\s*>=\s*|min-width\s*:\s*)([\d.]+)(rem|px|em)\s*\)/.exec(params)
  if (!m) return 0
  const rem = m[2] === 'px' ? Number(m[1]) / 16 : Number(m[1])
  let rank = 0
  for (const bp of BREAKPOINTS) if (rem >= bp) rank++
  return Math.max(rank, 1)
}

/** Adds `rank` classes of specificity to one selector. */
export function boost(selector, rank) {
  const extra = ':not(._)'.repeat(rank)
  const i = selector.indexOf('::')
  return i === -1 ? selector + extra : selector.slice(0, i) + extra + selector.slice(i)
}

/** Rewrites a stylesheet's min-width rules; other rules are left untouched. */
export function boostBreakpoints(css) {
  const root = postcss.parse(css)
  root.walkRules((rule) => {
    // Keyframe steps are not selectors.
    if (rule.parent?.type === 'atrule' && /keyframes$/.test(rule.parent.name)) return
    let rank = 0
    for (let p = rule.parent; p; p = p.parent) {
      if (p.type === 'atrule' && p.name === 'media') rank = Math.max(rank, rankOf(p.params))
    }
    if (rank > 0) rule.selectors = rule.selectors.map((s) => boost(s, rank))
  })
  return root.toString()
}

/** @returns {import('vite').Plugin} */
export function breakpointSpecificity() {
  return {
    name: 'freya-breakpoint-specificity',
    apply: 'build',
    enforce: 'post',
    generateBundle(_options, bundle) {
      for (const asset of Object.values(bundle)) {
        if (asset.type !== 'asset' || !asset.fileName.endsWith('.css')) continue
        const css = typeof asset.source === 'string' ? asset.source : new TextDecoder().decode(asset.source)
        asset.source = boostBreakpoints(css)
      }
    },
  }
}
