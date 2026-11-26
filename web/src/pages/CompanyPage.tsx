import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { Link } from '../components/Link'
import { OFFER_SORT_LABELS, OFFER_SORT_VALUES, OfferTable, defaultOfferSortOrder } from '../components/OfferTable'
import { PageSizeSelect, isPageSize, toggleSort, useDebouncedValue, type SortOrder } from '../components/TableControls'
import { fmtDateTime } from '../format'
import { currentRouteQuery, enumParam, intParam, replaceRouteQuery } from '../router'
import { companyDocumentTitle, useDocumentTitle } from '../title'
import type { NoticeKind } from '../App'
import type { CompanyCandidate, CompanyDetail, OfferList } from '../types'

function dismissedKey(companyID: number): string {
  return `jobwatch.dismissed-duplicates:${companyID}`
}

const COMPANY_PER_PAGE_KEY = 'companyOffers.perPage'
const COMPANY_DEFAULT_PER_PAGE = 20
const COMPANY_DEFAULT_SORT = 'last_seen'

interface CompanyOffersView {
  q?: string
  sort: string
  order: SortOrder
  page: number
  perPage: number
}

function readCompanyPerPage(): number {
  try {
    const raw = window.localStorage.getItem(COMPANY_PER_PAGE_KEY)
    const n = raw === null ? undefined : Number(raw)
    if (isPageSize(n)) return n
  } catch {
    // fall through to the default
  }
  return COMPANY_DEFAULT_PER_PAGE
}

function readCompanyOffersView(): CompanyOffersView {
  const params = currentRouteQuery()
  const sort = enumParam(params, 'sort', OFFER_SORT_VALUES) ?? COMPANY_DEFAULT_SORT
  const perPage = intParam(params, 'per_page')
  const page = intParam(params, 'page')
  return {
    q: params.get('q')?.trim() || undefined,
    sort,
    order: enumParam(params, 'order', ['asc', 'desc'] as const) ?? defaultOfferSortOrder(sort),
    page: page !== undefined && page > 0 ? page : 1,
    perPage: isPageSize(perPage) ? perPage : readCompanyPerPage(),
  }
}

function writeCompanyURL(view: CompanyOffersView): void {
  const params = new URLSearchParams()
  if (view.q) params.set('q', view.q)
  if (view.sort !== COMPANY_DEFAULT_SORT || view.order !== defaultOfferSortOrder(COMPANY_DEFAULT_SORT)) {
    params.set('sort', view.sort)
    params.set('order', view.order)
  }
  if (view.page > 1) params.set('page', String(view.page))
  params.set('per_page', String(view.perPage))
  replaceRouteQuery(params)
}

