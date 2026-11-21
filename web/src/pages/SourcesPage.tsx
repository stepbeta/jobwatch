import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import type { BatchScanOutcome, NoticeKind } from '../App'
import { ConfirmDialog, type ConfirmRequest } from '../components/ConfirmDialog'
import { ActionIcon } from '../components/icons'
import { Link } from '../components/Link'
import { Modal } from '../components/Modal'
import {
  ColumnPicker,
  PageSizeSelect,
  SortHeader,
  isPageSize,
  toggleSort,
  useColumnVisibility,
  useDebouncedValue,
  type SortOrder,
  type TableColumn,
} from '../components/TableControls'
import { SOURCE_TYPES, fmtDateTime, relativeTime, scanLabel } from '../format'
import { currentRouteQuery, enumParam, intParam, replaceRouteQuery } from '../router'
import type { Company, ScanResponse, ScanResult, Source, SourceType } from '../types'

const FETCHABLE: SourceType[] = [
  'greenhouse',
  'lever',
  'ashby',
  'personio',
  'workable',
  'bamboohr',
  'wttj',
  'rss',
  'html',
]

const COLUMNS_KEY = 'sources.columns'
const PER_PAGE_KEY = 'sources.perPage'
const DEFAULT_PER_PAGE = 20

const SOURCE_COLUMNS: TableColumn[] = [
  { id: 'name', label: 'Name' },
  { id: 'type', label: 'Type' },
  { id: 'last_scan', label: 'Last scan' },
  { id: 'status', label: 'Status' },
  { id: 'last_result', label: 'Last result' },
  { id: 'enabled', label: 'Enabled' },
]

const ENABLED_FILTERS = ['all', 'enabled', 'disabled'] as const
const STATUS_FILTERS = ['ok', 'error', 'blocked', 'never'] as const
const SORT_KEYS = ['name', 'type', 'last_scan', 'status', 'enabled'] as const

type EnabledFilter = (typeof ENABLED_FILTERS)[number]
type StatusFilter = (typeof STATUS_FILTERS)[number]
type SourceSort = (typeof SORT_KEYS)[number]

interface SourcesView {
  enabled: EnabledFilter
  type?: SourceType
  status?: StatusFilter
  q?: string
  sort: SourceSort
  order: SortOrder
  page: number
  perPage: number
}

function readPerPage(): number {
  try {
    const raw = window.localStorage.getItem(PER_PAGE_KEY)
    const n = raw === null ? undefined : Number(raw)
    if (isPageSize(n)) return n
  } catch {
    // fall through to the default
  }
  return DEFAULT_PER_PAGE
}

function writePerPage(value: number): void {
  try {
    window.localStorage.setItem(PER_PAGE_KEY, String(value))
  } catch {
    // presentation-only preference
  }
}

function readSourcesView(): SourcesView {
  const params = currentRouteQuery()
  const page = intParam(params, 'page')
  const perPage = intParam(params, 'per_page')
  const sort = enumParam(params, 'sort', SORT_KEYS) ?? 'name'
  return {
    enabled: enumParam(params, 'enabled', ENABLED_FILTERS) ?? 'all',
    type: enumParam(params, 'type', SOURCE_TYPES),
    status: enumParam(params, 'status', STATUS_FILTERS),
    q: params.get('q')?.trim() || undefined,
    sort,
    order: enumParam(params, 'order', ['asc', 'desc'] as const) ?? 'asc',
    page: page !== undefined && page > 0 ? page : 1,
    perPage: isPageSize(perPage) ? perPage : readPerPage(),
  }
}

function writeURL(view: SourcesView): void {
  const params = new URLSearchParams()
  if (view.enabled !== 'all') params.set('enabled', view.enabled)
  if (view.type) params.set('type', view.type)
  if (view.status) params.set('status', view.status)
  if (view.q) params.set('q', view.q)
  if (view.sort !== 'name' || view.order !== 'asc') {
    params.set('sort', view.sort)
    params.set('order', view.order)
  }
  if (view.page > 1) params.set('page', String(view.page))
  params.set('per_page', String(view.perPage))
  replaceRouteQuery(params)
}

