import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { Link } from '../components/Link'
import { ScoreBadge } from '../components/ScoreBadge'
import {
  APP_STATES,
  EVENT_TYPES,
  OFFER_STATES,
  appLabel,
  eventLabel,
  fmtDateTime,
  fromLocalInput,
  toLocalInput,
} from '../format'
import type { ApplicationState, EventType, OfferDetail, OfferState } from '../types'

function hostname(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

export function OfferPage({ id }: { id: number }) {
  const [detail, setDetail] = useState<OfferDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [appState, setAppState] = useState<ApplicationState>('todo')
  const [offerState, setOfferState] = useState<OfferState>('active')
  const [location, setLocation] = useState('')
  const [saving, setSaving] = useState(false)

  const [eventType, setEventType] = useState<EventType>('cv_sent')
  const [eventAt, setEventAt] = useState(() => toLocalInput(new Date()))
  const [eventDesc, setEventDesc] = useState('')

  const [noteBody, setNoteBody] = useState('')
  const [notePin, setNotePin] = useState('')
  const [editingNote, setEditingNote] = useState<{ id: number; body: string } | null>(null)

  const load = useCallback(async () => {
    try {
      const d = await api.getOffer(id)
      setDetail(d)
      setAppState(d.offer.application_state)
      setOfferState(d.offer.offer_state)
      setLocation(d.offer.location ?? '')
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

  async function saveStates(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      await api.patchOffer(id, {
        application_state: appState,
        offer_state: offerState,
        location: location.trim() === '' ? null : location.trim(),
      })
      await load()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  async function addEvent(e: FormEvent) {
    e.preventDefault()
    try {
      await api.createEvent(id, {
        type: eventType,
        description: eventDesc.trim() || undefined,
        occurred_at: fromLocalInput(eventAt) || undefined,
      })
      setEventDesc('')
      setEventAt(toLocalInput(new Date()))
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function deleteEvent(eventId: number) {
    if (!window.confirm('Delete this timeline event?')) return
    try {
      await api.deleteEvent(id, eventId)
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function addNote(e: FormEvent) {
    e.preventDefault()
    if (!noteBody.trim()) return
    try {
      await api.createNote({
        entity_type: 'offer',
        entity_id: id,
        body: noteBody.trim(),
        pinned_event_id: notePin ? Number(notePin) : null,
      })
      setNoteBody('')
      setNotePin('')
      await load()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  async function saveNote() {
    if (!editingNote) return
    try {
      await api.updateNote(editingNote.id, { body: editingNote.body })
      setEditingNote(null)
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

  if (loading && !detail) {
    return <p className="muted page">Loading offer…</p>
  }
  if (!detail) {
    return (
      <div className="page">
        <div className="error-box">{error || 'Offer not found'}</div>
        <Link to="/">← Back to dashboard</Link>
      </div>
    )
  }

  const pinLabel = (eventId: number | null): string | null => {
    if (eventId === null) return null
    const ev = detail.events.find((e) => e.id === eventId)
    if (!ev) return `event #${eventId}`
    return `${eventLabel(ev.type)} · ${fmtDateTime(ev.occurred_at)}`
  }

  return (
    <div className="page">
      <div className="breadcrumb">
        <Link to="/">Dashboard</Link> / <Link to={`/companies/${detail.company.id}`}>{detail.company.name}</Link>
      </div>

      {error && <div className="error-box">{error}</div>}

      <section className="panel offer-header">
        <div className="offer-title">
          <h1>{detail.offer.title}</h1>
          <div className="muted">
            {detail.offer.location ?? 'No location'} · {detail.offer.remote_policy} · first seen{' '}
            {fmtDateTime(detail.offer.first_seen_at)}
          </div>
        </div>
        <div className="badges">
          <ScoreBadge label="Title" value={detail.offer.title_confidence} reasons={detail.offer.match_reasons} />
          <ScoreBadge
            label="Location"
            value={detail.offer.location_confidence}
            reasons={detail.offer.match_reasons}
          />
        </div>
        <form className="state-controls" onSubmit={(e) => void saveStates(e)}>
          <label>
            Application
            <select value={appState} onChange={(e) => setAppState(e.target.value as ApplicationState)}>
              {APP_STATES.map((s) => (
                <option key={s} value={s}>
                  {appLabel(s)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Offer
            <select value={offerState} onChange={(e) => setOfferState(e.target.value as OfferState)}>
              {OFFER_STATES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label>
            Location
            <input
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder="correct the location"
            />
          </label>
          <button type="submit" className="primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </form>
      </section>

      <section className="panel">
        <div className="section-head">
          <h2>Provenance</h2>
          <span className="muted">Every link where this offer was seen</span>
        </div>
        {detail.provenance.length === 0 ? (
          <p className="empty">No provenance rows.</p>
        ) : (
          <div className="provenance">
            {detail.provenance.map((p) => (
              <div className="prov-card" key={p.id}>
                <div className="prov-head">
                  <a href={p.url} target="_blank" rel="noreferrer">
                    {hostname(p.url)}
                  </a>
                  <button type="button" disabled title="Splitting an offer is not implemented yet">
                    Split offer
                  </button>
                </div>
                <div className="muted small">
                  {p.source_name ?? 'Manual import'}
                  {p.external_uid ? ` · uid ${p.external_uid}` : ''}
                </div>
                <div className="small">
                  first seen {fmtDateTime(p.first_seen_at)}
                  <br />
                  last seen {fmtDateTime(p.last_seen_at)}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <div className="detail-columns">
        <section className="panel">
          <div className="section-head">
            <h2>Timeline</h2>
            <span className="muted">{detail.events.length} event(s)</span>
          </div>
          {detail.events.length === 0 ? (
            <p className="empty">No events yet.</p>
          ) : (
            <ol className="timeline">
              {detail.events.map((ev) => (
                <li key={ev.id}>
                  <span className="timeline-dot" />
                  <div className="timeline-body">
                    <div>
                      <strong>{eventLabel(ev.type)}</strong>{' '}
                      <span className="muted small">{fmtDateTime(ev.occurred_at)}</span>
                    </div>
                    {ev.description && <div>{ev.description}</div>}
                    <button type="button" className="link-button" onClick={() => void deleteEvent(ev.id)}>
                      Delete
                    </button>
                  </div>
                </li>
              ))}
            </ol>
          )}
          <form className="stack" onSubmit={(e) => void addEvent(e)}>
            <div className="row">
              <label>
                Type
                <select value={eventType} onChange={(e) => setEventType(e.target.value as EventType)}>
                  {EVENT_TYPES.map((t) => (
                    <option key={t} value={t}>
                      {eventLabel(t)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Occurred at
                <input type="datetime-local" value={eventAt} onChange={(e) => setEventAt(e.target.value)} />
              </label>
            </div>
            <label>
              Description
              <input
                value={eventDesc}
                onChange={(e) => setEventDesc(e.target.value)}
                placeholder="optional"
              />
            </label>
            <button type="submit" className="primary">
              Add event
            </button>
          </form>
        </section>

        <section className="panel">
          <div className="section-head">
            <h2>Notes</h2>
            <span className="muted">{detail.notes.length} note(s)</span>
          </div>
          {detail.notes.length === 0 ? (
            <p className="empty">No notes yet.</p>
          ) : (
            <div className="notes">
              {detail.notes.map((n) => (
                <article className="note" key={n.id}>
                  {editingNote?.id === n.id ? (
                    <>
                      <textarea
                        value={editingNote.body}
                        onChange={(e) => setEditingNote({ id: n.id, body: e.target.value })}
                        rows={4}
                      />
                      <div className="actions">
                        <button type="button" className="primary" onClick={() => void saveNote()}>
                          Save
                        </button>
                        <button type="button" onClick={() => setEditingNote(null)}>
                          Cancel
                        </button>
                      </div>
                    </>
                  ) : (
                    <>
                      <p>{n.body}</p>
                      <div className="muted small">
                        {fmtDateTime(n.created_at)}
                        {n.pinned_event_id !== null && ` · pinned to ${pinLabel(n.pinned_event_id)}`}
                      </div>
                      <div className="actions">
                        <button type="button" onClick={() => setEditingNote({ id: n.id, body: n.body })}>
                          Edit
                        </button>
                        <button type="button" onClick={() => void deleteNote(n.id)}>
                          Delete
                        </button>
                      </div>
                    </>
                  )}
                </article>
              ))}
            </div>
          )}
          <form className="stack" onSubmit={(e) => void addNote(e)}>
            <label>
              New note
              <textarea
                value={noteBody}
                onChange={(e) => setNoteBody(e.target.value)}
                rows={3}
                placeholder="what to remember"
              />
            </label>
            <label>
              Pin to event
              <select value={notePin} onChange={(e) => setNotePin(e.target.value)}>
                <option value="">Not pinned</option>
                {detail.events.map((ev) => (
                  <option key={ev.id} value={ev.id}>
                    {eventLabel(ev.type)} · {fmtDateTime(ev.occurred_at)}
                  </option>
                ))}
              </select>
            </label>
            <button type="submit" className="primary" disabled={!noteBody.trim()}>
              Add note
            </button>
          </form>
        </section>
      </div>

      <details className="panel snapshot">
        <summary>
          Description ({detail.snapshot_count} snapshot{detail.snapshot_count === 1 ? '' : 's'})
        </summary>
        {detail.latest_snapshot ? (
          <pre>{detail.latest_snapshot.raw_text || '(empty description)'}</pre>
        ) : (
          <p className="muted">No snapshot captured.</p>
        )}
        {detail.snapshot_count > 1 && (
          <p className="muted small">Older versions are stored as snapshots; the diff view is not implemented yet.</p>
        )}
      </details>
    </div>
  )
}
