// Shared front-end lint rules. Besides the Vue/TS recommended sets, the kit (and
// every migrated front-end) forbids inline styles (the edge CSP has no
// unsafe-inline for style-src), Vuetify imports and hand-written `:rules`
// validation (forms validate through Zod schemas only).
export const freyaRules = {
  'vue/multi-word-component-names': 'off',
  'vue/max-attributes-per-line': 'off',
  'vue/singleline-html-element-content-newline': 'off',
  'vue/no-restricted-static-attribute': ['error', { key: 'style', message: 'Inline styles are blocked by the CSP; use classes or a kit component.' }],
  'vue/no-restricted-v-bind': ['error', { argument: 'style', message: 'Inline styles are blocked by the CSP; use classes or a kit component.' }, { argument: 'rules', message: 'Validate through a Zod schema with useZodForm, not :rules.' }],
  'no-restricted-imports': ['error', { patterns: [{ group: ['vuetify', 'vuetify/*', '@mdi/font', '@mdi/font/*', 'vite-plugin-vuetify'], message: 'The platform UI is FlyonUI via @go-tangra/ui.' }] }],
}