function extraKeys(): string[] {
  const out: string[] = []
  for (const fields of Object.values(EXTRA_FIELDS)) {
    for (const f of fields ?? []) out.push(f.key)
  }
  return out
}

function canScan(source: Source): boolean {
  return source.enabled && source.source_type !== 'manual'
}

function cfgStr(config: Record<string, unknown>, key: string): string {
  const v = config[key]
  if (typeof v === 'string' || typeof v === 'number') return String(v)
  return ''
}

// effectiveCompanyName mirrors the scan pipeline's order: config override,
// then the source name. The sources payload does not carry a board-reported
// company, so a board whose name differs from its offers stays unlinked
// until an override is set.
function effectiveCompanyName(source: Source): string {
  return cfgStr(source.config, 'company').trim() || source.name
}

function reportBatch(
  label: string,
  outcomes: BatchScanOutcome[],
  onNotice: (kind: NoticeKind, text: string) => void,
): void {
  let added = 0
  let incomplete = 0
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
    incomplete += outcome.response?.result.incomplete ?? 0
    disappeared += outcome.response?.result.disappeared ?? 0
  }
  onNotice(
    failed > 0 ? 'error' : 'success',
    `${label}: ${added} new, ${incomplete} incomplete, ${disappeared} disappeared` +
      (failed > 0 ? `, ${failed} source(s) failed.` : '.'),
  )
}

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
  const [selected, setSelected] = useState<number[]>([])
  const [companies, setCompanies] = useState<Company[]>([])
  const [view, setView] = useState<SourcesView>(readSourcesView)
  const [nameFilter, setNameFilter] = useState(() => view.q ?? '')
  const debouncedName = useDebouncedValue(nameFilter)
  const columns = useColumnVisibility(COLUMNS_KEY, SOURCE_COLUMNS)

  useEffect(() => {
    setView((v) => {
      const q = debouncedName.trim() || undefined
      return v.q === q ? v : { ...v, q, page: 1 }
    })
  }, [debouncedName])

  useEffect(() => {
    writeURL(view)
  }, [view])

  useEffect(() => {
    setSelected((ids) => ids.filter((id) => sources.some((s) => s.id === id)))
  }, [sources])

  useEffect(() => {
    setSelected([])
  }, [view.enabled, view.type, view.status, view.q])

  useEffect(() => {
    let cancelled = false
    api
      .listCompanies({ limit: 500 })
      .then((r) => {
        if (!cancelled) setCompanies(r.companies)
      })
      .catch(() => {
        // The name link is a convenience; a failed lookup leaves plain names.
      })
    return () => {
      cancelled = true
    }
  }, [sources])

  const types = useMemo(
    () => [...new Set(sources.map((s) => s.source_type))].sort((a, b) => a.localeCompare(b)),
    [sources],
  )

  const filtered = useMemo(() => {
    const needle = (view.q ?? '').toLowerCase()
    return sources.filter((s) => {
      if (view.enabled === 'enabled' && !s.enabled) return false
      if (view.enabled === 'disabled' && s.enabled) return false
      if (view.type && s.source_type !== view.type) return false
      if (view.status) {
        const status = s.last_scan_status ?? 'never'
        if (status !== view.status) return false
      }
      if (needle && !s.name.toLowerCase().includes(needle)) return false
      return true
    })
  }, [sources, view.enabled, view.type, view.status, view.q])

  const sorted = useMemo(() => {
    const dir = view.order === 'asc' ? 1 : -1
    const compare = (a: Source, b: Source): number => {
      switch (view.sort) {
        case 'type':
          return a.source_type.localeCompare(b.source_type)
        case 'last_scan':
          return (a.last_scan_at ?? '').localeCompare(b.last_scan_at ?? '')
        case 'status':
          return scanLabel(a.last_scan_status).localeCompare(scanLabel(b.last_scan_status))
        case 'enabled':
          return Number(a.enabled) - Number(b.enabled)
        default:
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
      }
    }
    return [...filtered].sort((a, b) => {
      const primary = compare(a, b)
      return primary !== 0 ? primary * dir : (a.id - b.id) * dir
    })
  }, [filtered, view.sort, view.order])

  const total = sorted.length
  const offset = (view.page - 1) * view.perPage
  const pageRows = sorted.slice(offset, offset + view.perPage)

  function patchView(patch: Partial<SourcesView>) {
    setView((v) => ({ ...v, ...patch, page: 1 }))
  }

  function resetFilters() {
    setNameFilter('')
    setView((v) => ({ ...v, enabled: 'all', type: undefined, status: undefined, q: undefined, sort: 'name', order: 'asc', page: 1 }))
  }

  function handleSort(sort: string) {
    setView((v) => {
      const next = toggleSort(v.sort, v.order, sort, () => 'asc')
      return { ...v, sort: sort as SourceSort, order: next.order, page: 1 }
    })
  }

  function changePerPage(value: number) {
    writePerPage(value)
    setView((v) => ({ ...v, perPage: value, page: 1 }))
  }

  const visibleIDs = pageRows.map((s) => s.id)
  const allVisibleSelected =
    visibleIDs.length > 0 && visibleIDs.every((id) => selected.includes(id))

  function toggleAllVisible() {
    setSelected((ids) => {
      if (allVisibleSelected) return ids.filter((id) => !visibleIDs.includes(id))
      return [...new Set([...ids, ...visibleIDs])]
    })
  }

  function toggleSelected(id: number) {
    setSelected((ids) => (ids.includes(id) ? ids.filter((v) => v !== id) : [...ids, id]))
  }

  function companyFor(source: Source): Company | undefined {
    const name = effectiveCompanyName(source).toLowerCase().replace(/\s+/g, ' ').trim()
    return companies.find(
      (c) => c.name_normalized === name || c.name.toLowerCase().replace(/\s+/g, ' ').trim() === name,
    )
  }

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
    reportBatch('Refresh all', outcomes, onNotice)
  }

  async function refreshSelected() {
    const targets = sources.filter((s) => selected.includes(s.id) && canScan(s))
    if (targets.length === 0) {
      onNotice('info', 'No enabled sources selected to scan.')
      return
    }
    const outcomes = await onScanBatch(targets)
    reportBatch('Refresh selected', outcomes, onNotice)
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

  function askRemoveSelected() {
    const targets = sources.filter((s) => selected.includes(s.id))
    if (targets.length === 0) return
    const names = targets.map((s) => s.name).join(', ')
    setConfirm({
      title: `Delete ${targets.length} source${targets.length === 1 ? '' : 's'}`,
      message:
        `Delete these sources: ${names}? Their offers and provenance links are kept; each provenance ` +
        `row simply loses its source reference (source_id is detached). This only removes the source rows.`,
      confirmLabel: 'Delete selected',
      onConfirm: async () => {
        const failures: string[] = []
        for (const source of targets) {
          try {
            await api.deleteSource(source.id)
          } catch (e) {
            failures.push(`${source.name}: ${errorMessage(e)}`)
          }
        }
        setSelected([])
        await onReload()
        if (failures.length > 0) {
          const text = `${failures.length} source(s) could not be deleted. ${failures.join(' · ')}`
          setError(text)
          onNotice('error', text)
        } else {
          onNotice('success', `${targets.length} source${targets.length === 1 ? '' : 's'} deleted.`)
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
          <>
            <div className="filters">
              <div className="segmented" role="group" aria-label="Enabled">
                <span className="filter-label">Enabled</span>
                {ENABLED_FILTERS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={view.enabled === value}
                    className={view.enabled === value ? 'active' : ''}
                    onClick={() => patchView({ enabled: value })}
                  >
                    {value === 'all' ? 'All' : value === 'enabled' ? 'Enabled' : 'Disabled'}
                  </button>
                ))}
              </div>
              <label>
                Type
                <select
                  value={view.type ?? ''}
                  onChange={(e) =>
                    patchView({ type: (e.target.value || undefined) as SourceType | undefined })
                  }
                >
                  <option value="">Any</option>
                  {types.map((t) => (
                    <option key={t} value={t}>
                      {t}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Status
                <select
                  value={view.status ?? ''}
                  onChange={(e) =>
                    patchView({ status: (e.target.value || undefined) as StatusFilter | undefined })
                  }
                >
                  <option value="">Any</option>
                  <option value="ok">OK</option>
                  <option value="error">Error</option>
                  <option value="blocked">Blocked</option>
                  <option value="never">Never scanned</option>
                </select>
              </label>
              <label>
                Name
                <input
                  type="search"
                  placeholder="filter by name"
                  value={nameFilter}
                  onChange={(e) => setNameFilter(e.target.value)}
                />
              </label>
              <div className="filter-actions">
                <ColumnPicker
                  columns={SOURCE_COLUMNS}
                  visible={columns.visible}
                  onToggle={columns.toggle}
                  onReset={columns.reset}
                />
                <button type="button" onClick={resetFilters}>
                  Reset
                </button>
              </div>
            </div>

            <div className="bulk-actions">
              <span className="muted small">{selected.length} selected</span>
              <button
                type="button"
                disabled={selected.length === 0 || scanning}
                onClick={() => void refreshSelected()}
              >
                Refresh selected
              </button>
              <button
                type="button"
                disabled={selected.length === 0}
                onClick={askRemoveSelected}
              >
                Delete selected
              </button>
            </div>

            <div className="table-wrap">
              <table className="grid sources-table">
                <thead>
                  <tr>
                    <th className="select-cell">
                      <input
                        type="checkbox"
                        checked={allVisibleSelected}
                        onChange={toggleAllVisible}
                        aria-label="Select all visible sources"
                      />
                    </th>
                    {columns.visible.includes('name') && (
                      <SortHeader column="name" label="Name" sort={view.sort} order={view.order} onSort={handleSort} />
                    )}
                    {columns.visible.includes('type') && (
                      <SortHeader column="type" label="Type" sort={view.sort} order={view.order} onSort={handleSort} />
                    )}
                    {columns.visible.includes('last_scan') && (
                      <SortHeader
                        column="last_scan"
                        label="Last scan"
                        sort={view.sort}
                        order={view.order}
                        onSort={handleSort}
                      />
                    )}
                    {columns.visible.includes('status') && (
                      <SortHeader
                        column="status"
                        label="Status"
                        sort={view.sort}
                        order={view.order}
                        onSort={handleSort}
                      />
                    )}
                    {columns.visible.includes('last_result') && <th>Last result</th>}
                    {columns.visible.includes('enabled') && (
                      <SortHeader
                        column="enabled"
                        label="Enabled"
                        sort={view.sort}
                        order={view.order}
                        onSort={handleSort}
                      />
                    )}
                    <th className="actions" />
                  </tr>
                </thead>
                <tbody>
                  {pageRows.map((s) => {
                    const result = results[s.id]
                    const company = companyFor(s)
                    return (
                      <tr
                        key={s.id}
                        className={`${s.enabled ? '' : 'row-disabled'}${selected.includes(s.id) ? ' row-selected' : ''}`}
                      >
                        <td className="select-cell">
                          <input
                            type="checkbox"
                            checked={selected.includes(s.id)}
                            onChange={() => toggleSelected(s.id)}
                            aria-label={`Select ${s.name}`}
                          />
                        </td>
                        {columns.visible.includes('name') && (
                          <td>
                            {company ? (
                              <Link to={`/companies/${company.id}`}>
                                <strong>{s.name}</strong>
                              </Link>
                            ) : (
                              <strong>{s.name}</strong>
                            )}
                            <div className="muted small mono">{s.url}</div>
                            {cfgStr(s.config, 'note') && (
                              <div className="note-text small">{cfgStr(s.config, 'note')}</div>
                            )}
                          </td>
                        )}
                        {columns.visible.includes('type') && <td>{s.source_type}</td>}
                        {columns.visible.includes('last_scan') && (
                          <td title={fmtDateTime(s.last_scan_at)}>
                            {s.last_scan_at ? relativeTime(s.last_scan_at) : 'never'}
                          </td>
                        )}
                        {columns.visible.includes('status') && (
                          <td>
                            <span className={`pill pill-scan-${s.last_scan_status ?? 'none'}`}>
                              {scanLabel(s.last_scan_status)}
                            </span>
                          </td>
                        )}
                        {columns.visible.includes('last_result') && (
                          <td className="small">
                            {result ? (
                              <span className={result.status === 'ok' ? 'muted' : 'warn-text'} title={result.error}>
                                {result.status === 'ok'
                                  ? `${result.new} new · ${result.incomplete} incomplete · ${result.disappeared} gone`
                                  : result.error || result.status}
                              </span>
                            ) : (
                              <span className="muted">—</span>
                            )}
                          </td>
                        )}
                        {columns.visible.includes('enabled') && (
                          <td className="cell-enabled">
                            <label className="toggle-switch">
                              <input
                                type="checkbox"
                                role="switch"
                                checked={s.enabled}
                                disabled={busyId === s.id}
                                onChange={() => void toggle(s)}
                                aria-label={`Enable ${s.name}`}
                              />
                              <span className="toggle-track" aria-hidden="true" />
                            </label>
                          </td>
                        )}
                        <td className="actions">
                          <button
                            type="button"
                            className={`icon-button${busyId === s.id ? ' busy' : ''}`}
                            disabled={busyId === s.id || scanning || !canScan(s)}
                            title={
                              !s.enabled
                                ? 'Enable the source to scan it'
                                : s.source_type === 'manual'
                                  ? 'Manual sources are never fetched'
                                  : 'Refresh'
                            }
                            aria-label={`Refresh ${s.name}`}
                            onClick={() => void refresh(s)}
                          >
                            <ActionIcon name="refresh" />
                          </button>
                          <button
                            type="button"
                            className="icon-button"
                            disabled={busyId === s.id}
                            title="Edit"
                            aria-label={`Edit ${s.name}`}
                            onClick={() => openEdit(s)}
                          >
                            <ActionIcon name="edit" />
                          </button>
                          <button
                            type="button"
                            className="icon-button danger"
                            disabled={busyId === s.id}
                            title="Delete"
                            aria-label={`Delete ${s.name}`}
                            onClick={() => askRemove(s)}
                          >
                            <ActionIcon name="trash" />
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
              {pageRows.length === 0 && (
                <p className="empty">No sources match these filters.</p>
              )}
            </div>

            <div className="pagination">
              <button
                type="button"
                disabled={view.page <= 1}
                onClick={() => setView((v) => ({ ...v, page: Math.max(1, v.page - 1) }))}
              >
                ← Previous
              </button>
              <span className="muted">
                {total === 0 ? '0' : `${offset + 1}–${Math.min(offset + view.perPage, total)}`} of {total}
              </span>
              <button
                type="button"
                disabled={offset + view.perPage >= total}
                onClick={() => setView((v) => ({ ...v, page: v.page + 1 }))}
              >
                Next →
              </button>
              <PageSizeSelect value={view.perPage} onChange={changePerPage} />
            </div>
          </>
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
            Paste a Welcome to the Jungle company careers page, for example
            https://www.welcometothejungle.com/en/companies/fabriq/jobs. Search credentials are fetched
            from the page automatically; nothing has to be copied from the browser.
          </p>
        )}
        {!FETCHABLE.includes(type) && type !== 'manual' && (
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
