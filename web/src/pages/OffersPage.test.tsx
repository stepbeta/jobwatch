import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError } from '../api'
import { OffersPage } from './OffersPage'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    api: {
      listOffers: vi.fn(),
      listCompanies: vi.fn(),
      importOffer: vi.fn(),
      patchOffer: vi.fn(),
    },
  }
})

import { api } from '../api'

function renderPage() {
  const onDataChanged = vi.fn()
  const onNotice = vi.fn()
  render(
    <OffersPage refreshToken={0} onDataChanged={onDataChanged} onNotice={onNotice} />,
  )
  return { onDataChanged, onNotice }
}

beforeEach(() => {
  vi.clearAllMocks()
  window.location.hash = '#/offers'
  vi.mocked(api.listOffers).mockResolvedValue({ offers: [], total: 0, limit: 50, offset: 0 })
  vi.mocked(api.listCompanies).mockResolvedValue({ companies: [], total: 0, limit: 500, offset: 0 })
})

describe('OffersPage add-offer modal', () => {
  it('opens the import form from the page header instead of an inline panel', async () => {
    const user = userEvent.setup()
    renderPage()

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.querySelector('details')).toBeNull()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: 'Add an offer' })).toBeTruthy()
    expect(within(dialog).getByLabelText('Job URL')).toBeTruthy()
    expect(within(dialog).getByLabelText('Company')).toBeTruthy()
    expect(within(dialog).getByLabelText('Title')).toBeTruthy()
  })

  it('imports a URL, closes the modal, shows the toast and opens the new offer', async () => {
    const user = userEvent.setup()
    vi.mocked(api.importOffer).mockResolvedValue({ offer_id: 42, created: true })
    const { onDataChanged, onNotice } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Job URL'), 'https://example.com/jobs/42')
    await user.click(within(dialog).getByRole('button', { name: 'Fetch & import' }))

    await waitFor(() =>
      expect(api.importOffer).toHaveBeenCalledWith({
        company: undefined,
        title: undefined,
        location: undefined,
        url: 'https://example.com/jobs/42',
        description: undefined,
      }),
    )
    expect(onNotice).toHaveBeenCalledWith('success', 'Created offer #42.')
    expect(onDataChanged).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(window.location.hash).toBe('#/offers/42')
  })

  it('keeps the modal open and prefills fields on a 422 metadata fallback', async () => {
    const user = userEvent.setup()
    vi.mocked(api.importOffer).mockRejectedValue(
      new ApiError('metadata_incomplete', 'The page had no usable title.', 422, {
        company: 'Acme',
        title: 'Platform Engineer',
        location: 'Paris, France',
        url: 'https://example.com/jobs/42',
        description: 'Run the platform.',
      }),
    )
    const { onDataChanged, onNotice } = renderPage()

    await user.click(screen.getByRole('button', { name: 'Add offer' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText('Job URL'), 'https://example.com/jobs/42')
    await user.click(within(dialog).getByRole('button', { name: 'Fetch & import' }))

    expect(
      await within(dialog).findByText('The page had no usable title. The available fields were prefilled.'),
    ).toBeTruthy()
    expect((within(dialog).getByLabelText('Company') as HTMLInputElement).value).toBe('Acme')
    expect((within(dialog).getByLabelText('Title') as HTMLInputElement).value).toBe('Platform Engineer')
    expect((within(dialog).getByLabelText('Location') as HTMLInputElement).value).toBe('Paris, France')
    // The fallback is inline: no error toast, no navigation, no refresh.
    expect(onNotice).not.toHaveBeenCalled()
    expect(onDataChanged).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog')).toBeTruthy()
    expect(window.location.hash).toBe('#/offers')
  })
})
