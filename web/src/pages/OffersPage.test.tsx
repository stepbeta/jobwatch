import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError, type OfferQuery } from '../api'
import { OffersPage } from './OffersPage'
import type { Company, Offer } from '../types'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    api: {
      listOffers: vi.fn(),
      listCompanies: vi.fn(),
      importOffer: vi.fn(),
      patchOffer: vi.fn(),
    },
  }
})

import { api } from '../api'

function renderPage() {
  const onDataChanged = vi.fn()
  const onNotice = vi.fn()
  const view = render(
    <OffersPage refreshToken={0} onDataChanged={onDataChanged} onNotice={onNotice} />,
  )
  return { onDataChanged, onNotice, view }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/offers'
  vi.mocked(api.listOffers).mockResolvedValue({ offers: [], total: 0, limit: 50, offset: 0 })
  vi.mocked(api.listCompanies).mockResolvedValue({ companies: [], total: 0, limit: 500, offset: 0 })
})

describe('OffersPage add-offer modal', () => {
  it('opens the import form from the page header instead of an inline panel', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.querySelector('details')).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: 'Add an offer' })).toBeTruthy()
    expect(within(dialog).getByLabelText('Job URL')).toBeTruthy()
    expect(within(dialog).getByLabelText('Company')).toBeTruthy()
    expect(within(dialog).getByLabelText('Title')).toBeTruthy()
  })

  it('imports a URL, closes the modal, shows the toast and opens the new offer', async () => {
    const user = userEvent.setup()
    vi.mocked(api.importOffer).mockResolvedValue({ offer_id: 42, created: true })
    const { onDataChanged, onNotice } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Job URL'), 'https://example.com/jobs/42')
    await user.click(within(dialog).getByRole('button', { name: 'Fetch & import' }))

    await waitFor(() =>
      expect(api.importOffer).toHaveBeenCalledWith({
        company: undefined,
        title: undefined,
        location: undefined,
        url: 'https://example.com/jobs/42',
        description: undefined,
      }),
    )
    expect(onNotice).toHaveBeenCalledWith('success', 'Created offer #42.')
    expect(onDataChanged).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(window.location.hash).toBe('#/offers/42')
  })

  it('keeps the modal open and prefills fields on a 422 metadata fallback', async () => {
    const user = userEvent.setup()
    vi.mocked(api.importOffer).mockRejectedValue(
      new ApiError('metadata_incomplete', 'The page had no usable title.', 422, {
        company: 'Acme',
        title: 'Platform Engineer',
        location: 'Paris, France',
        url: 'https://example.com/jobs/42',
        description: 'Run the platform.',
      }),
    )
    const { onDataChanged, onNotice } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Job URL'), 'https://example.com/jobs/42')
    await user.click(within(dialog).getByRole('button', { name: 'Fetch & import' }))

    expect(
      await within(dialog).findByText('The page had no usable title. The available fields were prefilled.'),
    ).toBeTruthy()
    expect((within(dialog).getByLabelText('Company') as HTMLInputElement).value).toBe('Acme')
    expect((within(dialog).getByLabelText('Title') as HTMLInputElement).value).toBe('Platform Engineer')
    expect((within(dialog).getByLabelText('Location') as HTMLInputElement).value).toBe('Paris, France')
    // The fallback is inline: no error toast, no navigation, no refresh.
    expect(onNotice).not.toHaveBeenCalled()
    expect(onDataChanged).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(window.location.hash.startsWith('#/offers')).toBe(true)
    expect(window.location.hash).not.toContain('#/offers/')
  })
})

function makeOffer(overrides: Partial<Offer> = {}): Offer {
  return {
    id: 7,
    company_id: 1,
    company_name: 'Acme',
    title: 'Site Reliability Engineer',
    location: 'Paris, France',
    remote_policy: 'remote',
    url: null,
    offer_state: 'active',
    application_state: 'applied',
    first_seen_at: '2026-11-01T00:00:00Z',
    last_seen_at: '2026-11-02T00:00:00Z',
    updated_at: '2026-11-02T00:00:00Z',
    title_affinity: 90,
    location_affinity: 85,
    remote_policy_accepted: true,
    match_reasons: { title: [], location: [] },
    last_event_at: null,
    saved_at: null,
    ...overrides,
  }
}

