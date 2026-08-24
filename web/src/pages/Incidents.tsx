import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, Incident, Monitor } from '../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../components/ColumnResize'
import IncidentFilters, { IncidentFilterValues } from '../components/IncidentFilters'
import IncidentStatus from '../components/IncidentStatus'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import { colors } from '../theme'

const PAGE_SIZE = 20

const emptyFilters: IncidentFilterValues = {
  date: '',
  status: '',
  type: '',
  monitorId: '',
}

export default function Incidents() {
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [filters, setFilters] = useState<IncidentFilterValues>(emptyFilters)
  const [page, setPage] = useState(0)
  const [total, setTotal] = useState(0)
  const [openCount, setOpenCount] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('incidents', 6)
  const sortValue = useCallback((inc: Incident, key: string) => {
    if (key === 'monitor') return inc.monitor_name || inc.monitor_id
    if (key === 'type') return inc.type
    if (key === 'message') return inc.message || ''
    if (key === 'started') return inc.started_at
    if (key === 'resolved') return inc.resolved_at || ''
    if (key === 'status') return inc.resolved_at ? 'resolved' : 'open'
    return null
  }, [])
  const { sorted, header } = useTableSort(incidents, sortValue)

  useEffect(() => {
    api.monitors().then(setMonitors).catch(() => {})
  }, [])

  useEffect(() => {
    setPage(0)
  }, [filters.date, filters.status, filters.type, filters.monitorId])

  async function load() {
    try {
      setError('')
      setLoading(true)
      const [pageRes, openRes] = await Promise.all([
        api.incidents({
          date: filters.date || undefined,
          status: filters.status || undefined,
          type: filters.type || undefined,
          monitorId: filters.monitorId || undefined,
          limit: PAGE_SIZE,
          offset: page * PAGE_SIZE,
        }),
        api.incidents({
          status: 'open',
          monitorId: filters.monitorId || undefined,
          limit: 1,
          offset: 0,
        }),
      ])
      setIncidents(pageRes.items)
      setTotal(pageRes.total)
      setOpenCount(openRes.total)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    const id = setInterval(load, 30000)
    return () => clearInterval(id)
  }, [filters.date, filters.status, filters.type, filters.monitorId, page])

  const monitorOptions = monitors.map(m => ({ id: m.id, name: m.name }))
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const from = total === 0 ? 0 : page * PAGE_SIZE + 1
  const to = Math.min(total, (page + 1) * PAGE_SIZE)

  return (
    <div className="page">
      <PageHeader
        title="Incidents"
        subtitle={`${openCount} open · ${total} total${total > 0 ? ` · Showing ${from}–${to}` : ''}`}
      />

      <Panel style={{ padding: '14px 16px', marginBottom: 16 }}>
        <IncidentFilters
          value={filters}
          onChange={setFilters}
          monitors={monitorOptions}
          showMonitor
        />
      </Panel>

      {error && <div className="flash-error" role="alert">{error}</div>}

      {incidents.length === 0 && !loading ? (
        <div className="empty-state">
          {filters.date || filters.status || filters.type || filters.monitorId
            ? 'No incidents match these filters.'
            : 'No incidents recorded yet.'}
        </div>
      ) : (
        <Panel padded={false}>
          <div className="data-table-wrap">
          <table ref={tableRef} className="data-table">
            <ColGroup widths={widths} />
            <thead>
              <tr>
                <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('monitor')}>Monitor</ResizableTh>
                <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('type')}>Type</ResizableTh>
                <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('message')}>Message</ResizableTh>
                <ResizableTh index={3} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('started')}>Started</ResizableTh>
                <ResizableTh index={4} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('resolved')}>Resolved</ResizableTh>
                <ResizableTh index={5} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
              </tr>
            </thead>
            <tbody>
              {loading && incidents.length === 0 ? (
                <tr>
                  <td colSpan={6} style={{ padding: 24, color: colors.textMuted }}>Loading…</td>
                </tr>
              ) : (
                sorted.map(inc => {
                  const open = !inc.resolved_at
                  const warn = inc.type === 'slow' || inc.type === 'ssl_expiry'
                  return (
                  <tr key={inc.id} className={open ? (warn ? 'row-warn' : 'row-down') : undefined}>
                    <td>
                      <Link to={`/monitors/${inc.monitor_id}`} style={styles.link}>
                        {inc.monitor_name || inc.monitor_id}
                      </Link>
                    </td>
                    <td><span style={styles.type}>{inc.type}</span></td>
                    <td style={{ color: colors.textMuted }}>{inc.message || '—'}</td>
                    <td className="num">{new Date(inc.started_at).toLocaleString()}</td>
                    <td className="num" style={{ color: colors.textMuted }}>
                      {inc.resolved_at ? new Date(inc.resolved_at).toLocaleString() : '—'}
                    </td>
                    <td>
                      <IncidentStatus incident={inc} />
                    </td>
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
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  link: { color: colors.brand, textDecoration: 'none', fontWeight: 500 },
  type: { textTransform: 'uppercase', fontSize: 12, fontWeight: 700, color: colors.textMuted, letterSpacing: '0.04em' },
}
