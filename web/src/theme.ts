import { useCallback, useEffect, useState } from 'react'

export type Theme = 'system' | 'light' | 'dark'

export const THEME_KEY = 'jobwatch.theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

export function isTheme(value: string | null): value is Theme {
  return value === 'system' || value === 'light' || value === 'dark'
}

// readTheme returns the stored selection, defaulting to system when nothing
// valid is stored or localStorage is unavailable.
export function readTheme(): Theme {
  try {
    const raw = window.localStorage.getItem(THEME_KEY)
    return isTheme(raw) ? raw : 'system'
  } catch {
    return 'system'
  }
}

export function prefersDark(): boolean {
  return (
    typeof window.matchMedia === 'function' && window.matchMedia(DARK_QUERY).matches
  )
}

// resolvedTheme is the palette actually shown for a selection. `system`
// follows the operating-system preference.
export function resolvedTheme(theme: Theme): 'light' | 'dark' {
  if (theme === 'dark') return 'dark'
  if (theme === 'light') return 'light'
  return prefersDark() ? 'dark' : 'light'
}

// applyTheme writes the selection to <html> so the CSS palette switches
// immediately, and mirrors the resolved palette into color-scheme for
// native controls and scrollbars. It also persists the choice.
export function applyTheme(theme: Theme): void {
  const root = document.documentElement
  root.dataset.theme = theme
  root.style.colorScheme = resolvedTheme(theme)
  try {
    window.localStorage.setItem(THEME_KEY, theme)
  } catch {
    // A blocked localStorage keeps the current selection for this tab.
  }
}

export function useTheme(): { theme: Theme; setTheme: (theme: Theme) => void } {
  const [theme, setThemeState] = useState<Theme>(readTheme)

  useEffect(() => {
    applyTheme(theme)
    if (theme !== 'system' || typeof window.matchMedia !== 'function') return
    const query = window.matchMedia(DARK_QUERY)
    const onChange = () => {
      document.documentElement.style.colorScheme = resolvedTheme('system')
    }
    if (query.addEventListener) query.addEventListener('change', onChange)
    else query.addListener(onChange)
    return () => {
      if (query.removeEventListener) query.removeEventListener('change', onChange)
      else query.removeListener(onChange)
    }
  }, [theme])

  const setTheme = useCallback((next: Theme) => {
    // Apply before the re-render so the header and the palette change
    // together and the choice survives a reload.
    applyTheme(next)
    setThemeState(next)
  }, [])

  return { theme, setTheme }
}
