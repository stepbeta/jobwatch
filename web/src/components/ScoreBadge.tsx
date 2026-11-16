import { reasonsText, scoreClass } from '../format'
import type { MatchReasons } from '../types'

export function ScoreBadge({
  label,
  value,
  reasons,
  kind,
}: {
  label: string
  value: number | null
  reasons: MatchReasons | null
  kind: 'title' | 'location'
}) {
  const explanation = reasonsText(reasons, kind)
  const title =
    value === null
      ? 'Not classified yet — run `jobwatch reclassify` or scan the source'
      : explanation || `Score ${value}/100`
  return (
    <span className={scoreClass(value)} title={title}>
      {label} {value ?? '—'}
    </span>
  )
}
