import { useEffect, useMemo, useRef, useState } from 'react'

export const INTERNAL_CUSTOMER_ID = '__internal__'

export type CustomerOption = { id: string; name: string }

interface Props {
  customers: CustomerOption[]
  selectedIds: string[]
  onChange: (ids: string[]) => void
  includeInternal?: boolean
}

export default function CustomerFilter({
  customers,
  selectedIds,
  onChange,
  includeInternal = true,
}: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const rootRef = useRef<HTMLDivElement>(null)

  const options = useMemo(() => {
    const list: CustomerOption[] = includeInternal
      ? [{ id: INTERNAL_CUSTOMER_ID, name: 'Internal (unassigned)' }, ...customers]
      : [...customers]
    return list
  }, [customers, includeInternal])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return options
    return options.filter(o => o.name.toLowerCase().includes(q))
  }, [options, query])

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [])

  function toggle(id: string) {
    if (selectedIds.includes(id)) {
      onChange(selectedIds.filter(x => x !== id))
    } else {
      onChange([...selectedIds, id])
    }
  }

  function selectAll() {
    onChange(options.map(o => o.id))
  }

  function clearAll() {
    onChange([])
  }

  const label = (() => {
    if (selectedIds.length === 0) return 'All customers'
    if (selectedIds.length === 1) {
      return options.find(o => o.id === selectedIds[0])?.name || '1 customer'
    }
    return `${selectedIds.length} customers`
  })()

  return (
    <div ref={rootRef} className="customer-filter">
      <button
        type="button"
        className={`btn customer-filter-trigger${selectedIds.length > 0 ? ' is-filtered' : ''}`}
        onClick={() => setOpen(v => !v)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label="Filter by customer"
      >
        <span className="customer-filter-trigger-label">{label}</span>
        <span className="customer-filter-chevron">{open ? '▴' : '▾'}</span>
      </button>

      {open && (
        <div className="customer-filter-panel" role="listbox" aria-multiselectable="true">
          <input
            className="input"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="Search customers…"
            aria-label="Search customers"
            autoFocus
          />
          <div className="customer-filter-actions">
            <button type="button" className="btn" onClick={selectAll}>Select all</button>
            <button type="button" className="btn" onClick={clearAll}>Clear</button>
          </div>
          <div className="customer-filter-list">
            {filtered.length === 0 ? (
              <div className="customer-filter-empty">No matches</div>
            ) : (
              filtered.map(o => {
                const checked = selectedIds.includes(o.id)
                return (
                  <label key={o.id} className={`customer-filter-row${checked ? ' is-checked' : ''}`}>
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggle(o.id)}
                    />
                    <span className="customer-filter-name">{o.name}</span>
                  </label>
                )
              })
            )}
          </div>
        </div>
      )}
    </div>
  )
}

/** Match a resource tenant_id against selected filter IDs. Empty selection = all. */
export function matchesCustomerFilter(tenantId: string | undefined, selectedIds: string[]): boolean {
  if (selectedIds.length === 0) return true
  const tid = tenantId || ''
  if (!tid) return selectedIds.includes(INTERNAL_CUSTOMER_ID)
  return selectedIds.includes(tid)
}
