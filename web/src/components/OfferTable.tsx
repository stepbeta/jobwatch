import { appLabel, fmtDateTime, relativeTime } from '../format'
import type { Offer } from '../types'
import { BookmarkButton } from './BookmarkButton'
import { Link } from './Link'
import { ScoreBadge } from './ScoreBadge'
import { StatePill } from './StatePill'
import { SortHeader, type SortOrder, type TableColumn } from './TableControls'

export const OFFER_COLUMNS: TableColumn[] = [
  { id: 'bookmark', label: 'Bookmark' },
  { id: 'company', label: 'Company' },
  { id: 'title', label: 'Title' },
  { id: 'location', label: 'Location' },
  { id: 'remote', label: 'Remote' },
  { id: 'offer', label: 'Offer' },
  { id: 'application', label: 'Application' },
  { id: 'title_score', label: 'Title score' },
  { id: 'location_score', label: 'Location score' },
  { id: 'last_seen', label: 'Last seen' },
]

// Server-side sort key for each clickable header. Columns without a key
// (states, remote policy, bookmark) are display-only.
const SORT_KEYS: Record<string, string> = {
  company: 'company',
  title: 'title',
  location: 'location',
  title_score: 'title_confidence',
  location_score: 'location_confidence',
  last_seen: 'last_seen',
}

export const OFFER_SORT_VALUES = [
  'last_seen',
  'first_seen',
  'recent_event',
  'title',
  'company',
  'location',
  'title_confidence',
  'location_confidence',
  'saved',
] as const

export const OFFER_SORT_LABELS: Record<string, string> = {
  last_seen: 'Last seen',
  first_seen: 'First seen',
  recent_event: 'Recent activity',
  title: 'Title',
  company: 'Company',
  location: 'Location',
  title_confidence: 'Title confidence',
  location_confidence: 'Location confidence',
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
  onToggleSaved,
  busyOfferId,
  sort,
  order,
  onSort,
}: {
  offers: Offer[]
  columns?: string[]
  showCompany?: boolean
  showApplication?: boolean
  emptyText?: string
  onToggleSaved?: (offer: Offer) => void
  busyOfferId?: number | null
  sort?: string
  order?: SortOrder
  onSort?: (sortKey: string) => void
}) {
  if (offers.length === 0) {
    return <p className="empty">{emptyText}</p>
  }

  const applicable = OFFER_COLUMNS.filter((c) => {
    if (c.id === 'bookmark') return Boolean(onToggleSaved)
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
            {visible.includes('bookmark') && <th aria-label="Bookmark" />}
            {visible.includes('company') && header('company', 'Company')}
            {visible.includes('title') && header('title', 'Title')}
            {visible.includes('location') && header('location', 'Location')}
            {visible.includes('remote') && header('remote', 'Remote')}
            {visible.includes('offer') && header('offer', 'Offer')}
            {visible.includes('application') && header('application', 'Application')}
            {visible.includes('title_score') && header('title_score', 'Title score')}
            {visible.includes('location_score') && header('location_score', 'Location score')}
            {visible.includes('last_seen') && header('last_seen', 'Last seen')}
          </tr>
        </thead>
        <tbody>
          {offers.map((o) => (
            <tr key={o.id} className={o.saved_at ? 'row-saved' : undefined}>
              {visible.includes('bookmark') && (
                <td>
                  <BookmarkButton
                    saved={Boolean(o.saved_at)}
                    disabled={busyOfferId === o.id}
                    onClick={() => onToggleSaved?.(o)}
                  />
                </td>
              )}
              {visible.includes('company') && (
                <td>
                  <Link to={`/companies/${o.company_id}`}>{o.company_name}</Link>
                </td>
              )}
              {visible.includes('title') && (
                <td>
                  <Link to={`/offers/${o.id}`}>{o.title}</Link>
                </td>
              )}
              {visible.includes('location') && <td>{o.location ?? '—'}</td>}
              {visible.includes('remote') && <td>{o.remote_policy}</td>}
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
              {visible.includes('title_score') && (
                <td className="scores">
                  <ScoreBadge
                    label="T"
                    value={o.title_confidence}
                    reasons={o.match_reasons}
                    kind="title"
                  />
                </td>
              )}
              {visible.includes('location_score') && (
                <td className="scores">
                  <ScoreBadge
                    label="L"
                    value={o.location_confidence}
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
