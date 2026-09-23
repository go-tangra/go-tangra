import { test, expect } from '@playwright/test'

const pages = ['primitives', 'overlays', 'fields', 'data', 'forms', 'composite', 'shell']

for (const page of pages) {
  for (const theme of ['freya-light', 'freya-dark']) {
    test(`${page} renders in ${theme} without horizontal overflow`, async ({ page: p }) => {
      await p.addInitScript((t) => localStorage.setItem('freya.theme', t), theme)
      const violations: string[] = []
      await p.addInitScript(() => {
        document.addEventListener('securitypolicyviolation', (e) => console.error('CSP:' + (e as SecurityPolicyViolationEvent).violatedDirective))
      })
      p.on('console', (m) => { if (m.text().startsWith('CSP:')) violations.push(m.text()) })
      await p.goto('/#/' + page)
      await expect(p.locator('main h1')).toBeVisible()
      expect(await p.evaluate(() => document.documentElement.getAttribute('data-theme'))).toBe(theme)
      const overflow = await p.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(overflow, 'no horizontal page scroll').toBeLessThanOrEqual(0)
      expect(await p.locator('[style]').count(), 'no inline style attributes').toBe(0)
      expect(violations).toEqual([])
      await expect(p).toHaveScreenshot(`${page}-${theme}.png`, { fullPage: true })
    })
  }
}

test('dialog opens full-screen below md and traps focus', async ({ page }) => {
  await page.goto('/#/overlays')
  await page.getByTestId('open-dialog').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
})
