import { writable } from 'svelte/store'

export type Theme = 'dark' | 'light'

const KEY = 'cookarr:theme'

function initial(): Theme {
  return localStorage.getItem(KEY) === 'light' ? 'light' : 'dark'
}

export const currentTheme = writable<Theme>(initial())

export function setTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
  localStorage.setItem(KEY, theme)
  currentTheme.set(theme)
}

export function toggleTheme() {
  setTheme(document.documentElement.dataset.theme === 'light' ? 'dark' : 'light')
}

setTheme(initial())
