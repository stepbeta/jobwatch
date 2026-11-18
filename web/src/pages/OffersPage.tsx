import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, errorMessage, type OfferQuery } from '../api'
import { ColumnPicker, PageSizeSelect, isPageSize, toggleSort, useColumnVisibility, useDebouncedValue, type SortOrder } from '../components/TableControls'
import { CompanyCombobox } from '../components/CompanyCombobox'
import { Modal } from '../components/Modal'
import { OFFER_COLUMNS, OFFER_SORT_LABELS, OFFER_SORT_VALUES, OfferTable, defaultOfferSortOrder } from '../components/OfferTable'
import { APP_STATES, OFFER_STATES, REMOTE_POLICIES, appLabel } from '../format'
import { currentRouteQuery, enumParam, intParam, navigate, replaceRouteQuery } from '../router'
import type { Company, Offer, OfferList } from '../types'

const COLUMNS_KEY = 'offers.columns'
const PER_PAGE_KEY = 'offers.perPage'
const DEFAULT_PER_PAGE = 50
const DEFAULT_SORT = 'last_seen'

function selectValue(value: string): string | undefined {
  return value === '' ? undefined : value
}

function numberValue(value: string): number | undefined {
  if (value === '') return undefined
  const n = Number(value)
  return Number.isFinite(n) ? n : undefined
}

function savedValue(value: string): boolean | undefined {
  if (value === 'saved') return true
  if (value === 'unsaved') return false
  return undefined
}

interface OffersView {
  q?: string
  application_state?: string
  offer_state?: string
  remote_policy?: string
  saved?: boolean
  min_title_confidence?: number
  min_location_confidence?: number
  company_id?: number
  sort: string
  order: SortOrder
  page: number
  perPage: number
}

function readPerPage(): number {
  try {
    const raw = window.localStorage.getItem(PER_PAGE_KEY)
    const n = raw === null ? undefined : Number(raw)
    if (isPageSize(n)) return n
  } catch {
    // fall through to the default
  }
  return DEFAULT_PER_PAGE
}

function writePerPage(value: number): void {
  try {
    window.localStorage.setItem(PER_PAGE_KEY, String(value))
  } catch {
    // presentation-only preference; ignore blocked storage
  }
}

function scoreParam(value: number | undefined): number | undefined {
  return value !== undefined && value >= 0 && value <= 100 ? value : undefined
}

function positiveID(value: number | undefined): number | undefined {
  return value !== undefined && value > 0 ? value : undefined
}

export function readOffersView(initialSaved?: boolean): OffersView {
  const params = currentRouteQuery()
  const sort = enumParam(params, 'sort', OFFER_SORT_VALUES) ?? DEFAULT_SORT
  const rawOrder = enumParam(params, 'order', ['asc', 'desc'] as const)
  const perPage = intParam(params, 'per_page')
  const page = intParam(params, 'page')
  const savedRaw = params.get('saved')
  const saved =
    savedRaw === 'true' ? true : savedRaw === 'false' ? false : initialSaved
  return {
    q: params.get('q')?.trim() || undefined,
    application_state: enumParam(params, 'application_state', APP_STATES),
    offer_state: enumParam(params, 'offer_state', OFFER_STATES),
    remote_policy: enumParam(params, 'remote_policy', REMOTE_POLICIES),
    saved,
    min_title_confidence: scoreParam(intParam(params, 'min_title_confidence')),
    min_location_confidence: scoreParam(intParam(params, 'min_location_confidence')),
    company_id: positiveID(intParam(params, 'company_id')),
    sort,
    order: rawOrder ?? defaultOfferSortOrder(sort),
    page: page !== undefined && page > 0 ? page : 1,
    perPage: isPageSize(perPage) ? perPage : readPerPage(),
  }
}

