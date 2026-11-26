import { afterEach, describe, expect, it } from 'vitest'
import {
  DEFAULT_FAVICON,
  FAVICON_BADGE_COLORS,
  faviconFor,
  resetFavicon,
  setFavicon,
} from './favicon'
import type { ApplicationState } from './types'

const BADGED_STATES: Exclude<ApplicationState, 'none'>[] = [
  'saved',
  'applied',
  'interview',
  'offer',
  'accepted',
  'turned_down',
  'negative',
]

function decode(uri: string): string {
  return decodeURIComponent(uri.replace('data:image/svg+xml,', ''))
}

afterEach(() => {
  document.querySelectorAll('link[rel~="icon"]').forEach((link) => link.remove())
})

describe('faviconFor', () => {
  it('uses the static brand mark for none', () => {
    expect(faviconFor('none')).toBe(DEFAULT_FAVICON)
  })

  it('maps every application state to a distinct badge colour', () => {
    const uris = BADGED_STATES.map((state) => faviconFor(state))
    expect(new Set(uris).size).toBe(BADGED_STATES.length)
    const colors = BADGED_STATES.map((state) => FAVICON_BADGE_COLORS[state])
    expect(new Set(colors).size).toBe(BADGED_STATES.length)
    for (const state of BADGED_STATES) {
      const svg = decode(faviconFor(state))
      expect(svg).toContain(FAVICON_BADGE_COLORS[state])
      expect(svg).toContain('viewBox="0 0 32 32"')
      // The base watch mark stays behind every badge.
      expect(svg).toContain('cx="16" cy="18" r="11"')
      // The badge has the white cut-out ring.
      expect(svg).toContain('cx="24" cy="24" r="8" fill="#ffffff"')
    }
  })

  it('keeps none out of the badge colour map', () => {
    expect(Object.keys(FAVICON_BADGE_COLORS)).toEqual(BADGED_STATES)
  })
})

describe('favicon link', () => {
  it('creates the link when missing and restores the default', () => {
    setFavicon(faviconFor('applied'))
    const link = document.querySelector('link[rel~="icon"]') as HTMLLinkElement
    expect(link.getAttribute('href')).toMatch(/^data:image\/svg\+xml,/)
    expect(link.type).toBe('image/svg+xml')

    resetFavicon()
    expect(link.getAttribute('href')).toBe(DEFAULT_FAVICON)
  })
})
