import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

export type SortOrder = 'asc' | 'desc'

export interface TableColumn {
  id: string
  label: string
}

export const PAGE_SIZE_OPTIONS = [10, 20, 50] as const

export function isPageSize(value: number | undefined): value is number {
  return value !== undefined && (PAGE_SIZE_OPTIONS as readonly number[]).includes(value)
}

// useDebouncedValue delays a rapidly changing value (search boxes). It is
// only used for values that trigger a request or a URL rewrite, so typing
// stays responsive while the expensive work waits ~300 ms.
export function useDebouncedValue<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])
  return debounced
}

function readHidden(storageKey: string): string[] {
  try {
    const raw = window.localStorage.getItem(storageKey)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : []
  } catch {
    return []
  }
}

function writeHidden(storageKey: string, hidden: string[]): void {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(hidden))
  } catch {
    // A blocked localStorage must not break the table; the preference then
    // simply resets on reload.
  }
}

// useColumnVisibility stores the hidden column ids, not the visible ones.
// A column added in a later release is therefore visible for existing
// users: an unknown saved name is ignored, and a new name is absent from
// the hidden list. The last visible column can never be hidden.
export function useColumnVisibility(
  storageKey: string,
  columns: TableColumn[],
  defaultVisible?: string[],
) {
  const known = useMemo(() => columns.map((c) => c.id), [columns])
  const [hidden, setHidden] = useState<string[]>(() => readHidden(storageKey))

  useEffect(() => {
    // Drop saved names that no longer exist so the stored set stays small.
    setHidden((prev) => {
      const pruned = prev.filter((id) => known.includes(id))
      return pruned.length === prev.length ? prev : pruned
    })
  }, [known])

  const defaults = defaultVisible ?? known
  const visible = known.filter((id) => !hidden.includes(id))
  const effective = visible.length > 0 ? visible : defaults.filter((id) => known.includes(id))

  function toggle(id: string) {
    if (!known.includes(id)) return
    setHidden((prev) => {
      const isVisible = !prev.includes(id)
      if (isVisible && effective.length === 1) return prev
      const next = isVisible ? [...prev, id] : prev.filter((v) => v !== id)
      writeHidden(storageKey, next)
      return next
    })
  }

  function reset() {
    setHidden([])
    try {
      window.localStorage.removeItem(storageKey)
    } catch {
      // ignore
    }
  }

  return { visible: effective, hidden, toggle, reset }
}

export function ColumnPicker({
  columns,
  visible,
  onToggle,
  onReset,
}: {
  columns: TableColumn[]
  visible: string[]
  onToggle: (id: string) => void
  onReset: () => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function onPointerDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [open])

  return (
    <div className="column-picker" ref={ref}>
      <button
        type="button"
        className="column-picker-button"
        aria-expanded={open}
        aria-haspopup="true"
        title="Show or hide columns"
        onClick={() => setOpen((v) => !v)}
      >
        Columns
      </button>
      {open && (
        <div className="column-menu" role="group" aria-label="Visible columns">
          {columns.map((column) => {
            const checked = visible.includes(column.id)
            return (
              <label key={column.id} className="column-option">
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={checked && visible.length === 1}
                  onChange={() => onToggle(column.id)}
                />
                {column.label}
              </label>
            )
          })}
          <button type="button" className="link-button" onClick={onReset}>
            Reset columns
          </button>
        </div>
      )}
    </div>
  )
}

// SortHeader renders a clickable column header. active/order drive the
// aria-sort value and the visible directional indicator.
export function SortHeader({
  column,
  label,
  sort,
  order,
  onSort,
  children,
}: {
  column: string
  label?: string
  sort?: string
  order?: SortOrder
  onSort?: (column: string) => void
  children?: ReactNode
}) {
  const active = sort === column
  if (!onSort) return <th>{children ?? label}</th>
  return (
    <th
      aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : 'none'}
      className={active ? 'sort-active' : undefined}
    >
      <button
        type="button"
        className="sort-header"
        onClick={() => onSort(column)}
        title={`Sort by ${label ?? children}`}
      >
        {children ?? label}
        <span className="sort-indicator" aria-hidden="true">
          {active ? (order === 'asc' ? '▲' : '▼') : ''}
        </span>
      </button>
    </th>
  )
}

export function PageSizeSelect({
  value,
  onChange,
}: {
  value: number
  onChange: (value: number) => void
}) {
  return (
    <label className="page-size">
      Per page
      <select value={value} onChange={(e) => onChange(Number(e.target.value))}>
        {PAGE_SIZE_OPTIONS.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>
    </label>
  )
}

// toggleSort flips the direction when the same column is clicked again
// and otherwise starts from that sort's natural direction.
export function toggleSort(
  currentSort: string,
  currentOrder: SortOrder,
  column: string,
  defaultOrder: (sort: string) => SortOrder,
): { sort: string; order: SortOrder } {
  if (currentSort === column) {
    return { sort: column, order: currentOrder === 'asc' ? 'desc' : 'asc' }
  }
  return { sort: column, order: defaultOrder(column) }
}
