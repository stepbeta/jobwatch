import { useEffect, useId, type ReactNode } from 'react'

// Modal is the shared dialog shell: fixed backdrop, centered panel, a
// titled head with a Close action and an optional wide layout. It uses the
// same .modal-* classes as ConfirmDialog and the merge dialogs.
export function Modal({
  title,
  onClose,
  wide = false,
  children,
}: {
  title: string
  onClose: () => void
  wide?: boolean
  children: ReactNode
}) {
  const titleID = useId()

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className={`modal${wide ? ' modal-wide' : ''}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
      >
        <div className="modal-head">
          <h2 id={titleID}>{title}</h2>
          <button type="button" className="link-button" onClick={onClose}>
            Close
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}
