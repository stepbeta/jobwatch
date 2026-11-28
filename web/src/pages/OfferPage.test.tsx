import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { OfferPage } from './OfferPage'
import styles from '../styles.css?raw'
import { api } from '../api'
import type { Attachment, Note, OfferDetail } from '../types'

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
    uploadAttachment: vi.fn(),
    deleteAttachment: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

function makeAttachment(overrides: Partial<Attachment> = {}): Attachment {
  return {
    id: 3,
    offer_id: 7,
    sha256: '9f2c',
    kind: 'cv',
    filename: 'cv-2026.pdf',
    size: 122880,
    mime: 'application/pdf',
    attached_at: '2026-11-20T10:00:00Z',
    first_uploaded_at: '2026-10-02T09:00:00Z',
    pdf: {
      title: 'Acme CV',
      author: 'Alice Martin',
      producer: 'Test Producer',
      creation_date: '2026-10-02T08:30:00Z',
      modification_date: null,
      page_count: 2,
      version: '1.7',
    },
    download_url: '/api/offers/7/attachments/3/file',
    ...overrides,
  }
}

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
    attachments: [],
    snapshots: [],
    latest_snapshot: null,
    snapshot_diff: null,
    snapshot_count: 0,
    ...overrides,
  }
}

function renderPage() {
  const onNotice = vi.fn()
  const view = render(<OfferPage id={7} onNotice={onNotice} />)
  return { ...view, onNotice }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getOffer).mockResolvedValue(makeDetail())
  vi.mocked(api.uploadAttachment).mockResolvedValue({ attachment: makeAttachment() })
  vi.mocked(api.deleteAttachment).mockResolvedValue(undefined)
  vi.mocked(api.createEvent).mockResolvedValue({
    event: {
      id: 9,
      offer_id: 7,
      type: 'cv_sent',
      description: null,
      occurred_at: '2026-11-01T00:00:00Z',
      created_at: null,
    },
    application_state: 'none',
    state_changed: false,
  })
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
        occurred_at: new Date('2026-11-01T00:00').toISOString(),
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

  it('records a rejection instead of a message when the shortcut is used', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByRole('button', { name: 'Add event' })
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'message_received' } })
    await user.click(await screen.findByLabelText('This is a rejection'))
    await user.click(screen.getByRole('button', { name: 'Add event' }))

    await waitFor(() =>
      expect(api.createEvent).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ type: 'rejected' }),
      ),
    )
  })

  it('announces the application state change after adding an event', async () => {
    const user = userEvent.setup()
    vi.mocked(api.createEvent).mockResolvedValue({
      event: {
        id: 9,
        offer_id: 7,
        type: 'cv_sent',
        description: null,
        occurred_at: '2026-11-01T00:00:00Z',
        created_at: null,
      },
      application_state: 'applied',
      state_changed: true,
    })
    const { onNotice } = renderPage()

    await screen.findByRole('button', { name: 'Add event' })
    await user.click(screen.getByRole('button', { name: 'Add event' }))

    await waitFor(() =>
      expect(onNotice).toHaveBeenCalledWith(
        'success',
        'Timeline event added. Application state is now Applied.',
      ),
    )
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

describe('OfferPage header and description editor', () => {
  it('keeps identity and provenance side by side in the header panel', async () => {
    vi.mocked(api.getOffer).mockResolvedValue(
      makeDetail({
        provenance: [
          {
            id: 1,
            source_id: 1,
            source_name: 'Acme board',
            external_uid: 'uid-1',
            url: 'https://boards.greenhouse.io/acme/jobs/1',
            first_seen_at: '2026-11-01T00:00:00Z',
            last_seen_at: '2026-11-02T00:00:00Z',
          },
          {
            id: 2,
            source_id: 2,
            source_name: 'Acme careers',
            external_uid: null,
            url: 'https://acme.example/jobs/1',
            first_seen_at: '2026-11-01T00:00:00Z',
            last_seen_at: '2026-11-03T00:00:00Z',
          },
        ],
      }),
    )
    renderPage()

    await screen.findByRole('heading', { name: 'Site Reliability Engineer' })
    const header = document.querySelector('.offer-header') as HTMLElement
    expect(header).toBeTruthy()
    const identity = header.querySelector('.offer-identity') as HTMLElement
    const provenance = header.querySelector('.offer-provenance-panel') as HTMLElement
    expect(identity).toBeTruthy()
    expect(provenance).toBeTruthy()

    expect(within(identity).getByRole('heading', { name: 'Site Reliability Engineer' })).toBeTruthy()
    expect(within(identity).getByRole('button', { name: 'Merge into another offer…' })).toBeTruthy()
    expect(within(identity).getByRole('button', { name: 'Split offer…' })).toBeTruthy()

    expect(within(provenance).getByRole('heading', { name: 'Provenance' })).toBeTruthy()
    expect(within(provenance).getByText('boards.greenhouse.io')).toBeTruthy()
    expect(within(provenance).getByText(/Acme board/)).toBeTruthy()
    expect(within(provenance).getByText(/uid-1/)).toBeTruthy()
    expect(document.querySelector('details.panel.offer-header')).toBeNull()
    expect(styles).toMatch(
      /\.offer-header\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\) minmax\(0, 1\.1fr\);/,
    )
    expect(styles).toMatch(
      /@media \(max-width: 900px\) \{[\s\S]*?\.offer-header\s*\{\s*grid-template-columns: 1fr;/,
    )
  })

  it('edits the description through the shared Edit/View toggle and saves a manual snapshot', async () => {
    const user = userEvent.setup()
    const snap = {
      id: 3,
      origin: 'scan' as const,
      raw_text: 'old description',
      captured_at: '2026-11-01T00:00:00Z',
    }
    const withSnapshot = makeDetail({
      snapshots: [{ id: 3, origin: 'scan', captured_at: snap.captured_at, text_length: 15, raw_text: 'old description' }],
      latest_snapshot: snap,
      snapshot_count: 1,
    })
    vi.mocked(api.getOffer).mockResolvedValue(withSnapshot)
    vi.mocked(api.patchOffer).mockResolvedValue({ offer: withSnapshot.offer })
    renderPage()

    await user.click(await screen.findByRole('button', { name: 'Edit description' }))
    const snapshotPanel = document.querySelector('details.snapshot') as HTMLElement
    expect(within(snapshotPanel).getByRole('tab', { name: 'Edit' })).toBeTruthy()
    expect(within(snapshotPanel).getByRole('tab', { name: 'View' })).toBeTruthy()

    const textarea = within(snapshotPanel).getByLabelText('Description') as HTMLTextAreaElement
    expect(textarea.value).toBe('old description')
    await user.clear(textarea)
    await user.type(textarea, 'new description')
    await user.click(within(snapshotPanel).getByRole('tab', { name: 'View' }))
    expect(within(snapshotPanel).getByText('new description')).toBeTruthy()

    await user.click(within(snapshotPanel).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.patchOffer).toHaveBeenCalledWith(7, { description: 'new description' }),
    )
  })
})

describe('OfferPage classification signals', () => {
  it('shows title and location affinity with term reasons and the remote badge', async () => {    const base = makeDetail()
    vi.mocked(api.getOffer).mockResolvedValue({
      ...base,
      offer: {
        ...base.offer,
        title_affinity: 85,
        location_affinity: 60,
        match_reasons: {
          title: [{ term: 'site reliability', weight: 60, bucket: 'strong_positive' }],
          location: [{ term: 'france', weight: 35, bucket: 'strong_positive' }],
        },
      },
    })
    renderPage()

    expect(await screen.findByText('Title 85')).toBeTruthy()
    expect(screen.getByText('Location 60')).toBeTruthy()
    expect(screen.getByTitle(/Strong positive "site reliability" \(\+60\)/)).toBeTruthy()
    expect(screen.getByTitle(/Full remote — accepted/)).toBeTruthy()
  })

  it('shows a not-accepted remote policy from the profile', async () => {
    const base = makeDetail()
    vi.mocked(api.getOffer).mockResolvedValue({
      ...base,
      offer: { ...base.offer, remote_policy: 'hybrid', remote_policy_accepted: false },
    })
    renderPage()

    expect(await screen.findByTitle(/Hybrid — not accepted/)).toBeTruthy()
  })
})

describe('OfferPage browser identity', () => {
  it('titles the page with the offer and company but never the state', async () => {
    renderPage()

    await screen.findByRole('heading', { name: 'Site Reliability Engineer' })
    await waitFor(() =>
      expect(document.title).toBe('Site Reliability Engineer — Acme · JobWatch'),
    )
    expect(document.title).not.toContain('Applied')
  })

  it('uses the loading fallback title until the offer is known', () => {
    vi.mocked(api.getOffer).mockReturnValue(new Promise(() => {}))
    renderPage()

    expect(document.title).toBe('Offer — JobWatch')
  })

  it('shows the state favicon and restores the default on unmount', async () => {
    const base = makeDetail()
    vi.mocked(api.getOffer).mockResolvedValue({
      ...base,
      offer: { ...base.offer, application_state: 'interview' },
    })
    const { unmount } = renderPage()

    await screen.findByRole('heading', { name: 'Site Reliability Engineer' })
    const link = document.querySelector('link[rel~="icon"]') as HTMLLinkElement
    await waitFor(() =>
      expect(decodeURIComponent(link.getAttribute('href') ?? '')).toContain('#7c3aed'),
    )

    unmount()
    expect(link.getAttribute('href')).toBe('/favicon.svg')
  })

  it('updates the favicon when the application state changes', async () => {
    const user = userEvent.setup()
    const base = makeDetail()
    vi.mocked(api.getOffer)
      .mockResolvedValueOnce(base)
      .mockResolvedValueOnce({
        ...base,
        offer: { ...base.offer, application_state: 'interview' },
      })
    vi.mocked(api.patchOffer).mockResolvedValue({ offer: base.offer })
    renderPage()

    await screen.findByRole('heading', { name: 'Site Reliability Engineer' })
    const link = document.querySelector('link[rel~="icon"]') as HTMLLinkElement
    expect(link.getAttribute('href')).toMatch(/^data:image\/svg\+xml,/)

    await user.selectOptions(screen.getByLabelText('Application'), 'interview')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(decodeURIComponent(link.getAttribute('href') ?? '')).toContain('#7c3aed'),
    )
    expect(api.patchOffer).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ application_state: 'interview' }),
    )
  })
})

