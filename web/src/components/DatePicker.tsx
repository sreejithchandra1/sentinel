import { useEffect, useRef, useState } from 'react'
import { colors } from '../theme'

const WEEKDAYS = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

function pad(n: number) {
  return String(n).padStart(2, '0')
}

function toISODate(d: Date) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function parseISODate(value: string): Date | null {
  if (!value) return null
  const [y, m, d] = value.split('-').map(Number)
  if (!y || !m || !d) return null
  return new Date(y, m - 1, d)
}

function startOfMonth(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), 1)
}

function addMonths(d: Date, n: number) {
  return new Date(d.getFullYear(), d.getMonth() + n, 1)
}

function sameDay(a: Date, b: Date) {
  return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()
}

function monthLabel(d: Date) {
  return d.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
}

function buildCells(view: Date): (Date | null)[] {
  const first = startOfMonth(view)
  const startPad = first.getDay()
  const daysInMonth = new Date(view.getFullYear(), view.getMonth() + 1, 0).getDate()
  const cells: (Date | null)[] = []
  for (let i = 0; i < startPad; i++) cells.push(null)
  for (let day = 1; day <= daysInMonth; day++) {
    cells.push(new Date(view.getFullYear(), view.getMonth(), day))
  }
  while (cells.length % 7 !== 0) cells.push(null)
  return cells
}

export default function DatePicker({
  value,
  onChange,
  label = 'Date',
}: {
  value: string
  onChange: (date: string) => void
  label?: string
}) {
  const [open, setOpen] = useState(false)
  const selected = parseISODate(value)
  const [view, setView] = useState(() => startOfMonth(selected || new Date()))
  const wrapRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (selected) setView(startOfMonth(selected))
  }, [value])

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const cells = buildCells(view)
  const today = new Date()
  const display = selected
    ? selected.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
    : 'Any date'

  return (
    <div ref={wrapRef} style={styles.wrap}>
      <span style={styles.label}>{label}</span>
      <button
        type="button"
        className="input"
        style={styles.trigger}
        aria-expanded={open}
        onClick={() => setOpen(v => !v)}
      >
        <span style={{ color: value ? colors.text : colors.textMuted }}>{display}</span>
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden style={{ opacity: 0.75, flexShrink: 0 }}>
          <rect x="1.5" y="3" width="13" height="11.5" rx="2" stroke="currentColor" strokeWidth="1.4" />
          <path d="M1.5 6.5h13M5 1.5v3M11 1.5v3" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
        </svg>
      </button>
      {value && (
        <button type="button" className="btn" style={styles.clear} onClick={() => onChange('')}>
          Clear
        </button>
      )}
      {open && (
        <div style={styles.popover} role="dialog" aria-label="Choose date">
          <div style={styles.monthBar}>
            <button type="button" style={styles.navBtn} onClick={() => setView(v => addMonths(v, -1))} aria-label="Previous month">
              ‹
            </button>
            <span style={styles.monthTitle}>{monthLabel(view)}</span>
            <button type="button" style={styles.navBtn} onClick={() => setView(v => addMonths(v, 1))} aria-label="Next month">
              ›
            </button>
          </div>
          <div style={styles.weekRow}>
            {WEEKDAYS.map(d => (
              <span key={d} style={styles.weekday}>{d}</span>
            ))}
          </div>
          <div style={styles.grid}>
            {cells.map((day, i) => {
              if (!day) return <span key={`e-${i}`} style={styles.cellEmpty} />
              const isSelected = selected ? sameDay(day, selected) : false
              const isToday = sameDay(day, today)
              return (
                <button
                  key={toISODate(day)}
                  type="button"
                  style={{
                    ...styles.dayBtn,
                    ...(isToday ? styles.dayToday : {}),
                    ...(isSelected ? styles.daySelected : {}),
                  }}
                  onClick={() => {
                    onChange(toISODate(day))
                    setOpen(false)
                  }}
                >
                  {day.getDate()}
                </button>
              )
            })}
          </div>
          <div style={styles.footer}>
            <button
              type="button"
              style={styles.todayLink}
              onClick={() => {
                const t = new Date()
                onChange(toISODate(t))
                setView(startOfMonth(t))
                setOpen(false)
              }}
            >
              Today
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 16,
    color: colors.textMuted,
  },
  label: { fontWeight: 500, flexShrink: 0 },
  trigger: {
    width: 'auto',
    minWidth: 168,
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 10,
    cursor: 'pointer',
    textAlign: 'left',
    color: colors.text,
  },
  clear: { padding: '8px 12px', fontSize: 15 },
  popover: {
    position: 'absolute',
    top: 'calc(100% + 8px)',
    left: 0,
    zIndex: 40,
    width: 280,
    padding: 12,
    background: colors.card,
    border: `1px solid ${colors.border}`,
    borderRadius: 14,
    boxShadow: 'var(--shadow)',
  },
  monthBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 10,
  },
  monthTitle: { fontWeight: 600, fontSize: 16, color: colors.text },
  navBtn: {
    width: 32,
    height: 32,
    borderRadius: 8,
    border: `1px solid ${colors.border}`,
    background: colors.bg,
    color: colors.text,
    fontSize: 20,
    lineHeight: 1,
  },
  weekRow: {
    display: 'grid',
    gridTemplateColumns: 'repeat(7, 1fr)',
    marginBottom: 4,
  },
  weekday: {
    textAlign: 'center',
    fontSize: 13,
    fontWeight: 600,
    color: colors.textDim,
    padding: '4px 0',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(7, 1fr)',
    gap: 2,
  },
  cellEmpty: { height: 34 },
  dayBtn: {
    height: 34,
    borderRadius: 8,
    border: 'none',
    background: 'transparent',
    color: colors.text,
    fontSize: 15,
    fontWeight: 500,
  },
  dayToday: {
    boxShadow: `inset 0 0 0 1px ${colors.brand}`,
  },
  daySelected: {
    background: colors.brand,
    color: '#041016',
    fontWeight: 700,
  },
  footer: {
    display: 'flex',
    justifyContent: 'center',
    marginTop: 8,
    paddingTop: 8,
    borderTop: `1px solid ${colors.border}`,
  },
  todayLink: {
    border: 'none',
    background: 'transparent',
    color: colors.brand,
    fontSize: 15,
    fontWeight: 600,
  },
}
