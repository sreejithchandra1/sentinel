import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Area, AreaChart, CartesianGrid, Legend, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import { api, PerformanceResult, PerformanceStats, PerformanceTarget } from '../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../components/ColumnResize'
import { useAuth } from '../context/AuthContext'
import DatePicker from '../components/DatePicker'
import PerformanceForm from './PerformanceForm'
import MetricCard from '../components/MetricCard'
import NextCheckCountdown from '../components/NextCheckCountdown'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import SegmentedTabs from '../components/SegmentedTabs'
import { chartGridStroke, chartTick, chartTooltipLabel, chartTooltipStyle } from '../chartTheme'
import { colors, radius } from '../theme'
import { useAdaptivePoll } from '../utils/poll'

export default function PerformanceDetail() {
  const { isAdmin } = useAuth()
  const { id } = useParams<{ id: string }>()
  const [target, setTarget] = useState<PerformanceTarget | null>(null)
  const [stats, setStats] = useState<PerformanceStats | null>(null)
  const [period, setPeriod] = useState('24h')
  const [editing, setEditing] = useState(false)
  const periodRef = useRef(period)
  periodRef.current = period

  const load = useCallback(async () => {
    if (!id) return null
    const [t, s] = await Promise.all([
      api.getPerformanceTarget(id),
      api.performanceStats(id, periodRef.current),
    ])
    setTarget(t)
    setStats(s)
    return t
  }, [id])

  const refreshRef = useAdaptivePoll(id, load, [period])

  if (!target) return <div style={{ color: colors.textMuted }}>Loading…</div>

  const perf = stats?.performance
  const chartData = (stats?.points || []).map(p => ({
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    ms: p.response_time_ms,
    dns: p.dns_ms ?? 0,
    tcp: p.tcp_ms ?? 0,
    tls: p.tls_ms ?? 0,
    ttfb: p.ttfb_ms ?? 0,
    download: Math.max(0, p.response_time_ms - (p.ttfb_ms ?? p.response_time_ms)),
  }))

  return (
    <div className="page">
      <PageHeader
        title={target.name}
        badges={
          target.last_status !== 'up' && target.last_status !== 'unknown' ? (
            <span style={{
              color: colors.yellow, background: colors.yellowDim,
              padding: '4px 8px', borderRadius: radius.sm, fontSize: 12, fontWeight: 600,
            }}>
              Slow
            </span>
          ) : undefined
        }
        subtitle={
          <>
            <Link to="/performance" style={styles.back}>← Performance</Link>
            <span style={{ marginLeft: 10 }}>{target.url}</span>
          </>
        }
        actions={
          <>
            <SegmentedTabs
              label="Chart period"
              value={period}
              onChange={setPeriod}
              tabs={[
                { id: '24h', label: '24h' },
                { id: '7d', label: '7d' },
                { id: '30d', label: '30d' },
              ]}
            />
            {isAdmin && <button type="button" className="btn" onClick={() => setEditing(true)}>Edit</button>}
          </>
        }
      />

      <NextCheckCountdown
        target={target}
        onDue={() => refreshRef.current?.()}
        disabledLabel="Paused"
      />

      {perf && perf.p50_ms > 0 && (
        <div className="grid-4" style={{ marginBottom: 24 }}>
          <MetricCard label="Avg" value={`${perf.avg_ms} ms`} accent="blue" />
          <MetricCard label="P50" value={`${perf.p50_ms} ms`} />
          <MetricCard label="P95" value={`${perf.p95_ms} ms`} accent={perf.p95_ms > target.slow_threshold_ms ? 'yellow' : 'default'} />
          <MetricCard label="P99" value={`${perf.p99_ms} ms`} />
        </div>
      )}

      <Panel style={{ marginBottom: 20 }}>
        <h3 className="panel-title">Response Time</h3>
        <div style={{ height: 300 }}>
          {chartData.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <defs>
                  <linearGradient id="svcGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={colors.brand} stopOpacity={0.28} />
                    <stop offset="100%" stopColor={colors.brand} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke={chartGridStroke} vertical={false} />
                <XAxis dataKey="time" tick={chartTick} axisLine={false} tickLine={false} />
                <YAxis unit="ms" tick={chartTick} axisLine={false} tickLine={false} />
                <Tooltip contentStyle={chartTooltipStyle} labelStyle={chartTooltipLabel} />
                <ReferenceLine y={target.slow_threshold_ms} stroke={colors.yellow} strokeDasharray="4 4" label={{ value: 'SLA', fill: colors.yellow, fontSize: 12 }} />
                <Area type="monotone" dataKey="ms" stroke={colors.brand} fill="url(#svcGrad)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div style={styles.empty}>Collecting latency data…</div>
          )}
        </div>
      </Panel>

      {chartData.some(p => p.dns > 0 || p.ttfb > 0) && (
        <Panel style={{ marginBottom: 20 }}>
          <h3 className="panel-title">Timing Breakdown</h3>
          <div style={{ height: 280 }}>
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <CartesianGrid stroke={chartGridStroke} vertical={false} />
                <XAxis dataKey="time" tick={chartTick} axisLine={false} tickLine={false} />
                <YAxis unit="ms" tick={chartTick} axisLine={false} tickLine={false} />
                <Tooltip contentStyle={chartTooltipStyle} labelStyle={chartTooltipLabel} />
                <Legend wrapperStyle={{ fontSize: 13, color: colors.textMuted }} />
                <Area type="monotone" dataKey="dns" stackId="1" stroke="#58a6ff" fill="#58a6ff" fillOpacity={0.6} name="DNS" />
                <Area type="monotone" dataKey="tcp" stackId="1" stroke="#bc8cff" fill="#bc8cff" fillOpacity={0.6} name="TCP" />
                <Area type="monotone" dataKey="tls" stackId="1" stroke="#f778ba" fill="#f778ba" fillOpacity={0.6} name="TLS" />
                <Area type="monotone" dataKey="ttfb" stackId="1" stroke={colors.brand} fill={colors.brand} fillOpacity={0.6} name="TTFB" />
                <Area type="monotone" dataKey="download" stackId="1" stroke={colors.yellow} fill={colors.yellow} fillOpacity={0.5} name="Download" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </Panel>
      )}

      {id && (
        <SLABreachLog targetId={id} slaMs={target.slow_threshold_ms} />
      )}

      {editing && id && (
        <PerformanceForm
          targetId={id}
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false)
            load()
          }}
        />
      )}
    </div>
  )
}

