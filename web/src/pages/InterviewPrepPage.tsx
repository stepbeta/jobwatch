import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { ActionIcon } from '../components/icons'
import { Markdown } from '../components/Markdown'
import { Modal } from '../components/Modal'
import type { InterviewEntry } from '../types'

type Notice = (kind: 'info' | 'success' | 'error', text: string) => void

type EditorState = { mode: 'create' } | { mode: 'edit'; entry: InterviewEntry }

export function InterviewPrepPage({ onNotice }: { onNotice: Notice }) {
  const [entries, setEntries] = useState<InterviewEntry[] | null>(null)
  const [error, setError] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)

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

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Interview prep</h2>
          <div className="actions">
            <span className="muted">
              {entries ? `${entries.length} question${entries.length === 1 ? '' : 's'}` : ''}
            </span>
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
          <div className="interview-list">
            {entries.map((entry) => {
              const open = openId === entry.id
              return (
                <article className="interview-entry" key={entry.id}>
                  <div className="interview-head">
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
        )}
      </section>

      {editor && (
        <EntryEditorModal
          initial={editor.mode === 'edit' ? editor.entry : null}
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
  onClose,
  onSaved,
  onNotice,
}: {
  initial: InterviewEntry | null
  onClose: () => void
  onSaved: (entry: InterviewEntry, created: boolean) => void
  onNotice: Notice
}) {
  const [question, setQuestion] = useState(initial?.question ?? '')
  const [answer, setAnswer] = useState(initial?.answer ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (question.trim() === '') {
      setError('Question is required.')
      return
    }
    setBusy(true)
    try {
      if (initial) {
        const r = await api.updateInterviewEntry(initial.id, {
          question: question.trim(),
          answer,
        })
        onSaved(r.interview_entry, false)
      } else {
        const r = await api.createInterviewEntry({ question: question.trim(), answer })
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
      <form className="form-grid" onSubmit={(e) => void submit(e)}>
        <label className="span-2">
          Question
          <input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="e.g. How do you handle an incident?"
            autoFocus
          />
        </label>
        <div className="span-2 answer-editor">
          <label>
            Answer (markdown)
            <textarea
              value={answer}
              onChange={(e) => setAnswer(e.target.value)}
              rows={10}
              placeholder="Write your prepared answer…"
            />
          </label>
          <div className="answer-preview">
            <span className="preview-label">Preview</span>
            <div className="answer-preview-body" aria-live="polite">
              {answer.trim() ? (
                <Markdown source={answer} />
              ) : (
                <p className="muted">Nothing to preview yet.</p>
              )}
            </div>
          </div>
        </div>
        {error && <div className="error-box span-2">{error}</div>}
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
