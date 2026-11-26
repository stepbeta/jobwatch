import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { DocumentsPage } from './DocumentsPage'
import { api } from '../api'
import type { DocumentItem } from '../types'

vi.mock('../api', () => ({
  api: {
    listDocuments: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

function makeDocument(overrides: Partial<DocumentItem> = {}): DocumentItem {
  return {
    sha256: '9f2c4d5e6f708192a3b4c5d6e7f80912a3b4c5d6e7f80912a3b4c5d6e7f80912',
    size: 122880,
    mime: 'application/pdf',
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
    download_url: '/api/documents/9f2c/file',
    offers: [
      {
        attachment_id: 3,
        offer_id: 7,
        offer_title: 'Site Reliability Engineer',
        company_name: 'Acme',
        kind: 'cv',
        attached_at: '2026-11-20T10:00:00Z',
      },
    ],
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('DocumentsPage', () => {
  it('lists one row per stored file with every referencing offer and a download', async () => {
    const shared = makeDocument({
      offers: [
        {
          attachment_id: 3,
          offer_id: 7,
          offer_title: 'Site Reliability Engineer',
          company_name: 'Acme',
          kind: 'cv',
          attached_at: '2026-11-20T10:00:00Z',
        },
        {
          attachment_id: 4,
          offer_id: 8,
          offer_title: 'Platform Engineer',
          company_name: 'Acme',
          kind: 'other',
          attached_at: '2026-11-21T10:00:00Z',
        },
      ],
    })
    const other = makeDocument({
      sha256: 'aaaa4d5e6f708192a3b4c5d6e7f80912a3b4c5d6e7f80912a3b4c5d6e7f80912',
      pdf: { ...makeDocument().pdf, title: 'Cover letter' },
      download_url: '/api/documents/aaaa/file',
      offers: [
        {
          attachment_id: 5,
          offer_id: 9,
          offer_title: 'SRE',
          company_name: 'Beta',
          kind: 'cover_letter',
          attached_at: '2026-11-22T10:00:00Z',
        },
      ],
    })
    vi.mocked(api.listDocuments).mockResolvedValue({ documents: [shared, other] })
    render(<DocumentsPage />)

    await screen.findByText('Acme CV')
    expect(screen.getAllByText('Acme CV')).toHaveLength(1)
    expect(screen.getByText('2 files')).toBeTruthy()

    const offerLink = screen.getByRole('link', { name: 'Site Reliability Engineer' })
    expect(offerLink.getAttribute('href')).toBe('#/offers/7')
    expect(screen.getByRole('link', { name: 'Platform Engineer' }).getAttribute('href')).toBe(
      '#/offers/8',
    )
    expect(screen.getByRole('link', { name: 'SRE' }).getAttribute('href')).toBe('#/offers/9')

    expect(screen.getAllByRole('link', { name: 'Download' })[0].getAttribute('href')).toBe(
      '/api/documents/9f2c/file',
    )
    expect(screen.getAllByRole('link', { name: 'Download' })).toHaveLength(2)
    expect(screen.getAllByText(/Attached:/).length).toBeGreaterThanOrEqual(3)
  })

  it('renders unknown-metadata placeholders instead of guessing', async () => {
    const unknown = makeDocument({
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
    vi.mocked(api.listDocuments).mockResolvedValue({ documents: [unknown] })
    render(<DocumentsPage />)

    await screen.findByRole('heading', { name: 'PDF document' })
    expect(screen.getAllByText(/Unknown/).length).toBeGreaterThanOrEqual(4)
    expect(screen.getByText(/First uploaded:/)).toBeTruthy()
    expect(screen.getByText(/The PDF creation and modification dates come from the file/)).toBeTruthy()
  })

  it('stays read-only: no upload, rename or delete action', async () => {
    vi.mocked(api.listDocuments).mockResolvedValue({ documents: [makeDocument()] })
    const { container } = render(<DocumentsPage />)

    await screen.findByText('Acme CV')
    expect(screen.queryByRole('button', { name: /upload|rename|delete|detach/i })).toBeNull()
    expect(container.querySelector('input[type="file"]')).toBeNull()
    expect(screen.getAllByRole('link', { name: 'Download' })).toHaveLength(1)
  })

  it('shows an empty state when nothing is stored', async () => {
    vi.mocked(api.listDocuments).mockResolvedValue({ documents: [] })
    render(<DocumentsPage />)

    await waitFor(() => expect(screen.getByText('No documents stored yet.')).toBeTruthy())
  })
})
