import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { SearchProfile } from '../types'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    api: {
      listSearchProfiles: vi.fn(),
      replaceSearchProfile: vi.fn(),
    },
  }
})

import { ApiError, api } from '../api'
import { SettingsPage } from './SettingsPage'

function makeProfile(overrides: Partial<SearchProfile> = {}): SearchProfile {
  return {
    id: 1,
    name: 'Default',
    title_label: 'Platform Engineer / SRE',
    location_label: 'France / full remote',
    bucket_scale: 'five',
    is_default: true,
    terms: [
      {
        id: 1,
        profile_id: 1,
        affinity: 'title',
        bucket: 'strong_positive',
        term: 'platform engineer',
        weight: 60,
        sort_order: 0,
      },
      {
        id: 2,
        profile_id: 1,
        affinity: 'location',
        bucket: 'mild_positive',
        term: 'france',
        weight: 35,
        sort_order: 0,
      },
      {
        id: 3,
        profile_id: 1,
        affinity: 'description',
        bucket: 'mild_positive',
        term: 'paris',
        weight: 15,
        sort_order: 0,
      },
    ],
    remote: [
      { policy: 'remote', accepted: true },
      { policy: 'hybrid', accepted: false },
      { policy: 'onsite', accepted: false },
      { policy: 'unknown', accepted: true },
    ],
    created_at: null,
    updated_at: null,
    ...overrides,
  }
}

function renderPage() {
  const onNotice = vi.fn()
  const view = render(<SettingsPage onNotice={onNotice} />)
  return { onNotice, view }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/settings'
  vi.mocked(api.listSearchProfiles).mockResolvedValue({ profiles: [makeProfile()] })
  vi.mocked(api.replaceSearchProfile).mockImplementation(async (_id, input) => ({
    profile: makeProfile({
      name: input.name,
      bucket_scale: input.bucket_scale,
      terms: input.terms.map((t, i) => ({ ...t, id: i + 1, profile_id: 1, sort_order: i })),
      remote: input.remote,
    }),
  }))
})

