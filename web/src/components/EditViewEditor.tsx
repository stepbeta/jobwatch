import { useState, type ReactNode } from 'react'

// EditViewEditor is the shared Edit/View toggle used by the interview-prep
// answer editor and the offer description editor. The textarea and the
// rendered view share one container and the same styling classes.
export function EditViewEditor({
  value,
  onChange,
  editLabel,
  placeholder,
  rows = 16,
  viewEmptyText = 'Nothing to view yet.',
  initialMode = 'edit',
  renderView,
}: {
  value: string
  onChange: (value: string) => void
  editLabel: string
  placeholder?: string
  rows?: number
  viewEmptyText?: string
  initialMode?: 'edit' | 'view'
  renderView: (value: string) => ReactNode
}) {
  const [mode, setMode] = useState<'edit' | 'view'>(initialMode)
  return (
    <div className="answer-pane">
      <div className="answer-tabs" role="tablist" aria-label={`${editLabel} mode`}>
        <button
          type="button"
          role="tab"
          aria-selected={mode === 'edit'}
          className={mode === 'edit' ? 'active' : ''}
          onClick={() => setMode('edit')}
        >
          Edit
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mode === 'view'}
          className={mode === 'view' ? 'active' : ''}
          onClick={() => setMode('view')}
        >
          View
        </button>
      </div>
      {mode === 'edit' ? (
        <label className="answer-field">
          {editLabel}
          <textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            rows={rows}
            placeholder={placeholder}
          />
        </label>
      ) : (
        <div className="answer-view" aria-live="polite">
          {value.trim() ? renderView(value) : <p className="muted">{viewEmptyText}</p>}
        </div>
      )}
    </div>
  )
}
