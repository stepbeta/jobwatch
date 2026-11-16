import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { CompanyPage } from './CompanyPage'
import styles from '../styles.css?raw'
import type { CompanyDetail } from '../types'

vi.mock('../api', () => ({
  api: {
    getCompany: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

import { api } from '../api'

const detail: CompanyDetail = {
  company: {
    id: 5,
    name: 'Acme',
    name_normalized: 'acme',
    website: 'https://acme.example',
    created_at: '2026-01-02T03:04:05Z',
    offer_count: 2,
    note_count: 1,
  },
  offers: [],
  notes: [
    {
      id: 7,
      entity_type: 'company',
      entity_id: 5,
      body: 'Recruiter: Jane',
      pinned_event_id: null,
      created_at: '2026-01-03T00:00:00Z',
      updated_at: null,
    },
  ],
  possible_duplicates: [
    { id: 9, name: 'Acme Corp', name_normalized: 'acme corp', score: 88, reason: 'name contains' },
  ],
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.getCompany).mockResolvedValue(detail)
})

describe('CompanyPage header layout', () => {
  it('renders identity and notes in one two-column block', async () => {
    render(<CompanyPage id={5} onNotice={vi.fn()} />)

    await screen.findByRole('heading', { name: 'Acme' })

    const header = document.querySelector('.company-header')
    expect(header).toBeTruthy()
    const identity = header?.querySelector('.company-identity') as HTMLElement
    const notes = header?.querySelector('.company-notes-panel') as HTMLElement
    expect(identity).toBeTruthy()
    expect(notes).toBeTruthy()

    // Left column: identity, actions, possible-duplicate banner.
    expect(within(identity).getByRole('heading', { name: 'Acme' })).toBeTruthy()
    expect(within(identity).getByRole('link', { name: 'https://acme.example' })).toBeTruthy()
    expect(within(identity).getByRole('button', { name: 'Edit' })).toBeTruthy()
    expect(within(identity).getByRole('button', { name: 'Delete company' })).toBeTruthy()
    expect(within(identity).getByRole('status')).toBeTruthy()
    expect(within(identity).getByText('Possible duplicate companies')).toBeTruthy()

    // Right column: unchanged company note list and new-note form.
    expect(within(notes).getByRole('heading', { name: 'Company notes' })).toBeTruthy()
    expect(within(notes).getByText('Recruiter: Jane')).toBeTruthy()
    expect(within(notes).getByLabelText('New company note')).toBeTruthy()
    expect(within(notes).getByRole('button', { name: 'Add note' })).toBeTruthy()

    // The offers list stays a separate panel below the unified block.
    const offersHeading = screen.getByRole('heading', { name: 'Offers' })
    expect(header?.contains(offersHeading)).toBe(false)
  })

  it('collapses the header grid at the shared 900px breakpoint', () => {
    expect(styles).toMatch(
      /\.company-header\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\) minmax\(0, 1\.1fr\);/,
    )
    expect(styles).toMatch(
      /@media \(max-width: 900px\) \{[\s\S]*?\.company-header\s*\{\s*grid-template-columns: 1fr;/,
    )
  })
})
