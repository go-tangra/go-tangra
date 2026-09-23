import { defineConfig, devices } from '@playwright/test'

// Screenshot regression for the catalogue at the three reference widths.
// `npx playwright test -c catalogue/playwright.config.ts` (add --update-snapshots
// to refresh the baseline after an intentional visual change).
// PW_CHANNEL=chrome runs on the system Chrome (dev boxes); CI uses the bundled chromium.
const chrome = { ...devices['Desktop Chrome'], ...(process.env.PW_CHANNEL ? { channel: process.env.PW_CHANNEL } : {}) }

export default defineConfig({
  testDir: './tests',
  snapshotPathTemplate: '{testDir}/__screenshots__/{projectName}/{testFilePath}/{arg}{ext}',
  fullyParallel: true,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: { baseURL: 'http://127.0.0.1:5199', trace: 'retain-on-failure', colorScheme: 'light' },
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: 'disabled' } },
  webServer: { command: 'npx vite --config catalogue/vite.config.ts --host 127.0.0.1', url: 'http://127.0.0.1:5199', reuseExistingServer: !process.env.CI, cwd: '..' },
  projects: [
    { name: 'phone-320', use: { ...chrome, viewport: { width: 320, height: 640 } } },
    { name: 'tablet-768', use: { ...chrome, viewport: { width: 768, height: 1024 } } },
    { name: 'desktop-1280', use: { ...chrome, viewport: { width: 1280, height: 800 } } },
  ],
})
