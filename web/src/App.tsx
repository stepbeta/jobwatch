import { useCallback, useEffect, useState } from 'react'
import { api, errorMessage } from './api'
import { CompanyPage } from './pages/CompanyPage'
import { Dashboard } from './pages/Dashboard'
import { OfferPage } from './pages/OfferPage'
import { SourcesPage } from './pages/SourcesPage'
import { useRoute } from './router'
import type { ScanResponse, Source } from './types'

interface Notice {
  kind: 'info' | 'error'
  text: string
}

export default function App() {
  const route = useRoute()
  const [sources, setSources] = useState<Source[]>([])
  const [loadError, setLoadError] = useState('')
  const [notice, setNotice] = useState<Notice | null>(null)
  const [scanning, setScanning] = useState({ active: false, done: 0, total: 0 })
  const [refreshToken, setRefreshToken] = useState(0)

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

  const refreshData = useCallback(() => setRefreshToken((t) => t + 1), [])

  const scanOne = useCallback(async (id: number): Promise<ScanResponse> => {
    const r = await api.scanSource(id)
    setSources((prev) => prev.map((s) => (s.id === id ? r.source : s)))
    setRefreshToken((t) => t + 1)
    setNotice(
      r.result.error
        ? { kind: 'error', text: `${r.source.name}: ${r.result.error}` }
        : {
            kind: 'info',
            text: `${r.source.name}: ${r.result.new} new, ${r.result.changed} changed, ${r.result.disappeared} disappeared`,
          },
    )
    return r
  }, [])

  const scanAll = useCallback(async () => {
    const targets = sources.filter((s) => s.enabled)
    if (targets.length === 0) {
      setNotice({ kind: 'info', text: 'No enabled sources to scan.' })
      return
    }
    setScanning({ active: true, done: 0, total: targets.length })
    let failed = 0
    let added = 0
    let changed = 0
    for (const [i, s] of targets.entries()) {
      setScanning({ active: true, done: i, total: targets.length })
      try {
        const r = await api.scanSource(s.id)
        setSources((prev) => prev.map((x) => (x.id === s.id ? r.source : x)))
        added += r.result.new
        changed += r.result.changed
        if (r.result.error) failed++
      } catch (e) {
        failed++
        setNotice({ kind: 'error', text: `${s.name}: ${errorMessage(e)}` })
      }
    }
    setScanning({ active: false, done: targets.length, total: targets.length })
    await loadSources()
    setRefreshToken((t) => t + 1)
    setNotice({
      kind: failed > 0 ? 'error' : 'info',
      text: `Refresh all: ${added} new, ${changed} changed${failed ? `, ${failed} source(s) failed` : ''}.`,
    })
  }, [sources, loadSources])

  const attention = sources.filter(
    (s) => s.last_scan_status === 'error' || s.last_scan_status === 'blocked',
  ).length

  return (
    <>
      <header className="topbar">
        <span className="brand">JobWatch</span>
        <nav>
          <a href="#/" className={route.name === 'dashboard' ? 'active' : ''}>
            Dashboard
          </a>
          <a href="#/sources" className={route.name === 'sources' ? 'active' : ''}>
            Sources
          </a>
        </nav>
        <span className="spacer" />
        {scanning.active && (
          <span className="muted small">
            Scanning {scanning.done}/{scanning.total}…
          </span>
        )}
        {attention > 0 && (
          <a className="pill pill-warn" href="#/sources">
            {attention} source{attention === 1 ? '' : 's'} need attention
          </a>
        )}
        <button
          type="button"
          className="primary"
          onClick={() => void scanAll()}
          disabled={scanning.active || sources.filter((s) => s.enabled).length === 0}
        >
          Refresh all
        </button>
      </header>

      {loadError && <div className="error-box top-error">{loadError}</div>}
      {notice && (
        <div className={`notice notice-${notice.kind}`}>
          <span>{notice.text}</span>
          <button type="button" className="link-button" onClick={() => setNotice(null)}>
            Dismiss
          </button>
        </div>
      )}

      <main>
        {route.name === 'dashboard' && <Dashboard refreshToken={refreshToken} onDataChanged={refreshData} />}
        {route.name === 'sources' && (
          <SourcesPage
            sources={sources}
            onReload={loadSources}
            onScan={scanOne}
            scanning={scanning.active}
          />
        )}
        {route.name === 'offer' && <OfferPage id={route.id} />}
        {route.name === 'company' && <CompanyPage id={route.id} />}
        {route.name === 'notfound' && (
          <div className="page">
            <div className="panel empty">Page not found. <a href="#/">Back to dashboard</a></div>
          </div>
        )}
      </main>
    </>
  )
}
