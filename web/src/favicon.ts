import { useEffect } from 'react'
import { brandMarkSvg, type MarkColors } from './brand'
import type { ApplicationState } from './types'

export const DEFAULT_FAVICON = '/favicon.svg'

const BASE_COLORS: MarkColors = {
  body: '#2563eb',
  face: '#ffffff',
  hand: '#2563eb',
}

interface Badge {
  color: string
  glyph: string
}

// Badge glyphs stay to one or two strokes; the colour carries the state
// at 16px. `none` has no badge and keeps the plain brand mark.
const BADGES: Record<Exclude<ApplicationState, 'none'>, Badge> = {
  saved: {
    color: '#d97706',
    glyph: '<path d="M22 21.5h4v6l-2-1.5-2 1.5z" fill="#fff"/>',
  },
  applied: {
    color: '#2563eb',
    glyph:
      '<circle cx="24" cy="24" r="2.7" fill="none" stroke="#fff" stroke-width="1.5"/>' +
      '<path d="M24 24v-1.6M24 20.5v.8" fill="none" stroke="#fff" stroke-width="1.4" stroke-linecap="round"/>',
  },
  interview: {
    color: '#7c3aed',
    glyph: '<path d="M21.5 22h5v3.2h-2.6L22 26.8v-1.6h-.5z" fill="#fff"/>',
  },
  offer: {
    color: '#12805c',
    glyph:
      '<path d="m24 20.6 1.1 2.2 2.4.4-1.8 1.7.4 2.4-2.1-1.1-2.1 1.1.4-2.4-1.8-1.7 2.4-.4z" fill="#fff"/>',
  },
  accepted: {
    color: '#0e7490',
    glyph:
      '<path d="m21.3 24 1.9 1.9 3.6-4" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>',
  },
  turned_down: {
    color: '#667085',
    glyph:
      '<path d="M21.4 24h5.2" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round"/>',
  },
  negative: {
    color: '#b42318',
    glyph:
      '<path d="m22 22 4 4M26 22l-4 4" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round"/>',
  },
}

export const FAVICON_BADGE_COLORS: Record<
  Exclude<ApplicationState, 'none'>,
  string
> = Object.fromEntries(
  Object.entries(BADGES).map(([state, badge]) => [state, badge.color]),
) as Record<Exclude<ApplicationState, 'none'>, string>

function dataUri(svg: string): string {
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

// faviconFor returns the static default for `none` and a fresh data URI for
// every application state. Distinct URIs avoid the browser reusing the
// previous icon from its cache.
export function faviconFor(state: ApplicationState): string {
  if (state === 'none') return DEFAULT_FAVICON
  const badge = BADGES[state]
  const ring =
    '<circle cx="24" cy="24" r="8" fill="#ffffff"/>' +
    `<circle cx="24" cy="24" r="6.4" fill="${badge.color}"/>` +
    badge.glyph
  return dataUri(brandMarkSvg(BASE_COLORS, ring))
}

function iconLink(): HTMLLinkElement {
  const existing = document.querySelector<HTMLLinkElement>('link[rel~="icon"]')
  if (existing) return existing
  const link = document.createElement('link')
  link.rel = 'icon'
  link.type = 'image/svg+xml'
  document.head.appendChild(link)
  return link
}

export function setFavicon(href: string): void {
  iconLink().href = href
}

export function resetFavicon(): void {
  setFavicon(DEFAULT_FAVICON)
}

// useOfferFavicon owns the browser icon while an offer is open. It updates
// when the application state changes and restores the default on unmount.
export function useOfferFavicon(state: ApplicationState): void {
  useEffect(() => {
    setFavicon(faviconFor(state))
    return () => {
      resetFavicon()
    }
  }, [state])
}
