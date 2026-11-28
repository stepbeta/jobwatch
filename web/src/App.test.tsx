import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import App from './App'
import { ApiError, api } from './api'
import type { OfferDetail } from './types'

vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return {
    ...actual,
    api: {
      listSources: vi.fn(),
      listOffers: vi.fn(),
      getOffer: vi.fn(),
      health: vi.fn(),
    },
  }
})

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
  attachments: [],
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
  vi.mocked(api.health).mockResolvedValue({ status: 'ok', version: 'dev', db: 'ok' })
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

describe('App connection banner', () => {
  it('explains a refused host with the address and the flag', async () => {
    vi.mocked(api.health).mockRejectedValue(
      new ApiError('forbidden_host', 'host is not allowed', 403),
    )
    render(<App />)

    const banner = await screen.findByRole('alert')
    expect(banner.textContent).toContain('http://127.0.0.1:8080')
    expect(banner.textContent).toContain('--allow-host')
    expect(screen.getAllByRole('alert')).toHaveLength(1)
  })

  it('reports an unreachable server', async () => {
    vi.mocked(api.health).mockRejectedValue(new TypeError('Failed to fetch'))
    render(<App />)

    const banner = await screen.findByRole('alert')
    expect(banner.textContent).toContain('not responding')
  })

  it('clears the banner when a later check succeeds', async () => {
    vi.mocked(api.health)
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValue({ status: 'ok', version: 'dev', db: 'ok' })
    render(<App />)
    await screen.findByRole('alert')

    document.dispatchEvent(new Event('visibilitychange'))

    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
  })

  it('shows no banner when the API answers', async () => {
    render(<App />)

    await waitFor(() => expect(api.health).toHaveBeenCalled())
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
