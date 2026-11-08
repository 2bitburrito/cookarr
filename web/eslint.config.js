// Basic, Svelte-friendly lint config for the frontend. Deliberately light:
// catches real mistakes (unused vars, obvious bugs) without imposing a
// strict style, per docs/ROADMAP.md "a very basic svelte friendly linter".
import js from '@eslint/js'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import svelte from 'eslint-plugin-svelte'
import svelteParser from 'svelte-eslint-parser'
import globals from 'globals'

export default [
  js.configs.recommended,
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      globals: { ...globals.browser, ...globals.es2021 },
    },
    plugins: { '@typescript-eslint': tsPlugin },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      // Dockview's own callbacks and a few DOM handlers intentionally
      // ignore arguments; that's fine for a basic config.
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  ...svelte.configs.recommended,
  {
    files: ['**/*.svelte'],
    languageOptions: {
      parser: svelteParser,
      parserOptions: { parser: tsParser },
      globals: { ...globals.browser, ...globals.es2021 },
    },
    rules: {
      // svelte-eslint-parser understands Svelte 5 runes ($state, $props,
      // $derived, ...); no-undef still trips on them without this.
      'no-undef': 'off',
      // This is a basic config, not a strict style guide: keys on #each are
      // good practice but not worth enforcing for the small, static lists
      // used throughout (ingredients, steps, panels).
      'svelte/require-each-key': 'off',
      // story_html is server-rendered HTML that's already sanitised with
      // bluemonday before storage (internal/capture) — rendering it is the
      // whole point of the Story panel, so the generic XSS rule doesn't
      // apply here.
      'svelte/no-at-html-tags': 'off',
    },
  },
  {
    ignores: ['dist/**', 'node_modules/**'],
  },
]
