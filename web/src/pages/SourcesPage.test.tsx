import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SourcesPage } from './SourcesPage'
import { api } from '../api'
import type { Source } from '../types'

vi.mock('../api', () => ({
  api: {
    createSource: vi.fn(),
    updateSource: vi.fn(),
    deleteSource: vi.fn(),
    scanSource: vi.fn(),
  },
  errorMessage: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}))

const acme: Source = {
  id: 1,
  name: 'Acme board',
  url: 'https://boards.greenhouse.io/acme',
  source_type: 'greenhouse',
  config: {},
  enabled: true,
  last_scan_at: null,
  last_scan_status: null,
  created_at: null,
}

function renderPage(sources: Source[] = [acme]) {
  const onReload = vi.fn().mockResolvedValue(undefined)
  const onNotice = vi.fn()
  render(
    <SourcesPage
      sources={sources}
      onReload={onReload}
      onScan={vi.fn()}
      onScanBatch={vi.fn().mockResolvedValue([])}
      onNotice={onNotice}
      scanning={false}
    />,
  )
  return { onReload, onNotice }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(api.createSource).mockResolvedValue({ source: { ...acme, id: 2 } })
})

describe('SourcesPage add-source modal', () => {
  it('opens in a dialog without dropping the source list', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(screen.getByText('Acme board')).toBeTruthy()
    await user.click(screen.getByRole('button', { name: 'Add source' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: 'New source' })).toBeTruthy()
    // The list is still mounted behind the modal.
    expect(screen.getByText('Acme board')).toBeTruthy()
  })

  it('submits the type-dependent form and closes the modal', async () => {
    const user = userEvent.setup()
    const { onReload, onNotice } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Add source' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Name'), 'New board')
    await user.type(within(dialog).getByLabelText('URL'), 'https://boards.greenhouse.io/new')
    await user.click(within(dialog).getByRole('button', { name: 'Create source' }))

    await waitFor(() =>
      expect(api.createSource).toHaveBeenCalledWith({
        name: 'New board',
        url: 'https://boards.greenhouse.io/new',
        source_type: 'greenhouse',
        config: {},
        enabled: true,
      }),
    )
    expect(onReload).toHaveBeenCalled()
    expect(onNotice).toHaveBeenCalledWith('success', 'Source created.')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
