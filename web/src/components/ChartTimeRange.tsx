import { FormEvent, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  CHART_PERIOD_PRESETS,
  MAX_RANGE_MS,
  MIN_ZOOM_MS,
  fromDateTimeInput,
  resolveChartRange,
  toDateTimeInput,
  type ChartRange,
  type ChartTimeZone,
} from '../utils/period'

function IconCalendar() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3.5" y="5" width="17" height="15.5" rx="2.2" stroke="currentColor" strokeWidth="1.7" />
      <path d="M3.5 9.5h17M8 3.5v3.5M16 3.5v3.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  )
}

export default function ChartTimeRange({
  value,
  onChange,
  timeZone: timeZoneProp,
  onTimeZoneChange,
  label = 'Chart time range',
}: {
  value: ChartRange
  onChange: (range: ChartRange) => void
  timeZone?: ChartTimeZone
  onTimeZoneChange?: (tz: ChartTimeZone) => void
  label?: string
}) {
  const [internalTz, setInternalTz] = useState<ChartTimeZone>(timeZoneProp ?? 'utc')
  const timeZone = timeZoneProp ?? internalTz
  function setTimeZone(next: ChartTimeZone) {
    setInternalTz(next)
    onTimeZoneChange?.(next)
  }
  const resolved = resolveChartRange(value)
  const [startLocal, setStartLocal] = useState(() => toDateTimeInput(resolved.from, timeZone))
  const [endLocal, setEndLocal] = useState(() => toDateTimeInput(resolved.to, timeZone))
  const [open, setOpen] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLFormElement>(null)
  const [pos, setPos] = useState({ top: 0, left: 0 })

  useEffect(() => {
    const next = resolveChartRange(value)
    setStartLocal(toDateTimeInput(next.from, timeZone))
    setEndLocal(toDateTimeInput(next.to, timeZone))
  }, [value, timeZone])

  function place() {
    const wrap = wrapRef.current
    const menu = menuRef.current
    if (!wrap || !menu) return
    const r = wrap.getBoundingClientRect()
    const width = menu.offsetWidth || 280
    const height = menu.offsetHeight || 0
    let left = r.right - width
    left = Math.max(8, Math.min(left, window.innerWidth - width - 8))
    let top = r.bottom + 8
    if (height && top + height > window.innerHeight - 8) {
      top = Math.max(8, r.top - 8 - height)
    }
    setPos({ top, left })
  }

  useLayoutEffect(() => {
    if (!open) return
    place()
    const id = requestAnimationFrame(place)
    menuRef.current?.focus({ preventScroll: true })
    return () => cancelAnimationFrame(id)
  }, [open])

  useEffect(() => {
    if (!open) return
    function onDoc(e: MouseEvent) {
      const t = e.target as Node
      if (wrapRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [open])

  const presetIds = new Set(CHART_PERIOD_PRESETS.map(p => p.id))
  const activePreset = value.kind === 'relative' && presetIds.has(value.period) ? value.period : ''

  function preventFocusSteal(e: { preventDefault: () => void }) {
    e.preventDefault()
  }

  function applyAbsolute(e?: FormEvent) {
    e?.preventDefault()
    const from = fromDateTimeInput(startLocal, timeZone)
    const to = fromDateTimeInput(endLocal, timeZone)
    if (!from || !to || new Date(to).getTime() <= new Date(from).getTime()) return
    let fromMs = new Date(from).getTime()
    let toMs = new Date(to).getTime()
    let span = toMs - fromMs
    span = Math.min(MAX_RANGE_MS, Math.max(MIN_ZOOM_MS, span))
    const now = Date.now()
    if (toMs > now) toMs = now
    fromMs = toMs - span
    onChange({ kind: 'absolute', from: new Date(fromMs).toISOString(), to: new Date(toMs).toISOString() })
    setOpen(false)
  }

  function pickRelative(period: string) {
    onChange({ kind: 'relative', period })
    setOpen(false)
    const focused = document.activeElement
    if (focused instanceof HTMLElement) focused.blur()
  }

  return (
    <div className="ctr" ref={wrapRef} aria-label={label}>
      <div className="ctr-presets" role="group" aria-label="Quick range">
        {CHART_PERIOD_PRESETS.map(p => (
          <button
            key={p.id}
            type="button"
            aria-pressed={activePreset === p.id}
            className={'ctr-preset' + (activePreset === p.id ? ' is-active' : '')}
            onMouseDown={preventFocusSteal}
            onClick={() => pickRelative(p.id)}
          >
            {p.label}
          </button>
        ))}
      </div>

      <button
        type="button"
        className={'ctr-trigger' + (open ? ' is-active' : '')}
        aria-expanded={open}
        aria-haspopup="dialog"
        onMouseDown={preventFocusSteal}
        onClick={() => setOpen(v => !v)}
      >
        <IconCalendar />
        Custom Range
      </button>

      <select
        className="input ctr-tz"
        aria-label="Time zone"
        value={timeZone}
        onMouseDown={e => e.stopPropagation()}
        onWheel={e => {
          e.preventDefault()
          e.stopPropagation()
        }}
        onChange={e => setTimeZone(e.target.value as ChartTimeZone)}
      >
        <option value="utc">UTC</option>
        <option value="local">Local</option>
      </select>

      {open && createPortal(
        <form
          ref={menuRef}
          className="ctr-popover"
          role="dialog"
          tabIndex={-1}
          aria-label="Custom time range"
          style={{ top: pos.top, left: pos.left }}
          onSubmit={applyAbsolute}
        >
          <div className="ctr-section-title">Custom range</div>
          <label className="ctr-field">
            From
            <input
              className="input ctr-datetime"
              type="datetime-local"
              step={60}
              aria-label="Range start"
              value={startLocal}
              onChange={e => setStartLocal(e.target.value)}
            />
          </label>
          <label className="ctr-field">
            To
            <input
              className="input ctr-datetime"
              type="datetime-local"
              step={60}
              aria-label="Range end"
              value={endLocal}
              onChange={e => setEndLocal(e.target.value)}
            />
          </label>
          <button type="submit" className="btn btn-primary ctr-apply">Apply</button>
        </form>,
        document.body,
      )}
    </div>
  )
}