const PAGE_SIZE = 20

function SLABreachLog({ targetId, slaMs }: { targetId: string; slaMs: number }) {
  const [page, setPage] = useState(0)
  const [date, setDate] = useState('')
  const [items, setItems] = useState<PerformanceResult[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('sla-log', 6)
  const sortValue = useCallback((r: PerformanceResult, key: string) => {
    if (key === 'time') return r.checked_at
    if (key === 'status') return r.status
    if (key === 'total') return r.response_time_ms
    if (key === 'over') return Math.max(0, r.response_time_ms - slaMs)
    if (key === 'ttfb') return r.ttfb_ms ?? null
    if (key === 'dns') return r.dns_ms ?? null
    return null
  }, [slaMs])
  const { sorted, header } = useTableSort(items, sortValue)

  useEffect(() => {
    setPage(0)
  }, [targetId, date])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api.performanceResults(targetId, {
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      date: date || undefined,
      breaches: true,
    })
      .then(res => {
        if (cancelled) return
        setItems(res.items)
        setTotal(res.total)
      })
      .catch(() => {
        if (!cancelled) {
          setItems([])
          setTotal(0)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [targetId, page, date])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const from = total === 0 ? 0 : page * PAGE_SIZE + 1
  const to = Math.min(total, (page + 1) * PAGE_SIZE)

  return (
    <Panel>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16, gap: 12, flexWrap: 'wrap' }}>
        <div>
          <h3 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>SLA Breaches</h3>
          <div style={{ marginTop: 4, fontSize: 14, color: colors.textMuted }}>
            Probes slower than {slaMs > 0 ? `${slaMs} ms` : 'SLA'}
          </div>
        </div>
        <span style={{ fontSize: 14, color: colors.textMuted }}>
          {total === 0
            ? (date ? 'No breaches on this day' : 'No SLA breaches yet')
            : `Showing ${from}–${to} of ${total}`}
        </span>
      </div>

      <div style={styles.filterBar}>
        <DatePicker value={date} onChange={setDate} />
        {date && (
          <button type="button" className="btn" style={styles.resetBtn} onClick={() => setDate('')}>
            Clear date
          </button>
        )}
      </div>

      <div className="data-table-wrap">
        <table ref={tableRef} className="data-table">
          <ColGroup widths={widths} />
          <thead>
            <tr>
              <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('time')}>Time</ResizableTh>
              <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
              <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('total')}>Total</ResizableTh>
              <ResizableTh index={3} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('over')}>Over SLA</ResizableTh>
              <ResizableTh index={4} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('ttfb')}>TTFB</ResizableTh>
              <ResizableTh index={5} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('dns')}>DNS</ResizableTh>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={6} style={{ color: colors.textMuted }}>Loading…</td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={6} style={{ color: colors.textMuted }}>
                  {date ? 'No SLA breaches for this date.' : 'No probes have exceeded the SLA yet.'}
                </td>
              </tr>
            ) : (
              sorted.map(r => {
                const over = Math.max(0, r.response_time_ms - slaMs)
                return (
                  <tr key={r.id} className="row-warn">
                    <td className="num">{new Date(r.checked_at).toLocaleString()}</td>
                    <td style={{ color: colors.yellow, fontWeight: 600 }}>Slow</td>
                    <td className="num" style={{ color: colors.yellow }}>{r.response_time_ms} ms</td>
                    <td className="num" style={{ color: colors.yellow }}>+{over} ms</td>
                    <td className="num">{r.ttfb_ms != null ? `${r.ttfb_ms} ms` : '—'}</td>
                    <td className="num">{r.dns_ms != null ? `${r.dns_ms} ms` : '—'}</td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>

      {total > PAGE_SIZE && (
        <div className="table-pager">
          <button
            type="button"
            className="btn btn-sm"
            disabled={page <= 0 || loading}
            onClick={() => setPage(p => Math.max(0, p - 1))}
          >
            Previous
          </button>
          <span className="num" style={{ fontSize: 13, color: colors.textMuted }}>
            Page {page + 1} of {totalPages}
          </span>
          <button
            type="button"
            className="btn btn-sm"
            disabled={page + 1 >= totalPages || loading}
            onClick={() => setPage(p => p + 1)}
          >
            Next
          </button>
        </div>
      )}
    </Panel>
  )
}

const styles: Record<string, React.CSSProperties> = {
  back: { color: colors.textMuted, fontSize: 14, textDecoration: 'none' },
  empty: { color: colors.textMuted, textAlign: 'center', paddingTop: 120 },
  filterBar: {
    display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', marginBottom: 16,
  },
  resetBtn: { padding: '8px 12px', fontSize: 14, minHeight: 36 },
}
