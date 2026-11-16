import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, errorMessage, type OfferQuery } from '../api'
import { Modal } from '../components/Modal'
import { OfferTable } from '../components/OfferTable'
import { APP_STATES, OFFER_STATES, REMOTE_POLICIES, appLabel } from '../format'
import { navigate } from '../router'
import type { Company, Offer, OfferList } from '../types'

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
  const [filters, setFilters] = useState<OfferQuery>({
    sort: 'last_seen',
    limit: 50,
    offset: 0,
    saved: initialSaved,
  })
  const [search, setSearch] = useState('')
  const [companies, setCompanies] = useState<Company[]>([])
  const [offers, setOffers] = useState<OfferList | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busyOfferId, setBusyOfferId] = useState<number | null>(null)
  const [importOpen, setImportOpen] = useState(false)

  useEffect(() => {
    setFilters((f) => ({ ...f, saved: initialSaved, offset: 0 }))
  }, [initialSaved])

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
      .listOffers(filters)
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
  }, [filters, refreshToken])

  function patchFilters(patch: Partial<OfferQuery>) {
    setFilters((f) => ({ ...f, ...patch, offset: 0 }))
  }

  function resetFilters() {
    setSearch('')
    setFilters({ sort: 'last_seen', limit: 50, offset: 0 })
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

  const limit = offers?.limit ?? filters.limit ?? 50
  const offset = offers?.offset ?? 0
  const total = offers?.total ?? 0
  const hasFilters = Boolean(
    filters.q ||
      filters.application_state ||
      filters.offer_state ||
      filters.remote_policy ||
      filters.min_title_confidence !== undefined ||
      filters.min_location_confidence !== undefined ||
      filters.company_id !== undefined ||
      filters.saved !== undefined,
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

        <form
          className="filters"
          onSubmit={(e) => {
            e.preventDefault()
            setFilters((f) => ({ ...f, q: search.trim() || undefined, offset: 0 }))
          }}
        >
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
              value={filters.application_state ?? ''}
              onChange={(e) => patchFilters({ application_state: selectValue(e.target.value) })}
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
              value={filters.offer_state ?? ''}
              onChange={(e) => patchFilters({ offer_state: selectValue(e.target.value) })}
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
              value={filters.remote_policy ?? ''}
              onChange={(e) => patchFilters({ remote_policy: selectValue(e.target.value) })}
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
              value={filters.saved === true ? 'saved' : filters.saved === false ? 'unsaved' : ''}
              onChange={(e) => patchFilters({ saved: savedValue(e.target.value) })}
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
              value={filters.min_title_confidence ?? ''}
              onChange={(e) => patchFilters({ min_title_confidence: numberValue(e.target.value) })}
            />
          </label>
          <label>
            Location ≥
            <input
              type="number"
              min={0}
              max={100}
              placeholder="0-100"
              value={filters.min_location_confidence ?? ''}
              onChange={(e) => patchFilters({ min_location_confidence: numberValue(e.target.value) })}
            />
          </label>
          <label>
            Company
            <select
              value={filters.company_id ?? ''}
              onChange={(e) =>
                patchFilters({
                  company_id: e.target.value === '' ? undefined : Number(e.target.value),
                })
              }
            >
              <option value="">Any</option>
              {companies.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Sort
            <select value={filters.sort ?? 'last_seen'} onChange={(e) => patchFilters({ sort: e.target.value })}>
              <option value="last_seen">Last seen</option>
              <option value="recent_event">Recent activity</option>
              <option value="first_seen">First seen</option>
              <option value="title">Title</option>
              <option value="company">Company</option>
              <option value="saved">Recently saved</option>
            </select>
          </label>
          <div className="filter-actions">
            <button type="submit" className="primary">
              Search
            </button>
            <button type="button" onClick={resetFilters}>
              Reset
            </button>
          </div>
        </form>

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
            onToggleSaved={(o) => void toggleSaved(o)}
            busyOfferId={busyOfferId}
          />
        ) : null}

        <div className="pagination">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setFilters((f) => ({ ...f, offset: Math.max(0, offset - limit) }))}
          >
            ← Previous
          </button>
          <span className="muted">
            {total === 0 ? '0' : `${offset + 1}–${Math.min(offset + limit, total)}`} of {total}
          </span>
          <button
            type="button"
            disabled={offset + limit >= total}
            onClick={() => setFilters((f) => ({ ...f, offset: offset + limit }))}
          >
            Next →
          </button>
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
