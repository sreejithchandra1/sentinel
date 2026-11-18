import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, Incident, Monitor, Site } from '../api'
import { ColGroup, ResizableTh, useColumnResize } from '../components/ColumnResize'
import ConfirmDialog from '../components/ConfirmDialog'
import CustomerFilter, { matchesCustomerFilter } from '../components/CustomerFilter'
import DashboardRail from '../components/DashboardRail'
import KebabMenu from '../components/KebabMenu'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import MonitorForm from './MonitorForm'
import SegmentedTabs from '../components/SegmentedTabs'
import Sparkline from '../components/Sparkline'
import StatusBadge, { badgeStatusFor, isPaused } from '../components/StatusBadge'
import TablePager, { isPageSize, PageSize } from '../components/TablePager'
import { ChevronDown, ChevronRight } from 'lucide-react'
import CheckTypePill from '../components/CheckTypePill'
import MonitorKindIcon, { monitorKindFor } from '../components/MonitorKindIcon'
import { useAuth } from '../context/AuthContext'
import { colors } from '../theme'
import {
  aggregateSiteStatus,
  childCheckLabel,
  groupMonitorsBySite,
  healthyCheckCounts,
  primarySiteMonitor,
} from '../utils/monitorSiteGroups'

const SITE_EXPAND_KEY = 'sentinel-site-expand'
const MONITOR_PAGE_SIZE_KEY = 'sentinel-monitors-page-size'

type MonitorListRow =
  | { kind: 'site'; site: Site; monitors: Monitor[] }
  | { kind: 'standalone'; monitor: Monitor }

function loadMonitorPageSize(): PageSize {
  try {
    const n = Number(sessionStorage.getItem(MONITOR_PAGE_SIZE_KEY))
    if (isPageSize(n)) return n
  } catch {
    /* ignore */
  }
  return 10
}

function loadExpandedSites(): Set<string> {
  try {
    const raw = sessionStorage.getItem(SITE_EXPAND_KEY)
    if (!raw) return new Set()
    const parsed = JSON.parse(raw) as string[]
    return new Set(Array.isArray(parsed) ? parsed : [])
  } catch {
    return new Set()
  }
}

function saveExpandedSites(set: Set<string>) {
  sessionStorage.setItem(SITE_EXPAND_KEY, JSON.stringify([...set]))
}

type StatusTab = 'all' | 'up' | 'degraded' | 'down' | 'paused'

type RowStats = {
  uptime_pct: number
  points: number[]
}

