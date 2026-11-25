import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Dashboard } from './Dashboard'
import { api } from '../api'
import type { Offer, OfferDetail, Source } from '../types'

vi.mock('../api', () => ({
  api: {
    listOffers: vi.fn(),
    getOffer: vi.fn(),
    patchOffer: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

function makeOffer(overrides: Partial<Offer> = {}): Offer {
  return {
    id: 1,
    company_id: 1,
    company_name: 'Acme',
    title: 'Site Reliability Engineer',
    location: 'Paris',
    remote_policy: 'remote',
    url: null,
    offer_state: 'active',
    application_state: 'none',
    first_seen_at: '2026-11-01T00:00:00Z',
    last_seen_at: '2026-11-01T00:00:00Z',
    updated_at: '2026-11-01T00:00:00Z',
    title_affinity: 80,
    location_affinity: 80,
    remote_policy_accepted: true,
    match_reasons: { title: [], location: [] },
    last_event_at: null,
    saved_at: null,
    ...overrides,
  }
}

function makeDetail(offer: Offer, sourceIDs: (number | null)[]): OfferDetail {
  return {
    offer,
    company: {
      id: 1,
      name: 'Acme',
      name_normalized: 'acme',
      website: null,
      created_at: null,
      offer_count: 1,
      note_count: 0,
    },
    provenance: sourceIDs.map((sourceID, i) => ({
      id: i + 1,
      source_id: sourceID,
      source_name: sourceID === null ? null : `Source ${sourceID}`,
      external_uid: null,
      url: 'https://example.com/job',
      first_seen_at: null,
      last_seen_at: null,
    })),
    events: [],
    notes: [],
    snapshots: [],
    latest_snapshot: null,
    snapshot_diff: null,
    snapshot_count: 0,
  }
}

function makeSource(overrides: Partial<Source> = {}): Source {
  return {
    id: 1,
    name: 'Source 1',
    url: 'https://example.com',
    source_type: 'greenhouse',
    config: {},
    enabled: true,
    last_scan_at: null,
    last_scan_status: null,
    created_at: null,
    ...overrides,
  }
}

const applied = makeOffer({ id: 10, title: 'Applied role', application_state: 'applied' })
const savedOffer = makeOffer({
  id: 12,
  title: 'Saved role',
  application_state: 'saved',
  saved_at: '2026-11-15T09:00:00Z',
})

function renderDashboard(sources: Source[], onScanBatch = vi.fn().mockResolvedValue([])) {
  const onNotice = vi.fn()
  render(
    <Dashboard
      refreshToken={0}
      onNotice={onNotice}
      sources={sources}
      onScanBatch={onScanBatch}
      scanning={false}
    />,
  )
  return { onNotice, onScanBatch }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.listOffers).mockImplementation(async (query = {}) => {
    if (query.application_state === 'saved') {
      return { offers: [savedOffer], total: 1, limit: 200, offset: 0 }
    }
    if (query.has_application) {
      return { offers: [applied], total: 1, limit: query.limit ?? 50, offset: 0 }
    }
    return { offers: [], total: 0, limit: 50, offset: 0 }
  })
  vi.mocked(api.getOffer).mockResolvedValue(makeDetail(applied, [1, null]))
})

describe('Dashboard', () => {
  it('shows Applications and Saved sections independently', async () => {
    renderDashboard([makeSource()])

    expect(await screen.findByRole('heading', { name: 'Applications' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Saved' })).toBeTruthy()
    expect(await screen.findByText('Applied role')).toBeTruthy()
    expect(await screen.findByText('Saved role')).toBeTruthy()
  })

  it('loads the Saved section from the saved application state', async () => {
    renderDashboard([makeSource()])

    await screen.findByText('Saved role')
    expect(api.listOffers).toHaveBeenCalledWith(
      expect.objectContaining({ application_state: 'saved', sort: 'saved' }),
    )
  })

  it('refreshes only the enabled sources behind applications', async () => {
    const user = userEvent.setup()
    const sources = [
      makeSource({ id: 1, name: 'App source' }),
      makeSource({ id: 2, name: 'Unrelated source' }),
      makeSource({ id: 3, name: 'Disabled source', enabled: false }),
    ]
    vi.mocked(api.getOffer).mockResolvedValue(makeDetail(applied, [1, 3, null]))
    const { onNotice, onScanBatch } = renderDashboard(sources)

    await user.click(await screen.findByRole('button', { name: 'Refresh' }))

    await waitFor(() => expect(onScanBatch).toHaveBeenCalledTimes(1))
    const targets = vi.mocked(onScanBatch).mock.calls[0][0] as Source[]
    expect(targets.map((s) => s.id)).toEqual([1])
    expect(onNotice).toHaveBeenCalledWith(
      'success',
      '1 source scanned: 0 new, 0 incomplete, 0 disappeared; 1 manual link(s) skipped (no fetchable source).',
    )
  })
})
