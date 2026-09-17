import { FormEvent, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import {
  CHART_PERIOD_PRESETS,
  DEFAULT_CHART_RANGE,
  MAX_ZOOM_MS,
  MIN_ZOOM_MS,
  PERIOD_UNITS,
  autoGranularityLabel,
  formatPeriod,
  formatPeriodLabel,
  formatRangeLabel,
  fromDateTimeInput,
  parsePeriod,
  resolveChartRange,
  toDateTimeInput,
  zoomChartRange,
  type ChartRange,
  type ChartTimeZone,
  type PeriodUnit,
} from '../utils/period'
import { parseNumberInput } from '../utils/numberInput'

function IconClock() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="8.25" stroke="currentColor" strokeWidth="1.7" />
      <path d="M12 8v4.2l2.6 1.6" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function IconCalendar() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <rect x="3.5" y="5" width="17" height="15.5" rx="2.2" stroke="currentColor" strokeWidth="1.7" />
      <path d="M3.5 9.5h17M8 3.5v3.5M16 3.5v3.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  )
}

function IconChevron() {
  return (
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden>
      <path d="M3 4.5L6 8l3-3.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function IconZoomIn() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="11" cy="11" r="6.25" stroke="currentColor" strokeWidth="1.7" />
      <path d="M11 8.2v5.6M8.2 11h5.6M16 16.2l4 4" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  )
}

function IconZoomOut() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="11" cy="11" r="6.25" stroke="currentColor" strokeWidth="1.7" />
      <path d="M8.2 11h5.6M16 16.2l4 4" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
    </svg>
  )
}

