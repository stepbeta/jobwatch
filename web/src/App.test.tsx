import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import App from './App'
import { api } from './api'
import type { OfferDetail } from './types'

vi.mock('./api', () => ({
  api: {
    listSources: vi.fn(),
    listOffers: vi.fn(),
    getOffer: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

const detail: OfferDetail = {
  offer: {
    id: 7,
    company_id: 2,
    company_name: 'Acme',
    title: 'Site Reliability Engineer',
    location: 'Paris, France',
    remote_policy: 'remote',
    url: null,
    offer_state: 'active',
    application_state: 'applied',
    first_seen_at: '2026-11-01T00:00:00Z',
    last_seen_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
    title_affinity: 80,
    location_affinity: 80,
    remote_policy_accepted: true,
    match_reasons: { title: [], location: [] },
    last_event_at: null,
    saved_at: null,
  },
  company: {
    id: 2,
    name: 'Acme',
    name_normalized: 'acme',
    website: null,
    created_at: null,
    offer_count: 1,
    note_count: 0,
  },
  provenance: [],
  events: [],
  notes: [],
  snapshots: [],
  latest_snapshot: null,
  snapshot_diff: null,
  snapshot_count: 0,
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/offers/7'
  vi.mocked(api.listSources).mockResolvedValue({ sources: [] })
  vi.mocked(api.listOffers).mockResolvedValue({ offers: [], total: 0, limit: 20, offset: 0 })
  vi.mocked(api.getOffer).mockResolvedValue(detail)
})

describe('App browser identity handoff', () => {
  it('replaces the offer title with the dashboard title when navigating away', async () => {
    render(<App />)

    await screen.findByRole('heading', { name: 'Site Reliability Engineer' })
    await waitFor(() =>
      expect(document.title).toBe('Site Reliability Engineer — Acme · JobWatch'),
    )
    const icon = document.querySelector('link[rel~="icon"]') as HTMLLinkElement
    expect(icon.getAttribute('href')).toMatch(/^data:image\/svg\+xml,/)

    window.location.hash = '#/'
    window.dispatchEvent(new Event('hashchange'))

    await waitFor(() => expect(document.title).toBe('Dashboard — JobWatch'))
    expect(icon.getAttribute('href')).toBe('/favicon.svg')
  })
})
