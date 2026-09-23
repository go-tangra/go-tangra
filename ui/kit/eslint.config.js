import js from '@eslint/js'
import ts from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import globals from 'globals'
import { freyaRules } from './eslint.rules.js'

export { freyaRules }

export default [
  { ignores: ['dist/**', 'node_modules/**', 'catalogue/dist/**', 'coverage/**'] },
  js.configs.recommended,
  ...ts.configs.recommended,
  ...vue.configs['flat/recommended'],
  { languageOptions: { globals: { ...globals.browser } } },
  { files: ['**/*.vue'], languageOptions: { parser: vueParser, parserOptions: { parser: ts.parser } } },
  { rules: { ...freyaRules, 'vue/require-default-prop': 'off' } },
]
