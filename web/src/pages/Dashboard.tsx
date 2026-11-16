import { useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage, type OfferQuery } from '../api'
import { Link } from '../components/Link'
import { OfferTable } from '../components/OfferTable'
import { StatePill } from '../components/StatePill'
import { APP_STATES, OFFER_STATES, REMOTE_POLICIES, appLabel, fmtDateTime, relativeTime } from '../format'
import type { Company, Offer, OfferList } from '../types'

function selectValue(value: string): string | undefined {
  return value === '' ? undefined : value
}

function numberValue(value: string): number | undefined {
  if (value === '') return undefined
  const n = Number(value)
  return Number.isFinite(n) ? n : undefined
}

export function Dashboard({
  refreshToken,
  onDataChanged,
}: {
  refreshToken: number
  onDataChanged: () => void
}) {
  const [filters, setFilters] = useState<OfferQuery>({ sort: 'last_seen', limit: 50, offset: 0 })
  const [search, setSearch] = useState('')
  const [companies, setCompanies] = useState<Company[]>([])
  const [offers, setOffers] = useState<OfferList | null>(null)
  const [applications, setApplications] = useState<Offer[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

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

  useEffect(() => {
    let cancelled = false
    api
      .listOffers({ has_application: true, sort: 'recent_event', limit: 20 })
      .then((r) => {
        if (!cancelled) setApplications(r.offers)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken])

  function patchFilters(patch: Partial<OfferQuery>) {
    setFilters((f) => ({ ...f, ...patch, offset: 0 }))
  }

  const limit = offers?.limit ?? filters.limit ?? 50
  const offset = offers?.offset ?? 0
  const total = offers?.total ?? 0

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Applications</h2>
          <span className="muted">Offers with application_state ≠ todo, most recent activity first</span>
        </div>
        {applications.length === 0 ? (
          <p className="empty">No applications yet. Set an offer's application state to track it here.</p>
        ) : (
          <div className="table-wrap">
            <table className="grid">
              <thead>
                <tr>
                  <th>Company</th>
                  <th>Title</th>
                  <th>Application</th>
                  <th>Last activity</th>
                  <th>Offer</th>
                </tr>
              </thead>
              <tbody>
                {applications.map((o) => (
                  <tr key={o.id}>
                    <td>
                      <Link to={`/companies/${o.company_id}`}>{o.company_name}</Link>
                    </td>
                    <td>
                      <Link to={`/offers/${o.id}`}>{o.title}</Link>
                    </td>
                    <td>
                      <StatePill kind={`app-${o.application_state}`}>{appLabel(o.application_state)}</StatePill>
                    </td>
                    <td title={fmtDateTime(o.last_event_at ?? o.updated_at)}>
                      {relativeTime(o.last_event_at ?? o.updated_at)}
                    </td>
                    <td>
                      <StatePill kind={`offer-${o.offer_state}`}>{o.offer_state}</StatePill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel">
        <div className="section-head">
          <h2>Offers</h2>
          <span className="muted">
            {total} match{total === 1 ? '' : 'es'}
          </span>
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
            </select>
          </label>
          <div className="filter-actions">
            <button type="submit" className="primary">
              Search
            </button>
            <button
              type="button"
              onClick={() => {
                setSearch('')
                setFilters({ sort: 'last_seen', limit: 50, offset: 0 })
              }}
            >
              Reset
            </button>
          </div>
        </form>

        {error && <div className="error-box">{error}</div>}
        {loading && !offers ? <p className="muted">Loading…</p> : offers && <OfferTable offers={offers.offers} />}

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

      <ImportPanel onImported={onDataChanged} />
    </div>
  )
}

function ImportPanel({ onImported }: { onImported: () => void }) {
  const [company, setCompany] = useState('')
  const [title, setTitle] = useState('')
  const [location, setLocation] = useState('')
  const [url, setUrl] = useState('')
  const [description, setDescription] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const r = await api.importOffer({
        company: company.trim(),
        title: title.trim(),
        location: location.trim() || undefined,
        url: url.trim() || undefined,
        description: description.trim() || undefined,
      })
      setMessage(
        r.created
          ? `Created offer #${r.offer_id}.`
          : `Matched an existing offer (#${r.offer_id}); the link was added to it.`,
      )
      setCompany('')
      setTitle('')
      setLocation('')
      setUrl('')
      setDescription('')
      onImported()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <section className="panel">
      <details>
        <summary>Import an offer seen elsewhere</summary>
        <form className="form-grid" onSubmit={(e) => void submit(e)}>
          <label>
            Company
            <input value={company} onChange={(e) => setCompany(e.target.value)} required />
          </label>
          <label>
            Title
            <input value={title} onChange={(e) => setTitle(e.target.value)} required />
          </label>
          <label>
            Location
            <input value={location} onChange={(e) => setLocation(e.target.value)} placeholder="optional" />
          </label>
          <label>
            URL
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="optional" />
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
          {message && <p className="hint">{message}</p>}
          <div className="form-actions">
            <button type="submit" className="primary">
              Import
            </button>
          </div>
        </form>
      </details>
    </section>
  )
}