function greetingFor(hour: number): string {
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

function greetingName(name?: string, username?: string): string {
  const n = (name || '').trim()
  if (n) {
    // Prefer first name for the greeting.
    return n.split(/\s+/)[0]
  }
  if (!username) return 'there'
  if (username.includes('.') || username.includes('_') || username.includes(' ')) {
    return username.replace(/[._]/g, ' ').replace(/\b\w/g, c => c.toUpperCase()).split(/\s+/)[0]
  }
  return username.charAt(0).toUpperCase() + username.slice(1)
}

function timeAgo(iso: string): string {
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return new Date(iso).toLocaleDateString()
}

function monitorTarget(m: Monitor): string {
  if (m.type === 'heartbeat') return 'Heartbeat monitor'
  if (m.type === 'port') return `${m.url}:${m.port}`
  return m.url
}

export default function Monitors() {
  const { user, isAdmin, isPlatformAdmin } = useAuth()
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [sites, setSites] = useState<Site[]>([])
  const [expandedSites, setExpandedSites] = useState<Set<string>>(loadExpandedSites)
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [statsMap, setStatsMap] = useState<Record<string, RowStats>>({})
  const [tagFilter, setTagFilter] = useState('')
  const [statusTab, setStatusTab] = useState<StatusTab>('all')
  const [selectedCustomers, setSelectedCustomers] = useState<string[]>([])
  const [customers, setCustomers] = useState<{ id: string; name: string }[]>([])
  const [search, setSearch] = useState('')
  const [error, setError] = useState('')
  const [monitorForm, setMonitorForm] = useState<string | 'new' | null>(null)
  const [deleteMonitor, setDeleteMonitor] = useState<Monitor | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [togglingId, setTogglingId] = useState('')
  const [pageSize, setPageSize] = useState<PageSize>(loadMonitorPageSize)
  const [pageState, setPageState] = useState({ filterKey: '', page: 0 })
  const tableRef = useRef<HTMLTableElement>(null)
  // Site/Monitor is fluid (null); other cols stay compact under table-layout:fixed.
  const { widths, startResize, autoFit } = useColumnResize('monitors', 7, [
    null, 118, 128, 152, 88, 96, 52,
  ])

  async function load() {
    try {
      const [mons, incs, siteList] = await Promise.all([
        api.monitors({ tag: tagFilter || undefined }),
        api.incidents({ limit: 50, offset: 0 }),
        api.listSites(),
      ])
      setMonitors(mons)
      setSites(siteList)
      setIncidents(incs.items)
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load')
    }
  }

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(c => setCustomers(c.map(x => ({ id: x.id, name: x.name })))).catch(() => {})
  }, [isPlatformAdmin])

  useEffect(() => {
    load()
    const id = setInterval(load, 30000)
    return () => clearInterval(id)
  }, [tagFilter])

  const customerScoped = useMemo(
    () => monitors.filter(m => matchesCustomerFilter(m.tenant_id, selectedCustomers)),
    [monitors, selectedCustomers],
  )

  const q = search.trim().toLowerCase()
  const searched = useMemo(() => {
    if (!q) return customerScoped
    return customerScoped.filter(m => {
      const hay = [m.name, m.url, m.type, m.port != null ? String(m.port) : '', ...(m.tags || [])].join(' ').toLowerCase()
      return hay.includes(q)
    })
  }, [customerScoped, q])

  const filtered = useMemo(() => {
    if (statusTab === 'all') return searched
    if (statusTab === 'paused') return searched.filter(m => isPaused(m))
    return searched.filter(m => !isPaused(m) && m.last_status === statusTab)
  }, [searched, statusTab])

  const sitesScoped = useMemo(
    () => sites.filter(s => matchesCustomerFilter(s.tenant_id, selectedCustomers)),
    [sites, selectedCustomers],
  )

  const { groups: siteGroups, standalone: standaloneMonitors } = useMemo(
    () => groupMonitorsBySite(filtered, sitesScoped),
    [filtered, sitesScoped],
  )

  const listRows = useMemo<MonitorListRow[]>(() => [
    ...siteGroups.map(g => ({ kind: 'site' as const, site: g.site, monitors: g.monitors })),
    ...standaloneMonitors.map(m => ({ kind: 'standalone' as const, monitor: m })),
  ], [siteGroups, standaloneMonitors])

  const filterKey = `${q}\0${statusTab}\0${tagFilter}\0${selectedCustomers.join('\0')}`
  const page = pageState.filterKey === filterKey ? pageState.page : 0
  const setPage = (next: number) => setPageState({ filterKey, page: next })
  const totalPages = Math.max(1, Math.ceil(listRows.length / pageSize))
  const safePage = Math.min(page, Math.max(0, totalPages - 1))
  const pageRows = useMemo(
    () => listRows.slice(safePage * pageSize, safePage * pageSize + pageSize),
    [listRows, safePage, pageSize],
  )

  function changePageSize(size: PageSize) {
    setPageSize(size)
    setPage(0)
    try {
      sessionStorage.setItem(MONITOR_PAGE_SIZE_KEY, String(size))
    } catch {
      /* ignore */
    }
  }

  if (page !== safePage) setPage(safePage)

  function toggleSiteExpand(siteId: string) {
    setExpandedSites(prev => {
      const next = new Set(prev)
      if (next.has(siteId)) next.delete(siteId)
      else next.add(siteId)
      saveExpandedSites(next)
      return next
    })
  }

  useEffect(() => {
    if (!q) return
    setExpandedSites(prev => {
      const next = new Set(prev)
      for (const g of siteGroups) {
        if (g.monitors.length > 0) next.add(g.site.id)
      }
      saveExpandedSites(next)
      return next
    })
  }, [q, siteGroups])

  async function confirmDelete() {
    if (!deleteMonitor) return
    setDeleting(true)
    setError('')
    try {
      await api.deleteMonitor(deleteMonitor.id)
      setMonitors(prev => prev.filter(x => x.id !== deleteMonitor.id))
      setDeleteMonitor(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setDeleting(false)
    }
  }

  async function renameSite(site: Site) {
    const next = window.prompt('Site name', site.name)
    if (next == null) return
    const name = next.trim()
    if (!name || name === site.name) return
    setError('')
    try {
      await api.updateSite(site.id, { name, primary_host: site.primary_host })
      setSites(prev => prev.map(s => s.id === site.id ? { ...s, name } : s))
      setMonitors(prev => prev.map(m => m.site_id === site.id ? { ...m, name } : m))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not rename site')
    }
  }

  async function togglePause(m: Monitor) {
    setTogglingId(m.id)
    setError('')
    try {
      const updated = await api.setMonitorEnabled(m.id, isPaused(m))
      setMonitors(prev => prev.map(x => x.id === m.id ? { ...x, ...updated } : x))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update monitor')
    } finally {
      setTogglingId('')
    }
  }

  const active = searched.filter(m => !isPaused(m))
  const paused = searched.filter(m => isPaused(m)).length
  const up = active.filter(m => m.last_status === 'up').length
  const down = active.filter(m => m.last_status === 'down').length
  const degraded = active.filter(m => m.last_status === 'degraded').length

  const pageStatsKey = pageRows.flatMap(row =>
    row.kind === 'site' ? row.monitors.map(m => m.id) : [row.monitor.id],
  ).join(',')

  useEffect(() => {
    let cancelled = false
    const ids = [...new Set([
      ...filtered.slice(0, 40).map(m => m.id),
      ...(pageStatsKey ? pageStatsKey.split(',') : []),
    ])]
    if (ids.length === 0) {
      setStatsMap({})
      return
    }
    ;(async () => {
      try {
        const next = await api.monitorStatsSummary('30d', ids)
        if (!cancelled) setStatsMap(next)
      } catch {
        if (!cancelled) {
          const empty: Record<string, RowStats> = {}
          for (const id of ids) empty[id] = { uptime_pct: 0, points: [] }
          setStatsMap(empty)
        }
      }
    })()
    return () => { cancelled = true }
  }, [filtered.map(m => m.id).join(','), pageStatsKey])

  const overallUptime = useMemo(() => {
    const vals = filtered.map(m => statsMap[m.id]?.uptime_pct).filter((v): v is number => typeof v === 'number')
    if (vals.length === 0) return null
    return vals.reduce((a, b) => a + b, 0) / vals.length
  }, [filtered, statsMap])

  const allTags = [...new Set(monitors.flatMap(m => m.tags || []))].sort()
  const hour = new Date().getHours()
  const name = greetingName(user?.name, user?.username)

  const statusTabs: { id: StatusTab; label: string; count?: number }[] = [
    { id: 'all', label: 'All', count: searched.length },
    { id: 'up', label: 'Up', count: up },
    { id: 'degraded', label: 'Warning', count: degraded },
    { id: 'down', label: 'Down', count: down },
    { id: 'paused', label: 'Paused', count: paused },
  ]

  return (
    <div className="page" style={styles.page}>
      <ConfirmDialog
        open={!!deleteMonitor}
        title="Delete monitor?"
        message={
          deleteMonitor
            ? `Delete “${deleteMonitor.name}”? Check history for this monitor will be removed.`
            : 'Delete this monitor?'
        }
        confirmLabel="Delete"
        danger
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => { if (!deleting) setDeleteMonitor(null) }}
      />
      <PageHeader
        title={`${greetingFor(hour)}, ${name}`}
        subtitle="Here's what's happening with your monitors."
        actions={
          <>
            {isPlatformAdmin && (
              <CustomerFilter
                customers={customers}
                selectedIds={selectedCustomers}
                onChange={setSelectedCustomers}
              />
            )}
            {isAdmin && (
              <button type="button" className="btn btn-primary" onClick={() => setMonitorForm('new')}>+ Add Monitor</button>
            )}
          </>
        }
      />
      <div className="page-layout">
        <div className="page-layout-main">
          {searched.length > 0 && (
            <div className="kpi-strip">
              <div className="kpi-wide">
                <MetricCard
                  label="Overall Uptime"
                  value={overallUptime == null ? '—' : `${overallUptime.toFixed(2)}%`}
                  sub="Last 30 days · filtered monitors"
                />
              </div>
              <MetricCard label="Monitors" value={String(searched.length)} sub="Total monitors" />
              <MetricCard label="Healthy" value={String(up)} accent="green" />
              <MetricCard label="Warning" value={String(degraded)} accent="yellow" />
              <MetricCard label="Down" value={String(down)} accent="red" />
              <MetricCard label="Paused" value={String(paused)} />
            </div>
          )}

          {(monitors.length > 0 || search) && (
            <div className="toolbar-row">
              <input
                className="input search-field"
                value={search}
                onChange={e => setSearch(e.target.value)}
                placeholder="Search monitors…"
                aria-label="Search monitors"
              />
              <SegmentedTabs
                label="Filter by status"
                value={statusTab}
                onChange={id => setStatusTab(id as StatusTab)}
                tabs={statusTabs}
              />
              <div className="monitor-group-by">
                <span className="monitor-group-by-label">Group by:</span>
                <select className="input monitor-group-by-select" value="site" aria-label="Group monitors by">
                  <option value="site">Site</option>
                </select>
              </div>
            </div>
          )}

          {allTags.length > 0 && (
            <div className="chip-row" role="group" aria-label="Filter by tag">
              <button
                type="button"
                className="btn"
                style={{ fontSize: 14, minHeight: 36, ...(tagFilter === '' ? { background: colors.bgElevated } : {}) }}
                onClick={() => setTagFilter('')}
              >
                All tags
              </button>
              {allTags.map(tag => (
                <button
                  key={tag}
                  type="button"
                  className="btn"
                  style={{ fontSize: 14, minHeight: 36, ...(tagFilter === tag ? { background: colors.bgElevated } : {}) }}
                  onClick={() => setTagFilter(tag)}
                >
                  {tag}
                </button>
              ))}
            </div>
          )}

          {error && <div className="flash-error" role="alert">{error}</div>}

          {monitors.length === 0 ? (
            <div className="empty-state">
              <div style={{ fontWeight: 600, fontSize: 16, marginBottom: 8, color: colors.text }}>No monitors yet</div>
              <div style={{ marginBottom: 20 }}>
                Add your first website, port, SSL, or DNS monitor.
              </div>
              {isAdmin && <button type="button" className="btn btn-primary" onClick={() => setMonitorForm('new')}>Add Monitor</button>}
            </div>
          ) : filtered.length === 0 ? (
            <div className="empty-state">
              <div style={{ fontWeight: 600, marginBottom: 8, color: colors.text }}>No matches</div>
              <div>
                {search.trim()
                  ? `No monitors match “${search.trim()}”.`
                  : 'No monitors for the selected filters.'}
              </div>
            </div>
          ) : (
            <Panel padded={false}>
              <div className="data-table-wrap">
              <table ref={tableRef} className="data-table data-table--monitors">
                <ColGroup widths={widths} />
                <thead>
                  <tr>
                    <ResizableTh index={0} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Site / Monitor</ResizableTh>
                    <ResizableTh index={1} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Checks</ResizableTh>
                    <ResizableTh index={2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Status</ResizableTh>
                    <ResizableTh index={3} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Response Time</ResizableTh>
                    <ResizableTh index={4} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Uptime (30d)</ResizableTh>
                    <ResizableTh index={5} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Last Checked</ResizableTh>
                    <ResizableTh index={6} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                  </tr>
                </thead>
                <tbody>
                  {pageRows.map(row => {
                    if (row.kind === 'standalone') {
                      const m = row.monitor
                      return (
                        <MonitorTableRow
                          key={m.id}
                          m={m}
                          statsMap={statsMap}
                          standalone
                          label={m.name}
                          isAdmin={!!isAdmin}
                          togglingId={togglingId}
                          onTogglePause={togglePause}
                          onEdit={() => setMonitorForm(m.id)}
                          onDelete={() => setDeleteMonitor(m)}
                        />
                      )
                    }
                    const { site, monitors: children } = row
                    const expanded = expandedSites.has(site.id)
                    const agg = aggregateSiteStatus(children)
                    const counts = healthyCheckCounts(children)
                    const primary = primarySiteMonitor(children)
                    const primaryPaused = primary ? isPaused(primary) : true
                    const st = primary ? statsMap[primary.id] : undefined
                    const ms = primary?.latest_response_time_ms
                    const sparkColor = primaryPaused
                      ? colors.textMuted
                      : agg === 'down'
                        ? colors.red
                        : agg === 'degraded'
                          ? colors.yellow
                          : colors.green
                    const types = [...new Set(children.map(c => c.type))]
                    const visibleTypes = types.slice(0, 3)
                    const overflow = types.length - visibleTypes.length
                    return (
                      <Fragment key={site.id}>
                        <tr
                          className={agg === 'down' ? 'row-down' : agg === 'degraded' ? 'row-warn' : undefined}
                        >
                          <td>
                            <div className="monitor-name-cell">
                              <button
                                type="button"
                                className="monitor-tree-chevron"
                                aria-expanded={expanded}
                                aria-label={expanded ? 'Collapse site' : 'Expand site'}
                                onClick={() => toggleSiteExpand(site.id)}
                              >
                                {expanded
                                  ? <ChevronDown size={16} strokeWidth={2} />
                                  : <ChevronRight size={16} strokeWidth={2} />}
                              </button>
                              <span className="monitor-site-icon-wrap" aria-hidden>
                                <MonitorKindIcon kind="site" size={18} />
                              </span>
                              <div className="monitor-name-text">
                                <span style={styles.monitorName}>{site.name}</span>
                                <span style={styles.monitorUrl}>{site.primary_host || monitorTarget(primary || children[0])}</span>
                              </div>
                            </div>
                          </td>
                          <td>
                            <div style={styles.checkBadges}>
                              {visibleTypes.map(t => {
                                const sample = children.find(c => c.type === t)
                                return (
                                  <CheckTypePill key={t} type={t} url={sample?.url} />
                                )
                              })}
                              {overflow > 0 && <span style={styles.checkBadgeMuted}>+{overflow}</span>}
                            </div>
                          </td>
                          <td>
                            <StatusBadge status={badgeStatusFor('http', agg, agg !== 'paused')} />
                            <div style={styles.checkCount}>{counts.up} / {counts.total} checks</div>
                          </td>
                          <td>
                            <div style={styles.responseCell}>
                              <span className="num" style={{ fontWeight: 600 }}>
                                {typeof ms === 'number' ? `${ms}ms` : '—'}
                              </span>
                              {st?.points && st.points.length > 1 && (
                                <Sparkline values={st.points} color={sparkColor} />
                              )}
                            </div>
                          </td>
                          <td className="num">{st ? `${st.uptime_pct.toFixed(2)}%` : '—'}</td>
                          <td className="num" style={{ color: colors.textMuted }}>
                            {primary && !primaryPaused && primary.last_checked_at ? timeAgo(primary.last_checked_at) : primaryPaused ? 'Paused' : 'Waiting'}
                          </td>
                          <td className="col-actions">
                            {isAdmin && (
                              <KebabMenu>
                                {close => (
                                  <>
                                    {children[0] && (
                                      <button type="button" onClick={() => { close(); setMonitorForm(children[0].id) }}>
                                        Edit checks
                                      </button>
                                    )}
                                    <button type="button" onClick={() => { close(); renameSite(site) }}>
                                      Rename site
                                    </button>
                                  </>
                                )}
                              </KebabMenu>
                            )}
                          </td>
                        </tr>
                        {expanded && children.map((m, childIdx) => (
                          <MonitorTableRow
                            key={m.id}
                            m={m}
                            statsMap={statsMap}
                            indent
                            isLast={childIdx === children.length - 1}
                            label={childCheckLabel(m)}
                            isAdmin={!!isAdmin}
                            togglingId={togglingId}
                            onTogglePause={togglePause}
                            onEdit={() => setMonitorForm(m.id)}
                            onDelete={() => setDeleteMonitor(m)}
                          />
                        ))}
                      </Fragment>
                    )
                  })}
                </tbody>
              </table>
              </div>
              {listRows.length > 10 && (
                <TablePager
                  page={safePage}
                  pageSize={pageSize}
                  total={listRows.length}
                  onPageChange={setPage}
                  onPageSizeChange={changePageSize}
                />
              )}
            </Panel>
          )}
        </div>

        <DashboardRail incidents={incidents} uptimePct={overallUptime} />
      </div>

      {monitorForm != null && (
        <MonitorForm
          monitorId={monitorForm === 'new' ? undefined : monitorForm}
          onClose={() => setMonitorForm(null)}
          onSaved={() => {
            setMonitorForm(null)
            load()
          }}
        />
      )}
    </div>
  )
}

