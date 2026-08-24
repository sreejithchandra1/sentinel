import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, Customer, SLAMonitorRow, SLAReport } from '../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../components/ColumnResize'
import CustomerFilter, { matchesCustomerFilter } from '../components/CustomerFilter'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import { useAuth } from '../context/AuthContext'
import { colors } from '../theme'
import { formatDuration } from '../utils/duration'

function currentMonthValue(): string {
  const d = new Date()
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}`
}

function slaSummary(rows: SLAMonitorRow[]) {
  if (rows.length === 0) {
    return { availability: 100, mttr: null as number | null, incidents: 0, downtime: 0 }
  }
  const availability = rows.reduce((s, r) => s + r.availability_pct, 0) / rows.length
  const incidents = rows.reduce((s, r) => s + r.incident_count, 0)
  const downtime = rows.reduce((s, r) => s + r.downtime_seconds, 0)
  let mttrSum = 0
  let mttrN = 0
  for (const r of rows) {
    if (r.mttr_seconds != null) {
      mttrSum += r.mttr_seconds
      mttrN++
    }
  }
  return { availability, mttr: mttrN ? mttrSum / mttrN : null, incidents, downtime }
}

export default function SLAReportPage() {
  const { isPlatformAdmin } = useAuth()
  const [month, setMonth] = useState(currentMonthValue)
  const [selectedCustomers, setSelectedCustomers] = useState<string[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [report, setReport] = useState<SLAReport | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('sla-report', 5)
  const sortValue = useCallback((row: SLAReport['monitors'][number], key: string) => {
    if (key === 'name') return row.name
    if (key === 'incidents') return row.incident_count
    if (key === 'downtime') return row.downtime_seconds
    if (key === 'mttr') return row.mttr_seconds ?? -1
    if (key === 'avail') return row.availability_pct
    return null
  }, [])

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(setCustomers).catch(() => {})
  }, [isPlatformAdmin])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    api.slaReport(month)
      .then(rep => {
        if (!cancelled) setReport(rep)
      })
      .catch(err => {
        if (!cancelled) {
          setReport(null)
          setError(err instanceof Error ? err.message : 'Failed to load SLA report')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [month])

  const scoped = useMemo(() => {
    const rows = report?.monitors ?? []
    if (!isPlatformAdmin) return rows
    return rows.filter(r => matchesCustomerFilter(r.tenant_id, selectedCustomers))
  }, [report, selectedCustomers, isPlatformAdmin])
  const { sorted, header } = useTableSort(scoped, sortValue)
  const kpis = useMemo(() => slaSummary(scoped), [scoped])

  const periodLabel = useMemo(() => {
    if (!report) return ''
    const from = new Date(report.period_start)
    const to = new Date(report.period_end)
    return `${from.toLocaleString()} – ${to.toLocaleString()} UTC`
  }, [report])

  return (
    <div className="page">
      <PageHeader
        title="Availability SLA"
        subtitle="Downtime from DOWN incidents this month, minus maintenance windows. MTTR is the mean restore time for incidents that closed in the period."
      />

      <Panel style={{ padding: '14px 16px', marginBottom: 16, overflow: 'visible' }}>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'end' }}>
          <label style={styles.field}>
            <span style={styles.label}>Month</span>
            <input
              type="month"
              className="input"
              value={month}
              onChange={e => setMonth(e.target.value)}
            />
          </label>
          {isPlatformAdmin && (
            <div style={styles.field}>
              <span style={styles.label}>Customer</span>
              <CustomerFilter
                customers={customers}
                selectedIds={selectedCustomers}
                onChange={setSelectedCustomers}
              />
            </div>
          )}
        </div>
        {periodLabel && (
          <p style={{ margin: '10px 0 0', fontSize: 13, color: colors.textMuted }}>{periodLabel}</p>
        )}
      </Panel>

      {error && <div className="flash-error" role="alert">{error}</div>}

      <div className="grid-4" style={{ marginBottom: 20 }}>
        <MetricCard
          label="Availability"
          value={report ? `${kpis.availability.toFixed(3)}%` : '—'}
          accent="green"
        />
        <MetricCard
          label="MTTR"
          value={kpis.mttr != null ? formatDuration(kpis.mttr) : '—'}
          sub="Closed DOWN incidents"
        />
        <MetricCard
          label="Incidents"
          value={report ? String(kpis.incidents) : '—'}
          accent="yellow"
        />
        <MetricCard
          label="Downtime"
          value={report ? formatDuration(kpis.downtime) : '—'}
          sub="Maintenance excluded"
        />
      </div>

      <Panel padded={false}>
        <div className="data-table-wrap">
          <table ref={tableRef} className="data-table">
            <ColGroup widths={widths} />
            <thead>
              <tr>
                <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('name')}>Monitor</ResizableTh>
                <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('incidents')}>Incidents</ResizableTh>
                <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('downtime')}>Downtime</ResizableTh>
                <ResizableTh index={3} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('mttr')}>MTTR</ResizableTh>
                <ResizableTh index={4} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('avail')}>Availability</ResizableTh>
              </tr>
            </thead>
            <tbody>
              {loading && !report ? (
                <tr>
                  <td colSpan={5} style={{ padding: 24, color: colors.textMuted }}>Loading…</td>
                </tr>
              ) : sorted.length === 0 ? (
                <tr>
                  <td colSpan={5} style={{ padding: 24, color: colors.textMuted }}>No monitors in this scope.</td>
                </tr>
              ) : (
                sorted.map(row => (
                  <tr key={row.monitor_id}>
                    <td>
                      <Link to={`/monitors/${row.monitor_id}`} style={{ color: colors.brand, textDecoration: 'none', fontWeight: 500 }}>
                        {row.name}
                      </Link>
                    </td>
                    <td className="num">{row.incident_count}</td>
                    <td className="num">{formatDuration(row.downtime_seconds)}</td>
                    <td className="num">{row.mttr_seconds != null ? formatDuration(row.mttr_seconds) : '—'}</td>
                    <td className="num">{row.availability_pct.toFixed(3)}%</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Panel>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  field: { display: 'flex', flexDirection: 'column', gap: 6, minWidth: 180 },
  label: { fontSize: 12, fontWeight: 600, color: colors.textMuted, letterSpacing: '0.04em', textTransform: 'uppercase' },
}
