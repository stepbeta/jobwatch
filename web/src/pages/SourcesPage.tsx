import { useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { SOURCE_TYPES, fmtDateTime, relativeTime, scanLabel } from '../format'
import type { ScanResponse, ScanResult, Source, SourceType } from '../types'

const IMPLEMENTED: SourceType[] = ['greenhouse', 'lever', 'ashby']

function cfgStr(config: Record<string, unknown>, key: string): string {
  const v = config[key]
  if (typeof v === 'string' || typeof v === 'number') return String(v)
  return ''
}

export function SourcesPage({
  sources,
  onReload,
  onScan,
  scanning,
}: {
  sources: Source[]
  onReload: () => Promise<void>
  onScan: (id: number) => Promise<ScanResponse>
  scanning: boolean
}) {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Source | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [results, setResults] = useState<Record<number, ScanResult>>({})
  const [error, setError] = useState('')

  function openNew() {
    setEditing(null)
    setFormOpen(true)
  }

  function openEdit(source: Source) {
    setEditing(source)
    setFormOpen(true)
  }

  async function toggle(source: Source) {
    setBusyId(source.id)
    setError('')
    try {
      await api.updateSource(source.id, { enabled: !source.enabled })
      await onReload()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusyId(null)
    }
  }

  async function refresh(source: Source) {
    setBusyId(source.id)
    setError('')
    try {
      const r = await onScan(source.id)
      setResults((prev) => ({ ...prev, [source.id]: r.result }))
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusyId(null)
    }
  }

  async function remove(source: Source) {
    if (!window.confirm(`Delete source "${source.name}"? Its offers and links are kept.`)) return
    setBusyId(source.id)
    setError('')
    try {
      await api.deleteSource(source.id)
      await onReload()
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Sources</h2>
          <button type="button" className="primary" onClick={openNew}>
            Add source
          </button>
        </div>
        {error && <div className="error-box">{error}</div>}
        {sources.length === 0 ? (
          <p className="empty">No sources yet. Add a Greenhouse, Lever or Ashby board URL.</p>
        ) : (
          <div className="table-wrap">
            <table className="grid">
              <thead>
                <tr>
                  <th>Enabled</th>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Last scan</th>
                  <th>Status</th>
                  <th>Last result</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {sources.map((s) => {
                  const result = results[s.id]
                  return (
                    <tr key={s.id} className={s.enabled ? '' : 'row-disabled'}>
                      <td>
                        <input
                          type="checkbox"
                          checked={s.enabled}
                          disabled={busyId === s.id}
                          onChange={() => void toggle(s)}
                          aria-label={`Enable ${s.name}`}
                        />
                      </td>
                      <td>
                        <strong>{s.name}</strong>
                        <div className="muted small mono">{s.url}</div>
                      </td>
                      <td>{s.source_type}</td>
                      <td title={fmtDateTime(s.last_scan_at)}>
                        {s.last_scan_at ? relativeTime(s.last_scan_at) : 'never'}
                      </td>
                      <td>
                        <span className={`pill pill-scan-${s.last_scan_status ?? 'none'}`}>
                          {scanLabel(s.last_scan_status)}
                        </span>
                      </td>
                      <td className="small">
                        {result ? (
                          <span className={result.status === 'ok' ? 'muted' : 'warn-text'} title={result.error}>
                            {result.status === 'ok'
                              ? `${result.new} new · ${result.changed} changed · ${result.disappeared} gone`
                              : result.error || result.status}
                          </span>
                        ) : (
                          <span className="muted">—</span>
                        )}
                      </td>
                      <td className="actions">
                        <button
                          type="button"
                          disabled={busyId === s.id || scanning}
                          onClick={() => void refresh(s)}
                        >
                          {busyId === s.id ? 'Scanning…' : 'Refresh'}
                        </button>
                        <button type="button" disabled={busyId === s.id} onClick={() => openEdit(s)}>
                          Edit
                        </button>
                        <button type="button" disabled={busyId === s.id} onClick={() => void remove(s)}>
                          Delete
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {formOpen && (
        <SourceForm
          key={editing?.id ?? 'new'}
          source={editing}
          onClose={() => setFormOpen(false)}
          onSaved={async () => {
            setFormOpen(false)
            await onReload()
          }}
        />
      )}
    </div>
  )
}

function SourceForm({
  source,
  onClose,
  onSaved,
}: {
  source: Source | null
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const [name, setName] = useState(source?.name ?? '')
  const [url, setUrl] = useState(source?.url ?? '')
  const [type, setType] = useState<SourceType>(source?.source_type ?? 'greenhouse')
  const [enabled, setEnabled] = useState(source?.enabled ?? true)
  const [company, setCompany] = useState(cfgStr(source?.config ?? {}, 'company'))
  const [rateLimit, setRateLimit] = useState(cfgStr(source?.config ?? {}, 'rate_limit_ms'))
  const [boardToken, setBoardToken] = useState(cfgStr(source?.config ?? {}, 'board_token'))
  const [site, setSite] = useState(cfgStr(source?.config ?? {}, 'site'))
  const [region, setRegion] = useState(cfgStr(source?.config ?? {}, 'region'))
  const [org, setOrg] = useState(cfgStr(source?.config ?? {}, 'org'))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  function buildConfig(): Record<string, unknown> {
    const cfg: Record<string, unknown> = { ...(source?.config ?? {}) }
    const set = (key: string, value: string, numeric = false) => {
      const v = value.trim()
      if (!v) delete cfg[key]
      else cfg[key] = numeric ? Number(v) : v
    }
    delete cfg.board_token
    delete cfg.site
    delete cfg.region
    delete cfg.org
    set('company', company)
    set('rate_limit_ms', rateLimit, true)
    if (type === 'greenhouse') set('board_token', boardToken)
    if (type === 'lever') {
      set('site', site)
      set('region', region)
    }
    if (type === 'ashby') set('org', org)
    return cfg
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    const input = {
      name: name.trim(),
      url: url.trim(),
      source_type: type,
      config: buildConfig(),
      enabled,
    }
    try {
      if (source) {
        await api.updateSource(source.id, input)
      } else {
        await api.createSource(input)
      }
      await onSaved()
    } catch (err) {
      setError(errorMessage(err))
      setSaving(false)
    }
  }

  return (
    <section className="panel form-panel">
      <div className="section-head">
        <h2>{source ? `Edit source #${source.id}` : 'New source'}</h2>
        <button type="button" onClick={onClose}>
          Close
        </button>
      </div>
      <form className="form-grid" onSubmit={(e) => void submit(e)}>
        <label>
          Name
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          URL
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://boards.greenhouse.io/acme"
            required
          />
        </label>
        <label>
          Type
          <select value={type} onChange={(e) => setType(e.target.value as SourceType)}>
            {SOURCE_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>
        <label className="checkbox">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enabled
        </label>

        <label>
          Company override
          <input
            value={company}
            onChange={(e) => setCompany(e.target.value)}
            placeholder="defaults to the source name"
          />
        </label>
        <label>
          Rate limit (ms)
          <input
            type="number"
            min={0}
            value={rateLimit}
            onChange={(e) => setRateLimit(e.target.value)}
            placeholder="2000"
          />
        </label>

        {type === 'greenhouse' && (
          <label>
            Board token
            <input
              value={boardToken}
              onChange={(e) => setBoardToken(e.target.value)}
              placeholder="derived from URL if empty"
            />
          </label>
        )}
        {type === 'lever' && (
          <>
            <label>
              Site
              <input value={site} onChange={(e) => setSite(e.target.value)} placeholder="derived from URL if empty" />
            </label>
            <label>
              Region
              <select value={region} onChange={(e) => setRegion(e.target.value)}>
                <option value="">Global</option>
                <option value="eu">EU</option>
              </select>
            </label>
          </>
        )}
        {type === 'ashby' && (
          <label>
            Org
            <input value={org} onChange={(e) => setOrg(e.target.value)} placeholder="derived from URL if empty" />
          </label>
        )}

        {!IMPLEMENTED.includes(type) && (
          <p className="hint">
            {type === 'manual'
              ? 'Manual sources are never fetched; add offers with the import endpoint or CLI.'
              : 'No adapter for this source type yet: scans skip it without changing its status.'}
          </p>
        )}

        {error && <div className="error-box">{error}</div>}
        <div className="form-actions">
          <button type="submit" className="primary" disabled={saving}>
            {saving ? 'Saving…' : source ? 'Save changes' : 'Create source'}
          </button>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
        </div>
      </form>
    </section>
  )
}