export function CompanyPage({
  id,
  onNotice,
}: {
  id: number
  onNotice: (kind: NoticeKind, text: string) => void
}) {
  const [detail, setDetail] = useState<CompanyDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [noteBody, setNoteBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState('')
  const [website, setWebsite] = useState('')
  const [dismissed, setDismissed] = useState<number[]>([])
  const [offers, setOffers] = useState<OfferList | null>(null)
  const [offersView, setOffersView] = useState<CompanyOffersView>(readCompanyOffersView)
  const [offerSearch, setOfferSearch] = useState(() => offersView.q ?? '')
  const [offersLoading, setOffersLoading] = useState(false)
  const [offersToken, setOffersToken] = useState(0)
  const debouncedOfferSearch = useDebouncedValue(offerSearch)

  const [mergeTarget, setMergeTarget] = useState<CompanyCandidate | null>(null)
  const [keepID, setKeepID] = useState<number>(id)
  const [mergeBusy, setMergeBusy] = useState(false)
  const [mergeError, setMergeError] = useState('')
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)

  useDocumentTitle(
    detail && detail.company.id === id
      ? companyDocumentTitle(detail.company.name)
      : 'Company — JobWatch',
  )

  const load = useCallback(async () => {
    try {
      const d = await api.getCompany(id)
      setDetail(d)
      setName(d.company.name)
      setWebsite(d.company.website ?? '')
      setError('')
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    setLoading(true)
    setEditing(false)
    setMergeTarget(null)
    setMergeError('')
    setDismissed(readDismissed(id))
    void load()
  }, [id, load])

  useEffect(() => {
    const next = readCompanyOffersView()
    setOffersView(next)
    setOfferSearch(next.q ?? '')
  }, [id])

  useEffect(() => {
    setOffersView((v) => {
      const q = debouncedOfferSearch.trim() || undefined
      return v.q === q ? v : { ...v, q, page: 1 }
    })
  }, [debouncedOfferSearch])

  useEffect(() => {
    writeCompanyURL(offersView)
  }, [offersView])

  useEffect(() => {
    let cancelled = false
    setOffersLoading(true)
    api
      .listOffers({
        company_id: id,
        q: offersView.q,
        sort: offersView.sort,
        order: offersView.order,
        limit: offersView.perPage,
        offset: (offersView.page - 1) * offersView.perPage,
      })
      .then((r) => {
        if (!cancelled) setOffers(r)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
      .finally(() => {
        if (!cancelled) setOffersLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [id, offersView, offersToken])

  async function addNote(e: FormEvent) {
    e.preventDefault()
    if (!noteBody.trim()) return
    try {
      await api.createNote({ entity_type: 'company', entity_id: id, body: noteBody.trim() })
      setNoteBody('')
      await load()
      onNotice('success', 'Company note added.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  function deleteNote(noteId: number) {
    setConfirm({
      title: 'Delete company note',
      message: 'Delete this company note? This cannot be undone.',
      confirmLabel: 'Delete note',
      onConfirm: async () => {
        await api.deleteNote(noteId)
        await load()
        onNotice('success', 'Company note deleted.')
      },
    })
  }

  async function saveCompany(e: FormEvent) {
    e.preventDefault()
    try {
      await api.updateCompany(id, { name: name.trim(), website: website.trim() || null })
      setEditing(false)
      await load()
      onNotice('success', 'Company updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  function removeCompany() {
    if (!detail) return
    setConfirm({
      title: 'Delete company',
      message: `Delete company "${detail.company.name}"? It must have no offers. This cannot be undone.`,
      confirmLabel: 'Delete company',
      onConfirm: async () => {
        await api.deleteCompany(detail.company.id)
        window.location.hash = '#/'
      },
    })
  }

  function dismissCandidate(candidateID: number) {
    const next = Array.from(new Set([...dismissed, candidateID]))
    setDismissed(next)
    writeDismissed(id, next)
  }

  function dismissAll(candidates: CompanyCandidate[]) {
    const next = Array.from(new Set([...dismissed, ...candidates.map((c) => c.id)]))
    setDismissed(next)
    writeDismissed(id, next)
  }

  function openMerge(candidate: CompanyCandidate) {
    if (!detail) return
    setMergeTarget(candidate)
    setKeepID(detail.company.id)
    setMergeError('')
  }

  async function confirmMerge() {
    if (!detail || !mergeTarget) return
    setMergeBusy(true)
    setMergeError('')
    try {
      if (keepID === detail.company.id) {
        await api.mergeCompany(mergeTarget.id, detail.company.id)
        setMergeTarget(null)
        await load()
        setOffersToken((t) => t + 1)
      } else {
        const res = await api.mergeCompany(detail.company.id, keepID)
        window.location.hash = `#/companies/${res.company.id}`
      }
    } catch (err) {
      setMergeError(errorMessage(err))
    } finally {
      setMergeBusy(false)
    }
  }

  function handleOfferSort(sortKey: string) {
    setOffersView((v) => ({
      ...v,
      ...toggleSort(v.sort, v.order, sortKey, defaultOfferSortOrder),
      page: 1,
    }))
  }

  function changeCompanyPerPage(value: number) {
    try {
      window.localStorage.setItem(COMPANY_PER_PAGE_KEY, String(value))
    } catch {
      // presentation-only preference
    }
    setOffersView((v) => ({ ...v, perPage: value, page: 1 }))
  }

  function resetOfferFilters() {
    setOfferSearch('')
    setOffersView((v) => ({
      q: undefined,
      sort: COMPANY_DEFAULT_SORT,
      order: defaultOfferSortOrder(COMPANY_DEFAULT_SORT),
      page: 1,
      perPage: v.perPage,
    }))
  }

  if (loading && !detail) {
    return (
      <p className="muted page" aria-busy="true">
        Loading company…
      </p>
    )
  }
  if (!detail) {
    return (
      <div className="page">
        <div className="error-box">{error || 'Company not found'}</div>
        <Link to="/">← Back to dashboard</Link>
      </div>
    )
  }

  const duplicates = detail.possible_duplicates.filter((c) => !dismissed.includes(c.id))

  return (
    <div className="page">
      <div className="breadcrumb">
        <Link to="/">Dashboard</Link> / Company
      </div>
      {error && <div className="error-box">{error}</div>}

      <section className="panel company-header">
        <div className="company-identity">
          {editing ? (
            <form className="state-controls" onSubmit={(e) => void saveCompany(e)}>
              <label>
                Name
                <input value={name} onChange={(e) => setName(e.target.value)} required />
              </label>
              <label>
                Website
                <input value={website} onChange={(e) => setWebsite(e.target.value)} placeholder="https://…" />
              </label>
              <button type="submit" className="primary">
                Save
              </button>
              <button type="button" onClick={() => setEditing(false)}>
                Cancel
              </button>
            </form>
          ) : (
            <div className="offer-title">
              <h1>{detail.company.name}</h1>
              <div className="muted">
                {detail.company.website ? (
                  <a href={detail.company.website} target="_blank" rel="noreferrer">
                    {detail.company.website}
                  </a>
                ) : (
                  'No website'
                )}{' '}
                · {detail.company.offer_count} offer(s) · created {fmtDateTime(detail.company.created_at)}
              </div>
              <div className="muted small company-keys">
                {detail.keys.length === 0 ? (
                  'No company keys'
                ) : (
                  <>
                    Keys:{' '}
                    {detail.keys.map((key) => (
                      <code key={key} className="company-key">
                        {key}
                      </code>
                    ))}
                  </>
                )}
              </div>
              <div className="actions">
                <button type="button" onClick={() => setEditing(true)}>
                  Edit
                </button>
                <button type="button" onClick={removeCompany}>
                  Delete company
                </button>
              </div>
            </div>
          )}

          {duplicates.length > 0 && (
            <div className="panel merge-prompt" role="status">
              <div className="section-head">
                <h2>Possible duplicate companies</h2>
                <div className="actions">
                  <span className="muted small">Merging is never automatic</span>
                  <button type="button" onClick={() => dismissAll(duplicates)}>
                    Dismiss
                  </button>
                </div>
              </div>
              <p className="muted small">
                These names look like the same company. Review the candidate and choose which company row
                to keep; offers and notes are moved, never merged silently.
              </p>
              {duplicates.map((c) => (
                <div className="merge-row" key={c.id}>
                  <div>
                    <Link to={`/companies/${c.id}`}>{c.name}</Link>{' '}
                    <span className="muted small">
                      score {c.score} · {c.reason}
                    </span>
                  </div>
                  <div className="actions">
                    <button type="button" onClick={() => openMerge(c)}>
                      Merge…
                    </button>
                    <button type="button" className="link-button" onClick={() => dismissCandidate(c.id)}>
                      Dismiss
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="company-notes-panel">
          <div className="section-head">
            <h2>Company notes</h2>
            <span className="muted">{detail.notes.length} note(s)</span>
          </div>
          {detail.notes.length === 0 ? (
            <p className="empty">No company notes yet.</p>
          ) : (
            <div className="notes">
              {detail.notes.map((n) => (
                <article className="note" key={n.id}>
                  <p>{n.body}</p>
                  <div className="muted small">{fmtDateTime(n.created_at)}</div>
                  <div className="actions">
                    <button type="button" onClick={() => deleteNote(n.id)}>
                      Delete
                    </button>
                  </div>
                </article>
              ))}
            </div>
          )}
          <form className="stack" onSubmit={(e) => void addNote(e)}>
            <label>
              New company note
              <textarea
                value={noteBody}
                onChange={(e) => setNoteBody(e.target.value)}
                rows={3}
                placeholder="recruiter contact, salary notes, …"
              />
            </label>
            <button type="submit" className="primary" disabled={!noteBody.trim()}>
              Add note
            </button>
          </form>
        </div>
      </section>

      <section className="panel">
        <div className="section-head">
          <h2>Offers</h2>
          <span className="muted">
            {offers?.total ?? 0} match{(offers?.total ?? 0) === 1 ? '' : 'es'}
          </span>
        </div>
        <div className="filters">
          <label>
            Search
            <input
              type="search"
              placeholder="title or description (FTS)"
              value={offerSearch}
              onChange={(e) => setOfferSearch(e.target.value)}
            />
          </label>
          <label>
            Sort
            <select
              value={offersView.sort}
              onChange={(e) => {
                const sort = e.target.value
                setOffersView((v) => ({ ...v, sort, order: defaultOfferSortOrder(sort), page: 1 }))
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
            <button type="button" onClick={resetOfferFilters}>
              Reset
            </button>
          </div>
        </div>
        {offersLoading && !offers ? (
          <p className="muted" aria-busy="true">
            Loading offers…
          </p>
        ) : (
          <OfferTable
            offers={offers?.offers ?? []}
            showCompany={false}
            emptyText={
              offersView.q ? 'No offers match this search.' : 'This company has no offers yet.'
            }
            sort={offersView.sort}
            order={offersView.order}
            onSort={handleOfferSort}
          />
        )}
        <div className="pagination">
          <button
            type="button"
            disabled={offersView.page <= 1}
            onClick={() =>
              setOffersView((v) => ({ ...v, page: Math.max(1, v.page - 1) }))
            }
          >
            ← Previous
          </button>
          <span className="muted">
            {(offers?.total ?? 0) === 0
              ? '0'
              : `${(offersView.page - 1) * offersView.perPage + 1}–${Math.min(
                  offersView.page * offersView.perPage,
                  offers?.total ?? 0,
                )}`}{' '}
            of {offers?.total ?? 0}
          </span>
          <button
            type="button"
            disabled={
              (offers?.total ?? 0) === 0 ||
              offersView.page * offersView.perPage >= (offers?.total ?? 0)
            }
            onClick={() => setOffersView((v) => ({ ...v, page: v.page + 1 }))}
          >
            Next →
          </button>
          <PageSizeSelect value={offersView.perPage} onChange={changeCompanyPerPage} />
        </div>
      </section>

      {confirm && <ConfirmDialog request={confirm} onClose={() => setConfirm(null)} />}

      {mergeTarget && (
        <div className="modal-backdrop" role="presentation">
          <div className="modal" role="dialog" aria-modal="true" aria-labelledby="merge-company-title">
            <div className="modal-head">
              <h2 id="merge-company-title">Merge companies</h2>
              <button type="button" className="link-button" onClick={() => setMergeTarget(null)}>
                Close
              </button>
            </div>
            <p>
              <strong>{detail.company.name}</strong> and <strong>{mergeTarget.name}</strong> may be the same
              company. Choose the company row to keep:
            </p>
            <div className="merge-choice">
              <label>
                <input
                  type="radio"
                  name="keep-company"
                  checked={keepID === detail.company.id}
                  onChange={() => setKeepID(detail.company.id)}
                />
                Keep <strong>{detail.company.name}</strong> — merge “{mergeTarget.name}” into it
              </label>
              <label>
                <input
                  type="radio"
                  name="keep-company"
                  checked={keepID === mergeTarget.id}
                  onChange={() => setKeepID(mergeTarget.id)}
                />
                Keep <strong>{mergeTarget.name}</strong> — merge “{detail.company.name}” into it
              </label>
            </div>
            <p className="muted small">
              Offers, provenance and company notes move to the kept company. Its name stays; the other
              company row is removed. This cannot be undone automatically.
            </p>
            {mergeError && <div className="error-box">{mergeError}</div>}
            <div className="modal-actions">
              <button type="button" onClick={() => setMergeTarget(null)}>
                Cancel
              </button>
              <button type="button" className="primary" disabled={mergeBusy} onClick={() => void confirmMerge()}>
                {mergeBusy ? 'Merging…' : 'Merge companies'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function readDismissed(companyID: number): number[] {
  try {
    const raw = window.localStorage.getItem(dismissedKey(companyID))
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((v): v is number => typeof v === 'number')
  } catch {
    return []
  }
}

function writeDismissed(companyID: number, ids: number[]) {
  try {
    window.localStorage.setItem(dismissedKey(companyID), JSON.stringify(ids))
  } catch {
    // localStorage may be unavailable; the banner is simply shown again.
  }
}