function MonitorTableRow({
  m,
  statsMap,
  indent,
  isLast,
  standalone,
  label,
  isAdmin,
  togglingId,
  onTogglePause,
  onEdit,
  onDelete,
}: {
  m: Monitor
  statsMap: Record<string, RowStats>
  indent?: boolean
  isLast?: boolean
  standalone?: boolean
  label: string
  isAdmin: boolean
  togglingId: string
  onTogglePause: (m: Monitor) => void
  onEdit: () => void
  onDelete: () => void
}) {
  const pausedRow = isPaused(m)
  const st = statsMap[m.id]
  const ms = m.latest_response_time_ms
  const sparkColor = pausedRow
    ? colors.textMuted
    : m.last_status === 'down'
      ? colors.red
      : m.last_status === 'degraded'
        ? colors.yellow
        : colors.green
  return (
    <tr
      className={pausedRow ? undefined : m.last_status === 'down' ? 'row-down' : m.last_status === 'degraded' ? 'row-warn' : undefined}
    >
      <td>
        <div className={`monitor-name-cell${indent ? ' monitor-name-cell--child' : ''}`}>
          {indent && (
            <div className={`monitor-tree-gutter${isLast ? ' is-last' : ''}`} aria-hidden />
          )}
          {!indent && (
            <span className="monitor-kind-icon-wrap" aria-hidden>
              <MonitorKindIcon kind={monitorKindFor(m.type, standalone)} size={16} />
            </span>
          )}
          {indent && (
            <span className="monitor-kind-icon-wrap" aria-hidden>
              <MonitorKindIcon kind={monitorKindFor(m.type)} size={16} />
            </span>
          )}
          <Link to={`/monitors/${m.id}`} style={styles.monitorLink}>
            <span style={styles.monitorName}>
              {label}
              {standalone && <span className="monitor-standalone-badge">Standalone</span>}
            </span>
            <span style={styles.monitorUrl}>{monitorTarget(m)}</span>
          </Link>
        </div>
      </td>
      <td><CheckTypePill type={m.type} url={m.url} /></td>
      <td>
        <StatusBadge status={badgeStatusFor(m.type, m.last_status, m.enabled)} />
      </td>
      <td>
        <div style={styles.responseCell}>
          <span className="num" style={{ fontWeight: 600 }}>
            {typeof ms === 'number' ? `${ms}ms` : '—'}
          </span>
          {st?.points && st.points.length > 1 && (
            <Sparkline values={st.points} color={sparkColor} />
          )}
        </div>
      </td>
      <td className="num">{st ? `${st.uptime_pct.toFixed(2)}%` : '—'}</td>
      <td className="num" style={{ color: colors.textMuted }}>
        {pausedRow ? 'Paused' : m.last_checked_at ? timeAgo(m.last_checked_at) : 'Waiting'}
      </td>
      <td className="col-actions">
        <KebabMenu>
          {close => (
            <>
              <Link to={`/monitors/${m.id}`} onClick={close}>View</Link>
              {isAdmin && (
                <>
                  <button type="button" disabled={togglingId === m.id} onClick={() => { close(); onTogglePause(m) }}>
                    {pausedRow ? 'Resume' : 'Pause'}
                  </button>
                  <button type="button" onClick={() => { close(); onEdit() }}>Edit</button>
                  <button type="button" className="kebab-danger" onClick={() => { close(); onDelete() }}>Delete</button>
                </>
              )}
            </>
          )}
        </KebabMenu>
      </td>
    </tr>
  )
}

const styles: Record<string, React.CSSProperties> = {
  page: { maxWidth: '100%' },
  th: {},
  monitorLink: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    color: 'inherit',
    textDecoration: 'none',
    minWidth: 0,
  },
  monitorName: {
    fontWeight: 600,
  },
  monitorUrl: {
    fontSize: 12,
    color: colors.textMuted,
  },
  responseCell: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
  },
  checkBadges: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 6,
    alignItems: 'center',
  },
  checkBadgeMuted: {
    fontSize: 11,
    fontWeight: 600,
    padding: '2px 8px',
    borderRadius: 6,
    background: colors.bgElevated,
    color: colors.textMuted,
  },
  checkCount: {
    fontSize: 11,
    color: colors.textMuted,
    marginTop: 4,
  },
}
