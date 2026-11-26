import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ThemeControl } from './ThemeControl'
import { THEME_KEY } from '../theme'

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
    setMatches(next: boolean) {
      mql.matches = next
      listeners.forEach((listener) => listener())
    },
    restore: () => {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: original,
      })
    },
  }
}

beforeEach(() => {
  window.localStorage.clear()
  document.documentElement.removeAttribute('data-theme')
  document.documentElement.style.colorScheme = ''
})

afterEach(() => {
  document.documentElement.removeAttribute('data-theme')
  document.documentElement.style.colorScheme = ''
})

describe('ThemeControl', () => {
  it('defaults to system and follows a light operating system', () => {
    const media = installMatchMedia(false)
    render(<ThemeControl />)

    expect(screen.getByRole('button', { name: 'Theme: System. Switch to Light' })).toBeTruthy()
    expect(document.documentElement.dataset.theme).toBe('system')
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('system')
    media.restore()
  })

  it('restores an explicit persisted choice before rendering', () => {
    const media = installMatchMedia(false)
    window.localStorage.setItem(THEME_KEY, 'dark')
    render(<ThemeControl />)

    expect(screen.getByRole('button', { name: 'Theme: Dark. Switch to System' })).toBeTruthy()
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    media.restore()
  })

  it('cycles through every theme on click', async () => {
    const media = installMatchMedia(false)
    const user = userEvent.setup()
    render(<ThemeControl />)

    await user.click(screen.getByRole('button', { name: 'Theme: System. Switch to Light' }))
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('light')

    await user.click(screen.getByRole('button', { name: 'Theme: Light. Switch to Dark' }))
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('dark')

    await user.click(screen.getByRole('button', { name: 'Theme: Dark. Switch to System' }))
    expect(document.documentElement.dataset.theme).toBe('system')
    expect(document.documentElement.style.colorScheme).toBe('light')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('system')
    media.restore()
  })

  it('returns to system and follows later operating-system changes', async () => {
    const media = installMatchMedia(true)
    window.localStorage.setItem(THEME_KEY, 'dark')
    const user = userEvent.setup()
    render(<ThemeControl />)

    await user.click(screen.getByRole('button', { name: 'Theme: Dark. Switch to System' }))

    expect(document.documentElement.dataset.theme).toBe('system')
    expect(document.documentElement.style.colorScheme).toBe('dark')
    expect(window.localStorage.getItem(THEME_KEY)).toBe('system')

    act(() => media.setMatches(false))
    expect(document.documentElement.style.colorScheme).toBe('light')
    media.restore()
  })
})
