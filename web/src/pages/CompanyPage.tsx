import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { Link } from '../components/Link'
import { OfferTable } from '../components/OfferTable'
import { fmtDateTime } from '../format'
import type { CompanyDetail } from '../types'

export function CompanyPage({ id }: { id: number }) {
  const [detail, setDetail] = useState<CompanyDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [noteBody, setNoteBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState('')
  const [website, setWebsite] = useState('')

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
    void load()
  }, [load])

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

  async function merge(candidateId: number, candidateName: string) {
    if (!detail) return
    const ok = window.confirm(
      `Merge "${candidateName}" into "${detail.company.name}"?\n\n` +
        'All offers and company notes of the other company move here and its name is deleted. ' +
        'This cannot be undone automatically.',
    )
    if (!ok) return
    try {
      await api.mergeCompany(candidateId, detail.company.id)
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

      {detail.possible_duplicates.length > 0 && (
        <section className="panel merge-prompt">
          <div className="section-head">
            <h2>Possible duplicate companies</h2>
            <span className="muted">Merging is never automatic</span>
          </div>
          {detail.possible_duplicates.map((c) => (
            <div className="merge-row" key={c.id}>
              <div>
                <Link to={`/companies/${c.id}`}>{c.name}</Link>{' '}
                <span className="muted small">
                  score {c.score} · {c.reason}
                </span>
              </div>
              <button type="button" onClick={() => void merge(c.id, c.name)}>
                Merge into this company
              </button>
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
    </div>
  )
}
