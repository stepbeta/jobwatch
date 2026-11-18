import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
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

const CATEGORY_ORDER_KEY = 'jobwatch.interview.categoryOrder'

const incident: InterviewEntry = {
  id: 1,
  question: 'Tell me about a production incident.',
  answer: 'Two years of on-call rotation.',
  answer_format: 'markdown',
  category: 'Behavioural',
  display_order: null,
  created_at: '2026-11-16T09:00:00Z',
  updated_at: '2026-11-16T09:00:00Z',
}

const motivation: InterviewEntry = {
  id: 2,
  question: 'Why this company?',
  answer: '',
  answer_format: 'markdown',
  category: 'Behavioural',
  display_order: null,
  created_at: '2026-11-16T09:05:00Z',
  updated_at: '2026-11-16T09:05:00Z',
}

const values: InterviewEntry = {
  id: 3,
  question: 'What are your core values?',
  answer: 'Ownership and pragmatism.',
  answer_format: 'markdown',
  category: 'Values',
  display_order: null,
  created_at: '2026-11-16T09:10:00Z',
  updated_at: '2026-11-16T09:10:00Z',
}

const uncategorized: InterviewEntry = {
  id: 4,
  question: 'Any other question?',
  answer: '',
  answer_format: 'markdown',
  category: null,
  display_order: null,
  created_at: '2026-11-16T09:15:00Z',
  updated_at: '2026-11-16T09:15:00Z',
}

function renderPage(entries: InterviewEntry[] = [incident, motivation]) {
  vi.mocked(api.listInterviewEntries).mockResolvedValue({ interview_entries: entries })
  const onNotice = vi.fn()
  const view = render(<InterviewPrepPage onNotice={onNotice} />)
  return { ...view, onNotice }
}

function regionNames(): string[] {
  return screen
    .getAllByRole('region')
    .map((region) => region.getAttribute('aria-label') ?? '')
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  vi.mocked(api.listInterviewEntries).mockResolvedValue({
    interview_entries: [incident, motivation],
  })
  vi.mocked(api.createInterviewEntry).mockResolvedValue({
    interview_entry: { ...incident, id: 5, question: 'New question', category: null },
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
    const cue = await screen.findByText('No answer')
    expect(cue.className).toContain('interview-cue')

    await user.click(await screen.findByRole('button', { name: motivation.question }))
    expect(screen.getByText(/No answer yet/)).toBeTruthy()
  })
})

describe('InterviewPrepPage sections', () => {
  it('groups entries by category, sorts categories alphabetically and puts uncategorized last', async () => {
    renderPage([uncategorized, values, incident, motivation])
    await screen.findByRole('region', { name: 'Behavioural' })

    expect(regionNames()).toEqual(['Behavioural', 'Values', 'Uncategorized'])

    const behavioural = screen.getByRole('region', { name: 'Behavioural' })
    const valuesSection = screen.getByRole('region', { name: 'Values' })
    const uncategorizedSection = screen.getByRole('region', { name: 'Uncategorized' })
    expect(within(behavioural).getByRole('button', { name: incident.question })).toBeTruthy()
    expect(within(valuesSection).getByRole('button', { name: values.question })).toBeTruthy()
    expect(
      within(uncategorizedSection).getByRole('button', { name: uncategorized.question }),
    ).toBeTruthy()
  })

  it('persists a dragged section order and resets to alphabetical', async () => {
    const user = userEvent.setup()
    const first = renderPage([incident, values])
    await screen.findByRole('region', { name: 'Behavioural' })

    const valuesHeader = screen.getByRole('heading', { name: 'Values' }).parentElement
    const behaviouralSection = screen.getByRole('region', { name: 'Behavioural' })
    expect(valuesHeader).not.toBeNull()

    fireEvent.dragStart(valuesHeader!)
    fireEvent.dragOver(behaviouralSection)
    fireEvent.drop(behaviouralSection)

    await waitFor(() =>
      expect(localStorage.getItem(CATEGORY_ORDER_KEY)).toBe(
        JSON.stringify(['Values', 'Behavioural']),
      ),
    )
    expect(regionNames()).toEqual(['Values', 'Behavioural'])

    // A fresh mount restores the saved presentation order.
    first.unmount()
    renderPage([incident, values])
    await screen.findByRole('region', { name: 'Values' })
    expect(regionNames()).toEqual(['Values', 'Behavioural'])

    await user.click(screen.getByRole('button', { name: 'Reset to alphabetical' }))
    expect(localStorage.getItem(CATEGORY_ORDER_KEY)).toBeNull()
    await waitFor(() => expect(regionNames()).toEqual(['Behavioural', 'Values']))
  })
})

