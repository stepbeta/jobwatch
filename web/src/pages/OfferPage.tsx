import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { EditViewEditor } from '../components/EditViewEditor'
import { Link } from '../components/Link'
import { RemoteBadge, ScoreBadge } from '../components/ScoreBadge'
import { ActionIcon, EventIcon } from '../components/icons'
import {
  APP_STATES,
  EVENT_TYPES,
  OFFER_STATES,
  appLabel,
  eventLabel,
  eventTimestamp,
  fmtDate,
  fmtDateTime,
  remoteLabel,
  splitLocalInput,
  toLocalInput,
} from '../format'
import type { NoticeKind } from '../App'
import type {
  ApplicationState,
  EventType,
  Offer,
  OfferDetail,
  OfferState,
  SnapshotDiff,
  SnapshotOrigin,
} from '../types'

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

function originLabel(origin: SnapshotOrigin): string {
  return origin === 'manual' ? 'Manual' : 'Scan'
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

  const [appState, setAppState] = useState<ApplicationState>('none')
  const [offerState, setOfferState] = useState<OfferState>('active')
  const [location, setLocation] = useState('')
  const [saving, setSaving] = useState(false)

  const [eventType, setEventType] = useState<EventType>('cv_sent')
  const [eventDate, setEventDate] = useState(() => splitLocalInput(toLocalInput(new Date())).date)
  const [eventTime, setEventTime] = useState(() => splitLocalInput(toLocalInput(new Date())).time)
  const [eventDesc, setEventDesc] = useState('')
  const [editingEvent, setEditingEvent] = useState<{
    id: number
    type: EventType
    description: string
    date: string
    time: string
  } | null>(null)
  const [savingEvent, setSavingEvent] = useState(false)
  const [expandedEventNotes, setExpandedEventNotes] = useState<number[]>([])
  const [highlightEventID, setHighlightEventID] = useState<number | null>(null)

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

  const [editingDescription, setEditingDescription] = useState(false)
  const [descriptionDraft, setDescriptionDraft] = useState('')
  const [savingDescription, setSavingDescription] = useState(false)

  const [selectedSnapshotID, setSelectedSnapshotID] = useState<number | null>(null)
  const [snapshotTexts, setSnapshotTexts] = useState<Record<number, string>>({})
  const [snapshotDiffs, setSnapshotDiffs] = useState<Record<string, SnapshotDiff>>({})
  const [loadingSnapshotTextID, setLoadingSnapshotTextID] = useState<number | null>(null)
  const [loadingSnapshotDiffKey, setLoadingSnapshotDiffKey] = useState<string | null>(null)

  const load = useCallback(async (): Promise<OfferDetail | null> => {
    try {
      const d = await api.getOffer(id)
      setDetail(d)
      setSelectedSnapshotID((prev) => {
        if (prev !== null && d.snapshots.some((s) => s.id === prev)) return prev
        return d.snapshots.length > 0 ? d.snapshots[d.snapshots.length - 1].id : null
      })
      setAppState(d.offer.application_state)
      setOfferState(d.offer.offer_state)
      setLocation(d.offer.location ?? '')
      setError('')
      return d
    } catch (e) {
      setError(errorMessage(e))
      return null
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
    setEditingEvent(null)
    setExpandedEventNotes([])
    setHighlightEventID(null)
    setSelectedSnapshotID(null)
    setSnapshotTexts({})
    setSnapshotDiffs({})
    setLoadingSnapshotTextID(null)
    setLoadingSnapshotDiffKey(null)
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
    const snaps = detail?.snapshots ?? []
    const active =
      snaps.find((s) => s.id === selectedSnapshotID) ?? snaps[snaps.length - 1]
    let text = ''
    if (active) {
      text =
        active.id === detail?.latest_snapshot?.id
          ? detail?.latest_snapshot?.raw_text ?? ''
          : snapshotTexts[active.id] ?? ''
    }
    setDescriptionDraft(text)
    setEditingDescription(true)
  }

  async function saveDescription() {
    setSavingDescription(true)
    try {
      await api.patchOffer(id, { description: descriptionDraft })
      setEditingDescription(false)
      const d = await load()
      setSelectedSnapshotID(
        d && d.snapshots.length > 0 ? d.snapshots[d.snapshots.length - 1].id : null,
      )
      onNotice('success', 'Description updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setSavingDescription(false)
    }
  }

  // Fetch the selected version's text only when it is not the latest one
  // (the detail response already carries the latest full text).
  useEffect(() => {
    if (!detail || detail.offer.id !== id || detail.snapshots.length === 0) return
    const foundIndex = detail.snapshots.findIndex((s) => s.id === selectedSnapshotID)
    const activeIndex = foundIndex >= 0 ? foundIndex : detail.snapshots.length - 1
    const active = detail.snapshots[activeIndex]
    if (!active || active.id === detail.latest_snapshot?.id || snapshotTexts[active.id] !== undefined) {
      return
    }
    let cancelled = false
    setLoadingSnapshotTextID(active.id)
    api
      .getSnapshotText(id, active.id)
      .then((text) => {
        if (!cancelled) setSnapshotTexts((prev) => ({ ...prev, [active.id]: text }))
      })
      .catch((err) => {
        if (!cancelled) {
          const text = errorMessage(err)
          setError(text)
          onNotice('error', text)
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingSnapshotTextID(null)
      })
    return () => {
      cancelled = true
    }
  }, [detail, id, selectedSnapshotID, snapshotTexts, onNotice])

  // Fetch the diff between the selected version and the one before it.
  useEffect(() => {
    if (!detail || detail.offer.id !== id || detail.snapshots.length === 0) return
    const foundIndex = detail.snapshots.findIndex((s) => s.id === selectedSnapshotID)
    const activeIndex = foundIndex >= 0 ? foundIndex : detail.snapshots.length - 1
    const active = detail.snapshots[activeIndex]
    const previous = activeIndex > 0 ? detail.snapshots[activeIndex - 1] : undefined
    if (!active || !previous) return
    const key = `${previous.id}:${active.id}`
    if (snapshotDiffs[key]) return
    let cancelled = false
    setLoadingSnapshotDiffKey(key)
    api
      .getSnapshotDiff(id, previous.id, active.id)
      .then((res) => {
        if (!cancelled) setSnapshotDiffs((prev) => ({ ...prev, [key]: res.snapshot_diff }))
      })
      .catch((err) => {
        if (!cancelled) {
          const text = errorMessage(err)
          setError(text)
          onNotice('error', text)
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingSnapshotDiffKey(null)
      })
    return () => {
      cancelled = true
    }
  }, [detail, id, selectedSnapshotID, snapshotDiffs, onNotice])

  async function addEvent(e: FormEvent) {
    e.preventDefault()
    if (!eventDate) {
      onNotice('error', 'Occurred at date is required.')
      return
    }
    try {
      await api.createEvent(id, {
        type: eventType,
        description: eventDesc.trim() || undefined,
        occurred_at: eventTimestamp(eventDate, eventTime) || undefined,
      })
      setEventDesc('')
      const now = splitLocalInput(toLocalInput(new Date()))
      setEventDate(now.date)
      setEventTime(now.time)
      await load()
      onNotice('success', 'Timeline event added.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    }
  }

  function startEventEdit(ev: OfferDetail['events'][number]) {
    const local = splitLocalInput(toLocalInput(new Date(ev.occurred_at)))
    setEditingEvent({
      id: ev.id,
      type: ev.type,
      description: ev.description ?? '',
      date: local.date,
      time: local.time,
    })
  }

  // The PATCH always sends all three editable fields: the form shows the
  // complete event, so there is no partial-field ambiguity. An empty
  // description is sent as null, which clears it.
  async function saveEvent(e: FormEvent) {
    e.preventDefault()
    if (!editingEvent) return
    if (!editingEvent.date) {
      onNotice('error', 'Occurred at date is required.')
      return
    }
    setSavingEvent(true)
    try {
      await api.updateEvent(id, editingEvent.id, {
        type: editingEvent.type,
        description: editingEvent.description.trim() || null,
        occurred_at: eventTimestamp(editingEvent.date, editingEvent.time),
      })
      setEditingEvent(null)
      await load()
      onNotice('success', 'Timeline event updated.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setSavingEvent(false)
    }
  }

  function toggleEventNotes(eventId: number) {
    setExpandedEventNotes((ids) =>
      ids.includes(eventId) ? ids.filter((v) => v !== eventId) : [...ids, eventId],
    )
  }

  function focusEvent(eventId: number) {
    setHighlightEventID(eventId)
    const el = document.getElementById(`event-${eventId}`)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  useEffect(() => {
    if (highlightEventID === null) return
    const timer = window.setTimeout(() => setHighlightEventID(null), 1800)
    return () => window.clearTimeout(timer)
  }, [highlightEventID])

  function deleteEvent(eventId: number) {
    setConfirm({
      title: 'Delete timeline event',
      message: 'Delete this timeline event? Notes pinned to it keep their text but lose the pin.',
      confirmLabel: 'Delete event',
      onConfirm: async () => {
        await api.deleteEvent(id, eventId)
        setExpandedEventNotes((ids) => ids.filter((v) => v !== eventId))
        setEditingEvent((current) => (current?.id === eventId ? null : current))
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
    if (!detail || !detail.company) return
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

  // Both visual directions are derived from the detail payload: notes
  // carry pinned_event_id, so no extra request is needed.
  const notesByEvent = new Map<number, OfferDetail['notes']>()
  for (const n of detail.notes) {
    if (n.pinned_event_id === null) continue
    const list = notesByEvent.get(n.pinned_event_id) ?? []
    list.push(n)
    notesByEvent.set(n.pinned_event_id, list)
  }

  const snapshots = detail.snapshots
  const selectedIndex = snapshots.findIndex((s) => s.id === selectedSnapshotID)
  const activeIndex = selectedIndex >= 0 ? selectedIndex : snapshots.length - 1
  const activeSnapshot = activeIndex >= 0 ? snapshots[activeIndex] : undefined
  const previousSnapshot = activeIndex > 0 ? snapshots[activeIndex - 1] : undefined
  const activeIsLatest = activeSnapshot !== undefined && activeSnapshot.id === detail.latest_snapshot?.id
  const selectedText = !activeSnapshot
    ? ''
    : activeIsLatest
      ? detail.latest_snapshot?.raw_text ?? ''
      : snapshotTexts[activeSnapshot.id] ?? ''
  const selectedTextReady =
    !activeSnapshot || activeIsLatest || snapshotTexts[activeSnapshot.id] !== undefined
  const diffKey =
    activeSnapshot && previousSnapshot ? `${previousSnapshot.id}:${activeSnapshot.id}` : ''
  const selectedDiff = diffKey ? snapshotDiffs[diffKey] : undefined
  const textLoading = activeSnapshot !== undefined && loadingSnapshotTextID === activeSnapshot.id
  const diffLoading = diffKey !== '' && loadingSnapshotDiffKey === diffKey

  return (
    <div className="page">
      <div className="breadcrumb">
        <Link to="/">Dashboard</Link> /{' '}
        {detail.company ? (
          <Link to={`/companies/${detail.company.id}`}>{detail.company.name}</Link>
        ) : (
          <span>Incomplete offer</span>
        )}
      </div>

      {error && <div className="error-box">{error}</div>}

      <section className="panel offer-header">
        <div className="offer-identity">
          <div className="offer-title">
            <h1>{detail.offer.title || 'Untitled offer'}</h1>
            <div className="muted">
              {detail.offer.location ?? 'No location'} · {remoteLabel(detail.offer.remote_policy)} · first seen{' '}
              {fmtDateTime(detail.offer.first_seen_at)} · {detail.snapshot_count} description{' '}
              version{detail.snapshot_count === 1 ? '' : 's'}
            </div>
          </div>
          <div className="badges">
            <ScoreBadge
              label="Title"
              value={detail.offer.title_affinity}
              reasons={detail.offer.match_reasons}
              kind="title"
            />
            <ScoreBadge
              label="Location"
              value={detail.offer.location_affinity}
              reasons={detail.offer.match_reasons}
              kind="location"
            />
            <RemoteBadge
              policy={detail.offer.remote_policy}
              accepted={detail.offer.remote_policy_accepted}
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
        </div>

        <div className="offer-provenance-panel">
          <div className="section-head">
            <h2>Provenance</h2>
            <span className="muted">
              {detail.provenance.length} link{detail.provenance.length === 1 ? '' : 's'}
            </span>
          </div>
          <p className="muted small">
            Every link where this offer was seen. Open them side by side to decide whether they are really
            the same offer; otherwise split them into two offers.
          </p>
          {detail.provenance.length === 0 ? (
            <p className="empty">No provenance rows.</p>
          ) : (
            <div className="provenance provenance-compact">
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
        </div>
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
              {detail.events.map((ev) => {
                const pinnedNotes = notesByEvent.get(ev.id) ?? []
                const notesOpen = expandedEventNotes.includes(ev.id)
                return (
                  <li
                    key={ev.id}
                    id={`event-${ev.id}`}
                    tabIndex={-1}
                    className={highlightEventID === ev.id ? 'timeline-highlight' : undefined}
                  >
                    <span className="timeline-dot" />
                    <div className="timeline-body">
                      {editingEvent?.id === ev.id ? (
                        <form className="stack event-edit" onSubmit={(e) => void saveEvent(e)}>
                          <div className="row">
                            <label>
                              Type
                              <select
                                value={editingEvent.type}
                                onChange={(e) =>
                                  setEditingEvent({
                                    ...editingEvent,
                                    type: e.target.value as EventType,
                                  })
                                }
                              >
                                {EVENT_TYPES.map((t) => (
                                  <option key={t} value={t}>
                                    {eventLabel(t)}
                                  </option>
                                ))}
                              </select>
                            </label>
                            <label>
                              Occurred at
                              <span className="date-time">
                                <input
                                  type="date"
                                  required
                                  aria-label="Occurred at date"
                                  value={editingEvent.date}
                                  onChange={(e) =>
                                    setEditingEvent({ ...editingEvent, date: e.target.value })
                                  }
                                />
                                <input
                                  type="time"
                                  aria-label="Occurred at time (optional)"
                                  value={editingEvent.time}
                                  onChange={(e) =>
                                    setEditingEvent({ ...editingEvent, time: e.target.value })
                                  }
                                />
                              </span>
                            </label>
                          </div>
                          <label>
                            Description
                            <textarea
                              rows={2}
                              value={editingEvent.description}
                              onChange={(e) =>
                                setEditingEvent({ ...editingEvent, description: e.target.value })
                              }
                              placeholder="optional"
                            />
                          </label>
                          <div className="actions">
                            <button type="submit" className="primary" disabled={savingEvent}>
                              {savingEvent ? 'Saving…' : 'Save'}
                            </button>
                            <button
                              type="button"
                              disabled={savingEvent}
                              onClick={() => setEditingEvent(null)}
                            >
                              Cancel
                            </button>
                          </div>
                        </form>
                      ) : (
                        <>
                          <div className="timeline-head">
                            <div className="timeline-title">
                              <EventIcon type={ev.type} />
                              <strong>{eventLabel(ev.type)}</strong>{' '}
                              <span className="muted small">{fmtDateTime(ev.occurred_at)}</span>
                            </div>
                            <div className="timeline-actions">
                              {pinnedNotes.length > 0 && (
                                <button
                                  type="button"
                                  className="note-count"
                                  aria-expanded={notesOpen}
                                  aria-label={`${pinnedNotes.length} pinned note${pinnedNotes.length === 1 ? '' : 's'}`}
                                  title={`${pinnedNotes.length} pinned note${pinnedNotes.length === 1 ? '' : 's'}`}
                                  onClick={() => toggleEventNotes(ev.id)}
                                >
                                  <ActionIcon name="note" />
                                  {pinnedNotes.length}
                                </button>
                              )}
                              <button
                                type="button"
                                className="icon-button"
                                aria-label="Edit event"
                                title="Edit event"
                                onClick={() => startEventEdit(ev)}
                              >
                                <ActionIcon name="edit" />
                              </button>
                              <button
                                type="button"
                                className="icon-button danger"
                                aria-label="Delete event"
                                title="Delete event"
                                onClick={() => void deleteEvent(ev.id)}
                              >
                                <ActionIcon name="trash" />
                              </button>
                            </div>
                          </div>
                          {ev.description && <div>{ev.description}</div>}
                          {notesOpen && (
                            <div className="event-notes">
                              {pinnedNotes.map((n) => (
                                <div className="event-note" key={n.id}>
                                  <p>{n.body}</p>
                                  <span className="muted small">{fmtDateTime(n.created_at)}</span>
                                </div>
                              ))}
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  </li>
                )
              })}
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
                <span className="date-time">
                  <input
                    type="date"
                    required
                    aria-label="Occurred at date"
                    value={eventDate}
                    onChange={(e) => setEventDate(e.target.value)}
                  />
                  <input
                    type="time"
                    aria-label="Occurred at time (optional)"
                    value={eventTime}
                    onChange={(e) => setEventTime(e.target.value)}
                  />
                </span>
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
              {detail.notes.map((n) => {
                const pinnedEvent =
                  n.pinned_event_id === null
                    ? undefined
                    : detail.events.find((e) => e.id === n.pinned_event_id)
                return (
                  <article className={`note${pinnedEvent ? ' note-pinned' : ''}`} key={n.id}>
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
                        <div className="note-meta">
                          <span className="muted small">{fmtDateTime(n.created_at)}</span>
                          {pinnedEvent ? (
                            <button
                              type="button"
                              className="note-pin-cue"
                              title="Show this event in the timeline"
                              onClick={() => focusEvent(pinnedEvent.id)}
                            >
                              <EventIcon type={pinnedEvent.type} />
                              <span>
                                {eventLabel(pinnedEvent.type)} · {fmtDate(pinnedEvent.occurred_at)}
                              </span>
                            </button>
                          ) : (
                            n.pinned_event_id !== null && (
                              <span className="muted small">pinned to {pinLabel(n.pinned_event_id)}</span>
                            )
                          )}
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
                )
              })}
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
          Description ({detail.snapshot_count} version{detail.snapshot_count === 1 ? '' : 's'})
        </summary>
        {snapshots.length > 0 && activeSnapshot ? (
          <div className="snapshot-viewer">
            <div className="snapshot-head">
              <div className="snapshot-title">
                <strong>
                  Version {activeIndex + 1} of {snapshots.length}
                </strong>
                <span className="muted small">{fmtDateTime(activeSnapshot.captured_at)}</span>
                <span className={`origin-badge origin-${activeSnapshot.origin}`}>
                  {originLabel(activeSnapshot.origin)}
                </span>
              </div>
              <div className="snapshot-nav">
                <button
                  type="button"
                  aria-label="Older version"
                  title="Older version"
                  disabled={activeIndex <= 0}
                  onClick={() => setSelectedSnapshotID(snapshots[activeIndex - 1].id)}
                >
                  «
                </button>
                <select
                  aria-label="Version"
                  value={activeSnapshot.id}
                  onChange={(e) => setSelectedSnapshotID(Number(e.target.value))}
                >
                  {snapshots.map((s, i) => (
                    <option key={s.id} value={s.id}>
                      Version {i + 1} / {snapshots.length} — {originLabel(s.origin)}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  aria-label="Newer version"
                  title="Newer version"
                  disabled={activeIndex >= snapshots.length - 1}
                  onClick={() => setSelectedSnapshotID(snapshots[activeIndex + 1].id)}
                >
                  ›
                </button>
              </div>
            </div>

            {editingDescription ? (
              <div className="stack">
                <EditViewEditor
                  value={descriptionDraft}
                  onChange={setDescriptionDraft}
                  editLabel="Description"
                  placeholder="paste or edit the description"
                  rows={12}
                  viewEmptyText="No description yet."
                  renderView={(text) => <pre>{text}</pre>}
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
                  <button
                    type="button"
                    disabled={savingDescription}
                    onClick={() => setEditingDescription(false)}
                  >
                    Cancel
                  </button>
                </div>
                <p className="muted small">
                  Saving appends a manual snapshot and selects it. Clearing the text stores an empty
                  description.
                </p>
              </div>
            ) : (
              <>
                {textLoading || !selectedTextReady ? (
                  <p className="muted" aria-busy="true">
                    Loading version…
                  </p>
                ) : selectedText.trim() !== '' ? (
                  <pre>{selectedText}</pre>
                ) : (
                  <p className="muted">No description yet.</p>
                )}
                <button type="button" onClick={startDescriptionEdit} disabled={!selectedTextReady}>
                  {selectedText.trim() === '' ? 'Add description' : 'Edit description'}
                </button>
              </>
            )}

            {!editingDescription &&
              (previousSnapshot ? (
                <div className="diff-wrap">
                  <div className="muted small">
                    Changes from version {activeIndex} ({originLabel(previousSnapshot.origin)},{' '}
                    {fmtDateTime(previousSnapshot.captured_at)}) to version {activeIndex + 1} (
                    {originLabel(activeSnapshot.origin)}, {fmtDateTime(activeSnapshot.captured_at)})
                  </div>
                  {diffLoading && !selectedDiff ? (
                    <p className="muted small" aria-busy="true">
                      Loading diff…
                    </p>
                  ) : selectedDiff ? (
                    <div className="diff">
                      {selectedDiff.lines.map((line, i) => (
                        <div key={i} className={`diff-line diff-${line.op}`}>
                          <span className="diff-mark">{diffPrefix(line.op)}</span>
                          {line.text || ' '}
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : (
                <p className="muted small">First captured version — no earlier version to diff against.</p>
              ))}
          </div>
        ) : (
          <>
            <p className="muted">No description captured.</p>
            <button type="button" onClick={startDescriptionEdit}>
              Add description
            </button>
          </>
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
