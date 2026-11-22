import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { IncompletePage } from './IncompletePage'
import type { Company, Offer } from '../types'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    api: {
      listOffers: vi.fn(),
      listCompanies: vi.fn(),
      repairOffer: vi.fn(),
    },
  }
})

import { api } from '../api'

function makeIncomplete(overrides: Partial<Offer> = {}): Offer {
  return {
    id: 1,
    company_id: null,
    company_name: '',
    title: '',
    location: null,
    remote_policy: 'unknown',
    url: 'https://example.com/jobs/1',
    offer_state: 'active',
    application_state: 'todo',
    first_seen_at: '2026-11-01T00:00:00Z',
    last_seen_at: '2026-11-02T00:00:00Z',
    updated_at: '2026-11-02T00:00:00Z',
    title_affinity: 0,
    location_affinity: 0,
    remote_policy_accepted: true,
    match_reasons: { title: [], location: [] },
    last_event_at: null,
    saved_at: null,
    incomplete_reason: 'missing_company',
    incomplete_raw: {
      company: 'Mystery Co',
      title: 'Mystery Role',
      url: 'https://example.com/jobs/1',
    },
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
    offer_count: 0,
    note_count: 0,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/incomplete'
  vi.mocked(api.listCompanies).mockResolvedValue({
    companies: [makeCompany(2, 'Acme')],
    total: 1,
    limit: 500,
    offset: 0,
  })
  vi.mocked(api.listOffers).mockResolvedValue({
    offers: [makeIncomplete()],
    total: 1,
    limit: 200,
    offset: 0,
  })
})

describe('IncompletePage', () => {
  it('lists the count and reported values', async () => {
    render(<IncompletePage refreshToken={0} onDataChanged={vi.fn()} onNotice={vi.fn()} />)

    expect(await screen.findByText('1 item')).toBeTruthy()
    expect(screen.getByText('Missing company')).toBeTruthy()
    expect(screen.getByText('Mystery Co')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Mystery Role' })).toBeTruthy()
  })

  it('repairs an item with a company and title', async () => {
    const user = userEvent.setup()
    const onDataChanged = vi.fn()
    const onNotice = vi.fn()
    vi.mocked(api.repairOffer).mockResolvedValue({ offer: makeIncomplete() })

    render(<IncompletePage refreshToken={0} onDataChanged={onDataChanged} onNotice={onNotice} />)
    await user.click(await screen.findByRole('button', { name: 'Repair' }))

    const dialog = await screen.findByRole('dialog')
    const combobox = within(dialog).getByRole('combobox')
    await user.clear(combobox)
    await user.type(combobox, 'Acme')
    await user.click(within(dialog).getByRole('option', { name: 'Acme' }))

    const title = within(dialog).getByLabelText('Title')
    await user.clear(title)
    await user.type(title, 'Site Reliability Engineer')
    await user.click(within(dialog).getByRole('button', { name: 'Save repair' }))

    await waitFor(() =>
      expect(api.repairOffer).toHaveBeenCalledWith(1, {
        company_id: 2,
        title: 'Site Reliability Engineer',
        location: null,
      }),
    )
    expect(onNotice).toHaveBeenCalledWith('success', 'Offer #1 repaired.')
    expect(onDataChanged).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
