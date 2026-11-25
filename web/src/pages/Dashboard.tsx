import { useEffect, useState } from 'react'
import { api, errorMessage } from '../api'
import type { BatchScanOutcome } from '../App'
import { Link } from '../components/Link'
import { OfferTable } from '../components/OfferTable'
import { StatePill } from '../components/StatePill'
import { appLabel, fmtDateTime, relativeTime } from '../format'
import type { Offer, Source } from '../types'

const applicationPageSize = 200

export function Dashboard({
  refreshToken,
  onNotice,
  sources,
  onScanBatch,
  scanning,
}: {
  refreshToken: number
  onNotice: (kind: 'info' | 'success' | 'error', text: string) => void
  sources: Source[]
  onScanBatch: (targets: Source[]) => Promise<BatchScanOutcome[]>
  scanning: boolean
}) {
  const [applications, setApplications] = useState<Offer[]>([])
  const [saved, setSaved] = useState<Offer[]>([])
  const [savedTotal, setSavedTotal] = useState(0)
  const [error, setError] = useState('')
  const [refreshing, setRefreshing] = useState(false)

  useEffect(() => {
    let cancelled = false
    api
      .listOffers({ has_application: true, sort: 'recent_event', limit: 20 })
      .then((r) => {
        if (!cancelled) setApplications(r.offers)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken])

  useEffect(() => {
    let cancelled = false
    api
      .listOffers({ application_state: 'saved', sort: 'saved', limit: 200 })
      .then((r) => {
        if (!cancelled) {
          setSaved(r.offers)
          setSavedTotal(r.total)
        }
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken])

  // refreshApplications fans out over the existing per-source scan endpoint
  // for every enabled source that owns at least one non-none, non-saved
  // application.
  async function refreshApplications() {
    setRefreshing(true)
    try {
      const all = await fetchAllApplications()
      const sourceIDs = new Set<number>()
      let manualLinks = 0
      await Promise.all(
        all.map(async (offer) => {
          const detail = await api.getOffer(offer.id)
          for (const p of detail.provenance) {
            if (p.source_id === null) manualLinks++
            else sourceIDs.add(p.source_id)
          }
        }),
      )
      const targets = sources.filter((s) => s.enabled && sourceIDs.has(s.id))
      if (targets.length === 0) {
        onNotice(
          'info',
          all.length === 0
            ? 'No applications to refresh yet.'
            : 'No enabled source is behind these applications; manual imports have no fetchable source.',
        )
        return
      }
      const outcomes = await onScanBatch(targets)
      reportBatch(targets, outcomes, manualLinks, onNotice)
    } catch (e) {
      onNotice('error', errorMessage(e))
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <div>
            <h2>Applications</h2>
            <span className="muted">
              Offers with an application in progress (not None or Saved), most recent activity first
            </span>
          </div>
          <button
            type="button"
            className="primary"
            disabled={scanning || refreshing}
            title="Scans every enabled source behind your applications (application state not None or Saved). Sources with only None or Saved offers are not scanned."
            onClick={() => void refreshApplications()}
          >
            {refreshing || scanning ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
        {error && <div className="error-box">{error}</div>}
        {applications.length === 0 ? (
          <p className="empty">No applications yet. Set an offer's application state to track it here.</p>
        ) : (
          <div className="table-wrap">
            <table className="grid">
              <thead>
                <tr>
                  <th>Company</th>
                  <th>Title</th>
                  <th>Application</th>
                  <th>Last activity</th>
                  <th>Offer</th>
                </tr>
              </thead>
              <tbody>
                {applications.map((o) => (
                  <tr key={o.id}>
                    <td>
                      <Link to={`/companies/${o.company_id}`}>{o.company_name}</Link>
                    </td>
                    <td>
                      <Link to={`/offers/${o.id}`}>{o.title}</Link>
                    </td>
                    <td>
                      <StatePill kind={`app-${o.application_state}`}>{appLabel(o.application_state)}</StatePill>
                    </td>
                    <td title={fmtDateTime(o.last_event_at ?? o.updated_at)}>
                      {relativeTime(o.last_event_at ?? o.updated_at)}
                    </td>
                    <td>
                      <StatePill kind={`offer-${o.offer_state}`}>{o.offer_state}</StatePill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="panel">
        <div className="section-head">
          <div>
            <h2>Saved</h2>
            <span className="muted">
              Offers shortlisted before applying — the Saved application state
            </span>
          </div>
          <a href="#/offers?application_state=saved&sort=saved">Show all ({savedTotal})</a>
        </div>
        {saved.length === 0 ? (
          <p className="empty">
            No saved offers yet. Set an offer's application state to Saved to shortlist it.
          </p>
        ) : (
          <div className="saved-list">
            <OfferTable offers={saved} />
          </div>
        )}
      </section>
    </div>
  )
}

// fetchAllApplications pages through GET /api/offers?has_application=true so
// the refresh scope is complete even beyond the first page.
async function fetchAllApplications(): Promise<Offer[]> {
  const out: Offer[] = []
  let offset = 0
  for (;;) {
    const page = await api.listOffers({
      has_application: true,
      sort: 'recent_event',
      limit: applicationPageSize,
      offset,
    })
    out.push(...page.offers)
    offset += page.offers.length
    if (page.offers.length === 0 || offset >= page.total) return out
  }
}

function reportBatch(
  targets: Source[],
  outcomes: BatchScanOutcome[],
  manualLinks: number,
  onNotice: (kind: 'info' | 'success' | 'error', text: string) => void,
) {
  let added = 0
  let incomplete = 0
  let disappeared = 0
  let ghosted = 0
  let failed = 0
  for (const outcome of outcomes) {
    const err = outcome.error ?? outcome.response?.result.error
    if (err) {
      failed++
      onNotice('error', `${outcome.source.name}: ${err}`)
      continue
    }
    added += outcome.response?.result.new ?? 0
    incomplete += outcome.response?.result.incomplete ?? 0
    disappeared += outcome.response?.result.disappeared ?? 0
    ghosted += outcome.response?.result.ghosted ?? 0
  }
  let text =
    `${targets.length} source${targets.length === 1 ? '' : 's'} scanned: ` +
    `${added} new, ${incomplete} incomplete, ${disappeared} disappeared, ${ghosted} ghosted`
  if (failed > 0) text += `; ${failed} failed`
  if (manualLinks > 0) text += `; ${manualLinks} manual link(s) skipped (no fetchable source)`
  onNotice(failed > 0 ? 'error' : 'success', `${text}.`)
}