function viewQuery(view: OffersView): OfferQuery {
  return {
    q: view.q,
    application_state: view.application_state,
    offer_state: view.offer_state,
    remote_policy: view.remote_policy,
    saved: view.saved,
    min_title_confidence: view.min_title_confidence,
    min_location_confidence: view.min_location_confidence,
    company_id: view.company_id,
    sort: view.sort,
    order: view.order,
    limit: view.perPage,
    offset: (view.page - 1) * view.perPage,
  }
}

function writeURL(view: OffersView): void {
  const params = new URLSearchParams()
  if (view.q) params.set('q', view.q)
  if (view.application_state) params.set('application_state', view.application_state)
  if (view.offer_state) params.set('offer_state', view.offer_state)
  if (view.remote_policy) params.set('remote_policy', view.remote_policy)
  if (view.saved !== undefined) params.set('saved', view.saved ? 'true' : 'false')
  if (view.min_title_confidence !== undefined)
    params.set('min_title_confidence', String(view.min_title_confidence))
  if (view.min_location_confidence !== undefined)
    params.set('min_location_confidence', String(view.min_location_confidence))
  if (view.company_id !== undefined) params.set('company_id', String(view.company_id))
  if (view.sort !== DEFAULT_SORT || view.order !== defaultOfferSortOrder(DEFAULT_SORT)) {
    params.set('sort', view.sort)
    params.set('order', view.order)
  }
  if (view.page > 1) params.set('page', String(view.page))
  params.set('per_page', String(view.perPage))
  replaceRouteQuery(params)
}

