import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import dts from 'vite-plugin-dts'

// Library build of @freya/ui: three entries (components, forms, api), ESM only,
// peers externalised. Consumers compile the CSS themselves (Tailwind scans the
// dist through @source), so no stylesheet is emitted here.
export default defineConfig({
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  plugins: [vue(), dts({ include: ['src'], rollupTypes: false, tsconfigPath: './tsconfig.json' })],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    target: 'esnext',
    lib: {
      entry: {
        index: fileURLToPath(new URL('./src/index.ts', import.meta.url)),
        forms: fileURLToPath(new URL('./src/forms/index.ts', import.meta.url)),
        api: fileURLToPath(new URL('./src/api/index.ts', import.meta.url)),
      },
      formats: ['es'],
    },
    rollupOptions: { external: ['vue', 'vue-router', 'zod'] },
  },
  test: {
    environment: 'jsdom',
    environmentOptions: { jsdom: { url: 'https://localhost/' } },
    include: ['tests/**/*.spec.ts'],
    setupFiles: ['tests/setup.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/**/index.ts', 'src/icons.ts'],
      thresholds: { statements: 80, 'src/forms/**': { statements: 100 }, 'src/api/**': { statements: 100 } },
    },
  },
})
