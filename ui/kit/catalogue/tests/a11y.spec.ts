import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

// T070 / SC-006: every catalogue page, both themes, zero serious or critical
// axe findings (WCAG 2.x A/AA), plus reduced-motion and 200 % zoom spot checks.
const pages = ['primitives', 'overlays', 'fields', 'data', 'forms', 'composite', 'shell']

for (const page of pages) {
  for (const theme of ['freya-light', 'freya-dark']) {
    test(`${page} in ${theme} has no serious/critical axe findings`, async ({ page: p }) => {
      await p.addInitScript((t) => localStorage.setItem('freya.theme', t), theme)
      await p.goto('/#/' + page)
      await expect(p.locator('main h1')).toBeVisible()
      const results = await new AxeBuilder({ page: p }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
      const blocking = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
      expect(blocking, JSON.stringify(blocking.map((v) => ({ id: v.id, help: v.help, nodes: v.nodes.map((n) => n.target) })), null, 1)).toEqual([])
    })
  }
}

test('reduced motion disables transitions and 200% zoom keeps the page free of horizontal scroll', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/#/overlays')
  await page.getByTestId('open-dialog').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  // theme.css collapses every animation/transition to 0.01ms under prefers-reduced-motion.
  const durations = await dialog.evaluate((el) => [getComputedStyle(el).transitionDuration, getComputedStyle(el).animationDuration])
  for (const d of durations) for (const part of d.split(',')) expect(parseFloat(part), d).toBeLessThanOrEqual(0.01)
  await page.keyboard.press('Escape')
  // 200 % zoom ≈ half the CSS viewport at the same device width; WCAG 1.4.10
  // reflow is required down to 320 CSS px, which the phone project already is.
  const vp = page.viewportSize()!
  await page.setViewportSize({ width: Math.max(320, Math.floor(vp.width / 2)), height: Math.floor(vp.height / 2) })
  for (const p of ['forms', 'data']) {
    await page.goto('/#/' + p)
    await expect(page.locator('main h1')).toBeVisible()
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow, `${p}: no horizontal page scroll at 200% zoom`).toBeLessThanOrEqual(0)
  }
})
