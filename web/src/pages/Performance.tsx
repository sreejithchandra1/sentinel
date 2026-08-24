import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import { api, FleetPerformance, PerformanceHealth, PerformanceTarget, ServicePerformance } from '../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../components/ColumnResize'
import CustomerFilter, { matchesCustomerFilter } from '../components/CustomerFilter'
import ConfirmDialog from '../components/ConfirmDialog'
import KebabMenu from '../components/KebabMenu'
import PerformanceForm from './PerformanceForm'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import SegmentedTabs from '../components/SegmentedTabs'
import { useAuth } from '../context/AuthContext'
import { chartGridStroke, chartTick, chartTooltipLabel, chartTooltipStyle } from '../chartTheme'
import { colors, fonts } from '../theme'

type HealthTab = 'all' | 'good' | 'slow' | 'collecting'

const healthColors: Record<PerformanceHealth, string> = {
  good: colors.green,
  warning: colors.yellow,
  critical: colors.red,
  collecting: colors.textMuted,
}

function timeAgo(iso: string): string {
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return new Date(iso).toLocaleDateString()
}

function formatBucket(iso: string, period: string) {
  const d = new Date(iso)
  if (period === '24h') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function targetHealth(t: PerformanceTarget, svc?: ServicePerformance): PerformanceHealth {
  if (svc?.has_data) return svc.health
  const sla = t.slow_threshold_ms || 0
  const latest = t.latest_response_time_ms
  if (sla > 0 && latest != null && latest >= sla) return 'warning'
  if (t.last_checked_at) return 'good'
  return 'collecting'
}

function healthLabel(h: PerformanceHealth): string {
  if (h === 'good') return 'Within SLA'
  if (h === 'warning' || h === 'critical') return 'Slow'
  return 'Collecting'
}

export default function Performance() {
  const { isAdmin, isPlatformAdmin } = useAuth()
  const [targets, setTargets] = useState<PerformanceTarget[]>([])
  const [summary, setSummary] = useState<FleetPerformance | null>(null)
  const [period, setPeriod] = useState('24h')
  const [search, setSearch] = useState('')
  const [healthTab, setHealthTab] = useState<HealthTab>('all')
  const [selectedCustomers, setSelectedCustomers] = useState<string[]>([])
  const [customers, setCustomers] = useState<{ id: string; name: string }[]>([])
  const [error, setError] = useState('')
  const [deleteTargetRow, setDeleteTargetRow] = useState<PerformanceTarget | null>(null)
  const [targetForm, setTargetForm] = useState<string | 'new' | null>(null)
  const [deleting, setDeleting] = useState(false)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('performance', 7)

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(c => setCustomers(c.map(x => ({ id: x.id, name: x.name })))).catch(() => {})
  }, [isPlatformAdmin])

  useEffect(() => {
    Promise.all([
      api.performanceTargets().then(setTargets),
      api.performance(period).then(setSummary),
    ]).catch(err => setError(err instanceof Error ? err.message : 'Failed to load'))
    const id = setInterval(() => {
      api.performanceTargets().then(setTargets).catch(() => {})
      api.performance(period).then(setSummary).catch(() => {})
    }, 30000)
    return () => clearInterval(id)
  }, [period])

  const serviceById = useMemo(() => {
    const map: Record<string, ServicePerformance> = {}
    for (const s of summary?.services || []) map[s.service_id] = s
    return map
  }, [summary])

  const scopedTargets = useMemo(
    () => targets.filter(t => matchesCustomerFilter(t.tenant_id, selectedCustomers)),
    [targets, selectedCustomers],
  )

  const q = search.trim().toLowerCase()
  const searched = useMemo(() => {
    if (!q) return scopedTargets
    return scopedTargets.filter(t => `${t.name} ${t.url}`.toLowerCase().includes(q))
  }, [scopedTargets, q])

  const withHealth = useMemo(
    () => searched.map(t => ({ target: t, health: targetHealth(t, serviceById[t.id]) })),
    [searched, serviceById],
  )

  const filtered = useMemo(() => {
    if (healthTab === 'all') return withHealth
    if (healthTab === 'slow') return withHealth.filter(r => r.health === 'warning' || r.health === 'critical')
    return withHealth.filter(r => r.health === healthTab)
  }, [withHealth, healthTab])

  const sortValue = useCallback((row: (typeof filtered)[number], key: string) => {
    const t = row.target
    const svc = serviceById[t.id]
    if (key === 'name') return t.name
    if (key === 'status') return row.health
    if (key === 'latency') return t.latest_response_time_ms ?? null
    if (key === 'p95') return svc?.has_data ? svc.p95_ms : null
    if (key === 'sla') return t.slow_threshold_ms
    if (key === 'checked') return t.last_checked_at || ''
    return null
  }, [serviceById])
  const { sorted, header } = useTableSort(filtered, sortValue)

  const good = withHealth.filter(r => r.health === 'good').length
  const slow = withHealth.filter(r => r.health === 'warning' || r.health === 'critical').length
  const collecting = withHealth.filter(r => r.health === 'collecting').length

  const attention = (summary?.services || []).filter(s => {
    if (selectedCustomers.length > 0 && !scopedTargets.some(t => t.id === s.service_id)) return false
    return s.health === 'warning' || s.health === 'critical'
  })

  const timeline = (summary?.timeline || []).map(p => ({
    time: formatBucket(p.timestamp, period),
    avg: p.avg_ms,
  }))

  const slowRate = summary && summary.total_checks > 0
    ? ((summary.slow_checks / summary.total_checks) * 100).toFixed(1)
    : null

  const healthTabs: { id: HealthTab; label: string; count: number }[] = [
    { id: 'all', label: 'All', count: withHealth.length },
    { id: 'good', label: 'Within SLA', count: good },
    { id: 'slow', label: 'Slow', count: slow },
    { id: 'collecting', label: 'Collecting', count: collecting },
  ]

  async function confirmDelete() {
    if (!deleteTargetRow) return
    setDeleting(true)
    try {
      await api.deletePerformanceTarget(deleteTargetRow.id)
      setTargets(prev => prev.filter(t => t.id !== deleteTargetRow.id))
      setDeleteTargetRow(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="page" style={styles.page}>
      <ConfirmDialog
        open={!!deleteTargetRow}
        title="Delete performance target"
        message={deleteTargetRow ? `Delete “${deleteTargetRow.name}”? Latency history for this target will be removed.` : ''}
        confirmLabel="Delete"
        danger
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => { if (!deleting) setDeleteTargetRow(null) }}
      />
      <PageHeader
        title="Performance"
        subtitle="Measure website response time and latency — separate from uptime monitoring."
        actions={
          <>
            {isPlatformAdmin && (
              <CustomerFilter
                customers={customers}
                selectedIds={selectedCustomers}
                onChange={setSelectedCustomers}
              />
            )}
            <SegmentedTabs
              label="Latency period"
              value={period}
              onChange={setPeriod}
              tabs={[
                { id: '24h', label: '24h' },
                { id: '7d', label: '7d' },
                { id: '30d', label: '30d' },
              ]}
            />
            {isAdmin && (
              <button type="button" className="btn btn-primary" onClick={() => setTargetForm('new')}>+ Add Target</button>
            )}
          </>
        }
      />
      <div className="performance-layout">
        <div className="page-layout-main">
          {searched.length > 0 && (
            <div className="kpi-strip">
              <div className="kpi-wide">
                <MetricCard
                  label="Fleet P95"
                  value={summary?.p95_ms != null ? `${summary.p95_ms} ms` : '—'}
                  sub={`${period === '24h' ? 'Last 24 hours' : period === '7d' ? 'Last 7 days' : 'Last 30 days'}${slowRate != null ? ` · ${slowRate}% slow` : ''}`}
                />
              </div>
              <MetricCard label="Targets" value={String(searched.length)} sub="Total targets" />
              <MetricCard label="Within SLA" value={String(good)} accent="green" />
              <MetricCard label="Slow" value={String(slow)} accent="yellow" />
              <MetricCard label="Collecting" value={String(collecting)} />
            </div>
          )}

          {(targets.length > 0 || search) && (
            <div className="toolbar-row">
              <input
                className="input search-field"
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="Search targets…"
                aria-label="Search targets"
              />
              <SegmentedTabs
                label="Filter by health"
                value={healthTab}
                onChange={id => setHealthTab(id as HealthTab)}
                tabs={healthTabs}
              />
            </div>
          )}

          {error && <div className="flash-error" role="alert">{error}</div>}

          {targets.length === 0 ? (
            <div className="empty-state">
              <div style={{ fontWeight: 600, fontSize: 16, marginBottom: 8, color: colors.text }}>No performance targets yet</div>
              <div style={{ marginBottom: 20 }}>
                Add websites here to track response time and latency percentiles.
              </div>
              {isAdmin && <button type="button" className="btn btn-primary" onClick={() => setTargetForm('new')}>Add Target</button>}
            </div>
          ) : filtered.length === 0 ? (
            <div className="empty-state">
              <div style={{ fontWeight: 600, marginBottom: 8, color: colors.text }}>No matches</div>
              <div>
                {search.trim()
                  ? `No targets match “${search.trim()}”.`
                  : 'No targets for the selected filters.'}
              </div>
            </div>
          ) : (
            <Panel padded={false} className="data-table-wrap">
              <table ref={tableRef} className="data-table">
                <ColGroup widths={widths} />
                <thead>
                  <tr>
                    <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('name')}>Target</ResizableTh>
                    <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
                    <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('latency')}>Latency</ResizableTh>
                    <ResizableTh index={3} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('p95')}>P95</ResizableTh>
                    <ResizableTh index={4} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('sla')}>SLA</ResizableTh>
                    <ResizableTh index={5} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('checked')}>Last Checked</ResizableTh>
                    <ResizableTh index={6} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                  </tr>
                </thead>
                <tbody>
                  {sorted.map(({ target: t, health }) => {
                    const svc = serviceById[t.id]
                    const ms = t.latest_response_time_ms
                    return (
                      <tr key={t.id} className={health === 'warning' || health === 'critical' ? 'row-warn' : undefined}>
                        <td>
                          <Link to={`/performance/${t.id}`} style={styles.targetLink}>
                            <span style={styles.targetName}>{t.name}</span>
                            <span style={styles.targetUrl}>{t.url}</span>
                          </Link>
                        </td>
                        <td>
                          <span style={{
                            ...styles.healthBadge,
                            color: healthColors[health],
                            background: `${healthColors[health]}22`,
                          }}>
                            <span style={{ ...styles.healthDot, background: healthColors[health] }} />
                            {healthLabel(health)}
                          </span>
                        </td>
                        <td className="num">
                          <span style={{
                            fontWeight: 600,
                            color: health === 'good' || health === 'collecting' ? colors.text : colors.yellow,
                          }}>
                            {typeof ms === 'number' ? `${ms} ms` : '—'}
                          </span>
                        </td>
                        <td className="num">
                          {svc?.has_data ? `${svc.p95_ms} ms` : '—'}
                        </td>
                        <td className="num" style={{ color: colors.textMuted }}>
                          {t.slow_threshold_ms} ms
                        </td>
                        <td className="num" style={{ color: colors.textMuted }}>
                          {t.last_checked_at ? timeAgo(t.last_checked_at) : 'Waiting'}
                        </td>
                        <td className="col-actions">
                          <KebabMenu>
                            {close => (
                              <>
                                <Link to={`/performance/${t.id}`} onClick={close}>
                                  View
                                </Link>
                                {isAdmin && (
                                  <>
                                    <button
                                      type="button"
                                      onClick={() => {
                                        close()
                                        setTargetForm(t.id)
                                      }}
                                    >
                                      Edit
                                    </button>
                                    <button
                                      type="button"
                                      className="kebab-danger"
                                      onClick={() => {
                                        close()
                                        setDeleteTargetRow(t)
                                      }}
                                    >
                                      Delete
                                    </button>
                                  </>
                                )}
                              </>
                            )}
                          </KebabMenu>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </Panel>
          )}
        </div>

        <aside className="side-rail" aria-label="Latency summary">
          <Panel>
            <div style={styles.railLabel}>Fleet Latency</div>
            <div style={{ height: 140, marginTop: 8 }}>
              {timeline.length > 0 ? (
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={timeline}>
                    <defs>
                      <linearGradient id="perfRailGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor={colors.brand} stopOpacity={0.28} />
                        <stop offset="100%" stopColor={colors.brand} stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid stroke={chartGridStroke} vertical={false} />
                    <XAxis dataKey="time" hide tick={chartTick} />
                    <YAxis hide domain={['auto', 'auto']} tick={chartTick} />
                    <Tooltip
                      contentStyle={chartTooltipStyle}
                      labelStyle={chartTooltipLabel}
                      formatter={(v: number) => [`${v} ms`, 'Avg']}
                    />
                    <Area type="monotone" dataKey="avg" stroke={colors.brand} fill="url(#perfRailGrad)" strokeWidth={2} />
                  </AreaChart>
                </ResponsiveContainer>
              ) : (
                <div style={styles.railEmpty}>Waiting for data…</div>
              )}
            </div>
            <div style={styles.railMeta}>
              Avg {summary?.avg_ms != null ? `${summary.avg_ms} ms` : '—'}
              {slowRate != null ? ` · ${slowRate}% slow` : ''}
            </div>
          </Panel>

          <Panel>
            <div style={styles.railHeader}>
              <span style={styles.railLabel}>Latency Alerts</span>
            </div>
            {attention.length === 0 ? (
              <div style={{ ...styles.railEmpty, color: colors.green }}>All targets within SLA</div>
            ) : (
              <ul style={styles.railList}>
                {attention.slice(0, 6).map(s => (
                  <li key={s.service_id}>
                    <Link to={`/performance/${s.service_id}`} style={styles.alertRow}>
                      <span style={{ ...styles.alertDot, background: healthColors[s.health] }} />
                      <div style={{ minWidth: 0 }}>
                        <div style={styles.alertTitle}>{s.name}</div>
                        <div style={styles.alertMeta}>
                          P95 {s.p95_ms} ms · SLA {s.slow_threshold_ms} ms
                        </div>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>

          <Panel>
            <div style={styles.railLabel}>Fleet Avg</div>
            <div style={styles.railValue}>
              {summary?.avg_ms != null ? `${summary.avg_ms} ms` : '—'}
            </div>
            <div style={styles.railMeta}>
              {summary?.total_checks ?? 0} checks in selected period
            </div>
          </Panel>
        </aside>
      </div>

      {targetForm != null && (
        <PerformanceForm
          targetId={targetForm === 'new' ? undefined : targetForm}
          onClose={() => setTargetForm(null)}
          onSaved={() => {
            setTargetForm(null)
            api.performanceTargets().then(setTargets).catch(() => {})
            api.performance(period).then(setSummary).catch(() => {})
          }}
        />
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: { maxWidth: '100%' },
  metrics: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(120px, 1fr))',
    gap: 12,
    marginBottom: 20,
  },
  toolbar: {
    display: 'flex',
    gap: 12,
    alignItems: 'center',
    marginBottom: 16,
    flexWrap: 'wrap',
  },
  searchInput: { maxWidth: 280, width: '100%', height: 36, minHeight: 36 },
  actionsTd: {
    width: 52,
    paddingLeft: 4,
    paddingRight: 8,
    whiteSpace: 'nowrap',
    overflow: 'visible',
  },
  targetLink: { display: 'flex', flexDirection: 'column', gap: 2, textDecoration: 'none', color: colors.text, minWidth: 0 },
  targetName: { fontWeight: 600 },
  targetUrl: { fontSize: 12, color: colors.textMuted },
  healthBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    padding: '4px 8px',
    borderRadius: 6,
    fontSize: 12,
    fontWeight: 600,
  },
  healthDot: { width: 6, height: 6, borderRadius: 1, flexShrink: 0 },
  rail: {
    width: 300,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  },
  railHeader: { display: 'flex', justifyContent: 'space-between', alignItems: 'center' },
  railLabel: {
    fontSize: 12,
    fontWeight: 600,
    color: colors.textMuted,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  },
  railValue: {
    fontSize: 24,
    fontWeight: 600,
    marginTop: 8,
    letterSpacing: '-0.02em',
    fontFamily: fonts.mono,
    fontVariantNumeric: 'tabular-nums',
  },
  railMeta: { fontSize: 13, color: colors.textMuted, marginTop: 8, fontFamily: fonts.mono },
  railEmpty: { fontSize: 14, color: colors.textMuted, padding: '20px 0' },
  railList: { listStyle: 'none', margin: '8px 0 0', padding: 0 },
  alertRow: {
    display: 'flex',
    gap: 10,
    alignItems: 'flex-start',
    padding: '10px 0',
    color: colors.text,
    textDecoration: 'none',
    borderBottom: `1px solid ${colors.border}`,
  },
  alertDot: { width: 8, height: 8, borderRadius: 2, marginTop: 5, flexShrink: 0 },
  alertTitle: { fontWeight: 600, fontSize: 14 },
  alertMeta: { fontSize: 13, color: colors.textMuted, marginTop: 2, fontFamily: fonts.mono },
}

