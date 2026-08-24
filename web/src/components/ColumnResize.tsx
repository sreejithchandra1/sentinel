import { useCallback, useEffect, useMemo, useState } from 'react'

const MIN_COL = 64
const MAX_FIT = 720

export type SortDir = 'asc' | 'desc'

function storageKey(id: string) {
  return `sentinel.colw.${id}`
}

export function compareValues(a: unknown, b: unknown): number {
  const empty = (v: unknown) => v == null || v === ''
  if (empty(a) && empty(b)) return 0
  if (empty(a)) return 1
  if (empty(b)) return -1
  if (typeof a === 'number' && typeof b === 'number') return a - b
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
}

export function useTableSort<T>(
  rows: T[],
  getValue: (row: T, key: string) => unknown,
) {
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const toggleSort = useCallback((key: string) => {
    setSortKey(prev => {
      if (prev === key) {
        setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
        return prev
      }
      setSortDir('asc')
      return key
    })
  }, [])

  const sorted = useMemo(() => {
    if (!sortKey) return rows
    const copy = [...rows]
    copy.sort((a, b) => {
      const cmp = compareValues(getValue(a, sortKey), getValue(b, sortKey))
      return sortDir === 'asc' ? cmp : -cmp
    })
    return copy
  }, [rows, sortKey, sortDir, getValue])

  const header = useCallback((key: string) => ({
    columnKey: key,
    activeSortKey: sortKey,
    sortDir,
    onSort: toggleSort,
  }), [sortKey, sortDir, toggleSort])

  return { sorted, header }
}

export function useColumnResize(tableId: string, columnCount: number) {
  const [widths, setWidths] = useState<(number | null)[]>(() => {
    try {
      const raw = localStorage.getItem(storageKey(tableId))
      if (!raw) return Array(columnCount).fill(null)
      const parsed = JSON.parse(raw) as unknown
      if (!Array.isArray(parsed) || parsed.length !== columnCount) return Array(columnCount).fill(null)
      return parsed.map(v => (typeof v === 'number' && v > 0 ? v : null))
    } catch {
      return Array(columnCount).fill(null)
    }
  })

  useEffect(() => {
    try {
      localStorage.setItem(storageKey(tableId), JSON.stringify(widths))
    } catch {
      /* ignore quota */
    }
  }, [tableId, widths])

  const startResize = useCallback((index: number, e: React.PointerEvent) => {
    e.preventDefault()
    e.stopPropagation()
    const handle = e.currentTarget as HTMLElement
    const th = handle.closest('th')
    const startW = th?.getBoundingClientRect().width ?? 120
    const startX = e.clientX
    handle.setPointerCapture(e.pointerId)
    const prevCursor = document.body.style.cursor
    const prevSelect = document.body.style.userSelect
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    document.body.classList.add('is-col-resizing')

    const onMove = (ev: PointerEvent) => {
      const next = Math.max(MIN_COL, Math.round(startW + (ev.clientX - startX)))
      setWidths(prev => {
        const copy = [...prev]
        copy[index] = next
        return copy
      })
    }
    const onUp = () => {
      handle.releasePointerCapture(e.pointerId)
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', onUp)
      handle.removeEventListener('pointercancel', onUp)
      document.body.style.cursor = prevCursor
      document.body.style.userSelect = prevSelect
      document.body.classList.remove('is-col-resizing')
    }
    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', onUp)
    handle.addEventListener('pointercancel', onUp)
  }, [])

  const autoFit = useCallback((index: number, table: HTMLTableElement | null) => {
    if (!table) return
    let max = MIN_COL
    const cells = table.querySelectorAll(`tr > *:nth-child(${index + 1})`)
    cells.forEach(node => {
      const el = node as HTMLElement
      max = Math.max(max, Math.ceil(el.scrollWidth) + 20)
    })
    setWidths(prev => {
      const copy = [...prev]
      copy[index] = Math.min(MAX_FIT, max)
      return copy
    })
  }, [])

  return { widths, startResize, autoFit }
}

export function ResizeHandle({
  onDrag,
  onFit,
}: {
  onDrag: (e: React.PointerEvent) => void
  onFit?: () => void
}) {
  return (
    <span
      className="col-resize-handle"
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize column"
      title="Drag to resize column · Double-click to fit content"
      onPointerDown={onDrag}
      onDoubleClick={e => {
        e.preventDefault()
        e.stopPropagation()
        onFit?.()
      }}
    />
  )
}

export function ColGroup({ widths }: { widths: (number | null)[] }) {
  return (
    <colgroup>
      {widths.map((w, i) => (
        <col key={i} style={w ? { width: w } : undefined} />
      ))}
    </colgroup>
  )
}

export function ResizableTh({
  index,
  children,
  style,
  startResize,
  autoFit,
  tableRef,
  className,
  columnKey,
  activeSortKey,
  sortDir,
  onSort,
  resize = true,
}: {
  index: number
  children?: React.ReactNode
  style?: React.CSSProperties
  startResize: (index: number, e: React.PointerEvent) => void
  autoFit: (index: number, table: HTMLTableElement | null) => void
  tableRef: React.RefObject<HTMLTableElement | null>
  className?: string
  columnKey?: string
  activeSortKey?: string | null
  sortDir?: SortDir
  onSort?: (key: string) => void
  resize?: boolean
}) {
  const sortable = Boolean(columnKey && onSort)
  const active = sortable && activeSortKey === columnKey
  const ariaSort = !sortable
    ? undefined
    : active
      ? (sortDir === 'desc' ? 'descending' : 'ascending')
      : 'none'

  return (
    <th className={className} style={style} aria-sort={ariaSort}>
      {sortable ? (
        <button
          type="button"
          className={'th-sort' + (active ? ' is-active' : '')}
          onClick={() => onSort?.(columnKey!)}
        >
          <span className="th-sort-label">{children}</span>
          <span className="th-sort-icon" aria-hidden="true">
            {active ? (sortDir === 'desc' ? '▼' : '▲') : '↕'}
          </span>
        </button>
      ) : children}
      {resize && (
        <ResizeHandle onDrag={e => startResize(index, e)} onFit={() => autoFit(index, tableRef.current)} />
      )}
    </th>
  )
}
