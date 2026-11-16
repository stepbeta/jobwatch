import { appLabel, fmtDateTime, relativeTime } from '../format'
import type { Offer } from '../types'
import { Link } from './Link'
import { ScoreBadge } from './ScoreBadge'
import { StatePill } from './StatePill'

export function OfferTable({
  offers,
  showCompany = true,
  showApplication = true,
  emptyText = 'No offers match.',
}: {
  offers: Offer[]
  showCompany?: boolean
  showApplication?: boolean
  emptyText?: string
}) {
  if (offers.length === 0) {
    return <p className="empty">{emptyText}</p>
  }
  return (
    <div className="table-wrap">
      <table className="grid">
        <thead>
          <tr>
            {showCompany && <th>Company</th>}
            <th>Title</th>
            <th>Location</th>
            <th>Remote</th>
            <th>Offer</th>
            {showApplication && <th>Application</th>}
            <th>Scores</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {offers.map((o) => (
            <tr key={o.id}>
              {showCompany && (
                <td>
                  <Link to={`/companies/${o.company_id}`}>{o.company_name}</Link>
                </td>
              )}
              <td>
                <Link to={`/offers/${o.id}`}>{o.title}</Link>
              </td>
              <td>{o.location ?? '—'}</td>
              <td>{o.remote_policy}</td>
              <td>
                <StatePill kind={`offer-${o.offer_state}`}>{o.offer_state}</StatePill>
              </td>
              {showApplication && (
                <td>
                  <StatePill kind={`app-${o.application_state}`}>{appLabel(o.application_state)}</StatePill>
                </td>
              )}
              <td className="scores">
                <ScoreBadge label="T" value={o.title_confidence} reasons={o.match_reasons} kind="title" />
                <ScoreBadge
                  label="L"
                  value={o.location_confidence}
                  reasons={o.match_reasons}
                  kind="location"
                />
              </td>
              <td title={fmtDateTime(o.last_seen_at)}>{relativeTime(o.last_seen_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
