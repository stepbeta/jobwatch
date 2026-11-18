import { useEffect, useId, useRef, useState } from 'react'
import type { Company } from '../types'

// CompanyCombobox is a searchable replacement for the offers page's
// company <select>. It filters the option list client-side and keeps the
// company_id query parameter unchanged.
export function CompanyCombobox({
  companies,
  value,
  onChange,
}: {
  companies: Company[]
  value?: number
  onChange: (id: number | undefined) => void
}) {
  const listID = useId()
  const inputID = useId()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  const selected = companies.find((c) => c.id === value)
  const selectedName = selected?.name ?? ''

  useEffect(() => {
    if (!open) setQuery(selectedName)
  }, [selectedName, open])

  useEffect(() => {
    if (!open) return
    function onPointerDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [open])

  const needle = query.trim().toLowerCase()
  const filtered = needle
    ? companies.filter(
        (c) =>
          c.name.toLowerCase().includes(needle) ||
          c.name_normalized.toLowerCase().includes(needle),
      )
    : companies

  function choose(id: number | undefined) {
    onChange(id)
    setQuery(id === undefined ? '' : companies.find((c) => c.id === id)?.name ?? '')
    setOpen(false)
  }

  return (
    <div className="company-combobox" ref={ref}>
      <label htmlFor={inputID}>Company</label>
      <div className="combobox-control">
        <input
          id={inputID}
          type="text"
          role="combobox"
          aria-expanded={open}
          aria-controls={listID}
          aria-autocomplete="list"
          autoComplete="off"
          placeholder="Any company"
          value={query}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            const next = e.target.value
            setQuery(next)
            setOpen(true)
            if (next.trim() === '') onChange(undefined)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setOpen(false)
            if (e.key === 'Enter') e.preventDefault()
          }}
        />
        {open && (
          <ul className="combobox-menu" id={listID} role="listbox" aria-label="Companies">
            <li>
              <button
                type="button"
                role="option"
                aria-selected={value === undefined}
                className="combobox-option"
                onClick={() => choose(undefined)}
              >
                Any company
              </button>
            </li>
            {filtered.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  role="option"
                  aria-selected={c.id === value}
                  className="combobox-option"
                  onClick={() => choose(c.id)}
                >
                  {c.name}
                </button>
              </li>
            ))}
            {filtered.length === 0 && (
              <li className="combobox-empty muted small">No company matches.</li>
            )}
          </ul>
        )}
      </div>
    </div>
  )
}
