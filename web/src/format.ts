import type {
  ApplicationState,
  EventType,
  MatchReasons,
  OfferState,
  RemotePolicy,
  ScanStatus,
  SourceType,
  TermBucket,
} from './types'

export const APP_STATES: ApplicationState[] = [
  'none',
  'saved',
  'applied',
  'interview',
  'offer',
  'accepted',
  'turned_down',
  'negative',
]

export const OFFER_STATES: OfferState[] = ['active', 'disappeared']

export const REMOTE_POLICIES: RemotePolicy[] = ['remote', 'hybrid', 'onsite', 'unknown']

const REMOTE_LABELS: Record<RemotePolicy, string> = {
  remote: 'Full remote',
  hybrid: 'Hybrid',
  onsite: 'Onsite',
  unknown: 'Unknown',
}

export function remoteLabel(policy: RemotePolicy): string {
  return REMOTE_LABELS[policy] ?? policy
}

export const BUCKET_LABELS: Record<TermBucket, string> = {
  strong_positive: 'Strong positive',
  mild_positive: 'Mild positive',
  neutral: 'Neutral',
  mild_negative: 'Mild negative',
  strong_negative: 'Strong negative',
  positive: 'Positive',
  negative: 'Negative',
}

export function bucketLabel(bucket: TermBucket): string {
  return BUCKET_LABELS[bucket] ?? bucket
}

export const EVENT_TYPES: EventType[] = [
  'cv_sent',
  'message_received',
  'message_sent',
  'interview',
  'offer',
  'rejected',
  'ghosted',
  'other',
]

export const SOURCE_TYPES: SourceType[] = [
  'greenhouse',
  'lever',
  'ashby',
  'personio',
  'workable',
  'bamboohr',
  'wttj',
  'rss',
  'html',
  'manual',
]

const EVENT_LABELS: Record<EventType, string> = {
  cv_sent: 'CV sent',
  message_received: 'Message received',
  message_sent: 'Message sent',
  interview: 'Interview',
  offer: 'Offer',
  rejected: 'Rejected',
  ghosted: 'Ghosted',
  other: 'Other',
}

export function eventLabel(t: EventType): string {
  return EVENT_LABELS[t] ?? t
}

const APP_LABELS: Record<ApplicationState, string> = {
  none: 'None',
  saved: 'Saved',
  applied: 'Applied',
  interview: 'Interview',
  offer: 'Offer',
  accepted: 'Accepted',
  turned_down: 'Turned down',
  negative: 'Negative',
}

export function appLabel(s: ApplicationState): string {
  return APP_LABELS[s] ?? s
}

export function scanLabel(s: ScanStatus | null): string {
  if (s === 'ok') return 'OK'
  if (s === 'error') return 'Error'
  if (s === 'blocked') return 'Blocked'
  return 'Never'
}

export function scoreClass(value: number): string {
  if (value >= 80) return 'score score-good'
  if (value >= 50) return 'score score-mid'
  return 'score score-bad'
}

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

export function fmtDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleDateString(undefined, { dateStyle: 'medium' })
}

export function relativeTime(iso: string | null | undefined): string {
  if (!iso) return 'never'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return 'never'
  const seconds = Math.round((d.getTime() - Date.now()) / 1000)
  const abs = Math.abs(seconds)
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  if (abs < 60) return rtf.format(seconds, 'second')
  if (abs < 3600) return rtf.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(seconds / 3600), 'hour')
  if (abs < 86400 * 30) return rtf.format(Math.round(seconds / 86400), 'day')
  if (abs < 86400 * 365) return rtf.format(Math.round(seconds / (86400 * 30)), 'month')
  return rtf.format(Math.round(seconds / (86400 * 365)), 'year')
}

// toLocalInput renders a Date for an <input type="datetime-local">.
export function toLocalInput(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// fromLocalInput converts a datetime-local value to RFC3339.
export function fromLocalInput(value: string): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}

// splitLocalInput splits a datetime-local value into the date and time
// parts used by the separate date + optional time event inputs.
export function splitLocalInput(value: string): { date: string; time: string } {
  if (!value) return { date: '', time: '' }
  const [date, time = ''] = value.split('T')
  return { date, time: time.slice(0, 5) }
}

// eventTimestamp builds the API value for an event's occurred_at from a
// required date and an optional time. A date without a time is sent as
// YYYY-MM-DD, which the API accepts and interprets as UTC midnight.
export function eventTimestamp(date: string, time: string): string {
  if (!date) return ''
  if (!time) return date
  const d = new Date(`${date}T${time}`)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}

// reasonsText renders the profile terms that produced one affinity as a
// badge tooltip. Neutral terms appear with weight 0.
export function reasonsText(reasons: MatchReasons | null | undefined, kind: 'title' | 'location'): string {
  const list = reasons?.[kind] ?? []
  return list
    .map((r) => `${bucketLabel(r.bucket)} "${r.term}" (${r.weight >= 0 ? '+' : ''}${r.weight})`)
    .join('\n')
}
