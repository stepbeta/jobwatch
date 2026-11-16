import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { Link } from '../components/Link'
import { OfferTable } from '../components/OfferTable'
import { fmtDateTime } from '../format'
import type { CompanyCandidate, CompanyDetail } from '../types'

function dismissedKey(companyID: number): string {
  return `jobwatch.dismissed-duplicates:${companyID}`
}

export function CompanyPage({ id }: { id: number }) {
  const [detail, setDetail] = useState<CompanyDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [noteBody, setNoteBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState('')
  const [website, setWebsite] = useState('')
  const [dismissed, setDismissed] = useState<number[]>([])

  const [mergeTarget, setMergeTarget] = useState<CompanyCandidate | null>(null)
  const [keepID, setKeepID] = useState<number>(id)
  const [mergeBusy, setMergeBusy] = useState(false)
  const [mergeError, setMergeError] = useState('')

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

  async function addNote(e: FormEvent) {
    e.preventDefault()
    if (!noteBody.trim()) return
    try {
      await api.createNote({ entity_type: 'company', entity_id: id, body: noteBody.trim() })
      setNoteBody('')
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function deleteNote(noteId: number) {
    if (!window.confirm('Delete this note?')) return
    try {
      await api.deleteNote(noteId)
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function saveCompany(e: FormEvent) {
    e.preventDefault()
    try {
      await api.updateCompany(id, { name: name.trim(), website: website.trim() || null })
      setEditing(false)
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function removeCompany() {
    if (!detail) return
    if (!window.confirm(`Delete company "${detail.company.name}"? It must have no offers.`)) return
    try {
      await api.deleteCompany(detail.company.id)
      window.location.hash = '#/'
    } catch (err) {
      setError(errorMessage(err))
    }
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

  if (loading && !detail) {
    return <p className="muted page">Loading company…</p>
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

      <section className="panel offer-header">
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
            <div className="actions">
              <button type="button" onClick={() => setEditing(true)}>
                Edit
              </button>
              <button type="button" onClick={() => void removeCompany()}>
                Delete company
              </button>
            </div>
          </div>
        )}
      </section>

      {duplicates.length > 0 && (
        <section className="panel merge-prompt" role="status">
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
            These names look like the same company. Review the candidate and choose which company row to
            keep; offers and notes are moved, never merged silently.
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
        </section>
      )}

      <section className="panel">
        <div className="section-head">
          <h2>Offers</h2>
          <span className="muted">{detail.offers.length} shown</span>
        </div>
        <OfferTable offers={detail.offers} showCompany={false} />
      </section>

      <section className="panel">
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
                  <button type="button" onClick={() => void deleteNote(n.id)}>
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
      </section>

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
