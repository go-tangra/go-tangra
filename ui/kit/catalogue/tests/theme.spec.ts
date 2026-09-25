import { test, expect, type Page } from '@playwright/test'

// The theme the user picked must win over the operating system's colour
// scheme: a light choice on a dark OS (and the reverse) used to render the OS
// theme because the prefers-dark :root rule outranked [data-theme].
const bg = (p: Page) => p.evaluate(() => getComputedStyle(document.body).backgroundColor)
const scheme = (p: Page) => p.evaluate(() => getComputedStyle(document.documentElement).colorScheme)
// base-200 lightness: freya-light 97 %, freya-dark 19 %.
const lightness = (c: string) => Number(/oklch\(([\d.]+)/.exec(c)?.[1] ?? NaN)
const isLight = (c: string) => lightness(c) > 0.5 || lightness(c) > 50

for (const os of ['light', 'dark'] as const) {
  for (const theme of ['freya-light', 'freya-dark']) {
    test(`${theme} renders as chosen on a ${os} OS`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: os })
      await page.addInitScript((t) => localStorage.setItem('freya.theme', t), theme)
      await page.goto('/#/primitives')
      await expect(page.locator('main h1')).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.getAttribute('data-theme'))).toBe(theme)
      const want = theme === 'freya-light'
      expect(isLight(await bg(page)), `body background ${await bg(page)}`).toBe(want)
      expect(await scheme(page)).toBe(want ? 'light' : 'dark')
    })
  }

  test(`the toggle switches the rendered theme on a ${os} OS`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: os })
    await page.goto('/#/primitives')
    await expect(page.locator('main h1')).toBeVisible()
    const before = isLight(await bg(page))
    expect(before, 'first visit follows the OS').toBe(os === 'light')
    await page.getByTestId('theme-toggle').click()
    await expect.poll(async () => isLight(await bg(page))).toBe(!before)
    await page.getByTestId('theme-toggle').click()
    await expect.poll(async () => isLight(await bg(page))).toBe(before)
  })
}

test('without a data-theme the OS preference still picks the theme', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.goto('/#/primitives')
  await expect(page.locator('main h1')).toBeVisible()
  await page.evaluate(() => document.documentElement.removeAttribute('data-theme'))
  expect(isLight(await bg(page))).toBe(false)
  await page.emulateMedia({ colorScheme: 'light' })
  expect(isLight(await bg(page))).toBe(true)
})
