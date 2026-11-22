import { useCallback, useEffect, useState } from 'react'
import { api, errorMessage } from './api'
import { CompanyPage } from './pages/CompanyPage'
import { Dashboard } from './pages/Dashboard'
import { IncompletePage } from './pages/IncompletePage'
import { InterviewPrepPage } from './pages/InterviewPrepPage'
import { OfferPage } from './pages/OfferPage'
import { OffersPage } from './pages/OffersPage'
import { SearchProfilePage } from './pages/SearchProfilePage'
import { SourcesPage } from './pages/SourcesPage'
import { useRoute } from './router'
import type { ScanResponse, Source } from './types'

export type NoticeKind = 'info' | 'success' | 'error'

export interface BatchScanOutcome {
  source: Source
  response?: ScanResponse
  error?: string
}

interface Toast {
  id: number
  kind: NoticeKind
  text: string
}

let toastSeq = 0
const toastTTL = 8000

export default function App() {
  const route = useRoute()
  const [sources, setSources] = useState<Source[]>([])
  const [loadError, setLoadError] = useState('')
  const [toasts, setToasts] = useState<Toast[]>([])
  const [scanning, setScanning] = useState({ active: false, done: 0, total: 0 })
  const [refreshToken, setRefreshToken] = useState(0)
  const [incompleteCount, setIncompleteCount] = useState(0)

  const loadSources = useCallback(async () => {
    try {
      const r = await api.listSources()
      setSources(r.sources)
      setLoadError('')
    } catch (e) {
      setLoadError(errorMessage(e))
    }
  }, [])

  useEffect(() => {
    void loadSources()
  }, [loadSources])

  useEffect(() => {
    let cancelled = false
    api
      .listOffers({ incomplete: true, limit: 1 })
      .then((r) => {
        if (!cancelled) setIncompleteCount(r.total)
      })
      .catch(() => {
        // The dedicated page reports the error; the nav count is optional.
      })
    return () => {
      cancelled = true
    }
  }, [refreshToken])

  const refreshData = useCallback(() => setRefreshToken((t) => t + 1), [])

  const notify = useCallback((kind: NoticeKind, text: string) => {
    const id = ++toastSeq
    setToasts((prev) => [...prev, { id, kind, text }])
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, toastTTL)
  }, [])

  const scanOne = useCallback(async (id: number): Promise<ScanResponse> => {
    const r = await api.scanSource(id)
    setSources((prev) => prev.map((s) => (s.id === id ? r.source : s)))
    setRefreshToken((t) => t + 1)
    notify(
      r.result.error ? 'error' : 'success',
      r.result.error
        ? `${r.source.name}: ${r.result.error}`
        : `${r.source.name}: ${r.result.new} new, ${r.result.incomplete} incomplete, ${r.result.disappeared} disappeared`,
    )
    return r
  }, [notify])

  // scanBatch runs one scan at a time (the per-host rate limiter is shared)
  // and never aborts the remaining sources on a failure. Callers own the
  // aggregate toast; this helper only records per-source outcomes.
  const scanBatch = useCallback(async (targets: Source[]): Promise<BatchScanOutcome[]> => {
    if (targets.length === 0) return []
    const out: BatchScanOutcome[] = []
    setScanning({ active: true, done: 0, total: targets.length })
    for (const [i, s] of targets.entries()) {
      setScanning({ active: true, done: i, total: targets.length })
      try {
        const r = await api.scanSource(s.id)
        setSources((prev) => prev.map((x) => (x.id === s.id ? r.source : x)))
        out.push({ source: r.source, response: r })
      } catch (e) {
        out.push({ source: s, error: errorMessage(e) })
      }
    }
    setScanning({ active: false, done: targets.length, total: targets.length })
    await loadSources()
    setRefreshToken((t) => t + 1)
    return out
  }, [loadSources])

  return (
    <>
      <header className="topbar">
        <span className="brand">JobWatch</span>
        <nav>
          <a href="#/" className={route.name === 'dashboard' ? 'active' : ''}>
            Dashboard
          </a>
          <a
            href="#/offers"
            className={route.name === 'offers' || route.name === 'offer' ? 'active' : ''}
          >
            Offers
          </a>
          <a href="#/incomplete" className={route.name === 'incomplete' ? 'active' : ''}>
            Incomplete{incompleteCount > 0 ? ` (${incompleteCount})` : ''}
          </a>
          <a href="#/sources" className={route.name === 'sources' ? 'active' : ''}>
            Sources
          </a>
          <a
            href="#/interview-prep"
            className={route.name === 'interviewPrep' ? 'active' : ''}
          >
            Interview prep
          </a>
          <a
            href="#/search-profile"
            className={route.name === 'profile' ? 'active' : ''}
          >
            Search profile
          </a>
        </nav>
        <span className="spacer" />
      </header>

      {loadError && <div className="error-box top-error">{loadError}</div>}

      <main>
        {route.name === 'dashboard' && (
          <Dashboard
            refreshToken={refreshToken}
            onDataChanged={refreshData}
            onNotice={notify}
            sources={sources}
            onScanBatch={scanBatch}
            scanning={scanning.active}
          />
        )}
        {route.name === 'offers' && (
          <OffersPage
            refreshToken={refreshToken}
            onDataChanged={refreshData}
            onNotice={notify}
            initialSaved={route.saved}
          />
        )}
        {route.name === 'sources' && (
          <SourcesPage
            sources={sources}
            onReload={loadSources}
            onScan={scanOne}
            onScanBatch={scanBatch}
            onNotice={notify}
            scanning={scanning.active}
          />
        )}
        {route.name === 'incomplete' && (
          <IncompletePage
            refreshToken={refreshToken}
            onDataChanged={refreshData}
            onNotice={notify}
          />
        )}
        {route.name === 'offer' && <OfferPage id={route.id} onNotice={notify} />}
        {route.name === 'interviewPrep' && <InterviewPrepPage onNotice={notify} />}
        {route.name === 'profile' && <SearchProfilePage onNotice={notify} />}
        {route.name === 'company' && <CompanyPage id={route.id} onNotice={notify} />}
        {route.name === 'notfound' && (
          <div className="page">
            <div className="panel empty">Page not found. <a href="#/">Back to dashboard</a></div>
          </div>
        )}
      </main>

      {toasts.length > 0 && (
        <div className="toast-stack" aria-live="polite">
          {toasts.map((t) => (
            <div
              key={t.id}
              className={`toast toast-${t.kind}`}
              role={t.kind === 'error' ? 'alert' : 'status'}
            >
              <span>{t.text}</span>
              <button
                type="button"
                className="link-button"
                aria-label="Dismiss notification"
                onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}
              >
                Dismiss
              </button>
            </div>
          ))}
        </div>
      )}
    </>
  )
}
