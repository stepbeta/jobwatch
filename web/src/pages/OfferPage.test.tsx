import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { OfferPage } from './OfferPage'
import { api } from '../api'
import type { Note, OfferDetail } from '../types'

vi.mock('../api', () => ({
  api: {
    getOffer: vi.fn(),
    patchOffer: vi.fn(),
    createEvent: vi.fn(),
    updateEvent: vi.fn(),
    deleteEvent: vi.fn(),
    createNote: vi.fn(),
    updateNote: vi.fn(),
    deleteNote: vi.fn(),
    getSnapshotText: vi.fn(),
    getSnapshotDiff: vi.fn(),
    listOffers: vi.fn(),
    mergeOffer: vi.fn(),
    splitOffer: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

function makeNote(overrides: Partial<Note> = {}): Note {
  return {
    id: 2,
    entity_type: 'offer',
    entity_id: 7,
    body: 'prep notes',
    pinned_event_id: 4,
    created_at: '2026-11-12T11:00:00Z',
    updated_at: null,
    ...overrides,
  }
}

function makeDetail(overrides: Partial<OfferDetail> = {}): OfferDetail {
  return {
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
      title_confidence: 80,
      location_confidence: 80,
      match_reasons: null,
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
    events: [
      {
        id: 4,
        offer_id: 7,
        type: 'interview',
        description: 'call with the team',
        occurred_at: '2026-11-12T10:00:00Z',
        created_at: null,
      },
    ],
    notes: [makeNote()],
    snapshots: [],
    latest_snapshot: null,
    snapshot_diff: null,
    snapshot_count: 0,
    ...overrides,
  }
}

function renderPage() {
  return render(<OfferPage id={7} onNotice={vi.fn()} />)
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getOffer).mockResolvedValue(makeDetail())
})

describe('OfferPage timeline events', () => {
  it('saves a backdated event from the inline editor', async () => {
    const user = userEvent.setup()
    renderPage()

    const editButton = await screen.findByRole('button', { name: 'Edit event' })
    const row = editButton.closest('li') as HTMLElement
    await user.click(editButton)

    fireEvent.change(within(row).getByLabelText('Type'), { target: { value: 'cv_sent' } })
    fireEvent.change(within(row).getByLabelText('Occurred at date'), {
      target: { value: '2026-11-01' },
    })
    fireEvent.change(within(row).getByLabelText('Occurred at time (optional)'), {
      target: { value: '' },
    })
    fireEvent.change(within(row).getByLabelText('Description'), {
      target: { value: 'sent again' },
    })
    await user.click(within(row).getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.updateEvent).toHaveBeenCalledWith(7, 4, {
        type: 'cv_sent',
        description: 'sent again',
        occurred_at: '2026-11-01',
      }),
    )
  })

  it('discards the edit without a request when cancelled', async () => {
    const user = userEvent.setup()
    renderPage()

    const editButton = await screen.findByRole('button', { name: 'Edit event' })
    const row = editButton.closest('li') as HTMLElement
    await user.click(editButton)
    await user.click(within(row).getByRole('button', { name: 'Cancel' }))

    expect(api.updateEvent).not.toHaveBeenCalled()
    expect(within(row).getByText('call with the team')).toBeTruthy()
  })

  it('shows a pinned-note cue with the event type and date and highlights the event', async () => {
    const user = userEvent.setup()
    renderPage()

    const cue = await screen.findByRole('button', { name: /Interview ·/ })
    expect(cue.textContent).toContain('Interview ·')

    await user.click(cue)
    expect(document.getElementById('event-4')?.className).toContain('timeline-highlight')
  })

  it('marks an event with its pinned-note count and expands the notes inline', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getOffer).mockResolvedValue(
      makeDetail({
        notes: [
          makeNote({ id: 2, body: 'prep notes' }),
          makeNote({ id: 3, body: 'ask about on-call' }),
        ],
      }),
    )
    renderPage()

    const marker = await screen.findByTitle('2 pinned notes')
    const row = document.getElementById('event-4') as HTMLElement
    expect(within(row).queryByText('ask about on-call')).toBeNull()

    await user.click(marker)
    expect(within(row).getByText('prep notes')).toBeTruthy()
    expect(within(row).getByText('ask about on-call')).toBeTruthy()
  })

  it('clears the note pin display after deleting the pinned event', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getOffer)
      .mockResolvedValueOnce(makeDetail())
      .mockResolvedValueOnce(
        makeDetail({
          events: [],
          notes: [makeNote({ pinned_event_id: null })],
        }),
      )
    vi.mocked(api.deleteEvent).mockResolvedValue(undefined)
    renderPage()

    expect(await screen.findByTitle('Show this event in the timeline')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Delete event' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete event' }))

    await waitFor(() => expect(api.deleteEvent).toHaveBeenCalledWith(7, 4))
    await waitFor(() =>
      expect(screen.queryByTitle('Show this event in the timeline')).toBeNull(),
    )
    expect(screen.getByText('prep notes')).toBeTruthy()
  })
})
