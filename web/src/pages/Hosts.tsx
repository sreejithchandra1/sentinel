import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, Host } from '../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../components/ColumnResize'
import ConfirmDialog from '../components/ConfirmDialog'
import CustomerFilter, { matchesCustomerFilter } from '../components/CustomerFilter'
import KebabMenu from '../components/KebabMenu'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import SegmentedTabs from '../components/SegmentedTabs'
import StatusBadge, { isPaused } from '../components/StatusBadge'
import { useAuth } from '../context/AuthContext'
import HostForm from './HostForm'
import { colors } from '../theme'

type StatusTab = 'all' | 'online' | 'pending' | 'offline' | 'paused'

function timeAgo(iso?: string): string {
  if (!iso) return '—'
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return new Date(iso).toLocaleDateString()
}

function hostBadge(h: Host): string {
  if (h.enabled === false) return 'paused'
  return h.status || 'pending'
}

function alertsOn(h: Host): string {
  const on: string[] = []
  if (h.alert_cpu_enabled) on.push('CPU')
  if (h.alert_memory_enabled) on.push('Mem')
  if (h.alert_disk_enabled) on.push('Disk')
  if (h.alert_load_enabled) on.push('Load')
  return on.length ? on.join(', ') : 'Collecting only'
}

export default function Hosts() {
  const { isAdmin, isPlatformAdmin } = useAuth()
  const [hosts, setHosts] = useState<Host[]>([])
  const [search, setSearch] = useState('')
  const [statusTab, setStatusTab] = useState<StatusTab>('all')
  const [selectedCustomers, setSelectedCustomers] = useState<string[]>([])
  const [customers, setCustomers] = useState<{ id: string; name: string }[]>([])
  const [error, setError] = useState('')
  const [adding, setAdding] = useState(false)
  const [deleteHost, setDeleteHost] = useState<Host | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [togglingId, setTogglingId] = useState('')
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('hosts', 6)

  async function load() {
    try {
      setHosts(await api.hosts())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load hosts')
    }
  }

  useEffect(() => {
    load()
    const id = setInterval(load, 15000)
    return () => clearInterval(id)
  }, [])

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(c => setCustomers(c.map(x => ({ id: x.id, name: x.name })))).catch(() => {})
  }, [isPlatformAdmin])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return hosts.filter(h => {
      if (isPlatformAdmin && !matchesCustomerFilter(h.tenant_id, selectedCustomers)) return false
      if (q && !`${h.name} ${h.hostname}`.toLowerCase().includes(q)) return false
      const badge = hostBadge(h)
      if (statusTab !== 'all' && badge !== statusTab) return false
      return true
    })
  }, [hosts, search, statusTab, selectedCustomers, isPlatformAdmin])

  const online = hosts.filter(h => hostBadge(h) === 'online').length
  const pending = hosts.filter(h => hostBadge(h) === 'pending').length
  const offline = hosts.filter(h => hostBadge(h) === 'offline').length
  const paused = hosts.filter(h => hostBadge(h) === 'paused').length

  const sortValue = (h: Host, key: string) => {
    if (key === 'name') return h.name || h.hostname
    if (key === 'hostname') return h.hostname
    if (key === 'status') return hostBadge(h)
    if (key === 'seen') return h.last_seen_at || ''
    if (key === 'alerts') return alertsOn(h)
    return ''
  }
  const { sorted, header } = useTableSort(filtered, sortValue)

  async function confirmDelete() {
    if (!deleteHost) return
    setDeleting(true)
    try {
      await api.deleteHost(deleteHost.id)
      setDeleteHost(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setDeleting(false)
    }
  }

  async function toggle(h: Host) {
    setTogglingId(h.id)
    try {
      await api.setHostEnabled(h.id, h.enabled === false)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    } finally {
      setTogglingId('')
    }
  }

  const tabs: { id: StatusTab; label: string; count?: number }[] = [
    { id: 'all', label: 'All', count: filtered.length },
    { id: 'online', label: 'Online', count: online },
    { id: 'pending', label: 'Waiting', count: pending },
    { id: 'offline', label: 'Offline', count: offline },
    { id: 'paused', label: 'Paused', count: paused },
  ]

  return (
    <div className="page">
      <ConfirmDialog
        open={!!deleteHost}
        title="Delete host?"
        message={deleteHost ? `Delete “${deleteHost.name || deleteHost.hostname}”? Metric history will be removed.` : 'Delete this host?'}
        confirmLabel="Delete"
        danger
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => { if (!deleting) setDeleteHost(null) }}
      />
      {adding && <HostForm onClose={() => setAdding(false)} onSaved={load} />}
      <PageHeader
        title="Hosts"
        subtitle="Linux agents push metrics outbound. No extra ports on the host."
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
              <button type="button" className="btn btn-primary" onClick={() => setAdding(true)}>+ Add Host</button>
            )}
          </>
        }
      />

      {error && <div className="flash-error" role="alert">{error}</div>}

      {hosts.length > 0 && (
        <div className="kpi-strip">
          <MetricCard label="Hosts" value={String(hosts.length)} />
          <MetricCard label="Online" value={String(online)} accent="green" />
          <MetricCard label="Waiting" value={String(pending)} />
          <MetricCard label="Offline" value={String(offline)} accent="red" />
        </div>
      )}

      {(hosts.length > 0 || search) && (
        <div className="toolbar-row">
          <input
            className="input search-field"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search hosts…"
            aria-label="Search hosts"
          />
          <SegmentedTabs
            label="Filter by status"
            value={statusTab}
            onChange={id => setStatusTab(id as StatusTab)}
            tabs={tabs}
          />
        </div>
      )}

      {filtered.length === 0 ? (
        <div className="empty-state">
          {hosts.length === 0 ? 'No hosts yet. Add a host to get a one-line install command.' : 'No hosts match these filters.'}
        </div>
      ) : (
        <Panel padded={false}>
          <div className="data-table-wrap">
            <table ref={tableRef} className="data-table">
              <ColGroup widths={widths} />
              <thead>
                <tr>
                  <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('name')}>Name</ResizableTh>
                  <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('hostname')}>Hostname</ResizableTh>
                  <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
                  <ResizableTh index={3} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('seen')}>Last seen</ResizableTh>
                  <ResizableTh index={4} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('alerts')}>Monitoring</ResizableTh>
                  <ResizableTh index={5} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                </tr>
              </thead>
              <tbody>
                {sorted.map(h => (
                  <tr key={h.id}>
                    <td>
                      <Link to={`/hosts/${h.id}`} style={{ color: colors.text, fontWeight: 600 }}>{h.name || 'New host'}</Link>
                    </td>
                    <td style={{ color: colors.textMuted }}>{h.hostname || '—'}</td>
                    <td><StatusBadge status={hostBadge(h)} /></td>
                    <td className="num">{timeAgo(h.last_seen_at)}</td>
                    <td style={{ color: colors.textMuted }}>{alertsOn(h)}</td>
                    <td>
                      {isAdmin && (
                        <KebabMenu>
                          {close => (
                            <>
                              <button
                                type="button"
                                disabled={togglingId === h.id}
                                onClick={() => { close(); toggle(h) }}
                              >
                                {isPaused(h) ? 'Resume' : 'Pause'}
                              </button>
                              <button
                                type="button"
                                className="kebab-danger"
                                onClick={() => { close(); setDeleteHost(h) }}
                              >
                                Delete
                              </button>
                            </>
                          )}
                        </KebabMenu>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}
    </div>
  )
}