describe('InterviewPrepPage drag and drop', () => {
  it('reorders entries inside a section with fresh display_order values', async () => {
    const { container } = renderPage([incident, motivation])
    await screen.findByRole('button', { name: incident.question })

    const dragged = container.querySelector('[data-entry-id="2"]')
    const target = container.querySelector('[data-entry-id="1"]')
    expect(dragged).not.toBeNull()
    expect(target).not.toBeNull()

    fireEvent.dragStart(dragged!)
    fireEvent.dragOver(target!)
    fireEvent.drop(target!)

    await waitFor(() => expect(api.updateInterviewEntry).toHaveBeenCalledTimes(2))
    expect(api.updateInterviewEntry).toHaveBeenCalledWith(2, { display_order: 1 })
    expect(api.updateInterviewEntry).toHaveBeenCalledWith(1, { display_order: 2 })
  })

  it('moves an entry across sections and patches its category', async () => {
    const { container } = renderPage([incident, values])
    await screen.findByRole('region', { name: 'Values' })

    const dragged = container.querySelector('[data-entry-id="1"]')
    const targetSection = screen.getByRole('region', { name: 'Values' })
    expect(dragged).not.toBeNull()

    fireEvent.dragStart(dragged!)
    fireEvent.dragOver(targetSection)
    fireEvent.drop(targetSection)

    await waitFor(() => expect(api.updateInterviewEntry).toHaveBeenCalledTimes(2))
    expect(api.updateInterviewEntry).toHaveBeenCalledWith(3, { display_order: 1 })
    expect(api.updateInterviewEntry).toHaveBeenCalledWith(1, {
      display_order: 2,
      category: 'Values',
    })
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
  it('toggles the answer area between Edit and View', async () => {
    const user = userEvent.setup()
    renderPage([incident])

    await user.click(await screen.findByRole('button', { name: `Edit ${incident.question}` }))
    const dialog = await screen.findByRole('dialog')

    const answer = within(dialog).getByLabelText('Answer (markdown)')
    await user.clear(answer)
    await user.type(answer, '**prepared** answer')

    await user.click(within(dialog).getByRole('tab', { name: 'View' }))
    expect(within(dialog).queryByLabelText('Answer (markdown)')).toBeNull()
    expect(within(dialog).getByText('prepared').tagName).toBe('STRONG')

    await user.click(within(dialog).getByRole('tab', { name: 'Edit' }))
    expect((within(dialog).getByLabelText('Answer (markdown)') as HTMLTextAreaElement).value).toBe(
      '**prepared** answer',
    )
  })

  it('creates an entry with a new category from the add modal', async () => {
    const user = userEvent.setup()
    const { onNotice } = renderPage([incident, values])

    await screen.findByRole('button', { name: incident.question })
    await user.click(screen.getByRole('button', { name: 'Add' }))
    const dialog = await screen.findByRole('dialog')

    await user.type(within(dialog).getByLabelText('Question'), 'Tell me about a conflict')
    const category = within(dialog).getByLabelText('Category') as HTMLSelectElement
    expect([...category.options].map((option) => option.textContent)).toEqual([
      'Uncategorized',
      'Behavioural',
      'Values',
      'Add new category…',
    ])
    await user.selectOptions(category, '__new__')
    await user.type(within(dialog).getByLabelText('New category'), 'STAR')
    await user.type(within(dialog).getByLabelText('Answer (markdown)'), 'Use **STAR**')

    await user.click(within(dialog).getByRole('button', { name: 'Add entry' }))
    await waitFor(() =>
      expect(api.createInterviewEntry).toHaveBeenCalledWith({
        question: 'Tell me about a conflict',
        answer: 'Use **STAR**',
        category: 'STAR',
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
    expect((within(dialog).getByLabelText('Category') as HTMLSelectElement).value).toBe(
      'Behavioural',
    )

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
        category: 'Behavioural',
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
