import { reasonsText, scoreClass } from '../format'

export function ScoreBadge({
  label,
  value,
  reasons,
}: {
  label: string
  value: number | null
  reasons?: unknown
}) {
  const title = value === null ? 'Not classified yet' : reasonsText(reasons) || `Score ${value}/100`
  return (
    <span className={scoreClass(value)} title={title}>
      {label} {value ?? '—'}
    </span>
  )
}
