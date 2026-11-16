import { useState } from 'react'
import { errorMessage } from '../api'

export interface ConfirmRequest {
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
  onConfirm: () => Promise<void>
}

export function ConfirmDialog({
  request,
  onClose,
}: {
  request: ConfirmRequest
  onClose: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function confirm() {
    setBusy(true)
    setError('')
    try {
      await request.onConfirm()
      onClose()
    } catch (e) {
      setError(errorMessage(e))
      setBusy(false)
    }
  }

  return (
    <div className="modal-backdrop" role="presentation">
      <div className="modal" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
        <div className="modal-head">
          <h2 id="confirm-title">{request.title}</h2>
          <button type="button" className="link-button" onClick={onClose} disabled={busy}>
            Close
          </button>
        </div>
        <p>{request.message}</p>
        {error && <div className="error-box">{error}</div>}
        <div className="modal-actions">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            type="button"
            className={request.danger === false ? 'primary' : 'primary danger-button'}
            disabled={busy}
            onClick={() => void confirm()}
          >
            {busy ? 'Working…' : (request.confirmLabel ?? 'Delete')}
          </button>
        </div>
      </div>
    </div>
  )
}
