import { useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import type { BatchScanOutcome, NoticeKind } from '../App'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { Modal } from '../components/Modal'
import { SOURCE_TYPES, fmtDateTime, relativeTime, scanLabel } from '../format'
import type { ScanResponse, ScanResult, Source, SourceType } from '../types'

const FETCHABLE: SourceType[] = [
  'greenhouse',
  'lever',
  'ashby',
  'personio',
  'workable',
  'bamboohr',
  'rss',
  'html',
]

interface ExtraField {
  key: string
  label: string
  placeholder?: string
}

const EXTRA_FIELDS: Partial<Record<SourceType, ExtraField[]>> = {
  personio: [
    { key: 'tenant', label: 'Tenant', placeholder: 'acme (from acme.jobs.personio.de)' },
    { key: 'language', label: 'Feed language', placeholder: 'de or en (optional)' },
  ],
  workable: [{ key: 'account', label: 'Account', placeholder: 'acme (from apply.workable.com/acme)' }],
  bamboohr: [
    { key: 'subdomain', label: 'Subdomain', placeholder: 'acme (from acme.bamboohr.com/careers)' },
  ],
  rss: [
    { key: 'item', label: 'Item element', placeholder: 'item / entry (auto-detected)' },
    { key: 'title_field', label: 'Title field', placeholder: 'title' },
    { key: 'link_field', label: 'Link field', placeholder: 'link / link@href' },
    { key: 'description_field', label: 'Description field', placeholder: 'description / summary / content' },
    { key: 'location_field', label: 'Location field', placeholder: 'e.g. region (optional)' },
    { key: 'company_field', label: 'Company field', placeholder: 'e.g. author (optional)' },
    { key: 'uid_field', label: 'UID field', placeholder: 'guid / id (optional)' },
  ],
  html: [
    { key: 'item_selector', label: 'Item selector', placeholder: 'li.job-card' },
    { key: 'title_selector', label: 'Title selector', placeholder: 'h2 a' },
    { key: 'link_selector', label: 'Link selector', placeholder: 'defaults to the title selector' },
    { key: 'link_attr', label: 'Link attribute', placeholder: 'href' },
    { key: 'location_selector', label: 'Location selector', placeholder: '.location (optional)' },
    { key: 'description_selector', label: 'Description selector', placeholder: '.summary (optional)' },
    { key: 'company_selector', label: 'Company selector', placeholder: '.company (optional)' },
    { key: 'uid_selector', label: 'UID selector', placeholder: 'e.g. a (optional)' },
    { key: 'uid_attr', label: 'UID attribute', placeholder: 'e.g. data-id (optional)' },
  ],
}

function extraKeys(): string[] {
  const out: string[] = []
  for (const fields of Object.values(EXTRA_FIELDS)) {
    for (const f of fields ?? []) out.push(f.key)
  }
  return out
}

function canScan(source: Source): boolean {
  return source.enabled && source.source_type !== 'manual' && source.source_type !== 'wttj'
}

function cfgStr(config: Record<string, unknown>, key: string): string {
  const v = config[key]
  if (typeof v === 'string' || typeof v === 'number') return String(v)
  return ''
}

export function SourcesPage({
  sources,
  onReload,
  onScan,
  onScanBatch,
  onNotice,
  scanning,
}: {
  sources: Source[]
  onReload: () => Promise<void>
  onScan: (id: number) => Promise<ScanResponse>
  onScanBatch: (targets: Source[]) => Promise<BatchScanOutcome[]>
  onNotice: (kind: NoticeKind, text: string) => void
  scanning: boolean
}) {
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<Source | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [results, setResults] = useState<Record<number, ScanResult>>({})
  const [error, setError] = useState('')
  const [confirm, setConfirm] = useState<ConfirmRequest | null>(null)

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
      onNotice('info', `${source.name} ${source.enabled ? 'disabled' : 'enabled'}.`)
    } catch (e) {
      const text = errorMessage(e)
      setError(text)
      onNotice('error', text)
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
      const text = errorMessage(e)
      setError(text)
      onNotice('error', text)
    } finally {
      setBusyId(null)
    }
  }

  async function refreshAll() {
    const targets = sources.filter(canScan)
    if (targets.length === 0) {
      onNotice('info', 'No enabled sources to scan.')
      return
    }
    const outcomes = await onScanBatch(targets)
    let added = 0
    let changed = 0
    let disappeared = 0
    let failed = 0
    for (const outcome of outcomes) {
      const err = outcome.error ?? outcome.response?.result.error
      if (err) {
        failed++
        onNotice('error', `${outcome.source.name}: ${err}`)
        continue
      }
      added += outcome.response?.result.new ?? 0
      changed += outcome.response?.result.changed ?? 0
      disappeared += outcome.response?.result.disappeared ?? 0
    }
    onNotice(
      failed > 0 ? 'error' : 'success',
      `Refresh all: ${added} new, ${changed} changed, ${disappeared} disappeared` +
        (failed > 0 ? `, ${failed} source(s) failed.` : '.'),
    )
  }

  function askRemove(source: Source) {
    setConfirm({
      title: 'Delete source',
      message: `Delete source "${source.name}"? Its offers and provenance links are kept. This only removes the source row.`,
      confirmLabel: 'Delete source',
      onConfirm: async () => {
        setBusyId(source.id)
        try {
          await api.deleteSource(source.id)
          await onReload()
          onNotice('success', `Source "${source.name}" deleted.`)
        } finally {
          setBusyId(null)
        }
      },
    })
  }

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Sources</h2>
          <div className="actions">
            <button
              type="button"
              disabled={scanning || sources.filter(canScan).length === 0}
              title="Scan every enabled source with a supported adapter"
              onClick={() => void refreshAll()}
            >
              {scanning ? 'Scanning…' : 'Refresh all'}
            </button>
            <button type="button" className="primary" onClick={openNew}>
              Add source
            </button>
          </div>
        </div>
        {error && <div className="error-box">{error}</div>}
        {sources.length === 0 ? (
          <div className="empty-state">
            <p>No sources yet.</p>
            <p className="muted">
              Add a Greenhouse, Lever, Ashby, Personio, Workable or BambooHR board URL, an RSS/Atom
              feed, or a careers page with CSS selectors.
            </p>
          </div>
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
                        {cfgStr(s.config, 'note') && (
                          <div className="note-text small">{cfgStr(s.config, 'note')}</div>
                        )}
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
                          disabled={busyId === s.id || scanning || !canScan(s)}
                          title={
                            !s.enabled
                              ? 'Enable the source to scan it'
                              : s.source_type === 'wttj'
                                ? 'No adapter for this source type yet'
                                : s.source_type === 'manual'
                                  ? 'Manual sources are never fetched'
                                  : undefined
                          }
                          onClick={() => void refresh(s)}
                        >
                          {busyId === s.id ? 'Scanning…' : 'Refresh'}
                        </button>
                        <button type="button" disabled={busyId === s.id} onClick={() => openEdit(s)}>
                          Edit
                        </button>
                        <button type="button" disabled={busyId === s.id} onClick={() => askRemove(s)}>
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
          onSaved={async (created) => {
            setFormOpen(false)
            await onReload()
            onNotice('success', created ? 'Source created.' : 'Source updated.')
          }}
        />
      )}

      {confirm && <ConfirmDialog request={confirm} onClose={() => setConfirm(null)} />}
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
  onSaved: (created: boolean) => Promise<void>
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
  const [extra, setExtra] = useState<Record<string, string>>(() => {
    const out: Record<string, string> = {}
    for (const key of extraKeys()) out[key] = cfgStr(source?.config ?? {}, key)
    return out
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  function setExtraField(key: string, value: string) {
    setExtra((prev) => ({ ...prev, [key]: value }))
  }

  function buildConfig(): Record<string, unknown> {
    const cfg: Record<string, unknown> = { ...(source?.config ?? {}) }
    const set = (key: string, value: string, numeric = false) => {
      const v = value.trim()
      if (!v) delete cfg[key]
      else cfg[key] = numeric ? Number(v) : v
    }
    for (const key of [
      'board_token',
      'site',
      'region',
      'org',
      'company',
      'rate_limit_ms',
      ...extraKeys(),
    ]) {
      delete cfg[key]
    }
    set('company', company)
    set('rate_limit_ms', rateLimit, true)
    if (type === 'greenhouse') set('board_token', boardToken)
    if (type === 'lever') {
      set('site', site)
      set('region', region)
    }
    if (type === 'ashby') set('org', org)
    for (const field of EXTRA_FIELDS[type] ?? []) set(field.key, extra[field.key] ?? '')
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
      await onSaved(source === null)
    } catch (err) {
      setError(errorMessage(err))
      setSaving(false)
    }
  }

  return (
    <Modal title={source ? `Edit source #${source.id}` : 'New source'} onClose={onClose} wide>
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

        {(EXTRA_FIELDS[type] ?? []).map((field) => (
          <label key={field.key}>
            {field.label}
            <input
              value={extra[field.key] ?? ''}
              onChange={(e) => setExtraField(field.key, e.target.value)}
              placeholder={field.placeholder}
            />
          </label>
        ))}

        {type === 'html' && (
          <p className="hint span-2">
            The listing page is fetched at most once per scan and never faster than every 5 seconds.
            Selectors use standard CSS syntax (tag, class, id, attributes and descendant/child
            combinators).
          </p>
        )}
        {type === 'rss' && (
          <p className="hint span-2">
            Field paths are child element local names; use <code>@name</code> for an attribute and
            <code> #text</code> for the element text. Defaults cover RSS &lt;item&gt; and Atom
            &lt;entry&gt; feeds.
          </p>
        )}
        {cfgStr(source?.config ?? {}, 'note') && (
          <p className="hint span-2">{cfgStr(source?.config ?? {}, 'note')}</p>
        )}

        {type === 'manual' && (
          <p className="hint span-2">
            Manual sources are never fetched; add offers with the import form or the CLI.
          </p>
        )}
        {type === 'wttj' && (
          <p className="hint span-2">
            No adapter for this source type yet: scans skip it without changing its status. Welcome to
            the Jungle has no public contract; use manual import or an RSS/HTML source instead.
          </p>
        )}
        {!FETCHABLE.includes(type) && type !== 'manual' && type !== 'wttj' && (
          <p className="hint span-2">
            No adapter for this source type yet: scans skip it without changing its status.
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
    </Modal>
  )
}