describe('SettingsPage', () => {
  it('loads the default profile and exposes every tab', async () => {
    renderPage()

    expect(await screen.findByDisplayValue('Default')).toBeTruthy()
    expect(screen.getByDisplayValue('Platform Engineer / SRE')).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Profile' }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('tab', { name: 'Title terms' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Location terms' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Description' })).toBeTruthy()
    expect(screen.getByRole('tab', { name: 'Remote' })).toBeTruthy()
    expect(screen.getByText(/Buckets are editor labels/)).toBeTruthy()
  })

  it('shows each field group on its own tab with its explanation', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')

    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    expect(screen.getAllByLabelText('title term')[0]).toHaveProperty('value', 'platform engineer')
    expect(screen.getByText(/Positive title terms raise title affinity/)).toBeTruthy()

    await user.click(screen.getByRole('tab', { name: 'Location terms' }))
    expect(screen.getByDisplayValue('france')).toBeTruthy()
    expect(screen.getByText(/Location terms match the offer location/)).toBeTruthy()

    await user.click(screen.getByRole('tab', { name: 'Description' }))
    expect(screen.getByText(/Description terms add weaker location evidence only/)).toBeTruthy()
    expect(screen.getByDisplayValue('paris')).toBeTruthy()

    await user.click(screen.getByRole('tab', { name: 'Remote' }))
    expect(screen.getByText(/membership check, not a weighted score/)).toBeTruthy()
  })

  it('switches the bucket scale, maps buckets and saves the new scale', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage()
    await screen.findByDisplayValue('Default')

    await user.selectOptions(screen.getByLabelText('Bucket scale'), 'three')
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    const buckets = screen.getAllByLabelText(/title bucket for/) as HTMLSelectElement[]
    expect(buckets[0].value).toBe('positive')

    await user.click(screen.getByRole('button', { name: 'Save profile' }))
    await waitFor(() =>
      expect(api.replaceSearchProfile).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          bucket_scale: 'three',
          terms: expect.arrayContaining([
            expect.objectContaining({ affinity: 'title', bucket: 'positive', term: 'platform engineer' }),
          ]),
        }),
      ),
    )
    expect(onNotice).toHaveBeenCalledWith('success', expect.stringContaining('Search profile saved'))
  })

  it('sets a default weight from the bucket and lets the user fine-tune it', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))

    const bucketSelect = screen.getByLabelText('title bucket for platform engineer')
    await user.selectOptions(bucketSelect, 'mild_negative')
    const weight = screen.getByLabelText('title weight for platform engineer') as HTMLInputElement
    expect(weight.value).toBe('-20')

    fireEvent.change(weight, { target: { value: '-5' } })
    expect(weight.value).toBe('-5')
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    await waitFor(() =>
      expect(api.replaceSearchProfile).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          terms: expect.arrayContaining([
            expect.objectContaining({ term: 'platform engineer', weight: -5, bucket: 'mild_negative' }),
          ]),
        }),
      ),
    )
  })

  it('edits remote policy checkboxes and sends the full selection', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')
    await user.click(screen.getByRole('tab', { name: 'Remote' }))

    await user.click(screen.getByRole('checkbox', { name: 'Hybrid' }))
    await user.click(screen.getByRole('checkbox', { name: 'Unknown' }))
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    await waitFor(() =>
      expect(api.replaceSearchProfile).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          remote: [
            { policy: 'remote', accepted: true },
            { policy: 'hybrid', accepted: true },
            { policy: 'onsite', accepted: false },
            { policy: 'unknown', accepted: false },
          ],
        }),
      ),
    )
  })

  it('adds and removes a term row', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')
    await user.click(screen.getByRole('tab', { name: 'Location terms' }))

    await user.click(screen.getByRole('button', { name: 'Add location term' }))
    const terms = screen.getAllByLabelText('location term') as HTMLInputElement[]
    expect(terms).toHaveLength(2)
    await user.type(terms[1], 'lyon')

    const section = terms[1].closest('fieldset') as HTMLElement
    const rows = within(section).getAllByRole('row')
    await user.click(within(rows[rows.length - 1]).getByRole('button', { name: 'Remove' }))
    expect(screen.getAllByLabelText('location term')).toHaveLength(1)
  })

  it('switches tabs without losing unsaved edits', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))

    const title = screen.getAllByLabelText('title term')[0] as HTMLInputElement
    await user.clear(title)
    await user.type(title, 'site reliability')

    await user.click(screen.getByRole('tab', { name: 'Remote' }))
    expect(screen.queryByLabelText('title term')).toBeNull()
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    expect((screen.getAllByLabelText('title term')[0] as HTMLInputElement).value).toBe(
      'site reliability',
    )
  })

  it('applies edits from several tabs in one save', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')

    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    const title = screen.getAllByLabelText('title term')[0] as HTMLInputElement
    await user.clear(title)
    await user.type(title, 'sre')

    await user.click(screen.getByRole('tab', { name: 'Remote' }))
    await user.click(screen.getByRole('checkbox', { name: 'Hybrid' }))
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    await waitFor(() => expect(api.replaceSearchProfile).toHaveBeenCalledTimes(1))
    expect(api.replaceSearchProfile).toHaveBeenCalledWith(
      1,
      expect.objectContaining({
        terms: expect.arrayContaining([
          expect.objectContaining({ affinity: 'title', term: 'sre' }),
        ]),
        remote: expect.arrayContaining([{ policy: 'hybrid', accepted: true }]),
      }),
    )
  })

  it('writes the active tab to the URL and restores it on load', async () => {
    const user = userEvent.setup()
    window.location.hash = '#/settings?tab=remote'
    const { view } = renderPage()
    await screen.findByRole('tab', { name: 'Remote' })

    expect(screen.getByRole('tab', { name: 'Remote' }).getAttribute('aria-selected')).toBe('true')
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    await waitFor(() => expect(window.location.hash).toContain('tab=title'))

    view.unmount()
    window.location.hash = '#/settings?tab=description'
    renderPage()
    await screen.findByRole('tab', { name: 'Description' })
    expect(screen.getByRole('tab', { name: 'Description' }).getAttribute('aria-selected')).toBe(
      'true',
    )
  })

  it('rejects an empty term on its tab before calling the API', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')
    await user.click(screen.getByRole('tab', { name: 'Title terms' }))

    await user.clear(screen.getAllByLabelText('title term')[0])
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(await screen.findByText(/Every term needs a value/)).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Title terms/ }).getAttribute('aria-selected')).toBe(
      'true',
    )
    expect(screen.getByRole('tab', { name: /Title terms/ }).querySelector('[aria-label="has errors"]')).toBeTruthy()
    expect(api.replaceSearchProfile).not.toHaveBeenCalled()
  })

  it('marks every invalid tab and opens the first one on save', async () => {
    const user = userEvent.setup()
    renderPage()
    await screen.findByDisplayValue('Default')

    await user.click(screen.getByRole('tab', { name: 'Title terms' }))
    await user.clear(screen.getAllByLabelText('title term')[0])
    await user.click(screen.getByRole('tab', { name: 'Location terms' }))
    await user.clear(screen.getAllByLabelText('location term')[0])
    await user.click(screen.getByRole('tab', { name: 'Profile' }))
    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(await screen.findByText(/Every term needs a value/)).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Title terms/ }).getAttribute('aria-selected')).toBe(
      'true',
    )
    expect(screen.getByRole('tab', { name: /Location terms/ }).querySelector('[aria-label="has errors"]')).toBeTruthy()
  })

  it('switches to the tab named by a server validation error', async () => {
    const user = userEvent.setup()
    vi.mocked(api.replaceSearchProfile).mockRejectedValue(
      new ApiError('validation_failed', 'invalid search profile: duplicate title term "sre"', 400),
    )
    const { onNotice } = renderPage()
    await screen.findByDisplayValue('Default')

    await user.click(screen.getByRole('button', { name: 'Save profile' }))

    expect(await screen.findByText(/duplicate title term/)).toBeTruthy()
    expect(screen.getByRole('tab', { name: /Title terms/ }).getAttribute('aria-selected')).toBe(
      'true',
    )
    expect(onNotice).toHaveBeenCalledWith('error', expect.stringContaining('duplicate title term'))
    expect(screen.getByRole('button', { name: 'Save profile' })).toBeTruthy()
  })
})