describe('OfferPage documents', () => {
  it('shows the empty state and the upload row when nothing is attached', async () => {
    renderPage()

    expect(await screen.findByText('No document is attached.')).toBeTruthy()
    expect(
      (screen.getByRole('button', { name: 'Upload' }) as HTMLButtonElement).disabled,
    ).toBe(true)
    expect(screen.queryByRole('button', { name: '+ Add another' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Detach' })).toBeNull()
  })

  it('enables the upload action only once a PDF is chosen, and can clear the choice', async () => {
    const user = userEvent.setup()
    renderPage()

    const upload = (await screen.findByRole('button', { name: 'Upload' })) as HTMLButtonElement
    expect(upload.disabled).toBe(true)
    expect(screen.getByText('Choose PDF…')).toBeTruthy()

    const file = new File(['%PDF-1.7\n'], 'letter.pdf', { type: 'application/pdf' })
    await user.upload(screen.getByLabelText('PDF file') as HTMLInputElement, file)

    expect(upload.disabled).toBe(false)
    const chip = screen.getByText('letter.pdf').closest('.file-chip') as HTMLElement
    expect(chip).toBeTruthy()
    expect(within(chip).getByText('9 B')).toBeTruthy()
    // The chooser button is replaced by the chip.
    expect(screen.queryByText('Choose PDF…')).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Remove chosen file' }))
    expect(upload.disabled).toBe(true)
    expect(screen.queryByText('letter.pdf')).toBeNull()
    expect(screen.getByText('Choose PDF…')).toBeTruthy()
  })

  it('reveals the upload row through Add another when documents exist', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getOffer).mockResolvedValue(makeDetail({ attachments: [makeAttachment()] }))
    renderPage()

    await screen.findByText('cv-2026.pdf')
    expect(screen.queryByRole('button', { name: 'Upload' })).toBeNull()

    await user.click(screen.getByRole('button', { name: '+ Add another' }))
    expect(screen.getByRole('button', { name: 'Upload' })).toBeTruthy()
  })

  it('filters documents by kind and shows the counts', async () => {
    const user = userEvent.setup()
    const cvA = makeAttachment()
    const cvB = makeAttachment({ id: 5, filename: 'cv-2026-v2.pdf', sha256: 'beef' })
    const letter = makeAttachment({
      id: 4,
      kind: 'cover_letter',
      filename: 'letter.pdf',
      download_url: '/api/offers/7/attachments/4/file',
    })
    vi.mocked(api.getOffer).mockResolvedValue(
      makeDetail({ attachments: [cvA, cvB, letter] }),
    )
    renderPage()

    await screen.findByText('cv-2026.pdf')
    const filters = screen.getByRole('group', { name: 'Filter documents by kind' })
    expect(within(filters).getByRole('button', { name: 'All 3' })).toBeTruthy()
    expect(within(filters).getByRole('button', { name: 'CV 2' })).toBeTruthy()
    expect(within(filters).getByRole('button', { name: 'Cover letter 1' })).toBeTruthy()

    await user.click(within(filters).getByRole('button', { name: 'CV 2' }))
    expect(screen.getByText('cv-2026.pdf')).toBeTruthy()
    expect(screen.getByText('cv-2026-v2.pdf')).toBeTruthy()
    expect(screen.queryByText('letter.pdf')).toBeNull()

    await user.click(within(filters).getByRole('button', { name: 'All 3' }))
    expect(screen.getByText('letter.pdf')).toBeTruthy()
  })

  it('hides the kind filters when every document has the same kind', async () => {
    vi.mocked(api.getOffer).mockResolvedValue(
      makeDetail({
        attachments: [makeAttachment(), makeAttachment({ id: 5, sha256: 'beef', filename: 'cv-2026-v2.pdf' })],
      }),
    )
    renderPage()

    await screen.findByText('cv-2026.pdf')
    expect(screen.queryByRole('group', { name: 'Filter documents by kind' })).toBeNull()
  })

  it('lists two documents with kind, size, pages and a download link', async () => {
    const cv = makeAttachment()
    const letter = makeAttachment({
      id: 4,
      kind: 'cover_letter',
      filename: 'letter.pdf',
      size: 2048,
      download_url: '/api/offers/7/attachments/4/file',
      pdf: {
        title: null,
        author: null,
        producer: null,
        creation_date: null,
        modification_date: null,
        page_count: null,
        version: null,
      },
    })
    vi.mocked(api.getOffer).mockResolvedValue(makeDetail({ attachments: [cv, letter] }))
    renderPage()

    await screen.findByText('cv-2026.pdf')
    const section = document.querySelector('section.documents') as HTMLElement
    expect(within(section).getByText('2 files')).toBeTruthy()
    const badges = Array.from(section.querySelectorAll('.kind-badge')).map((b) => b.textContent)
    expect(badges).toEqual(['CV', 'Cover letter'])
    expect(within(section).getByText('cv-2026.pdf')).toBeTruthy()
    expect(within(section).getByText('letter.pdf')).toBeTruthy()
    expect(within(section).getByText('120 KB')).toBeTruthy()
    expect(within(section).getByText('2 KB')).toBeTruthy()
    expect(within(section).getByText('2 pages')).toBeTruthy()

    const downloads = within(section).getAllByRole('link', { name: 'Download' })
    expect(downloads).toHaveLength(2)
    expect(downloads[0].getAttribute('href')).toBe('/api/offers/7/attachments/3/file')
    expect(downloads[0].getAttribute('download')).toBe('cv-2026.pdf')
    expect(downloads[1].getAttribute('href')).toBe('/api/offers/7/attachments/4/file')
  })

  it('uploads a PDF with the selected kind and reloads the offer', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage()

    const file = new File(['%PDF-1.7\n'], 'letter.pdf', {
      type: 'application/pdf',
      lastModified: Date.UTC(2020, 0, 1),
    })
    const input = (await screen.findByLabelText('PDF file')) as HTMLInputElement
    await user.upload(input, file)
    await user.selectOptions(screen.getByLabelText('Kind'), 'cover_letter')
    await user.click(screen.getByRole('button', { name: 'Upload' }))

    await waitFor(() =>
      expect(api.uploadAttachment).toHaveBeenCalledWith(7, file, 'cover_letter'),
    )
    await waitFor(() => expect(api.getOffer).toHaveBeenCalledTimes(2))
    expect(onNotice).toHaveBeenCalledWith('success', 'Document attached.')
    expect(input.value).toBe('')
  })

  it('detaches a document after confirmation', async () => {
    const user = userEvent.setup()
    const attachment = makeAttachment()
    vi.mocked(api.getOffer).mockResolvedValue(makeDetail({ attachments: [attachment] }))
    const { onNotice } = renderPage()

    await screen.findByText('cv-2026.pdf')
    const section = document.querySelector('section.documents') as HTMLElement
    await user.click(within(section).getByRole('button', { name: 'Detach' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/The stored file is deleted only/)).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Detach document' }))

    await waitFor(() => expect(api.deleteAttachment).toHaveBeenCalledWith(7, 3))
    await waitFor(() => expect(api.getOffer).toHaveBeenCalledTimes(2))
    expect(onNotice).toHaveBeenCalledWith('success', 'Document detached.')
  })

  it('labels the PDF dates and the two JobWatch times without a browser modification time', async () => {
    const attachment = makeAttachment({
      pdf: {
        title: null,
        author: null,
        producer: null,
        creation_date: null,
        modification_date: null,
        page_count: null,
        version: null,
      },
    })
    vi.mocked(api.getOffer).mockResolvedValue(makeDetail({ attachments: [attachment] }))
    renderPage()

    await screen.findByText('cv-2026.pdf')
    expect(screen.getByText(/PDF creation: Unknown/)).toBeTruthy()
    expect(screen.getByText(/PDF modified: Unknown/)).toBeTruthy()
    expect(screen.getByText(/First uploaded:/)).toBeTruthy()
    expect(screen.getByText(/Attached:/)).toBeTruthy()
    expect(screen.getByText(/The PDF creation and modification dates come from the file/)).toBeTruthy()
    expect(screen.queryByText(/last modified/i)).toBeNull()
    expect(screen.queryByText(/file modified/i)).toBeNull()
  })
})
