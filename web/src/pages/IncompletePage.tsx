import { useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { CompanyCombobox } from '../components/CompanyCombobox'
import { Link } from '../components/Link'
import { Modal } from '../components/Modal'
import { fmtDateTime, relativeTime } from '../format'
import type { NoticeKind } from '../App'
import type { Company, Offer, OfferList } from '../types'

function reasonLabel(reason: string | undefined): string {
  switch (reason) {
    case 'missing_company':
      return 'Missing company'
    case 'missing_title':
      return 'Missing title'
    case 'missing_company_title':
      return 'Missing company and title'
    default:
      return reason ?? 'Incomplete'
  }
}

function reportedCompany(offer: Offer): string {
  return offer.company_name || offer.incomplete_raw?.company || '—'
}

function reportedTitle(offer: Offer): string {
  return offer.title || offer.incomplete_raw?.title || '—'
}

// IncompletePage lists the items a source could not normalize and lets the
// user assign a company and title. The repair form reuses the manual
// import form's layout and company combobox.
export function IncompletePage({
  refreshToken,
  onDataChanged,
  onNotice,
}: {
  refreshToken: number
  onDataChanged: () => void
  onNotice: (kind: NoticeKind, text: string) => void
}) {
  const [offers, setOffers] = useState<OfferList | null>(null)
  const [companies, setCompanies] = useState<Company[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [repairing, setRepairing] = useState<Offer | null>(null)
  const [token, setToken] = useState(0)

  useEffect(() => {
    let cancelled = false
    api
      .listCompanies({ limit: 500 })
      .then((r) => {
        if (!cancelled) setCompanies(r.companies)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken, token])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api
      .listOffers({ incomplete: true, sort: 'last_seen', limit: 200 })
      .then((r) => {
        if (!cancelled) {
          setOffers(r)
          setError('')
        }
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken, token])

  const total = offers?.total ?? 0

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Incomplete offers</h2>
          <span className="muted">
            {total} item{total === 1 ? '' : 's'}
          </span>
        </div>
        <p className="hint">
          A source reported these items without enough information to create a usable offer. Choose the
          real company and title; the source link and history are kept.
        </p>

        {error && <div className="error-box">{error}</div>}
        {loading && !offers ? (
          <p className="muted" aria-busy="true">
            Loading incomplete offers…
          </p>
        ) : offers && offers.offers.length === 0 ? (
          <div className="empty-state">
            <p>No incomplete offers.</p>
            <p className="muted">Every scanned item has a company and a title.</p>
          </div>
        ) : offers ? (
          <div className="table-wrap">
            <table className="grid">
              <thead>
                <tr>
                  <th>Reported company</th>
                  <th>Reported title</th>
                  <th>Reason</th>
                  <th>Location</th>
                  <th>Source link</th>
                  <th>Last seen</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {offers.offers.map((offer) => (
                  <tr key={offer.id}>
                    <td>{reportedCompany(offer)}</td>
                    <td>
                      <Link to={`/offers/${offer.id}`}>{reportedTitle(offer)}</Link>
                    </td>
                    <td>{reasonLabel(offer.incomplete_reason)}</td>
                    <td>{offer.location ?? offer.incomplete_raw?.location ?? '—'}</td>
                    <td>
                      {offer.url ? (
                        <a href={offer.url} target="_blank" rel="noreferrer">
                          {offer.url}
                        </a>
                      ) : (
                        '—'
                      )}
                    </td>
                    <td title={fmtDateTime(offer.last_seen_at)}>{relativeTime(offer.last_seen_at)}</td>
                    <td>
                      <button type="button" onClick={() => setRepairing(offer)}>
                        Repair
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      {repairing && (
        <RepairOfferModal
          offer={repairing}
          companies={companies}
          onClose={() => setRepairing(null)}
          onRepaired={() => {
            setRepairing(null)
            setToken((t) => t + 1)
            onDataChanged()
          }}
          onNotice={onNotice}
        />
      )}
    </div>
  )
}

function RepairOfferModal({
  offer,
  companies,
  onClose,
  onRepaired,
  onNotice,
}: {
  offer: Offer
  companies: Company[]
  onClose: () => void
  onRepaired: () => void
  onNotice: (kind: NoticeKind, text: string) => void
}) {
  const [companyID, setCompanyID] = useState<number | undefined>(offer.company_id ?? undefined)
  const [title, setTitle] = useState(offer.title || offer.incomplete_raw?.title || '')
  const [location, setLocation] = useState(offer.location ?? offer.incomplete_raw?.location ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!companyID) {
      setError('Choose a company.')
      return
    }
    if (!title.trim()) {
      setError('Enter a title.')
      return
    }
    setBusy(true)
    try {
      await api.repairOffer(offer.id, {
        company_id: companyID,
        title: title.trim(),
        location: location.trim() || null,
      })
      onNotice('success', `Offer #${offer.id} repaired.`)
      onRepaired()
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal title={`Repair offer #${offer.id}`} onClose={onClose}>
      <p className="hint">
        {reasonLabel(offer.incomplete_reason)}. The source reported
        {offer.incomplete_raw?.company ? ` company “${offer.incomplete_raw.company}”` : ' no company'}
        {offer.incomplete_raw?.title ? ` and title “${offer.incomplete_raw.title}”` : ' and no title'}.
      </p>
      <form className="form-grid" onSubmit={(e) => void submit(e)}>
        <div className="span-2">
          <CompanyCombobox companies={companies} value={companyID} onChange={setCompanyID} />
        </div>
        <label>
          Title
          <input value={title} onChange={(e) => setTitle(e.target.value)} required />
        </label>
        <label>
          Location
          <input
            value={location}
            onChange={(e) => setLocation(e.target.value)}
            placeholder="optional"
          />
        </label>
        {error && <div className="error-box span-2">{error}</div>}
        <div className="form-actions">
          <button type="submit" className="primary" disabled={busy || !companyID || !title.trim()}>
            {busy ? 'Saving…' : 'Save repair'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
