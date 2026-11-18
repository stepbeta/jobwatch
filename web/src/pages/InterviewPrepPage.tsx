import { useCallback, useEffect, useMemo, useState, type DragEvent, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { ActionIcon } from '../components/icons'
import { Markdown } from '../components/Markdown'
import { Modal } from '../components/Modal'
import type { InterviewEntry } from '../types'

type Notice = (kind: 'info' | 'success' | 'error', text: string) => void

type EditorState = { mode: 'create' } | { mode: 'edit'; entry: InterviewEntry }

type DragState = { kind: 'entry'; id: number } | { kind: 'section'; category: string }

type EntryPatch = {
  question?: string
  answer?: string
  category?: string | null
  display_order?: number | null
}

const UNCATEGORIZED = 'Uncategorized'
const CATEGORY_ORDER_KEY = 'jobwatch.interview.categoryOrder'
const NEW_CATEGORY = '__new__'

function compareEntries(a: InterviewEntry, b: InterviewEntry): number {
  if (a.display_order !== null && b.display_order !== null) {
    return a.display_order - b.display_order || a.id - b.id
  }
  if (a.display_order !== null) return -1
  if (b.display_order !== null) return 1
  return a.id - b.id
}

function compareCategories(a: string, b: string): number {
  return a.localeCompare(b, undefined, { sensitivity: 'base' })
}

function readSavedOrder(): string[] {
  try {
    const raw = localStorage.getItem(CATEGORY_ORDER_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : []
  } catch {
    return []
  }
}

function groupEntries(entries: InterviewEntry[]): Map<string | null, InterviewEntry[]> {
  const grouped = new Map<string | null, InterviewEntry[]>()
  for (const entry of entries) {
    const list = grouped.get(entry.category) ?? []
    list.push(entry)
    grouped.set(entry.category, list)
  }
  for (const list of grouped.values()) list.sort(compareEntries)
  return grouped
}

function buildSections(
  entries: InterviewEntry[],
  savedOrder: string[],
): Array<{ category: string | null; entries: InterviewEntry[] }> {
  const grouped = groupEntries(entries)
  const present = [...grouped.keys()].filter((key): key is string => key !== null)
  present.sort(compareCategories)

  const ordered: string[] = []
  for (const name of savedOrder) {
    if (grouped.has(name) && !ordered.includes(name)) ordered.push(name)
  }
  for (const name of present) {
    if (!ordered.includes(name)) ordered.push(name)
  }

  const sections: Array<{ category: string | null; entries: InterviewEntry[] }> = ordered.map(
    (category) => ({ category, entries: grouped.get(category) ?? [] }),
  )
  const uncategorized = grouped.get(null)
  if (uncategorized) sections.push({ category: null, entries: uncategorized })
  return sections
}

function deriveCategories(entries: InterviewEntry[]): string[] {
  const names = new Set<string>()
  for (const entry of entries) {
    if (entry.category !== null) names.add(entry.category)
  }
  return [...names].sort(compareCategories)
}

function sectionName(category: string | null): string {
  return category ?? UNCATEGORIZED
}

export function InterviewPrepPage({ onNotice }: { onNotice: Notice }) {
  const [entries, setEntries] = useState<InterviewEntry[] | null>(null)
  const [error, setError] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)
  const [savedOrder, setSavedOrder] = useState<string[]>(readSavedOrder)
  const [drag, setDrag] = useState<DragState | null>(null)
  const [dropTarget, setDropTarget] = useState<string | null | undefined>(undefined)

  const load = useCallback(async () => {
    try {
      const r = await api.listInterviewEntries()
      setEntries(r.interview_entries)
      setError('')
    } catch (e) {
      setError(errorMessage(e))
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const sections = useMemo(
    () => (entries ? buildSections(entries, savedOrder) : []),
    [entries, savedOrder],
  )
  const categories = useMemo(() => (entries ? deriveCategories(entries) : []), [entries])

  function persistCategoryOrder(order: string[]) {
    setSavedOrder(order)
    try {
      localStorage.setItem(CATEGORY_ORDER_KEY, JSON.stringify(order))
    } catch {
      // A blocked localStorage must not break the page; the order then
      // simply resets on reload.
    }
  }

  function resetCategoryOrder() {
    setSavedOrder([])
    try {
      localStorage.removeItem(CATEGORY_ORDER_KEY)
    } catch {
      // ignore
    }
  }

  function saved(entry: InterviewEntry, created: boolean) {
    setEntries((prev) => {
      if (!prev) return [entry]
      const exists = prev.some((e) => e.id === entry.id)
      return exists ? prev.map((e) => (e.id === entry.id ? entry : e)) : [...prev, entry]
    })
    setEditor(null)
    onNotice('success', created ? 'Interview entry added.' : 'Interview entry saved.')
    void load()
  }

  function remove(entry: InterviewEntry): ConfirmRequest {
    return {
      title: 'Delete interview entry',
      message: `Delete “${entry.question}”? This cannot be undone.`,
      onConfirm: async () => {
        await api.deleteInterviewEntry(entry.id)
        setEntries((prev) => prev?.filter((e) => e.id !== entry.id) ?? prev)
        if (openId === entry.id) setOpenId(null)
        onNotice('success', 'Interview entry deleted.')
      },
    }
  }

  async function persistPatches(patches: Array<{ id: number; patch: EntryPatch }>) {
    try {
      for (const { id, patch } of patches) {
        await api.updateInterviewEntry(id, patch)
      }
      await load()
    } catch (e) {
      onNotice('error', errorMessage(e))
      await load()
    }
  }

  function moveEntry(dragState: Extract<DragState, { kind: 'entry' }>, targetCategory: string | null, beforeId?: number) {
    if (!entries) return
    const dragged = entries.find((e) => e.id === dragState.id)
    if (!dragged) return
    const sameSection = dragged.category === targetCategory
    if (sameSection && beforeId === undefined) return
    if (beforeId === dragged.id) return

    const grouped = groupEntries(entries)
    const source = grouped.get(dragged.category) ?? []
    const sourceWithout = source.filter((e) => e.id !== dragged.id)
    const target = sameSection ? sourceWithout : [...(grouped.get(targetCategory) ?? [])]

    const moved = { ...dragged, category: targetCategory }
    let index = target.length
    if (beforeId !== undefined) {
      const i = target.findIndex((e) => e.id === beforeId)
      if (i >= 0) index = i
    }
    target.splice(index, 0, moved)

    const displayOrder = new Map<number, number>()
    target.forEach((entry, i) => displayOrder.set(entry.id, i + 1))
    if (!sameSection) sourceWithout.forEach((entry, i) => displayOrder.set(entry.id, i + 1))

    const patches: Array<{ id: number; patch: EntryPatch }> = []
    target.forEach((entry, i) => {
      const patch: EntryPatch = { display_order: i + 1 }
      if (!sameSection && entry.id === dragged.id) patch.category = targetCategory
      patches.push({ id: entry.id, patch })
    })
    if (!sameSection) {
      sourceWithout.forEach((entry, i) => {
        patches.push({ id: entry.id, patch: { display_order: i + 1 } })
      })
    }

    setEntries(
      entries.map((entry) => {
        const order = displayOrder.get(entry.id)
        if (order === undefined) return entry
        return {
          ...entry,
          display_order: order,
          ...(entry.id === dragged.id ? { category: targetCategory } : {}),
        }
      }),
    )
    void persistPatches(patches)
  }

  function moveSection(dragCategory: string, targetCategory: string) {
    if (dragCategory === targetCategory) return
    const names = sections
      .map((section) => section.category)
      .filter((category): category is string => category !== null)
    const next = names.filter((name) => name !== dragCategory)
    const index = next.indexOf(targetCategory)
    if (index < 0) return
    next.splice(index, 0, dragCategory)
    persistCategoryOrder(next)
  }

  function startEntryDrag(e: DragEvent<HTMLElement>, entry: InterviewEntry) {
    setDrag({ kind: 'entry', id: entry.id })
    e.dataTransfer?.setData('text/plain', String(entry.id))
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }

  function startSectionDrag(e: DragEvent<HTMLElement>, category: string | null) {
    if (category === null) return
    e.stopPropagation()
    setDrag({ kind: 'section', category })
    e.dataTransfer?.setData('text/plain', category)
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }

  function dragOverSection(e: DragEvent<HTMLElement>, category: string | null) {
    if (!drag) return
    if (drag.kind === 'section' && category === null) return
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
    setDropTarget(category)
  }

  function dropOnSection(e: DragEvent<HTMLElement>, category: string | null) {
    e.preventDefault()
    const current = drag
    setDrag(null)
    setDropTarget(undefined)
    if (!current) return
    if (current.kind === 'section') {
      if (category !== null) moveSection(current.category, category)
      return
    }
    moveEntry(current, category)
  }

  function dropOnEntry(e: DragEvent<HTMLElement>, entry: InterviewEntry, category: string | null) {
    e.preventDefault()
    e.stopPropagation()
    const current = drag
    setDrag(null)
    setDropTarget(undefined)
    if (current?.kind === 'entry') moveEntry(current, category, entry.id)
  }

  function endDrag() {
    setDrag(null)
    setDropTarget(undefined)
  }

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Interview prep</h2>
          <div className="actions">
            <span className="muted">
              {entries ? `${entries.length} question${entries.length === 1 ? '' : 's'}` : ''}
            </span>
            <button
              type="button"
              className="link-button"
              onClick={resetCategoryOrder}
              title="Restore alphabetical section order"
            >
              Reset to alphabetical
            </button>
            <button type="button" className="primary" onClick={() => setEditor({ mode: 'create' })}>
              Add
            </button>
          </div>
        </div>

        {error && <div className="error-box">{error}</div>}

        {entries === null ? (
          <p className="muted" aria-busy="true">
            Loading interview prep…
          </p>
        ) : entries.length === 0 ? (
          <div className="empty-state">
            <p>No interview questions yet.</p>
            <p className="muted">
              Use Add to write an anticipated question and prepare your answer in markdown.
            </p>
          </div>
        ) : (
          <div className="interview-sections">
            {sections.map((section) => {
              const name = sectionName(section.category)
              const dropEnabled =
                drag !== null && (drag.kind === 'entry' || section.category !== null)
              const isTarget = drag !== null && dropTarget === section.category
              return (
                <section
                  className={`interview-section${isTarget ? ' is-drop-target' : ''}`}
                  key={section.category ?? '__uncategorized__'}
                  aria-label={name}
                  onDragOver={(e) => dragOverSection(e, section.category)}
                  onDrop={(e) => dropOnSection(e, section.category)}
                >
                  <div
                    className={`interview-section-head${dropEnabled ? ' can-drop' : ''}`}
                    draggable={section.category !== null}
                    onDragStart={(e) => startSectionDrag(e, section.category)}
                    onDragEnd={endDrag}
                  >
                    {section.category !== null && (
                      <span className="interview-grip" aria-hidden="true">
                        ⠿
                      </span>
                    )}
                    <h3>{name}</h3>
                    <span className="interview-section-count">{section.entries.length}</span>
                  </div>

                  <div className="interview-list">
                    {section.entries.map((entry) => {
                      const open = openId === entry.id
                      const missingAnswer = entry.answer.trim() === ''
                      return (
                        <article
                          className={`interview-entry${isTarget ? ' is-drop-target' : ''}`}
                          key={entry.id}
                          data-entry-id={entry.id}
                          draggable
                          onDragStart={(e) => startEntryDrag(e, entry)}
                          onDragEnd={endDrag}
                          onDragOver={(e) => {
                            if (!drag) return
                            e.preventDefault()
                            e.stopPropagation()
                            setDropTarget(section.category)
                          }}
                          onDrop={(e) => dropOnEntry(e, entry, section.category)}
                        >
                          <div className="interview-head">
                            <span className="interview-grip" aria-hidden="true">
                              ⠿
                            </span>
                            <button
                              type="button"
                              className="interview-toggle"
                              aria-expanded={open}
                              aria-controls={`interview-answer-${entry.id}`}
                              onClick={() => setOpenId(open ? null : entry.id)}
                            >
                              <span className="interview-chevron" aria-hidden="true">
                                {open ? '▾' : '▸'}
                              </span>
                              <span>{entry.question}</span>
                            </button>
                            {missingAnswer && (
                              <span
                                className="interview-cue"
                                title="No answer yet"
                                aria-label="No answer yet"
                              >
                                No answer
                              </span>
                            )}
                            <div className="interview-actions">
                              <button
                                type="button"
                                className="icon-button"
                                aria-label={`Edit ${entry.question}`}
                                onClick={() => setEditor({ mode: 'edit', entry })}
                              >
                                <ActionIcon name="edit" />
                              </button>
                              <button
                                type="button"
                                className="icon-button danger"
                                aria-label={`Delete ${entry.question}`}
                                onClick={() => setConfirm(remove(entry))}
                              >
                                <ActionIcon name="trash" />
                              </button>
                            </div>
                          </div>
                          {open && (
                            <div className="interview-answer" id={`interview-answer-${entry.id}`}>
                              {entry.answer.trim() ? (
                                <Markdown source={entry.answer} />
                              ) : (
                                <p className="muted">No answer yet. Use Edit to write one.</p>
                              )}
                            </div>
                          )}
                        </article>
                      )
                    })}
                  </div>
                </section>
              )
            })}
          </div>
        )}
      </section>

      {editor && (
        <EntryEditorModal
          initial={editor.mode === 'edit' ? editor.entry : null}
          categories={categories}
          onClose={() => setEditor(null)}
          onSaved={saved}
          onNotice={onNotice}
        />
      )}
      {confirm && <ConfirmDialog request={confirm} onClose={() => setConfirm(null)} />}
    </div>
  )
}

function EntryEditorModal({
  initial,
  categories,
  onClose,
  onSaved,
  onNotice,
}: {
  initial: InterviewEntry | null
  categories: string[]
  onClose: () => void
  onSaved: (entry: InterviewEntry, created: boolean) => void
  onNotice: Notice
}) {
  const [question, setQuestion] = useState(initial?.question ?? '')
  const [answer, setAnswer] = useState(initial?.answer ?? '')
  const [category, setCategory] = useState(initial?.category ?? '')
  const [newCategoryMode, setNewCategoryMode] = useState(false)
  const [newCategory, setNewCategory] = useState('')
  const [answerMode, setAnswerMode] = useState<'edit' | 'view'>('edit')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (question.trim() === '') {
      setError('Question is required.')
      return
    }
    let finalCategory: string | null = null
    if (newCategoryMode) {
      const trimmed = newCategory.trim()
      if (trimmed === '') {
        setError('Category name is required.')
        return
      }
      finalCategory = trimmed
    } else if (category !== '') {
      finalCategory = category
    }

    setBusy(true)
    try {
      if (initial) {
        const r = await api.updateInterviewEntry(initial.id, {
          question: question.trim(),
          answer,
          category: finalCategory,
        })
        onSaved(r.interview_entry, false)
      } else {
        const r = await api.createInterviewEntry({
          question: question.trim(),
          answer,
          category: finalCategory,
        })
        onSaved(r.interview_entry, true)
      }
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
      setBusy(false)
    }
  }

  return (
    <Modal title={initial ? 'Edit interview question' : 'New interview question'} onClose={onClose} wide>
      <form className="form-grid interview-editor" onSubmit={(e) => void submit(e)}>
        <label className="span-full">
          Question
          <input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="e.g. How do you handle an incident?"
            autoFocus
          />
        </label>
        <label className="span-full">
          Category
          <select
            value={newCategoryMode ? NEW_CATEGORY : category}
            onChange={(e) => {
              const value = e.target.value
              if (value === NEW_CATEGORY) {
                setNewCategoryMode(true)
              } else {
                setNewCategoryMode(false)
                setCategory(value)
              }
            }}
          >
            <option value="">Uncategorized</option>
            {categories.map((name) => (
              <option key={name} value={name}>
                {name}
              </option>
            ))}
            <option value={NEW_CATEGORY}>Add new category…</option>
          </select>
        </label>
        {newCategoryMode && (
          <label className="span-full">
            New category
            <input
              value={newCategory}
              onChange={(e) => setNewCategory(e.target.value)}
              placeholder="e.g. Behavioural"
            />
          </label>
        )}

        <div className="span-full answer-pane">
          <div className="answer-tabs" role="tablist" aria-label="Answer mode">
            <button
              type="button"
              role="tab"
              aria-selected={answerMode === 'edit'}
              className={answerMode === 'edit' ? 'active' : ''}
              onClick={() => setAnswerMode('edit')}
            >
              Edit
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={answerMode === 'view'}
              className={answerMode === 'view' ? 'active' : ''}
              onClick={() => setAnswerMode('view')}
            >
              View
            </button>
          </div>
          {answerMode === 'edit' ? (
            <label className="answer-field">
              Answer (markdown)
              <textarea
                value={answer}
                onChange={(e) => setAnswer(e.target.value)}
                rows={16}
                placeholder="Write your prepared answer…"
              />
            </label>
          ) : (
            <div className="answer-view" aria-live="polite">
              {answer.trim() ? (
                <Markdown source={answer} />
              ) : (
                <p className="muted">Nothing to view yet.</p>
              )}
            </div>
          )}
        </div>

        {error && <div className="error-box span-full">{error}</div>}
        <div className="form-actions">
          <button type="submit" className="primary" disabled={busy}>
            {busy ? 'Saving…' : initial ? 'Save changes' : 'Add entry'}
          </button>
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
        </div>
      </form>
    </Modal>
  )
}