function IconReset() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden>
      <path d="M4.5 12a7.5 7.5 0 1 0 2.1-5.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      <path d="M4.5 5.2v4.2h4.2" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" />
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
  const parsed = parsePeriod(value.kind === 'relative' ? value.period : '24h')
  const [amount, setAmount] = useState(String(parsed.amount))
  const [unit, setUnit] = useState<PeriodUnit>(parsed.unit)
  const resolved = resolveChartRange(value)
  const [startLocal, setStartLocal] = useState(() => toDateTimeInput(resolved.from, timeZone))
  const [endLocal, setEndLocal] = useState(() => toDateTimeInput(resolved.to, timeZone))
  const [open, setOpen] = useState(false)
  const [openFrom, setOpenFrom] = useState<'clock' | 'custom'>('clock')
  const wrapRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ top: 0, left: 0 })

  useEffect(() => {
    if (value.kind === 'relative') {
      const next = parsePeriod(value.period)
      setAmount(String(next.amount))
      setUnit(next.unit)
    }
    const next = resolveChartRange(value)
    setStartLocal(toDateTimeInput(next.from, timeZone))
    setEndLocal(toDateTimeInput(next.to, timeZone))
  }, [value, timeZone])

  function place() {
    const wrap = wrapRef.current
    const menu = menuRef.current
    if (!wrap || !menu) return
    const r = wrap.getBoundingClientRect()
    const width = menu.offsetWidth || 560
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
  const spanMs = resolved.spanMs
  const canZoomIn = spanMs > MIN_ZOOM_MS
  const canZoomOut = spanMs < MAX_ZOOM_MS
  const rangeLabel = value.kind === 'relative'
    ? (CHART_PERIOD_PRESETS.find(p => p.id === value.period)?.name || formatPeriodLabel(value.period))
    : formatRangeLabel(value)
  const customOpen = open && openFrom === 'custom'
  const clockOpen = open && openFrom === 'clock'

  function toggleOpen(from: 'clock' | 'custom') {
    setOpenFrom(from)
    setOpen(v => (openFrom === from ? !v : true))
  }

  function preventFocusSteal(e: { preventDefault: () => void }) {
    e.preventDefault()
  }

  function applyRelative(e?: FormEvent) {
    e?.preventDefault()
    const n = parseNumberInput(amount)
    if (n == null || n < 1) return
    onChange({ kind: 'relative', period: formatPeriod(n, unit) })
    setOpen(false)
  }

  function applyAbsolute(e?: FormEvent) {
    e?.preventDefault()
    const from = fromDateTimeInput(startLocal, timeZone)
    const to = fromDateTimeInput(endLocal, timeZone)
    if (!from || !to || new Date(to).getTime() <= new Date(from).getTime()) return
    let fromMs = new Date(from).getTime()
    let toMs = new Date(to).getTime()
    let span = toMs - fromMs
    span = Math.min(MAX_ZOOM_MS, Math.max(MIN_ZOOM_MS, span))
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
        className={'ctr-trigger' + (clockOpen ? ' is-active' : '')}
        aria-expanded={clockOpen}
        aria-haspopup="dialog"
        onMouseDown={preventFocusSteal}
        onClick={() => toggleOpen('clock')}
      >
        <IconClock />
        <span>{rangeLabel}</span>
        <IconChevron />
      </button>

      <span className="ctr-or">or</span>

      <button
        type="button"
        className={'ctr-trigger' + (customOpen ? ' is-active' : '')}
        aria-expanded={customOpen}
        onMouseDown={preventFocusSteal}
        onClick={() => toggleOpen('custom')}
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
        <div
          ref={menuRef}
          className="ctr-popover"
          role="dialog"
          tabIndex={-1}
          aria-label="Select time range"
          style={{ top: pos.top, left: pos.left }}
        >
          <div className="ctr-popover-col">
            <div className="ctr-section-title">Relative time</div>
            <div className="ctr-relative-list">
              {CHART_PERIOD_PRESETS.map(p => (
                <button
                  key={p.id}
                  type="button"
                  className={'ctr-relative-item' + (activePreset === p.id ? ' is-active' : '')}
                  onClick={() => pickRelative(p.id)}
                >
                  <span>{p.name}</span>
                  <span className="ctr-relative-id">{p.label}</span>
                </button>
              ))}
            </div>
            <form className="ctr-custom-relative" onSubmit={applyRelative}>
              <span>Last</span>
              <input
                className="input ctr-amount"
                type="number"
                min={1}
                step={1}
                inputMode="numeric"
                aria-label="Relative amount"
                value={amount}
                onChange={e => setAmount(e.target.value)}
              />
              <select
                className="input ctr-unit"
                aria-label="Relative unit"
                value={unit}
                onChange={e => setUnit(e.target.value as PeriodUnit)}
              >
                {PERIOD_UNITS.map(u => (
                  <option key={u.id} value={u.id}>{u.label}</option>
                ))}
              </select>
              <button type="submit" className="btn btn-sm">Apply</button>
            </form>
          </div>

          <form className="ctr-popover-col ctr-popover-custom" onSubmit={applyAbsolute}>
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

            <div className="ctr-section-title ctr-zoom-title">Quick zoom</div>
            <button
              type="button"
              className="ctr-zoom-btn"
              disabled={!canZoomIn}
              onClick={() => onChange(zoomChartRange(value, 0.5))}
            >
              <IconZoomIn /> Zoom In
            </button>
            <button
              type="button"
              className="ctr-zoom-btn"
              disabled={!canZoomOut}
              onClick={() => onChange(zoomChartRange(value, 2))}
            >
              <IconZoomOut /> Zoom Out
            </button>
            <button
              type="button"
              className="ctr-zoom-btn"
              onClick={() => { onChange(DEFAULT_CHART_RANGE); setOpen(false) }}
            >
              <IconReset /> Reset
            </button>
            <div className="ctr-granularity" title="Bucket size is chosen from the selected window so charts stay fast with thousands of points.">
              Auto granularity · {autoGranularityLabel(spanMs)}
            </div>
          </form>
        </div>,
        document.body,
      )}
    </div>
  )
}
