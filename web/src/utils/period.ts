const PERIOD_RE = /^([1-9]\d*)([mhdwM])$/

export type PeriodUnit = 'm' | 'h' | 'd' | 'w' | 'M'

export type ParsedPeriod = {
  amount: number
  unit: PeriodUnit
}

export type ChartRange =
  | { kind: 'relative'; period: string }
  | { kind: 'absolute'; from: string; to: string }

export const PERIOD_UNITS: { id: PeriodUnit; label: string }[] = [
  { id: 'm', label: 'Minutes' },
  { id: 'h', label: 'Hours' },
  { id: 'd', label: 'Days' },
  { id: 'w', label: 'Weeks' },
  { id: 'M', label: 'Months' },
]

export type ChartTimeZone = 'utc' | 'local'

export const CHART_PERIOD_PRESETS: { id: string; label: string; name: string }[] = [
  { id: '15m', label: '15m', name: 'Last 15 minutes' },
  { id: '1h', label: '1h', name: 'Last 1 hour' },
  { id: '6h', label: '6h', name: 'Last 6 hours' },
  { id: '24h', label: '24h', name: 'Last 24 hours' },
  { id: '7d', label: '7d', name: 'Last 7 days' },
  { id: '30d', label: '30d', name: 'Last 30 days' },
  { id: '90d', label: '90d', name: 'Last 90 days' },
]

export const DEFAULT_CHART_RANGE: ChartRange = { kind: 'relative', period: '24h' }

export const MIN_RANGE_MS = 60 * 1000
export const MAX_RANGE_MS = 366 * 24 * 60 * 60 * 1000
/** Smallest window wheel / zoom buttons will use — matches the 15m preset. */
export const MIN_ZOOM_MS = 15 * 60 * 1000
/** Largest window wheel / zoom buttons will use — matches the 90d preset. */
export const MAX_ZOOM_MS = 90 * 24 * 60 * 60 * 1000

export function parsePeriod(period: string): ParsedPeriod {
  const m = PERIOD_RE.exec(period.trim())
  if (!m) return { amount: 24, unit: 'h' }
  return { amount: Number(m[1]), unit: m[2] as PeriodUnit }
}

export function formatPeriod(amount: number, unit: PeriodUnit): string {
  const n = Math.floor(amount)
  if (!Number.isFinite(n) || n < 1) return '24h'
  return `${n}${unit}`
}

export function periodDurationMs(period: string, now = Date.now()): number {
  const { amount, unit } = parsePeriod(period)
  if (unit === 'm') return amount * 60 * 1000
  if (unit === 'h') return amount * 60 * 60 * 1000
  if (unit === 'd') return amount * 24 * 60 * 60 * 1000
  if (unit === 'w') return amount * 7 * 24 * 60 * 60 * 1000
  const from = new Date(now)
  from.setMonth(from.getMonth() - amount)
  return Math.max(MIN_RANGE_MS, now - from.getTime())
}

export function resolveChartRange(range: ChartRange, now = Date.now()): { from: Date; to: Date; spanMs: number } {
  if (range.kind === 'absolute') {
    const from = new Date(range.from)
    const to = new Date(range.to)
    if (!Number.isNaN(from.getTime()) && !Number.isNaN(to.getTime()) && to.getTime() > from.getTime()) {
      return { from, to, spanMs: to.getTime() - from.getTime() }
    }
  }
  const period = range.kind === 'relative' ? range.period : '24h'
  const spanMs = Math.min(MAX_RANGE_MS, Math.max(MIN_RANGE_MS, periodDurationMs(period, now)))
  const to = new Date(now)
  return { from: new Date(now - spanMs), to, spanMs }
}

export function formatPeriodLabel(period: string): string {
  const { amount, unit } = parsePeriod(period)
  const noun = unitNoun(unit, amount)
  return amount === 1 ? `Last 1 ${noun}` : `Last ${amount} ${noun}`
}

function unitNoun(unit: PeriodUnit, amount: number): string {
  const plural = amount !== 1
  switch (unit) {
    case 'm': return plural ? 'minutes' : 'minute'
    case 'h': return plural ? 'hours' : 'hour'
    case 'd': return plural ? 'days' : 'day'
    case 'w': return plural ? 'weeks' : 'week'
    default: return plural ? 'months' : 'month'
  }
}

export function formatRangeLabel(range: ChartRange): string {
  if (range.kind === 'relative') return formatPeriodLabel(range.period)
  const { from, to } = resolveChartRange(range)
  const fmt = (d: Date) => d.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
  return `${fmt(from)} – ${fmt(to)}`
}