export function OffersPage({
  refreshToken,
  onDataChanged,
  onNotice,
  initialSaved,
}: {
  refreshToken: number
  onDataChanged: () => void
  onNotice: (kind: 'info' | 'success' | 'error', text: string) => void
  initialSaved?: boolean
}) {
  const [view, setView] = useState<OffersView>(() => readOffersView(initialSaved))
  const [search, setSearch] = useState(() => view.q ?? '')
  const debouncedSearch = useDebouncedValue(search)
  const [companies, setCompanies] = useState<Company[]>([])
  const [offers, setOffers] = useState<OfferList | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busyOfferId, setBusyOfferId] = useState<number | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const columns = useColumnVisibility(COLUMNS_KEY, OFFER_COLUMNS)

  useEffect(() => {
    setView((v) => {
      const q = debouncedSearch.trim() || undefined
      return v.q === q ? v : { ...v, q, page: 1 }
    })
  }, [debouncedSearch])

  useEffect(() => {
    setView((v) => (v.saved === initialSaved ? v : { ...v, saved: initialSaved, page: 1 }))
  }, [initialSaved])

  useEffect(() => {
    writeURL(view)
  }, [view])

  useEffect(() => {
    let cancelled = false
    api
      .listCompanies({ limit: 500 })
      .then((r) => {
        if (!cancelled) setCompanies(r.companies)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api
      .listOffers(viewQuery(view))
      .then((r) => {
        if (!cancelled) {
          setOffers(r)
          setError('')
        }
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [view, refreshToken])

  function patchView(patch: Partial<OffersView>) {
    setView((v) => ({ ...v, ...patch, page: 1 }))
  }

  function resetFilters() {
    setSearch('')
    setView((v) => ({
      sort: DEFAULT_SORT,
      order: defaultOfferSortOrder(DEFAULT_SORT),
      page: 1,
      perPage: v.perPage,
    }))
  }

  function handleSort(sortKey: string) {
    setView((v) => ({ ...v, ...toggleSort(v.sort, v.order, sortKey, defaultOfferSortOrder), page: 1 }))
  }

  function changePerPage(value: number) {
    writePerPage(value)
    setView((v) => ({ ...v, perPage: value, page: 1 }))
  }

  async function toggleSaved(offer: Offer) {
    setBusyOfferId(offer.id)
    try {
      const r = await api.patchOffer(offer.id, { saved: !offer.saved_at })
      setOffers((prev) =>
        prev
          ? {
              ...prev,
              offers: prev.offers.map((o) => (o.id === r.offer.id ? { ...o, ...r.offer } : o)),
            }
          : prev,
      )
      onNotice('success', r.offer.saved_at ? 'Offer saved.' : 'Bookmark removed.')
      onDataChanged()
    } catch (e) {
      onNotice('error', errorMessage(e))
    } finally {
      setBusyOfferId(null)
    }
  }

  const limit = offers?.limit ?? view.perPage
  const offset = offers?.offset ?? (view.page - 1) * view.perPage
  const total = offers?.total ?? 0
  const hasFilters = Boolean(
    view.q ||
      view.application_state ||
      view.offer_state ||
      view.remote_policy ||
      view.min_title_confidence !== undefined ||
      view.min_location_confidence !== undefined ||
      view.company_id !== undefined ||
      view.saved !== undefined,
  )

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Offers</h2>
          <div className="actions">
            <span className="muted">
              {total} match{total === 1 ? 'es' : ''}
            </span>
            <button type="button" className="primary" onClick={() => setImportOpen(true)}>
              Add offer
            </button>
          </div>
        </div>

        <div className="filters">
          <label>
            Search
            <input
              type="search"
              placeholder="title or description (FTS)"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </label>
          <label>
            Application
            <select
              value={view.application_state ?? ''}
              onChange={(e) => patchView({ application_state: selectValue(e.target.value) })}
            >
              <option value="">Any</option>
              {APP_STATES.map((s) => (
                <option key={s} value={s}>
                  {appLabel(s)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Offer
            <select
              value={view.offer_state ?? ''}
              onChange={(e) => patchView({ offer_state: selectValue(e.target.value) })}
            >
              <option value="">Any</option>
              {OFFER_STATES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label>
            Remote
            <select
              value={view.remote_policy ?? ''}
              onChange={(e) => patchView({ remote_policy: selectValue(e.target.value) })}
            >
              <option value="">Any</option>
              {REMOTE_POLICIES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label>
            Saved
            <select
              value={view.saved === true ? 'saved' : view.saved === false ? 'unsaved' : ''}
              onChange={(e) => patchView({ saved: savedValue(e.target.value) })}
            >
              <option value="">Any</option>
              <option value="saved">Saved only</option>
              <option value="unsaved">Not saved</option>
            </select>
          </label>
          <label>
            Title ≥
            <input
              type="number"
              min={0}
              max={100}
              placeholder="0-100"
              value={view.min_title_confidence ?? ''}
              onChange={(e) => patchView({ min_title_confidence: numberValue(e.target.value) })}
            />
          </label>
          <label>
            Location ≥
            <input
              type="number"
              min={0}
              max={100}
              placeholder="0-100"
              value={view.min_location_confidence ?? ''}
              onChange={(e) => patchView({ min_location_confidence: numberValue(e.target.value) })}
            />
          </label>
          <CompanyCombobox
            companies={companies}
            value={view.company_id}
            onChange={(company_id) => patchView({ company_id })}
          />
          <label>
            Sort
            <select
              value={view.sort}
              onChange={(e) => {
                const sort = e.target.value
                patchView({ sort, order: defaultOfferSortOrder(sort) })
              }}
            >
              {OFFER_SORT_VALUES.map((s) => (
                <option key={s} value={s}>
                  {OFFER_SORT_LABELS[s]}
                </option>
              ))}
            </select>
          </label>
          <div className="filter-actions">
            <ColumnPicker
              columns={OFFER_COLUMNS}
              visible={columns.visible}
              onToggle={columns.toggle}
              onReset={columns.reset}
            />
            <button type="button" onClick={resetFilters}>
              Reset
            </button>
          </div>
        </div>

        {error && <div className="error-box">{error}</div>}
        {loading && !offers ? (
          <p className="muted" aria-busy="true">
            Loading offers…
          </p>
        ) : offers && offers.offers.length === 0 ? (
          <div className="empty-state">
            {hasFilters ? (
              <>
                <p>No offers match these filters.</p>
                <button type="button" onClick={resetFilters}>
                  Clear filters
                </button>
              </>
            ) : (
              <>
                <p>No offers yet.</p>
                <p className="muted">
                  Add a source on the <a href="#/sources">Sources</a> page, or use Add offer above.
                </p>
              </>
            )}
          </div>
        ) : offers ? (
          <OfferTable
            offers={offers.offers}
            columns={columns.visible}
            onToggleSaved={(o) => void toggleSaved(o)}
            busyOfferId={busyOfferId}
            sort={view.sort}
            order={view.order}
            onSort={handleSort}
          />
        ) : null}

        <div className="pagination">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setView((v) => ({ ...v, page: Math.max(1, v.page - 1) }))}
          >
            ← Previous
          </button>
          <span className="muted">
            {total === 0 ? '0' : `${offset + 1}–${Math.min(offset + limit, total)}`} of {total}
          </span>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => setView((v) => ({ ...v, page: v.page + 1 }))}
          >
            Next →
          </button>
          <PageSizeSelect value={view.perPage} onChange={changePerPage} />
        </div>
      </section>

      {importOpen && (
        <ImportOfferModal
          onClose={() => setImportOpen(false)}
          onImported={(offerID) => {
            setImportOpen(false)
            onDataChanged()
            navigate(`/offers/${offerID}`)
          }}
          onNotice={onNotice}
        />
      )}
    </div>
  )
}

function ImportOfferModal({
  onClose,
  onImported,
  onNotice,
}: {
  onClose: () => void
  onImported: (offerID: number) => void
  onNotice: (kind: 'info' | 'success' | 'error', text: string) => void
}) {
  const [company, setCompany] = useState('')
  const [title, setTitle] = useState('')
  const [location, setLocation] = useState('')
  const [url, setUrl] = useState('')
  const [description, setDescription] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    const hasFields = company.trim() !== '' && title.trim() !== ''
    if (url.trim() === '' && !hasFields) {
      setError('Enter a URL, or both company and title.')
      return
    }
    setBusy(true)
    try {
      const r = await api.importOffer({
        company: company.trim() || undefined,
        title: title.trim() || undefined,
        location: location.trim() || undefined,
        url: url.trim() || undefined,
        description: description.trim() || undefined,
      })
      onNotice(
        'success',
        r.created
          ? `Created offer #${r.offer_id}.`
          : `Matched an existing offer (#${r.offer_id}); the link was added to it.`,
      )
      onImported(r.offer_id)
    } catch (err) {
      if (
        err instanceof ApiError &&
        err.metadata &&
        (err.code === 'metadata_incomplete' || err.code === 'metadata_unavailable')
      ) {
        const m = err.metadata
        if (!company && m.company) setCompany(m.company)
        if (!title && m.title) setTitle(m.title)
        if (!location && m.location) setLocation(m.location)
        if (!url && m.url) setUrl(m.url)
        if (!description && m.description) setDescription(m.description)
        setError(`${err.message} The available fields were prefilled.`)
      } else {
        const text = errorMessage(err)
        setError(text)
        onNotice('error', text)
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title="Add an offer" onClose={onClose} wide>
      <p className="hint">
        Paste a job URL and let JobWatch read its title, company and description (JSON-LD, Open Graph
        or the page title). If the page cannot be read, the form falls back to manual entry.
      </p>
      <form className="form-grid" onSubmit={(e) => void submit(e)}>
        <label className="span-2">
          Job URL
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://… (fetched only if company and title are empty)"
          />
        </label>
        <label>
          Company
          <input
            value={company}
            onChange={(e) => setCompany(e.target.value)}
            placeholder="filled from the page when possible"
          />
        </label>
        <label>
          Title
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="filled from the page when possible"
          />
        </label>
        <label>
          Location
          <input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="optional" />
        </label>
        <label className="span-2">
          Description
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            placeholder="optional"
          />
        </label>
        {error && <div className="error-box">{error}</div>}
        <div className="form-actions">
          <button type="submit" className="primary" disabled={busy}>
            {busy ? 'Importing…' : 'Fetch & import'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
