import { reasonsText, remoteLabel, scoreClass } from '../format'
import type { MatchReasons, RemotePolicy } from '../types'

export function ScoreBadge({
  label,
  value,
  reasons,
  kind,
}: {
  label: string
  value: number
  reasons: MatchReasons
  kind: 'title' | 'location'
}) {
  const explanation = reasonsText(reasons, kind)
  const title = explanation ? `${explanation}\n\nAffinity ${value}/100` : `Affinity ${value}/100`
  return (
    <span className={scoreClass(value)} title={title}>
      {label} {value}
    </span>
  )
}

// RemoteBadge shows the offer's derived policy and whether the current
// profile accepts it. Remote is a fact plus a checkbox, not a score.
export function RemoteBadge({
  policy,
  accepted,
}: {
  policy: RemotePolicy
  accepted: boolean
}) {
  const title = accepted
    ? `${remoteLabel(policy)} — accepted by the current search profile`
    : `${remoteLabel(policy)} — not accepted by the current search profile`
  return (
    <span className={accepted ? 'remote-badge remote-ok' : 'remote-badge remote-no'} title={title}>
      {remoteLabel(policy)}
      <span aria-hidden="true"> {accepted ? '✓' : '✗'}</span>
      <span className="sr-only">{accepted ? ' accepted' : ' not accepted'}</span>
    </span>
  )
}
