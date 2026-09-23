import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// Component catalogue: a plain Vue app that renders every kit component in
// every variant (synthetic data only). Used for manual review and for the
// Playwright screenshot baseline at 320/768/1280 px.
export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
  plugins: [vue(), tailwindcss()],
  server: { port: 5199, strictPort: true },
  preview: { port: 5199, strictPort: true },
  build: { outDir: fileURLToPath(new URL('./dist', import.meta.url)), emptyOutDir: true },
})
