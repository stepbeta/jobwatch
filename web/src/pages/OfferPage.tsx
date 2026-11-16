import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
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
import type { NoticeKind } from '../App'
import type { ApplicationState, EventType, Offer, OfferDetail, OfferState } from '../types'

function hostname(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

function diffPrefix(op: 'equal' | 'added' | 'removed'): string {
  if (op === 'added') return '+'
  if (op === 'removed') return '−'
  return ' '
}

function provenanceStorageKey(offerId: number): string {
  return `jobwatch:provenance-open:${offerId}`
}

function readProvenanceOpen(offerId: number): boolean {
  try {
    return window.localStorage.getItem(provenanceStorageKey(offerId)) === '1'
  } catch {
    return false
  }
}

function writeProvenanceOpen(offerId: number, open: boolean): void {
  try {
    window.localStorage.setItem(provenanceStorageKey(offerId), open ? '1' : '0')
  } catch {
    // Private browsing or a disabled storage API: keep the in-memory state.
  }
}

export function OfferPage({
  id,
  onNotice,
}: {
  id: number
  onNotice: (kind: NoticeKind, text: string) => void
}) {
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

  const [splitMode, setSplitMode] = useState(false)
  const [splitIDs, setSplitIDs] = useState<number[]>([])
  const [splitTitle, setSplitTitle] = useState('')
  const [splitLocation, setSplitLocation] = useState('')
  const [splitting, setSplitting] = useState(false)

  const [mergeOpen, setMergeOpen] = useState(false)
  const [mergeCandidates, setMergeCandidates] = useState<Offer[]>([])
  const [mergeTargetID, setMergeTargetID] = useState('')
  const [mergeLoading, setMergeLoading] = useState(false)
  const [mergeBusy, setMergeBusy] = useState(false)
  const [mergeError, setMergeError] = useState('')
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)

  const [provenanceOpen, setProvenanceOpen] = useState(false)
  const [editingDescription, setEditingDescription] = useState(false)
  const [descriptionDraft, setDescriptionDraft] = useState('')
  const [savingDescription, setSavingDescription] = useState(false)

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
    setSplitMode(false)
    setSplitIDs([])
    setMergeOpen(false)
    setEditingDescription(false)
    setProvenanceOpen(readProvenanceOpen(id))
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
      onNotice('success', 'Offer updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setSaving(false)
    }
  }

  function startDescriptionEdit() {
    setDescriptionDraft(detail?.latest_snapshot?.raw_text ?? '')
    setEditingDescription(true)
  }

  async function saveDescription() {
    setSavingDescription(true)
    try {
      await api.patchOffer(id, { description: descriptionDraft })
      setEditingDescription(false)
      await load()
      onNotice('success', 'Description updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setSavingDescription(false)
    }
  }

  function setProvenanceExpanded(open: boolean) {
    setProvenanceOpen(open)
    writeProvenanceOpen(id, open)
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
      onNotice('success', 'Timeline event added.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  function deleteEvent(eventId: number) {
    setConfirm({
      title: 'Delete timeline event',
      message: 'Delete this timeline event? Notes pinned to it keep their text but lose the pin.',
      confirmLabel: 'Delete event',
      onConfirm: async () => {
        await api.deleteEvent(id, eventId)
        await load()
        onNotice('success', 'Timeline event deleted.')
      },
    })
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
      onNotice('success', 'Note added.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  async function saveNote() {
    if (!editingNote) return
    try {
      await api.updateNote(editingNote.id, { body: editingNote.body })
      setEditingNote(null)
      await load()
      onNotice('success', 'Note updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  function deleteNote(noteId: number) {
    setConfirm({
      title: 'Delete note',
      message: 'Delete this note? This cannot be undone.',
      confirmLabel: 'Delete note',
      onConfirm: async () => {
        await api.deleteNote(noteId)
        await load()
        onNotice('success', 'Note deleted.')
      },
    })
  }

  function startSplit() {
    if (!detail) return
    setSplitMode(true)
    setSplitIDs([])
    setSplitTitle(detail.offer.title)
    setSplitLocation(detail.offer.location ?? '')
  }

  function cancelSplit() {
    setSplitMode(false)
    setSplitIDs([])
  }

  function toggleSplitID(provID: number) {
    setSplitIDs((ids) =>
      ids.includes(provID) ? ids.filter((v) => v !== provID) : [...ids, provID],
    )
  }

  async function splitSelected() {
    if (splitIDs.length === 0) return
    setSplitting(true)
    setError('')
    try {
      const res = await api.splitOffer(id, {
        provenance_ids: splitIDs,
        title: splitTitle.trim() || undefined,
        location: splitLocation.trim() === '' ? null : splitLocation.trim(),
      })
      window.location.hash = `#/offers/${res.offer.id}`
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSplitting(false)
    }
  }

  async function openMerge() {
    if (!detail) return
    setMergeOpen(true)
    setMergeLoading(true)
    setMergeError('')
    setMergeCandidates([])
    setMergeTargetID('')
    try {
      const res = await api.listOffers({ company_id: detail.company.id, limit: 200 })
      const candidates = res.offers.filter((o) => o.id !== id)
      setMergeCandidates(candidates)
      setMergeTargetID(candidates.length > 0 ? String(candidates[0].id) : '')
    } catch (err) {
      setMergeError(errorMessage(err))
    } finally {
      setMergeLoading(false)
    }
  }

  async function confirmMerge() {
    const target = Number(mergeTargetID)
    if (!target) return
    setMergeBusy(true)
    setMergeError('')
    try {
      const res = await api.mergeOffer(id, target)
      window.location.hash = `#/offers/${res.offer.id}`
    } catch (err) {
      setMergeError(errorMessage(err))
    } finally {
      setMergeBusy(false)
    }
  }

  if (loading && !detail) {
    return (
      <p className="muted page" aria-busy="true">
        Loading offer…
      </p>
    )
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
      {detail.offer.offer_state === 'changed' && (
        <div className="changed-banner">
          <strong>Description changed.</strong> Review the added and removed lines in the description diff
          below. Set the offer state back to <em>active</em> to acknowledge it.
        </div>
      )}

      <section className="panel offer-header">
        <div className="offer-title">
          <h1>{detail.offer.title}</h1>
          <div className="muted">
            {detail.offer.location ?? 'No location'} · {detail.offer.remote_policy} · first seen{' '}
            {fmtDateTime(detail.offer.first_seen_at)}
          </div>
        </div>
        <div className="badges">
          <ScoreBadge
            label="Title"
            value={detail.offer.title_confidence}
            reasons={detail.offer.match_reasons}
            kind="title"
          />
          <ScoreBadge
            label="Location"
            value={detail.offer.location_confidence}
            reasons={detail.offer.match_reasons}
            kind="location"
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

      <details
        className="panel"
        open={provenanceOpen}
        onToggle={(e) => setProvenanceExpanded(e.currentTarget.open)}
      >
        <summary>
          Provenance ({detail.provenance.length} link{detail.provenance.length === 1 ? '' : 's'})
        </summary>
        <div className="actions">
          <button type="button" onClick={() => void openMerge()} disabled={mergeLoading}>
            Merge into another offer…
          </button>
          {detail.provenance.length > 1 &&
            (splitMode ? (
              <button type="button" onClick={cancelSplit}>
                Cancel split
              </button>
            ) : (
              <button type="button" onClick={startSplit}>
                Split offer…
              </button>
            ))}
        </div>
        <p className="muted small">
          Every link where this offer was seen. Open them side by side to decide whether they are really the
          same offer; otherwise split them into two offers.
        </p>
        {detail.provenance.length === 0 ? (
          <p className="empty">No provenance rows.</p>
        ) : (
          <div className="provenance">
            {detail.provenance.map((p) => (
              <div className={`prov-card${splitIDs.includes(p.id) ? ' prov-selected' : ''}`} key={p.id}>
                {splitMode && (
                  <label className="prov-select">
                    <input
                      type="checkbox"
                      checked={splitIDs.includes(p.id)}
                      onChange={() => toggleSplitID(p.id)}
                    />
                    Move to a new offer
                  </label>
                )}
                <div className="prov-head">
                  <a href={p.url} target="_blank" rel="noreferrer">
                    {hostname(p.url)}
                  </a>
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
        {splitMode && (
          <div className="split-bar">
            <div className="section-head">
              <h3>Split selected links</h3>
              <span className="muted small">
                {splitIDs.length} selected · at least one link must stay on this offer
              </span>
            </div>
            <div className="form-grid">
              <label>
                New offer title
                <input value={splitTitle} onChange={(e) => setSplitTitle(e.target.value)} />
              </label>
              <label>
                New offer location
                <input
                  value={splitLocation}
                  onChange={(e) => setSplitLocation(e.target.value)}
                  placeholder="e.g. Lyon, France"
                />
              </label>
            </div>
            <p className="muted small">
              Events, notes and snapshots stay with this offer. Change the title or location if the new
              offer would otherwise have the same company, title and location.
            </p>
            <button
              type="button"
              className="primary"
              disabled={splitting || splitIDs.length === 0}
              onClick={() => void splitSelected()}
            >
              {splitting ? 'Splitting…' : 'Create offer from selected links'}
            </button>
          </div>
        )}
      </details>

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

      <details className="panel snapshot" open={detail.offer.offer_state === 'changed'}>
        <summary>
          Description ({detail.snapshot_count} snapshot{detail.snapshot_count === 1 ? '' : 's'})
        </summary>
        {detail.snapshot_diff && (
          <div className="diff-wrap">
            <div className="muted small">
              Changes from snapshot #{detail.snapshot_diff.from.id} (
              {fmtDateTime(detail.snapshot_diff.from.captured_at)}) to snapshot #
              {detail.snapshot_diff.to.id} ({fmtDateTime(detail.snapshot_diff.to.captured_at)})
            </div>
            <div className="diff">
              {detail.snapshot_diff.lines.map((line, i) => (
                <div key={i} className={`diff-line diff-${line.op}`}>
                  <span className="diff-mark">{diffPrefix(line.op)}</span>
                  {line.text || ' '}
                </div>
              ))}
            </div>
          </div>
        )}
        {editingDescription ? (
          <div className="stack">
            <textarea
              value={descriptionDraft}
              onChange={(e) => setDescriptionDraft(e.target.value)}
              rows={12}
              placeholder="paste or edit the description"
            />
            <div className="actions">
              <button
                type="button"
                className="primary"
                disabled={savingDescription}
                onClick={() => void saveDescription()}
              >
                {savingDescription ? 'Saving…' : 'Save'}
              </button>
              <button type="button" disabled={savingDescription} onClick={() => setEditingDescription(false)}>
                Cancel
              </button>
            </div>
            <p className="muted small">
              Saving appends a description snapshot. Clearing the text stores an empty description.
            </p>
          </div>
        ) : detail.latest_snapshot ? (
          <>
            {detail.snapshot_diff && <h3 className="diff-heading">Latest description</h3>}
            {detail.latest_snapshot.raw_text.trim() !== '' ? (
              <pre>{detail.latest_snapshot.raw_text}</pre>
            ) : (
              <p className="muted">No description yet.</p>
            )}
            <button type="button" onClick={startDescriptionEdit}>
              {detail.latest_snapshot.raw_text.trim() === '' ? 'Add description' : 'Edit description'}
            </button>
          </>
        ) : (
          <>
            <p className="muted">No description captured.</p>
            <button type="button" onClick={startDescriptionEdit}>
              Add description
            </button>
          </>
        )}
        {detail.snapshot_count > 1 && !detail.snapshot_diff && (
          <p className="muted small">Older versions are stored as snapshots.</p>
        )}
      </details>

      {confirm && <ConfirmDialog request={confirm} onClose={() => setConfirm(null)} />}

      {mergeOpen && (
        <div className="modal-backdrop" role="presentation">
          <div className="modal" role="dialog" aria-modal="true" aria-labelledby="merge-offer-title">
            <div className="modal-head">
              <h2 id="merge-offer-title">Merge this offer</h2>
              <button type="button" className="link-button" onClick={() => setMergeOpen(false)}>
                Close
              </button>
            </div>
            <p>
              The selected offer keeps its identity and application state. All links, timeline events, notes
              and description snapshots of <strong>{detail.offer.title}</strong> move to it, and the
              redundant offer row is removed.
            </p>
            {mergeError && <div className="error-box">{mergeError}</div>}
            {mergeLoading ? (
              <p className="muted">Loading offers…</p>
            ) : mergeCandidates.length === 0 ? (
              <p className="muted">This company has no other offer to merge into.</p>
            ) : (
              <>
                <label>
                  Merge into
                  <select value={mergeTargetID} onChange={(e) => setMergeTargetID(e.target.value)}>
                    {mergeCandidates.map((o) => (
                      <option key={o.id} value={o.id}>
                        {o.title} — {o.location ?? 'no location'} (#{o.id})
                      </option>
                    ))}
                  </select>
                </label>
                <div className="modal-actions">
                  <button type="button" onClick={() => setMergeOpen(false)}>
                    Cancel
                  </button>
                  <button
                    type="button"
                    className="primary"
                    disabled={mergeBusy || !mergeTargetID}
                    onClick={() => void confirmMerge()}
                  >
                    {mergeBusy ? 'Merging…' : 'Merge offers'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
