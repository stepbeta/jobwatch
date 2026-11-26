import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import indexHtml from '../index.html?raw'
import styles from './styles.css?raw'
import {
  THEME_KEY,
  applyTheme,
  readTheme,
  resolvedTheme,
  type Theme,
} from './theme'

const DARK_QUERY = '(prefers-color-scheme: dark)'

function installMatchMedia(matches: boolean) {
  const listeners = new Set<() => void>()
  const mql = {
    matches,
    media: DARK_QUERY,
    onchange: null,
    addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
    removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
    addListener: (listener: () => void) => listeners.add(listener),
    removeListener: (listener: () => void) => listeners.delete(listener),
    dispatchEvent: () => true,
  }
  const original = window.matchMedia
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => (query === DARK_QUERY ? mql : { ...mql, matches: false }),
  })
  return {
    mql,
    restore: () => {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: original,
      })
    },
    setMatches(next: boolean) {
      mql.matches = next
      listeners.forEach((listener) => listener())
    },
  }
}

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
  document.documentElement.style.colorScheme = ''
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('theme selection', () => {
  it('defaults to system and ignores unknown stored values', () => {
    expect(readTheme()).toBe('system')
    window.localStorage.setItem(THEME_KEY, 'sepia')
    expect(readTheme()).toBe('system')
    window.localStorage.setItem(THEME_KEY, 'dark')
    expect(readTheme()).toBe('dark')
  })

  it('resolves explicit choices and follows the OS for system', () => {
    expect(resolvedTheme('light')).toBe('light')
    expect(resolvedTheme('dark')).toBe('dark')

    const media = installMatchMedia(true)
    expect(resolvedTheme('system')).toBe('dark')
    expect(resolvedTheme('light')).toBe('light')
    media.setMatches(false)
    expect(resolvedTheme('system')).toBe('light')
    media.restore()
  })

  it('applies and persists the selection on <html>', () => {
    applyTheme('dark')
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('dark')

    applyTheme('light')
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('light')
  })

  it('keeps the light palette on :root and scopes the system dark palette', () => {
    expect(styles).toMatch(/:root\s*\{[\s\S]*?--bg: #f5f6f8;/)
    expect(styles).toMatch(/:root\[data-theme='dark'\]\s*\{[\s\S]*?--bg: #14161a;/)
    expect(styles).toMatch(
      /@media \(prefers-color-scheme: dark\) \{\s*:root:not\(\[data-theme='light'\]\)\s*\{/,
    )
  })
})

describe('pre-paint theme script', () => {
  const html = indexHtml
  const script = html.match(/<script>([\s\S]*?)<\/script>/)?.[1] ?? ''

  function runScript(stored: Theme | null, systemDark: boolean) {
    const media = installMatchMedia(systemDark)
    if (stored === null) window.localStorage.removeItem(THEME_KEY)
    else window.localStorage.setItem(THEME_KEY, stored)
    document.documentElement.removeAttribute('data-theme')
    document.documentElement.style.colorScheme = ''
    new Function(script)()
    const result = {
      theme: document.documentElement.dataset.theme,
      colorScheme: document.documentElement.style.colorScheme,
    }
    media.restore()
    return result
  }

  it('exists in the document head before the module bundle', () => {
    expect(script).toContain('jobwatch.theme')
    expect(script).toContain("setAttribute('data-theme'")
    expect(script).toContain('colorScheme')
  })

  it('paints system, light and dark correctly before React loads', () => {
    expect(runScript(null, false)).toEqual({ theme: 'system', colorScheme: 'light' })
    expect(runScript(null, true)).toEqual({ theme: 'system', colorScheme: 'dark' })
    expect(runScript('light', true)).toEqual({ theme: 'light', colorScheme: 'light' })
    expect(runScript('dark', false)).toEqual({ theme: 'dark', colorScheme: 'dark' })
  })
})
