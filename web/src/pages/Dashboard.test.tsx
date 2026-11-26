import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
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
  const view = render(
    <Dashboard
      refreshToken={0}
      onNotice={onNotice}
      sources={sources}
      onScanBatch={onScanBatch}
      scanning={false}
    />,
  )
  return { ...view, onNotice, onScanBatch }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/'
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

  it('shows disappeared offers in both sections with their listing state', async () => {
    const disappearedApplied = makeOffer({
      id: 20,
      title: 'Disappeared application',
      offer_state: 'disappeared',
      application_state: 'applied',
    })
    const disappearedSaved = makeOffer({
      id: 21,
      title: 'Disappeared saved',
      offer_state: 'disappeared',
      application_state: 'saved',
    })
    vi.mocked(api.listOffers).mockImplementation(async (query = {}) => {
      if (query.application_state === 'saved') {
        return { offers: [disappearedSaved], total: 1, limit: 200, offset: 0 }
      }
      if (query.has_application) {
        return { offers: [disappearedApplied], total: 1, limit: query.limit ?? 50, offset: 0 }
      }
      return { offers: [], total: 0, limit: 50, offset: 0 }
    })
    renderDashboard([makeSource()])

    const applications = (await screen.findByRole('heading', { name: 'Applications' }))
      .closest('section') as HTMLElement
    const saved = screen.getByRole('heading', { name: 'Saved' }).closest('section') as HTMLElement

    expect(await within(applications).findByText('Disappeared application')).toBeTruthy()
    expect(within(saved).getByText('Disappeared saved')).toBeTruthy()
    expect(within(applications).getByText('disappeared')).toBeTruthy()
    expect(within(saved).getByText('disappeared')).toBeTruthy()
  })

  it('loads the Saved section from the saved application state', async () => {
    renderDashboard([makeSource()])

    await screen.findByText('Saved role')
    expect(api.listOffers).toHaveBeenCalledWith(
      expect.objectContaining({ application_state: 'saved', sort: 'saved' }),
    )
  })

  it('shows every application by default with no application-state filter', async () => {
    renderDashboard([makeSource()])

    expect(await screen.findByText('Applied role')).toBeTruthy()
    const applicationsCall = vi
      .mocked(api.listOffers)
      .mock.calls.map(([query]) => query)
      .find((query) => query?.has_application)
    expect(applicationsCall?.application_state).toBeUndefined()
    expect(
      (screen.getByLabelText('Hide closed applications') as HTMLInputElement).checked,
    ).toBe(false)
  })

  it('restores the hide-closed toggle from the URL and applies it to Applications only', async () => {
    window.location.hash = '#/?closed=1'
    renderDashboard([makeSource()])

    await screen.findByText('Applied role')
    expect(api.listOffers).toHaveBeenCalledWith(
      expect.objectContaining({
        has_application: true,
        application_state: 'applied,interview,offer,accepted,turned_down',
      }),
    )
    expect(api.listOffers).toHaveBeenCalledWith(
      expect.objectContaining({
        application_state: 'saved',
        sort: 'saved',
      }),
    )
    const savedCall = vi
      .mocked(api.listOffers)
      .mock.calls.map(([query]) => query)
      .find((query) => query?.application_state === 'saved')
    expect(savedCall).not.toHaveProperty('offer_state')
    expect(
      (screen.getByLabelText('Hide closed applications') as HTMLInputElement).checked,
    ).toBe(true)
    expect(screen.getByRole('link', { name: /Show all/ }).getAttribute('href')).toBe(
      '#/offers?application_state=saved&sort=saved',
    )
  })

  it('ignores the old listing and application parameters', async () => {
    window.location.hash = '#/?listing=active&application=hide_negative'
    renderDashboard([makeSource()])

    await screen.findByText('Applied role')
    const applicationsCall = vi
      .mocked(api.listOffers)
      .mock.calls.map(([query]) => query)
      .find((query) => query?.has_application)
    expect(applicationsCall?.application_state).toBeUndefined()
    expect(
      (screen.getByLabelText('Hide closed applications') as HTMLInputElement).checked,
    ).toBe(false)
    await waitFor(() => expect(window.location.hash).toBe('#/'))
  })

  it('persists the hide-closed toggle as closed=1', async () => {
    const user = userEvent.setup()
    renderDashboard([makeSource()])
    await screen.findByText('Applied role')

    await user.click(screen.getByLabelText('Hide closed applications'))
    await waitFor(() => expect(window.location.hash).toContain('closed=1'))
    expect(api.listOffers).toHaveBeenCalledWith(
      expect.objectContaining({
        has_application: true,
        application_state: 'applied,interview,offer,accepted,turned_down',
      }),
    )

    await user.click(screen.getByLabelText('Hide closed applications'))
    await waitFor(() => expect(window.location.hash).not.toContain('closed=1'))
  })

  it('restores the toggle after a reload or history navigation', async () => {
    window.location.hash = '#/?closed=1'
    const first = renderDashboard([makeSource()])
    expect(
      (await screen.findByLabelText('Hide closed applications') as HTMLInputElement).checked,
    ).toBe(true)
    first.unmount()

    window.location.hash = '#/'
    renderDashboard([makeSource()])
    expect(
      (await screen.findByLabelText('Hide closed applications') as HTMLInputElement).checked,
    ).toBe(false)
  })

  it('refreshes only the sources behind the visible applications', async () => {
    const user = userEvent.setup()
    window.location.hash = '#/?closed=1'
    const negative = makeOffer({ id: 11, title: 'Negative role', application_state: 'negative' })
    vi.mocked(api.listOffers).mockImplementation(async (query = {}) => {
      if (query.application_state === 'saved') {
        return { offers: [], total: 0, limit: 200, offset: 0 }
      }
      if (query.has_application) {
        const visible = query.application_state ? [applied] : [applied, negative]
        return { offers: visible, total: visible.length, limit: query.limit ?? 50, offset: 0 }
      }
      return { offers: [], total: 0, limit: 50, offset: 0 }
    })
    vi.mocked(api.getOffer).mockImplementation(async (id) =>
      makeDetail(id === applied.id ? applied : negative, id === applied.id ? [1] : [2]),
    )
    const { onScanBatch } = renderDashboard([
      makeSource({ id: 1, name: 'Visible source' }),
      makeSource({ id: 2, name: 'Hidden source' }),
    ])

    await user.click(await screen.findByRole('button', { name: 'Refresh' }))

    await waitFor(() => expect(onScanBatch).toHaveBeenCalledTimes(1))
    const targets = vi.mocked(onScanBatch).mock.calls[0][0] as Source[]
    expect(targets.map((s) => s.id)).toEqual([1])
  })

  it('uses a filter-aware empty state and refresh notice', async () => {
    const user = userEvent.setup()
    window.location.hash = '#/?closed=1'
    vi.mocked(api.listOffers).mockResolvedValue({ offers: [], total: 0, limit: 20, offset: 0 })
    const { onNotice } = renderDashboard([makeSource()])

    expect(await screen.findByText('No applications match this filter.')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() =>
      expect(onNotice).toHaveBeenCalledWith('info', 'No applications match this filter.'),
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
      '1 source scanned: 0 new, 0 incomplete, 0 disappeared, 0 ghosted; 1 manual link(s) skipped (no fetchable source).',
    )
  })
})
