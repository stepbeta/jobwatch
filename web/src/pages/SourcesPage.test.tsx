import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SourcesPage } from './SourcesPage'
import { api } from '../api'
import type { Company, ScanResponse, Source } from '../types'

vi.mock('../api', () => ({
  api: {
    createSource: vi.fn(),
    updateSource: vi.fn(),
    deleteSource: vi.fn(),
    scanSource: vi.fn(),
    listCompanies: vi.fn(),
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
  const onScanBatch = vi.fn().mockResolvedValue([])
  const onScan = vi.fn()
  const view = render(
    <SourcesPage
      sources={sources}
      onReload={onReload}
      onScan={onScan}
      onScanBatch={onScanBatch}
      onNotice={onNotice}
      scanning={false}
    />,
  )
  return { onReload, onNotice, onScanBatch, onScan, view }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/sources'
  vi.mocked(api.createSource).mockResolvedValue({ source: { ...acme, id: 2 } })
  vi.mocked(api.listCompanies).mockResolvedValue({ companies: [], total: 0, limit: 500, offset: 0 })
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

function makeSource(overrides: Partial<Source> = {}): Source {
  return { ...acme, ...overrides }
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

describe('SourcesPage table controls', () => {
  it('uses a right-aligned toggle switch that PATCHes enabled', async () => {
    const user = userEvent.setup()
    const { onReload } = renderPage()

    const toggle = screen.getByRole('switch', { name: 'Enable Acme board' })
    await user.click(toggle)

    await waitFor(() => expect(api.updateSource).toHaveBeenCalledWith(1, { enabled: false }))
    await waitFor(() => expect(onReload).toHaveBeenCalled())
    expect(toggle.closest('td')?.className).toContain('cell-enabled')
  })

  it('renders icon-only row actions with accessible labels', async () => {
    renderPage()

    expect(screen.getByRole('button', { name: 'Refresh Acme board' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Edit Acme board' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Delete Acme board' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Refresh' })).toBeNull()
  })

  it('links the source name to its company when the names resolve', async () => {
    vi.mocked(api.listCompanies).mockResolvedValue({
      companies: [makeCompany(5, 'Acme board')],
      total: 1,
      limit: 500,
      offset: 0,
    })
    renderPage()

    const link = await screen.findByRole('link', { name: 'Acme board' })
    expect(link.getAttribute('href')).toBe('#/companies/5')
  })

  it('leaves the name as plain text when no company matches yet', async () => {
    renderPage()
    expect(screen.queryByRole('link', { name: 'Acme board' })).toBeNull()
    expect(screen.getByText('Acme board')).toBeTruthy()
  })

  it('restores enabled, status and name filters from the URL and writes changes back', async () => {
    window.location.hash = '#/sources?enabled=disabled&status=error&q=beta'
    renderPage([
      makeSource({ id: 1, name: 'Acme board', last_scan_status: 'ok' }),
      makeSource({ id: 2, name: 'Beta board', enabled: false, last_scan_status: 'error' }),
      makeSource({ id: 3, name: 'Beta careers', enabled: false, last_scan_status: 'blocked' }),
    ])

    expect(screen.queryByText('Acme board')).toBeNull()
    expect(screen.getByText('Beta board')).toBeTruthy()
    expect(screen.queryByText('Beta careers')).toBeNull()
    expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('beta')

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'All' }))
    await waitFor(() => expect(window.location.hash).not.toContain('enabled=disabled'))
    expect(window.location.hash).toContain('status=error')
  })

  it('sorts client-side from header clicks and reflects it in the URL', async () => {
    const user = userEvent.setup()
    renderPage([
      makeSource({ id: 1, name: 'Zeta' }),
      makeSource({ id: 2, name: 'Alpha' }),
    ])

    const rows = () => screen.getAllByRole('row').slice(1).map((r) => r.textContent ?? '')
    expect(rows()[0]).toContain('Alpha')

    await user.click(screen.getByRole('button', { name: 'Name' }))
    await waitFor(() => expect(rows()[0]).toContain('Zeta'))
    expect(window.location.hash).toContain('sort=name&order=desc')
  })

  it('persists hidden source columns', async () => {
    const user = userEvent.setup()
    const { view } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Columns' }))
    const menu = screen.getByRole('group', { name: 'Visible columns' })
    await user.click(within(menu).getByRole('checkbox', { name: 'Type' }))

    expect(window.localStorage.getItem('sources.columns')).toBe('["type"]')
    expect(screen.queryByRole('button', { name: 'Type' })).toBeNull()

    view.unmount()
    renderPage()
    await screen.findByRole('button', { name: 'Columns' })
    expect(screen.queryByRole('button', { name: 'Type' })).toBeNull()
  })

  it('deletes selected sources sequentially after one shared confirmation', async () => {
    const user = userEvent.setup()
    vi.mocked(api.deleteSource).mockResolvedValue(undefined)
    const { onReload, onNotice } = renderPage([
      makeSource({ id: 1, name: 'Acme board' }),
      makeSource({ id: 2, name: 'Beta board' }),
    ])

    await user.click(screen.getByRole('checkbox', { name: 'Select Acme board' }))
    await user.click(screen.getByRole('checkbox', { name: 'Select Beta board' }))
    await user.click(screen.getByRole('button', { name: 'Delete selected' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/Acme board, Beta board/)).toBeTruthy()
    expect(within(dialog).getByText(/source_id is detached/)).toBeTruthy()
    await user.click(within(dialog).getByRole('button', { name: 'Delete selected' }))

    await waitFor(() => expect(api.deleteSource).toHaveBeenCalledTimes(2))
    expect(vi.mocked(api.deleteSource).mock.calls.map((c) => c[0])).toEqual([1, 2])
    await waitFor(() => expect(onReload).toHaveBeenCalled())
    expect(onNotice).toHaveBeenCalledWith('success', '2 sources deleted.')
  })

  it('refreshes only the selected scannable sources through the shared batch helper', async () => {
    const user = userEvent.setup()
    const beta = makeSource({ id: 2, name: 'Beta board' })
    vi.mocked(api.scanSource).mockResolvedValue({
      source: beta,
      result: { status: 'ok', new: 1, incomplete: 0, disappeared: 0 } as ScanResponse['result'],
    })
    const { onScanBatch, onNotice } = renderPage([makeSource({ id: 1, name: 'Acme board' }), beta])
    vi.mocked(onScanBatch).mockResolvedValue([
      {
        source: beta,
        response: {
          source: beta,
          result: { status: 'ok', new: 1, incomplete: 0, disappeared: 0 } as ScanResponse['result'],
        },
      },
    ])

    expect((screen.getByRole('button', { name: 'Refresh selected' }) as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('checkbox', { name: 'Select Beta board' }))
    await user.click(screen.getByRole('button', { name: 'Refresh selected' }))

    await waitFor(() => expect(onScanBatch).toHaveBeenCalledWith([beta]))
    expect(onNotice).toHaveBeenCalledWith('success', 'Refresh selected: 1 new, 0 incomplete, 0 disappeared.')
  })
})
