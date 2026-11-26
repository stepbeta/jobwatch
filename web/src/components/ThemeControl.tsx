import { useTheme, type Theme } from '../theme'
import { ThemeIcon } from './icons'

const LABELS: Record<Theme, string> = {
  system: 'System',
  light: 'Light',
  dark: 'Dark',
}

// The icon cycles in a fixed order so a click always has a predictable
// result and the control never needs a menu.
const NEXT: Record<Theme, Theme> = {
  system: 'light',
  light: 'dark',
  dark: 'system',
}

export function ThemeControl() {
  const { theme, setTheme } = useTheme()
  const next = NEXT[theme]
  const label = `Theme: ${LABELS[theme]}. Switch to ${LABELS[next]}`

  return (
    <button
      type="button"
      className="icon-button theme-toggle"
      aria-label={label}
      title={label}
      onClick={() => setTheme(next)}
    >
      <ThemeIcon theme={theme} />
    </button>
  )
}