export function autoGranularityLabel(spanMs: number): string {
  if (spanMs <= 30 * 60 * 1000) return '1m'
  if (spanMs <= 2 * 60 * 60 * 1000) return '2m'
  if (spanMs <= 6 * 60 * 60 * 1000) return '5m'
  if (spanMs <= 24 * 60 * 60 * 1000) return '15m'
  if (spanMs <= 7 * 24 * 60 * 60 * 1000) return '1h'
  if (spanMs <= 30 * 24 * 60 * 60 * 1000) return '6h'
  return '1d'
}

export function formatChartTick(iso: string, range: ChartRange, timeZone: ChartTimeZone = 'utc'): string {
  const d = new Date(iso)
  const ms = resolveChartRange(range).spanMs
  const tz = timeZone === 'utc' ? { timeZone: 'UTC' as const } : {}
  if (ms <= 48 * 60 * 60 * 1000) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false, ...tz })
  }
  if (ms <= 14 * 24 * 60 * 60 * 1000) {
    return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false, ...tz })
  }
  return d.toLocaleDateString([], { month: 'short', day: 'numeric', ...tz })
}

export function chartTimeDomain(range: ChartRange, now = Date.now()): [number, number] {
  const { from, to } = resolveChartRange(range, now)
  return [from.getTime(), to.getTime()]
}

export function zoomChartRange(range: ChartRange, factor: number, now = Date.now(), anchorRatio = 0.5): ChartRange {
  const { from, spanMs } = resolveChartRange(range, now)
  const zoomingIn = factor < 1
  if (zoomingIn && spanMs <= MIN_ZOOM_MS) return range
  if (!zoomingIn && spanMs >= MAX_ZOOM_MS) return range

  let nextSpan = Math.round(spanMs * factor)
  nextSpan = Math.min(MAX_ZOOM_MS, Math.max(MIN_ZOOM_MS, nextSpan))
  if (Math.abs(nextSpan - spanMs) < 1000) return range

  const r = Math.min(1, Math.max(0, Number.isFinite(anchorRatio) ? anchorRatio : 0.5))
  const anchor = from.getTime() + spanMs * r
  let nextFrom = anchor - nextSpan * r
  let nextTo = nextFrom + nextSpan
  if (nextTo > now) {
    nextTo = now
    nextFrom = nextTo - nextSpan
  }
  if (nextFrom < now - MAX_ZOOM_MS) {
    nextFrom = now - MAX_ZOOM_MS
    nextTo = Math.min(now, nextFrom + nextSpan)
  }
  return {
    kind: 'absolute',
    from: new Date(nextFrom).toISOString(),
    to: new Date(nextTo).toISOString(),
  }
}

export function chartRangesEqual(a: ChartRange, b: ChartRange): boolean {
  if (a.kind !== b.kind) return false
  if (a.kind === 'relative' && b.kind === 'relative') return a.period === b.period
  if (a.kind === 'absolute' && b.kind === 'absolute') return a.from === b.from && a.to === b.to
  return false
}

export function emptyChartMessage(range: ChartRange): string {
  if (range.kind === 'absolute') return 'No data in this time range'
  return 'No data yet — waiting for first check'
}

export function statsQuery(range: ChartRange): { period?: string; from?: string; to?: string } {
  if (range.kind === 'absolute') {
    return { from: range.from, to: range.to }
  }
  return { period: range.period || '24h' }
}

export function toDateTimeInput(isoOrDate: string | Date, tz: ChartTimeZone = 'local'): string {
  const d = typeof isoOrDate === 'string' ? new Date(isoOrDate) : isoOrDate
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  const y = tz === 'utc' ? d.getUTCFullYear() : d.getFullYear()
  const m = (tz === 'utc' ? d.getUTCMonth() : d.getMonth()) + 1
  const day = tz === 'utc' ? d.getUTCDate() : d.getDate()
  const h = tz === 'utc' ? d.getUTCHours() : d.getHours()
  const min = tz === 'utc' ? d.getUTCMinutes() : d.getMinutes()
  return `${y}-${pad(m)}-${pad(day)}T${pad(h)}:${pad(min)}`
}

export function fromDateTimeInput(raw: string, tz: ChartTimeZone = 'local'): string | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(raw.trim())
  if (!m) return null
  const y = Number(m[1])
  const mo = Number(m[2]) - 1
  const d = Number(m[3])
  const h = Number(m[4])
  const min = Number(m[5])
  const date = tz === 'utc' ? new Date(Date.UTC(y, mo, d, h, min)) : new Date(y, mo, d, h, min)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString()
}