function makeCompany(id: number, name: string): Company {
  return {
    id,
    name,
    name_normalized: name.toLowerCase(),
    website: null,
    created_at: null,
    offer_count: 1,
    note_count: 0,
  }
}

function lastOfferQuery(): OfferQuery {
  const calls = vi.mocked(api.listOffers).mock.calls
  return calls[calls.length - 1][0] ?? {}
}

describe('OffersPage table controls', () => {
  beforeEach(() => {
    vi.mocked(api.listOffers).mockResolvedValue({
      offers: [makeOffer()],
      total: 1,
      limit: 20,
      offset: 0,
    })
  })

  it('restores filters, sort, page and page size from the URL', async () => {
    window.location.hash =
      '#/offers?q=sre&remote_policy=remote&sort=title&order=asc&page=2&per_page=20'
    renderPage()

    await waitFor(() =>
      expect(api.listOffers).toHaveBeenCalledWith(
        expect.objectContaining({
          q: 'sre',
          remote_policy: 'remote',
          sort: 'title',
          order: 'asc',
          limit: 20,
          offset: 20,
        }),
      ),
    )
    expect((screen.getByLabelText('Search') as HTMLInputElement).value).toBe('sre')
    expect(window.location.hash).toContain('q=sre')
  })

  it('ignores invalid URL parameter values instead of failing', async () => {
    window.location.hash =
      '#/offers?remote_policy=nope&application_state=nope&sort=nope&order=sideways&page=-3&per_page=999'
    renderPage()

    await waitFor(() => expect(api.listOffers).toHaveBeenCalled())
    expect(lastOfferQuery()).toMatchObject({ sort: 'last_seen', order: 'desc', limit: 50, offset: 0 })
    expect(lastOfferQuery().remote_policy).toBeUndefined()
    expect(lastOfferQuery().application_state).toBeUndefined()
  })

  it('filters as the user types and reflects the debounced search in the URL', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByLabelText('Search'), 'sre')

    await waitFor(() => expect(lastOfferQuery().q).toBe('sre'))
    await waitFor(() => expect(window.location.hash).toContain('q=sre'))

    await user.click(screen.getByRole('button', { name: 'Reset' }))
    await waitFor(() => expect(lastOfferQuery().q).toBeUndefined())
    await waitFor(() => expect(window.location.hash).not.toContain('q=sre'))
  })

  it('sorts by clicking a column header and toggles asc then desc', async () => {
    const user = userEvent.setup()
    renderPage()

    const titleHeader = await screen.findByRole('button', { name: 'Title' })
    await user.click(titleHeader)
    await waitFor(() => expect(lastOfferQuery()).toMatchObject({ sort: 'title', order: 'asc' }))

    await user.click(screen.getByRole('button', { name: 'Title' }))
    await waitFor(() => expect(lastOfferQuery()).toMatchObject({ sort: 'title', order: 'desc' }))
  })

  it('persists hidden columns and never allows hiding every column', async () => {
    const user = userEvent.setup()
    const { view } = renderPage()
    await screen.findByRole('button', { name: 'Columns' })

    await user.click(screen.getByRole('button', { name: 'Columns' }))
    const menu = screen.getByRole('group', { name: 'Visible columns' })
    await user.click(within(menu).getByRole('checkbox', { name: 'Location' }))

    expect(within(menu).getByRole('checkbox', { name: 'Location' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Location' })).toBeNull()
    expect(window.localStorage.getItem('offers.columns')).toBe('["location"]')

    await user.click(within(menu).getByRole('checkbox', { name: 'Company' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Title' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Remote' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Offer' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Application' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Title affinity' }))
    await user.click(within(menu).getByRole('checkbox', { name: 'Location affinity' }))
    expect((within(menu).getByRole('checkbox', { name: 'Last seen' }) as HTMLInputElement).disabled).toBe(true)

    view.unmount()
    renderPage()
    await screen.findByRole('button', { name: 'Columns' })
    expect(screen.queryByRole('button', { name: 'Location' })).toBeNull()
  })

  it('ignores unknown column names in a stale saved preference', async () => {
    window.localStorage.setItem('offers.columns', JSON.stringify(['retired_column']))
    renderPage()
    await screen.findByRole('button', { name: 'Columns' })
    expect(screen.getByRole('button', { name: 'Location' })).toBeTruthy()
  })

  it('filters the company dropdown and sends company_id', async () => {
    const user = userEvent.setup()
    vi.mocked(api.listCompanies).mockResolvedValue({
      companies: [makeCompany(1, 'Acme'), makeCompany(2, 'Other')],
      total: 2,
      limit: 500,
      offset: 0,
    })
    renderPage()

    const input = await screen.findByLabelText('Company')
    await user.click(input)
    await user.type(input, 'Ac')
    expect(screen.getByRole('option', { name: 'Acme' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: 'Other' })).toBeNull()

    await user.click(screen.getByRole('option', { name: 'Acme' }))
    await waitFor(() => expect(lastOfferQuery().company_id).toBe(1))
    await waitFor(() => expect(window.location.hash).toContain('company_id=1'))
  })

  it('filters by title affinity, location affinity and remote acceptance', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Title affinity ≥'), '80')
    await waitFor(() => expect(lastOfferQuery().min_title_affinity).toBe(80))
    await waitFor(() => expect(window.location.hash).toContain('min_title_affinity=80'))

    await user.type(screen.getByLabelText('Location affinity ≥'), '60')
    await waitFor(() => expect(lastOfferQuery().min_location_affinity).toBe(60))

    await user.selectOptions(screen.getByLabelText('Remote accepted'), 'no')
    await waitFor(() => expect(lastOfferQuery().remote_accepted).toBe(false))
    await waitFor(() => expect(window.location.hash).toContain('remote_accepted=false'))
  })

  it('sorts by affinity columns and defaults to descending', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Title affinity' }))
    await waitFor(() =>
      expect(lastOfferQuery()).toMatchObject({ sort: 'title_affinity', order: 'desc' }),
    )

    await user.click(screen.getByRole('button', { name: 'Location affinity' }))
    await waitFor(() =>
      expect(lastOfferQuery()).toMatchObject({ sort: 'location_affinity', order: 'desc' }),
    )
  })

  it('renders affinities, term reasons and the remote policy badge', async () => {
    vi.mocked(api.listOffers).mockResolvedValue({
      offers: [
        makeOffer({
          match_reasons: {
            title: [{ term: 'site reliability', weight: 60, bucket: 'strong_positive' }],
            location: [{ term: 'paris', weight: 25, bucket: 'mild_positive' }],
          },
        }),
      ],
      total: 1,
      limit: 50,
      offset: 0,
    })
    renderPage()

    expect(await screen.findByText('T 90')).toBeTruthy()
    expect(screen.getByText('L 85')).toBeTruthy()
    expect(screen.getByTitle(/Full remote — accepted/)).toBeTruthy()
    expect(screen.getByTitle(/Strong positive "site reliability" \(\+60\)/)).toBeTruthy()
  })

  it('changes the page size, sends it to the API and stores the personal default', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.selectOptions(await screen.findByLabelText('Per page'), '20')
    await waitFor(() => expect(lastOfferQuery()).toMatchObject({ limit: 20, offset: 0 }))
    await waitFor(() => expect(window.location.hash).toContain('per_page=20'))
    expect(window.localStorage.getItem('offers.perPage')).toBe('20')
  })
})
