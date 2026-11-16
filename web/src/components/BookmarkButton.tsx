export function BookmarkButton({
  saved,
  onClick,
  disabled = false,
  size = 'normal',
}: {
  saved: boolean
  onClick: () => void
  disabled?: boolean
  size?: 'normal' | 'large'
}) {
  const label = saved ? 'Remove bookmark' : 'Save offer'
  return (
    <button
      type="button"
      className={`bookmark-button${saved ? ' is-saved' : ''}${size === 'large' ? ' bookmark-large' : ''}`}
      aria-pressed={saved}
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
    >
      <span aria-hidden="true">{saved ? '★' : '☆'}</span>
    </button>
  )
}
