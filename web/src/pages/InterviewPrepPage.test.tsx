import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { InterviewPrepPage } from './InterviewPrepPage'
import { api } from '../api'
import type { InterviewEntry } from '../types'

vi.mock('../api', () => ({
  api: {
    listInterviewEntries: vi.fn(),
    createInterviewEntry: vi.fn(),
    updateInterviewEntry: vi.fn(),
    deleteInterviewEntry: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

const incident: InterviewEntry = {
  id: 1,
  question: 'Tell me about a production incident.',
  answer: 'Two years of on-call rotation.',
  answer_format: 'markdown',
  display_order: null,
  created_at: '2026-11-16T09:00:00Z',
  updated_at: '2026-11-16T09:00:00Z',
}

const motivation: InterviewEntry = {
  id: 2,
  question: 'Why this company?',
  answer: '',
  answer_format: 'markdown',
  display_order: null,
  created_at: '2026-11-16T09:05:00Z',
  updated_at: '2026-11-16T09:05:00Z',
}

function renderPage(entries: InterviewEntry[] = [incident, motivation]) {
  vi.mocked(api.listInterviewEntries).mockResolvedValue({ interview_entries: entries })
  const onNotice = vi.fn()
  const view = render(<InterviewPrepPage onNotice={onNotice} />)
  return { ...view, onNotice }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.listInterviewEntries).mockResolvedValue({ interview_entries: [incident, motivation] })
  vi.mocked(api.createInterviewEntry).mockResolvedValue({
    interview_entry: { ...incident, id: 3, question: 'New question' },
  })
  vi.mocked(api.updateInterviewEntry).mockResolvedValue({
    interview_entry: { ...incident, question: 'Updated question' },
  })
  vi.mocked(api.deleteInterviewEntry).mockResolvedValue(undefined)
})

describe('InterviewPrepPage accordion', () => {
  it('expands and collapses an answer from the question header', async () => {
    const user = userEvent.setup()
    renderPage()

    const toggle = await screen.findByRole('button', { name: incident.question })
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByText(/on-call rotation/)).toBeNull()

    await user.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText(/on-call rotation/)).toBeTruthy()

    await user.click(toggle)
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    expect(screen.queryByText(/on-call rotation/)).toBeNull()
  })

  it('shows a hint for an entry without an answer', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: motivation.question }))
    expect(screen.getByText(/No answer yet/)).toBeTruthy()
  })
})

describe('InterviewPrepPage markdown', () => {
  it('renders markdown and escapes raw HTML instead of executing it', async () => {
    const user = userEvent.setup()
    const unsafe: InterviewEntry = {
      ...incident,
      answer:
        '## Incident\n\n' +
        '**Bold** and *italic* and `kubectl`\n\n' +
        '- first item\n- second item\n\n' +
        '```sh\nkubectl get pods\n```\n\n' +
        '<script>alert("x")</script>',
    }
    const { container } = renderPage([unsafe])

    await user.click(await screen.findByRole('button', { name: unsafe.question }))

    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByRole('heading', { name: 'Incident' })).toBeTruthy()
    expect(screen.getByText('Bold').tagName).toBe('STRONG')
    expect(screen.getByText('italic').tagName).toBe('EM')
    expect(screen.getByText('kubectl').tagName).toBe('CODE')
    expect(screen.getAllByRole('listitem').map((li) => li.textContent)).toEqual([
      'first item',
      'second item',
    ])
    expect(screen.getByText('kubectl get pods')).toBeTruthy()
    expect(screen.getByText('<script>alert("x")</script>')).toBeTruthy()
  })
})

describe('InterviewPrepPage editor', () => {
  it('creates an entry from the add modal with a live preview', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage()

    await screen.findByRole('button', { name: incident.question })
    await user.click(screen.getByRole('button', { name: 'Add' }))
    const dialog = await screen.findByRole('dialog')

    await user.type(within(dialog).getByLabelText('Question'), 'New question')
    await user.type(within(dialog).getByLabelText('Answer (markdown)'), '**prepared** answer')

    // The preview pane renders the markdown while typing.
    expect(within(dialog).getByText('prepared').tagName).toBe('STRONG')

    await user.click(within(dialog).getByRole('button', { name: 'Add entry' }))
    await waitFor(() =>
      expect(api.createInterviewEntry).toHaveBeenCalledWith({
        question: 'New question',
        answer: '**prepared** answer',
      }),
    )
    expect(onNotice).toHaveBeenCalledWith('success', 'Interview entry added.')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('edits an entry in the same modal', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage([incident])

    await user.click(await screen.findByRole('button', { name: `Edit ${incident.question}` }))
    const dialog = await screen.findByRole('dialog')
    const question = within(dialog).getByLabelText('Question') as HTMLInputElement
    expect(question.value).toBe(incident.question)

    await user.clear(question)
    await user.type(question, 'Updated question')
    const answer = within(dialog).getByLabelText('Answer (markdown)')
    await user.clear(answer)
    await user.type(answer, 'Updated **answer**')
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(api.updateInterviewEntry).toHaveBeenCalledWith(incident.id, {
        question: 'Updated question',
        answer: 'Updated **answer**',
      }),
    )
    expect(onNotice).toHaveBeenCalledWith('success', 'Interview entry saved.')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('deletes an entry after the shared confirmation', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage([incident, motivation])

    await user.click(await screen.findByRole('button', { name: `Delete ${incident.question}` }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(api.deleteInterviewEntry).toHaveBeenCalledWith(incident.id))
    expect(onNotice).toHaveBeenCalledWith('success', 'Interview entry deleted.')
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: incident.question })).toBeNull(),
    )
    expect(screen.getByRole('button', { name: motivation.question })).toBeTruthy()
  })
})
