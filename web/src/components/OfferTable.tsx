import { appLabel, fmtDateTime, relativeTime } from '../format'
import type { Offer } from '../types'
import { Link } from './Link'
import { RemoteBadge, ScoreBadge } from './ScoreBadge'
import { StatePill } from './StatePill'
import { SortHeader, type SortOrder, type TableColumn } from './TableControls'

export const OFFER_COLUMNS: TableColumn[] = [
  { id: 'company', label: 'Company' },
  { id: 'title', label: 'Title' },
  { id: 'location', label: 'Location' },
  { id: 'remote', label: 'Remote' },
  { id: 'offer', label: 'Offer' },
  { id: 'application', label: 'Application' },
  { id: 'title_affinity', label: 'Title affinity' },
  { id: 'location_affinity', label: 'Location affinity' },
  { id: 'last_seen', label: 'Last seen' },
]

// Server-side sort key for each clickable header. Columns without a key
// (states, remote policy) are display-only.
const SORT_KEYS: Record<string, string> = {
  company: 'company',
  title: 'title',
  location: 'location',
  title_affinity: 'title_affinity',
  location_affinity: 'location_affinity',
  last_seen: 'last_seen',
}

export const OFFER_SORT_VALUES = [
  'last_seen',
  'first_seen',
  'recent_event',
  'title',
  'company',
  'location',
  'title_affinity',
  'location_affinity',
  'saved',
] as const

export const OFFER_SORT_LABELS: Record<string, string> = {
  last_seen: 'Last seen',
  first_seen: 'First seen',
  recent_event: 'Recent activity',
  title: 'Title',
  company: 'Company',
  location: 'Location',
  title_affinity: 'Title affinity',
  location_affinity: 'Location affinity',
  saved: 'Recently saved',
}

export function defaultOfferSortOrder(sort: string): SortOrder {
  return sort === 'title' || sort === 'company' || sort === 'location' ? 'asc' : 'desc'
}

export function OfferTable({
  offers,
  columns,
  showCompany = true,
  showApplication = true,
  emptyText = 'No offers match.',
  sort,
  order,
  onSort,
}: {
  offers: Offer[]
  columns?: string[]
  showCompany?: boolean
  showApplication?: boolean
  emptyText?: string
  sort?: string
  order?: SortOrder
  onSort?: (sortKey: string) => void
}) {
  if (offers.length === 0) {
    return <p className="empty">{emptyText}</p>
  }

  const applicable = OFFER_COLUMNS.filter((c) => {
    if (c.id === 'company') return showCompany
    if (c.id === 'application') return showApplication
    return true
  })
  const fallback = applicable.map((c) => c.id)
  const selected = columns === undefined ? fallback : columns.filter((id) => fallback.includes(id))
  const visible = selected.length > 0 ? selected : fallback

  function header(id: string, label: string) {
    const key = SORT_KEYS[id]
    if (!key) return <SortHeader key={id} column={id} label={label} />
    return (
      <SortHeader
        key={id}
        column={key}
        label={label}
        sort={sort}
        order={order}
        onSort={onSort}
      />
    )
  }

  return (
    <div className="table-wrap">
      <table className="grid offers-table">
        <thead>
          <tr>
            {visible.includes('company') && header('company', 'Company')}
            {visible.includes('title') && header('title', 'Title')}
            {visible.includes('location') && header('location', 'Location')}
            {visible.includes('remote') && header('remote', 'Remote')}
            {visible.includes('offer') && header('offer', 'Offer')}
            {visible.includes('application') && header('application', 'Application')}
            {visible.includes('title_affinity') && header('title_affinity', 'Title affinity')}
            {visible.includes('location_affinity') && header('location_affinity', 'Location affinity')}
            {visible.includes('last_seen') && header('last_seen', 'Last seen')}
          </tr>
        </thead>
        <tbody>
          {offers.map((o) => (
            <tr key={o.id} className={o.application_state === 'saved' ? 'row-saved' : undefined}>
              {visible.includes('company') && (
                <td>
                  {o.company_id === null ? (
                    <span className="muted">—</span>
                  ) : (
                    <Link to={`/companies/${o.company_id}`}>{o.company_name}</Link>
                  )}
                </td>
              )}
              {visible.includes('title') && (
                <td>
                  <Link to={`/offers/${o.id}`}>{o.title}</Link>
                </td>
              )}
              {visible.includes('location') && <td>{o.location ?? '—'}</td>}
              {visible.includes('remote') && (
                <td>
                  <RemoteBadge policy={o.remote_policy} accepted={o.remote_policy_accepted} />
                </td>
              )}
              {visible.includes('offer') && (
                <td>
                  <StatePill kind={`offer-${o.offer_state}`}>{o.offer_state}</StatePill>
                </td>
              )}
              {visible.includes('application') && (
                <td>
                  <StatePill kind={`app-${o.application_state}`}>{appLabel(o.application_state)}</StatePill>
                </td>
              )}
              {visible.includes('title_affinity') && (
                <td className="scores">
                  <ScoreBadge
                    label="T"
                    value={o.title_affinity}
                    reasons={o.match_reasons}
                    kind="title"
                  />
                </td>
              )}
              {visible.includes('location_affinity') && (
                <td className="scores">
                  <ScoreBadge
                    label="L"
                    value={o.location_affinity}
                    reasons={o.match_reasons}
                    kind="location"
                  />
                </td>
              )}
              {visible.includes('last_seen') && (
                <td title={fmtDateTime(o.last_seen_at)}>{relativeTime(o.last_seen_at)}</td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
