import { CSSProperties, FormEvent, ReactNode, useCallback, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import { api, Host, HostDisk, HostStats, Incident } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import IncidentStatus, { incidentLifecycleLabel, incidentStatusLabel } from '../components/IncidentStatus'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import SegmentedTabs from '../components/SegmentedTabs'
import StatusBadge, { isPaused } from '../components/StatusBadge'
import { useAuth } from '../context/AuthContext'
import { chartGridStroke, chartTick, chartTooltipLabel, chartTooltipStyle } from '../chartTheme'
import { colors, fonts } from '../theme'
import { formatDuration } from '../utils/duration'
import { useAdaptivePoll } from '../utils/poll'

type Tab = 'overview' | 'performance' | 'storage' | 'security' | 'services' | 'alerts'
type Band = 'Normal' | 'Warning' | 'Critical'
type HealthStatus = 'HEALTHY' | 'WARNING' | 'CRITICAL' | 'OFFLINE' | 'PAUSED' | 'WAITING'

function samplesFromMinutes(minutes: number, interval: number): number {
  const sec = Math.max(60, Math.round(minutes * 60))
  return Math.max(1, Math.ceil(sec / Math.max(interval, 30)))
}

function minutesFromSamples(after: number, interval: number): number {
  return Math.max(1, Math.round((after || 1) * Math.max(interval, 30) / 60))
}

function fmt(n?: number | null, digits = 1): string {
  if (n == null || Number.isNaN(n)) return '—'
  return n.toFixed(digits)
}

function hostBadge(h: Host): string {
  if (h.enabled === false) return 'paused'
  return h.status || 'pending'
}

function formatBucket(iso: string, period: string): string {
  const d = new Date(iso)
  if (period === '24h') return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

function timeAgo(iso?: string): string {
  if (!iso) return 'Never'
  const sec = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return new Date(iso).toLocaleString()
}

function usageColor(pct: number, warning = 80, critical = 90): string {
  if (pct >= critical) return colors.red
  if (pct >= warning) return colors.yellow
  return colors.green
}

function metricBand(value: number | null | undefined, warning: number, critical: number): Band {
  if (value == null || Number.isNaN(value)) return 'Normal'
  if (value >= critical) return 'Critical'
  if (value >= warning) return 'Warning'
  return 'Normal'
}

function bandAccent(b: Band): 'green' | 'yellow' | 'red' {
  if (b === 'Critical') return 'red'
  if (b === 'Warning') return 'yellow'
  return 'green'
}

function serviceOk(active?: string): boolean {
  return active === 'active' || active === 'activating' || active === 'reloading'
}

function hostHealth(host: Host, latest: HostStats['points'][number] | undefined, ncpu: number): { score: number; status: HealthStatus } {
  if (host.enabled === false) return { score: 100, status: 'PAUSED' }
  if (host.status === 'pending') return { score: 0, status: 'WAITING' }

  const loadPct = latest?.load1 != null ? (latest.load1 / Math.max(ncpu || 1, 1)) * 100 : null
  const loadCrit = host.alert_load_threshold > 0 && ncpu ? (host.alert_load_threshold / ncpu) * 100 : 90
  const gauges: Band[] = [
    metricBand(latest?.cpu_percent, host.alert_cpu_warning || 80, host.alert_cpu_threshold || 90),
    metricBand(latest?.mem_percent, host.alert_memory_warning || 80, host.alert_memory_threshold || 90),
    metricBand(latest?.swap_percent, host.alert_swap_warning || 80, host.alert_swap_threshold || 90),
    metricBand(latest?.disk_percent, host.alert_disk_warning || 80, host.alert_disk_threshold || 90),
    metricBand(loadPct, host.alert_load_warning || 80, loadCrit),
    metricBand(latest?.iowait_percent, host.alert_iowait_warning || 80, host.alert_iowait_threshold || 90),
  ]

  let score = 100
  let status: HealthStatus = 'HEALTHY'
  const raise = (next: HealthStatus) => {
    if (next === 'CRITICAL' || (next === 'WARNING' && status === 'HEALTHY')) status = next
  }
  for (const b of gauges) {
    if (b === 'Critical') { score -= 18; raise('CRITICAL') }
    else if (b === 'Warning') { score -= 8; raise('WARNING') }
  }
  const failed = (host.services || []).filter(name => {
    const st = (host.service_status || []).find(s => s.name === name)
    return !serviceOk(st?.active)
  }).length
  if (failed) { score -= Math.min(36, failed * 12); raise('CRITICAL') }
  if (host.reboot_required) { score -= 6; raise('WARNING') }
  // Auth bursts and root logins stay on the Security tab; they are not host health.
  if (host.status === 'offline') {
    score = Math.min(score, 25)
    status = 'OFFLINE'
  }
  return { score: Math.max(0, Math.min(100, Math.round(score))), status }
}

function healthColor(status: HealthStatus): string {
  if (status === 'HEALTHY') return colors.green
  if (status === 'WARNING' || status === 'WAITING' || status === 'PAUSED') return colors.yellow
  return colors.red
}

type ChartPoint = {
  time: string
  cpu: number | null
  mem: number | null
  swap: number | null
  disk: number | null
  iowait: number | null
  load: number | null
  loadPct: number | null
}

export default function HostDetail() {
  const { isAdmin } = useAuth()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [host, setHost] = useState<Host | null>(null)
  const [stats, setStats] = useState<HostStats | null>(null)
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [tab, setTab] = useState<Tab>('overview')
  const [period, setPeriod] = useState('24h')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [toggling, setToggling] = useState(false)
  const [command, setCommand] = useState('')
  const [copied, setCopied] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [servicesText, setServicesText] = useState('')
  const servicesHostId = useRef('')

  const load = useCallback(async () => {
    if (!id) return null
    const [h, s, inc] = await Promise.all([
      api.getHost(id),
      api.hostStats(id, period),
      api.incidents({ monitorId: id, limit: 50, offset: 0 }).catch(() => ({ items: [] as Incident[] })),
    ])
    setHost(h)
    setStats(s)
    setIncidents(inc.items || [])
    if (servicesHostId.current !== h.id) {
      servicesHostId.current = h.id
      setServicesText((h.services || []).join('\n'))
    }
    return { ...h, last_checked_at: h.last_seen_at }
  }, [id, period])

  useAdaptivePoll(id, load, [period])

  async function togglePause() {
    if (!host) return
    setToggling(true)
    setError('')
    try {
      setHost(await api.setHostEnabled(host.id, host.enabled === false))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update host')
    } finally {
      setToggling(false)
    }
  }

  async function regenerate() {
    if (!host) return
    setError('')
    try {
      const updated = await api.enrollHost(host.id)
      setHost(updated)
      setCommand(updated.install_command || '')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not create install command')
    }
  }

  async function saveMonitoring(e: FormEvent) {
    e.preventDefault()
    if (!host) return
    setSaving(true)
    setError('')
    try {
      const services = servicesText.split('\n').map(s => s.trim()).filter(Boolean)
      const updated = await api.updateHost(host.id, { ...host, services })
      setHost(updated)
      setServicesText((updated.services || []).join('\n'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  async function confirmDelete() {
    if (!host) return
    setDeleting(true)
    try {
      await api.deleteHost(host.id)
      navigate('/hosts')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
      setDeleting(false)
    }
  }

  if (!host) return <div style={{ color: colors.textMuted }}>Loading…</div>

  const latest = stats?.points?.[stats.points.length - 1]
  const ncpu = host.num_cpu || latest?.num_cpu || 0
  const loadPct = latest?.load1 != null ? (latest.load1 / Math.max(ncpu || 1, 1)) * 100 : null
  const loadCrit = host.alert_load_threshold > 0 && ncpu ? (host.alert_load_threshold / ncpu) * 100 : 90
  const chartData: ChartPoint[] = (stats?.points || []).map(p => {
    const cores = p.num_cpu || ncpu || 1
    return {
      time: formatBucket(p.timestamp, period),
      cpu: p.cpu_percent ?? null,
      mem: p.mem_percent ?? null,
      swap: p.swap_percent ?? null,
      disk: p.disk_percent ?? null,
      iowait: p.iowait_percent ?? null,
      load: p.load1 ?? null,
      loadPct: p.load1 != null ? (p.load1 / Math.max(cores, 1)) * 100 : null,
    }
  })
  const disks: HostDisk[] = (host.disks && host.disks.length > 0) ? host.disks : (latest?.disks || [])
  const health = hostHealth(host, latest, ncpu)
  const kpis: { label: string; value: string; band: Band }[] = [
    { label: 'CPU', value: `${fmt(latest?.cpu_percent)}%`, band: metricBand(latest?.cpu_percent, host.alert_cpu_warning || 80, host.alert_cpu_threshold || 90) },
    { label: 'Memory', value: `${fmt(latest?.mem_percent)}%`, band: metricBand(latest?.mem_percent, host.alert_memory_warning || 80, host.alert_memory_threshold || 90) },
    { label: 'Swap', value: `${fmt(latest?.swap_percent)}%`, band: metricBand(latest?.swap_percent, host.alert_swap_warning || 80, host.alert_swap_threshold || 90) },
    { label: 'Disk', value: `${fmt(latest?.disk_percent)}%`, band: metricBand(latest?.disk_percent, host.alert_disk_warning || 80, host.alert_disk_threshold || 90) },
    { label: 'Load', value: `${fmt(loadPct)}%`, band: metricBand(loadPct, host.alert_load_warning || 80, loadCrit) },
    { label: 'I/O Wait', value: `${fmt(latest?.iowait_percent)}%`, band: metricBand(latest?.iowait_percent, host.alert_iowait_warning || 80, host.alert_iowait_threshold || 90) },
  ]

  function setHostField<K extends keyof Host>(key: K, value: Host[K]) {
    setHost(h => h ? { ...h, [key]: value } : h)
  }

  return (
    <div className="page">
      <ConfirmDialog
        open={deleteOpen}
        title="Delete host?"
        message={`Delete “${host.name || host.hostname}”? Metric history will be removed.`}
        confirmLabel="Delete"
        danger
        busy={deleting}
        onConfirm={confirmDelete}
        onCancel={() => { if (!deleting) setDeleteOpen(false) }}
      />
      <PageHeader
        title={host.name || 'New host'}
        badges={<StatusBadge status={hostBadge(host)} />}
        subtitle={<Link to="/hosts" style={styles.back}>← Hosts</Link>}
        actions={
          isAdmin ? (
            <>
              <button type="button" className="btn" disabled={toggling} onClick={togglePause}>
                {isPaused(host) ? 'Resume' : 'Pause'}
              </button>
              <button type="button" className="btn" onClick={regenerate}>Install command</button>
              <button type="button" className="btn" onClick={() => setDeleteOpen(true)}>Delete</button>
            </>
          ) : undefined
        }
      />

      {error && <div className="flash-error" role="alert" style={{ marginBottom: 16 }}>{error}</div>}

      {command && (
        <Panel style={{ marginBottom: 20 }}>
          <h3 className="panel-title">Install command</h3>
          <p style={{ color: colors.textMuted, fontSize: 14, marginTop: 0 }}>
            Short-lived token. sudo is install-only; the agent runs as <code>sentinel-agent</code>.
          </p>
          <textarea className="input" readOnly rows={3} value={command} style={{ fontFamily: 'ui-monospace, monospace', fontSize: 13 }} />
          <button
            type="button"
            className="btn"
            style={{ marginTop: 8 }}
            onClick={async () => {
              await navigator.clipboard.writeText(command)
              setCopied(true)
              setTimeout(() => setCopied(false), 1500)
            }}
          >
            {copied ? 'Copied' : 'Copy'}
          </button>
        </Panel>
      )}

      {host.status === 'pending' && !command && (
        <Panel style={{ marginBottom: 20 }}>
          <p style={{ margin: 0, color: colors.textMuted }}>
            This host has not checked in. Generate an install command to enroll the agent.
          </p>
        </Panel>
      )}

      <Panel style={{ marginBottom: 16 }}>
        <div className="host-health">
          <HealthStat label="Host" value={host.name || host.hostname || '—'} />
          <HealthStat label="Status" value={health.status} color={healthColor(health.status)} />
          <HealthStat label="Health Score" value={`${health.score}/100`} color={healthColor(health.status)} />
          <HealthStat label="Last Check-in" value={timeAgo(host.last_seen_at)} />
        </div>
      </Panel>

      <div className="grid-6" style={{ marginBottom: 16 }}>
        {kpis.map(k => (
          <MetricCard key={k.label} label={k.label} value={k.value} sub={k.band} accent={bandAccent(k.band)} />
        ))}
      </div>

      <div className="detail-grid">
        <div>
          <div style={{ marginBottom: 16 }}>
            <SegmentedTabs
              label="Host sections"
              value={tab}
              onChange={id => setTab(id as Tab)}
              tabs={[
                { id: 'overview', label: 'Overview' },
                { id: 'performance', label: 'Performance' },
                { id: 'storage', label: 'Storage' },
                { id: 'security', label: 'Security' },
                { id: 'services', label: 'Services' },
                { id: 'alerts', label: 'Alert Rules' },
              ]}
            />
          </div>

          {tab === 'overview' && (
            <OverviewTab host={host} incidents={incidents} />
          )}
          {tab === 'performance' && (
            <PerformanceTab
              host={host}
              chartData={chartData}
              period={period}
              onPeriod={setPeriod}
              ncpu={ncpu}
              loadCrit={loadCrit}
            />
          )}
          {tab === 'storage' && (
            <StorageTab host={host} disks={disks} chartData={chartData} />
          )}
          {tab === 'security' && <SecurityTab host={host} />}
          {tab === 'services' && <ServicesTab host={host} />}
          {tab === 'alerts' && (
            isAdmin ? (
              <AlertRulesTab
                host={host}
                setHostField={setHostField}
                servicesText={servicesText}
                setServicesText={setServicesText}
                saving={saving}
                onSubmit={saveMonitoring}
              />
            ) : (
              <Panel><div style={styles.empty}>Only admins can edit alert rules.</div></Panel>
            )
          )}
        </div>

        <aside className="host-side" aria-label="Host information">
          <Panel style={{ marginBottom: 0 }}>
            <h3 className="panel-title">Host Information</h3>
            <InfoRow label="Hostname" value={host.hostname || '—'} />
            <InfoRow label="OS" value={host.os_version || host.os || '—'} />
            <InfoRow label="Kernel" value={host.kernel_version || '—'} />
            <InfoRow label="Agent" value={host.agent_version || '—'} />
            <InfoRow label="CPUs" value={ncpu ? String(ncpu) : '—'} />
            <InfoRow label="Arch" value={host.arch || '—'} />
            <InfoRow label="Uptime" value={(host.uptime_seconds ?? 0) > 0 ? formatDuration(host.uptime_seconds) : '—'} />
            <InfoRow label="Last check-in" value={timeAgo(host.last_seen_at)} />
            <InfoRow label="Health" value={`${health.score}/100`} color={healthColor(health.status)} />
            <InfoRow label="Status" value={health.status} color={healthColor(health.status)} />
          </Panel>
        </aside>
      </div>
    </div>
  )
}

function OverviewTab({
  host, incidents,
}: {
  host: Host
  incidents: Incident[]
}) {
  return (
    <>
      <SecurityEventCards host={host} />
      <IncidentTimeline incidents={incidents} />
    </>
  )
}

function PerformanceTab({
  host, chartData, period, onPeriod, ncpu, loadCrit,
}: {
  host: Host
  chartData: ChartPoint[]
  period: string
  onPeriod: (id: string) => void
  ncpu: number
  loadCrit: number
}) {
  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <SegmentedTabs
          label="Chart period"
          value={period}
          onChange={onPeriod}
          tabs={[{ id: '24h', label: '24h' }, { id: '7d', label: '7d' }, { id: '30d', label: '30d' }]}
        />
      </div>
      <div className="grid-2" style={{ marginBottom: 16, alignItems: 'stretch' }}>
        {host.collect_cpu && (
          <MetricChart title="CPU" data={chartData} dataKey="cpu" color={colors.brand} unit="%" domain={[0, 100]}
            warning={host.alert_cpu_enabled ? host.alert_cpu_warning : undefined}
            critical={host.alert_cpu_enabled ? host.alert_cpu_threshold : undefined} />
        )}
        {host.collect_memory && (
          <MetricChart title="Memory" data={chartData} dataKey="mem" color={colors.blue} unit="%" domain={[0, 100]}
            warning={host.alert_memory_enabled ? host.alert_memory_warning : undefined}
            critical={host.alert_memory_enabled ? host.alert_memory_threshold : undefined} />
        )}
        {host.collect_load && (
          <MetricChart title={ncpu ? `Load (% of ${ncpu} cores)` : 'Load'} data={chartData} dataKey="loadPct" color={colors.green} unit="%"
            warning={host.alert_load_enabled ? host.alert_load_warning : undefined}
            critical={host.alert_load_enabled ? loadCrit : undefined} />
        )}
        {host.collect_disk && (
          <MetricChart title="Disk" data={chartData} dataKey="disk" color={colors.yellow} unit="%" domain={[0, 100]}
            warning={host.alert_disk_enabled ? host.alert_disk_warning : undefined}
            critical={host.alert_disk_enabled ? host.alert_disk_threshold : undefined} />
        )}
        {host.collect_swap && (
          <MetricChart title="Swap" data={chartData} dataKey="swap" color="#bc8cff" unit="%" domain={[0, 100]}
            warning={host.alert_swap_enabled ? host.alert_swap_warning : undefined}
            critical={host.alert_swap_enabled ? host.alert_swap_threshold : undefined} />
        )}
        {host.collect_iowait && (
          <MetricChart title="I/O wait" data={chartData} dataKey="iowait" color={colors.yellow} unit="%" domain={[0, 100]}
            warning={host.alert_iowait_enabled ? host.alert_iowait_warning : undefined}
            critical={host.alert_iowait_enabled ? host.alert_iowait_threshold : undefined} />
        )}
      </div>
    </>
  )
}

function StorageTab({ host, disks, chartData }: { host: Host; disks: HostDisk[]; chartData: ChartPoint[] }) {
  return (
    <>
      <Panel style={{ marginBottom: 16 }}>
        <h3 className="panel-title">Disk Usage</h3>
        {disks.length === 0 ? (
          <div style={styles.empty}>Waiting for samples…</div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
            {disks.slice().sort((a, b) => b.percent - a.percent).map(d => (
              <div key={d.mount} style={{ display: 'grid', gridTemplateColumns: 'minmax(72px, 120px) 1fr auto', gap: 12, alignItems: 'center' }}>
                <code style={{ fontSize: 13, color: colors.text }}>{d.mount}</code>
                <div style={styles.barTrack}>
                  <div style={{
                    ...styles.barFill,
                    width: `${Math.min(100, d.percent)}%`,
                    background: usageColor(d.percent, host.alert_disk_warning, host.alert_disk_threshold),
                  }} />
                </div>
                <span style={{
                  fontWeight: 600,
                  fontVariantNumeric: 'tabular-nums',
                  fontFamily: fonts.mono,
                  color: usageColor(d.percent, host.alert_disk_warning, host.alert_disk_threshold),
                }}>{fmt(d.percent)}%</span>
              </div>
            ))}
          </div>
        )}
      </Panel>
      {host.collect_disk && (
        <MetricChart title="Worst disk" data={chartData} dataKey="disk" color={colors.yellow} unit="%" domain={[0, 100]}
          warning={host.alert_disk_enabled ? host.alert_disk_warning : undefined}
          critical={host.alert_disk_enabled ? host.alert_disk_threshold : undefined} />
      )}
    </>
  )
}

function SecurityTab({ host }: { host: Host }) {
  const sec = host.security
  return (
    <>
      <SecurityEventCards host={host} />
      <Panel>
        <h3 className="panel-title">Security details</h3>
        {sec && !sec.logs_readable && (
          <p style={{ color: colors.yellow, fontSize: 13, marginTop: 0 }}>
            Auth logs are not readable on this host yet. Re-run the installer so sentinel-agent is in the adm group.
          </p>
        )}
        <InfoRow label="Reboot required" value={host.reboot_required ? 'Yes' : 'No'} color={host.reboot_required ? colors.red : undefined} />
        <InfoRow label="sudo failures (5m)" value={sec ? String(sec.sudo_failed_5m) : '—'} />
        <InfoRow label="Last root login" value={sec?.last_root_login ? new Date(sec.last_root_login).toLocaleString() : '—'} />
        <InfoRow label="Kernel" value={host.kernel_version || '—'} />
        <InfoRow label="OS" value={host.os_version || host.os || '—'} />
      </Panel>
    </>
  )
}

function SecurityEventCards({ host }: { host: Host }) {
  const sec = host.security
  const cards = [
    { label: 'SSH Failures', value: sec ? String(sec.ssh_failed_5m) : '—', hot: (sec?.ssh_failed_5m || 0) > 0 },
    { label: 'Auth Failed', value: sec ? String(sec.auth_failed_5m) : '—', hot: (sec?.auth_failed_5m || 0) > 0 },
    { label: 'Root Logins', value: sec ? String(sec.root_logins_5m) : '—', hot: (sec?.root_logins_5m || 0) > 0 },
  ]
  return (
    <Panel style={{ marginBottom: 16 }}>
      <h3 className="panel-title">Security Events (Last 5 Minutes)</h3>
      <div className="grid-3">
        {cards.map(c => (
          <MetricCard key={c.label} label={c.label} value={c.value} accent={c.hot ? 'red' : 'green'} />
        ))}
      </div>
      <div style={{ marginTop: 14, color: colors.textMuted, fontSize: 14 }}>
        Last root login:{' '}
        <span style={{ color: colors.text, fontWeight: 600 }}>
          {sec?.last_root_login ? new Date(sec.last_root_login).toLocaleString() : '—'}
        </span>
      </div>
    </Panel>
  )
}

function ServicesTab({ host }: { host: Host }) {
  const watched = host.services || []
  const byName = Object.fromEntries((host.service_status || []).map(s => [s.name, s]))
  const healthy = watched.filter(name => serviceOk(byName[name]?.active)).length
  const failed = watched.length - healthy
  return (
    <>
      <div className="grid-3" style={{ marginBottom: 16 }}>
        <MetricCard label="Watched" value={String(watched.length)} />
        <MetricCard label="Healthy" value={String(healthy)} accent="green" />
        <MetricCard label="Failed" value={String(failed)} accent={failed ? 'red' : 'green'} />
      </div>
      {watched.length === 0 ? (
        <Panel><div style={styles.empty}>Add systemd units under Alert Rules to watch them.</div></Panel>
      ) : (
        <div className="grid-2">
          {watched.map(name => {
            const st = byName[name]
            const ok = serviceOk(st?.active)
            return (
              <div key={name} className="svc-card">
                <div>
                  <div style={{ fontWeight: 600 }}>{ok ? '✓' : '✕'} {name}</div>
                  <div style={{ color: colors.textMuted, fontSize: 13, marginTop: 4 }}>
                    {st?.active || 'unknown'}{st?.sub ? ` (${st.sub})` : ''}
                  </div>
                </div>
                <span style={{ color: ok ? colors.green : colors.red, fontWeight: 600 }}>{ok ? 'Healthy' : 'Failed'}</span>
              </div>
            )
          })}
        </div>
      )}
    </>
  )
}

function IncidentTimeline({ incidents }: { incidents: Incident[] }) {
  return (
    <Panel>
      <h3 className="panel-title">Incident timeline</h3>
      {incidents.length === 0 ? (
        <div style={styles.empty}>No incidents for this host.</div>
      ) : (
        <ul className="host-timeline">
          {incidents.map(inc => {
            const open = !inc.resolved_at
            return (
              <li key={inc.id}>
                <span className="host-timeline-dot" style={{ background: open ? (incidentStatusLabel(inc) === 'Warning' ? colors.yellow : colors.red) : colors.green }} />
                <div>
                  <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
                    <Link to={`/incidents/${inc.id}`} style={{ color: colors.text, fontWeight: 600, textDecoration: 'none' }}>
                      {inc.type.replace('host_', '').replace(/_/g, ' ')}
                    </Link>
                    <IncidentStatus incident={inc} />
                    <span style={{ color: colors.textMuted, fontSize: 13 }}>{timeAgo(inc.started_at)}</span>
                  </div>
                  <div style={{ color: colors.textMuted, fontSize: 13, marginTop: 4 }}>
                    {inc.message || incidentLifecycleLabel(inc)}
                  </div>
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}

function AlertRulesTab({
  host, setHostField, servicesText, setServicesText, saving, onSubmit,
}: {
  host: Host
  setHostField: <K extends keyof Host>(key: K, value: Host[K]) => void
  servicesText: string
  setServicesText: (v: string) => void
  saving: boolean
  onSubmit: (e: FormEvent) => void
}) {
  return (
    <form onSubmit={onSubmit}>
      <p style={{ color: colors.textMuted, fontSize: 14, marginTop: 0 }}>
        Warning defaults to 80, critical to 90. Load is compared to CPU cores. Alerts fire only after the value stays high for the duration.
      </p>
      <AlertFold title="CPU Alert" on={host.alert_cpu_enabled}>
        <MetricToggle
          collect={host.collect_cpu} onCollect={v => setHostField('collect_cpu', v)}
          alert={host.alert_cpu_enabled} onAlert={v => setHostField('alert_cpu_enabled', v)}
          warning={host.alert_cpu_warning} onWarning={v => setHostField('alert_cpu_warning', v)}
          threshold={host.alert_cpu_threshold} onThreshold={v => setHostField('alert_cpu_threshold', v)}
          minutes={minutesFromSamples(host.alert_cpu_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_cpu_after', samplesFromMinutes(m, host.interval_seconds))}
        />
      </AlertFold>
      <AlertFold title="Memory Alert" on={host.alert_memory_enabled}>
        <MetricToggle
          collect={host.collect_memory} onCollect={v => setHostField('collect_memory', v)}
          alert={host.alert_memory_enabled} onAlert={v => setHostField('alert_memory_enabled', v)}
          warning={host.alert_memory_warning} onWarning={v => setHostField('alert_memory_warning', v)}
          threshold={host.alert_memory_threshold} onThreshold={v => setHostField('alert_memory_threshold', v)}
          minutes={minutesFromSamples(host.alert_memory_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_memory_after', samplesFromMinutes(m, host.interval_seconds))}
        />
      </AlertFold>
      <AlertFold title="Swap Alert" on={host.alert_swap_enabled}>
        <MetricToggle
          collect={host.collect_swap} onCollect={v => setHostField('collect_swap', v)}
          alert={host.alert_swap_enabled} onAlert={v => setHostField('alert_swap_enabled', v)}
          warning={host.alert_swap_warning} onWarning={v => setHostField('alert_swap_warning', v)}
          threshold={host.alert_swap_threshold} onThreshold={v => setHostField('alert_swap_threshold', v)}
          minutes={minutesFromSamples(host.alert_swap_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_swap_after', samplesFromMinutes(m, host.interval_seconds))}
        />
      </AlertFold>
      <AlertFold title="Disk Alert" on={host.alert_disk_enabled}>
        <MetricToggle
          collect={host.collect_disk} onCollect={v => setHostField('collect_disk', v)}
          alert={host.alert_disk_enabled} onAlert={v => setHostField('alert_disk_enabled', v)}
          warning={host.alert_disk_warning} onWarning={v => setHostField('alert_disk_warning', v)}
          threshold={host.alert_disk_threshold} onThreshold={v => setHostField('alert_disk_threshold', v)}
          minutes={minutesFromSamples(host.alert_disk_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_disk_after', samplesFromMinutes(m, host.interval_seconds))}
        />
      </AlertFold>
      <AlertFold title="Load Alert" on={host.alert_load_enabled}>
        <MetricToggle
          collect={host.collect_load} onCollect={v => setHostField('collect_load', v)}
          alert={host.alert_load_enabled} onAlert={v => setHostField('alert_load_enabled', v)}
          warning={host.alert_load_warning} onWarning={v => setHostField('alert_load_warning', v)}
          threshold={host.alert_load_threshold} onThreshold={v => setHostField('alert_load_threshold', v)}
          minutes={minutesFromSamples(host.alert_load_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_load_after', samplesFromMinutes(m, host.interval_seconds))}
          thresholdHint="0 = 90% of CPU cores"
        />
      </AlertFold>
      <AlertFold title="I/O Wait Alert" on={host.alert_iowait_enabled}>
        <MetricToggle
          collect={host.collect_iowait} onCollect={v => setHostField('collect_iowait', v)}
          alert={host.alert_iowait_enabled} onAlert={v => setHostField('alert_iowait_enabled', v)}
          warning={host.alert_iowait_warning} onWarning={v => setHostField('alert_iowait_warning', v)}
          threshold={host.alert_iowait_threshold} onThreshold={v => setHostField('alert_iowait_threshold', v)}
          minutes={minutesFromSamples(host.alert_iowait_after, host.interval_seconds)}
          onMinutes={m => setHostField('alert_iowait_after', samplesFromMinutes(m, host.interval_seconds))}
        />
      </AlertFold>
      <AlertFold title="Security Alert" on={host.alert_auth_enabled || host.alert_root_login_enabled || host.alert_reboot_enabled}>
        <label style={styles.check}>
          <input type="checkbox" checked={host.collect_security} onChange={e => setHostField('collect_security', e.target.checked)} />
          Collect auth logs
        </label>
        <div style={{ ...styles.metricRow, borderBottom: 'none' }}>
          <label style={styles.check}>
            <input type="checkbox" checked={host.alert_auth_enabled} disabled={!host.collect_security} onChange={e => setHostField('alert_auth_enabled', e.target.checked)} />
            Auth burst
          </label>
          <label className="field" style={{ margin: 0, width: 180 }}>
            <span className="field-label">Failures / 5 min</span>
            <input type="number" min={1} className="input" disabled={!host.alert_auth_enabled} value={host.alert_auth_threshold} onChange={e => setHostField('alert_auth_threshold', Number(e.target.value) || 50)} />
          </label>
          <label style={styles.check}>
            <input type="checkbox" checked={host.alert_root_login_enabled} disabled={!host.collect_security} onChange={e => setHostField('alert_root_login_enabled', e.target.checked)} />
            Root login
          </label>
          <label style={styles.check}>
            <input type="checkbox" checked={host.alert_reboot_enabled} onChange={e => setHostField('alert_reboot_enabled', e.target.checked)} />
            Reboot required
          </label>
        </div>
      </AlertFold>
      <AlertFold title="Service Alert" on={host.alert_service_enabled}>
        <p style={{ color: colors.textMuted, fontSize: 13, marginTop: 12 }}>One systemd unit per line (e.g. nginx, sshd, postgresql).</p>
        <label style={styles.check}>
          <input type="checkbox" checked={host.collect_services} onChange={e => setHostField('collect_services', e.target.checked)} />
          Collect service status
        </label>
        <label style={{ ...styles.check, marginLeft: 16 }}>
          <input type="checkbox" checked={host.alert_service_enabled} disabled={!host.collect_services} onChange={e => setHostField('alert_service_enabled', e.target.checked)} />
          Alert if a watched service is not active
        </label>
        <textarea
          className="input"
          rows={4}
          disabled={!host.collect_services}
          value={servicesText}
          onChange={e => setServicesText(e.target.value)}
          placeholder={'nginx\nsshd'}
          style={{ marginTop: 12, fontFamily: 'ui-monospace, monospace', fontSize: 13 }}
        />
      </AlertFold>

      <Panel style={{ marginTop: 8 }}>
        <div className="grid-2">
          <label className="field">
            <span className="field-label">Report interval (seconds)</span>
            <input type="number" min={30} max={300} className="input" value={host.interval_seconds} onChange={e => setHostField('interval_seconds', Number(e.target.value) || 30)} />
          </label>
          <label className="field">
            <span className="field-label">Offline after missed reports</span>
            <input type="number" min={1} className="input" value={host.alert_after_failures} onChange={e => setHostField('alert_after_failures', Number(e.target.value) || 2)} />
          </label>
        </div>
        <div style={{ display: 'flex', gap: 16, marginTop: 16, flexWrap: 'wrap' }}>
          <label style={styles.check}>
            <input type="checkbox" checked={host.notify_email} onChange={e => setHostField('notify_email', e.target.checked)} />
            Email
          </label>
          <label style={styles.check}>
            <input type="checkbox" checked={host.notify_slack} onChange={e => setHostField('notify_slack', e.target.checked)} />
            Slack
          </label>
          <label style={styles.check}>
            <input type="checkbox" checked={host.notify_webhooks} onChange={e => setHostField('notify_webhooks', e.target.checked)} />
            Webhooks
          </label>
        </div>
        <div style={{ marginTop: 20 }}>
          <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? 'Saving…' : 'Save monitoring'}</button>
        </div>
      </Panel>
    </form>
  )
}

function AlertFold({ title, on, children }: { title: string; on: boolean; children: ReactNode }) {
  return (
    <details className="alert-fold">
      <summary>
        <span>{title}</span>
        <span style={{ color: on ? colors.yellow : colors.textMuted, fontSize: 13, fontWeight: 500, marginLeft: 'auto', marginRight: 8 }}>
          {on ? 'On' : 'Off'}
        </span>
      </summary>
      <div className="alert-fold-body">{children}</div>
    </details>
  )
}

function HealthStat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div>
      <div style={{ fontSize: 12, fontWeight: 600, color: colors.textMuted, letterSpacing: '0.04em', textTransform: 'uppercase', marginBottom: 6 }}>{label}</div>
      <div style={{ fontSize: 20, fontWeight: 600, color: color || colors.text, fontFamily: fonts.mono, fontVariantNumeric: 'tabular-nums' }}>{value}</div>
    </div>
  )
}

function InfoRow({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <div style={styles.dlRow}>
      <div style={styles.dt}>{label}</div>
      <div style={{ ...styles.dd, color: color || colors.text }} title={value}>{value}</div>
    </div>
  )
}

function MetricChart({
  title, data, dataKey, color, unit, warning, critical, domain,
}: {
  title: string
  data: ChartPoint[]
  dataKey: keyof Omit<ChartPoint, 'time'>
  color: string
  unit: string
  warning?: number
  critical?: number
  domain?: [number, number]
}) {
  const gradId = `host-grad-${dataKey}`
  const has = data.some(p => p[dataKey] != null)
  return (
    <Panel style={{ marginBottom: 0 }}>
      <h3 className="panel-title">{title}</h3>
      <div style={{ height: 240 }}>
        {has ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data}>
              <defs>
                <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={color} stopOpacity={0.28} />
                  <stop offset="100%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid stroke={chartGridStroke} vertical={false} />
              <XAxis dataKey="time" tick={chartTick} axisLine={false} tickLine={false} />
              <YAxis domain={domain} unit={unit} tick={chartTick} axisLine={false} tickLine={false} width={unit ? 52 : 40} />
              <Tooltip
                contentStyle={chartTooltipStyle}
                labelStyle={chartTooltipLabel}
                formatter={(value: number) => [`${fmt(value, dataKey === 'load' ? 2 : 1)}${unit}`, title]}
              />
              {warning != null && (
                <ReferenceLine y={warning} stroke={colors.yellow} strokeDasharray="4 4" label={{ value: 'Warn', fill: colors.yellow, fontSize: 12 }} />
              )}
              {critical != null && (
                <ReferenceLine y={critical} stroke={colors.red} strokeDasharray="4 4" label={{ value: 'Crit', fill: colors.red, fontSize: 12 }} />
              )}
              <Area type="monotone" dataKey={dataKey} stroke={color} fill={`url(#${gradId})`} strokeWidth={2} name={title} connectNulls />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div style={styles.empty}>Waiting for samples…</div>
        )}
      </div>
    </Panel>
  )
}

function MetricToggle({
  collect, onCollect, alert, onAlert, warning, onWarning, threshold, onThreshold, minutes, onMinutes, thresholdHint,
}: {
  collect: boolean
  onCollect: (v: boolean) => void
  alert: boolean
  onAlert: (v: boolean) => void
  warning: number
  onWarning: (v: number) => void
  threshold: number
  onThreshold: (v: number) => void
  minutes: number
  onMinutes: (v: number) => void
  thresholdHint?: string
}) {
  return (
    <div style={{ ...styles.metricRow, borderBottom: 'none', paddingTop: 16 }}>
      <label style={styles.check}>
        <input type="checkbox" checked={collect} onChange={e => onCollect(e.target.checked)} />
        Collect
      </label>
      <label style={styles.check}>
        <input type="checkbox" checked={alert} disabled={!collect} onChange={e => onAlert(e.target.checked)} />
        Alert
      </label>
      <label className="field" style={{ margin: 0, flex: 1 }}>
        <span className="field-label">Warning</span>
        <input type="number" className="input" disabled={!alert} value={warning} onChange={e => onWarning(Number(e.target.value))} />
      </label>
      <label className="field" style={{ margin: 0, flex: 1 }}>
        <span className="field-label">Critical</span>
        <input type="number" className="input" disabled={!alert} value={threshold} onChange={e => onThreshold(Number(e.target.value))} />
        {thresholdHint && <span style={{ fontSize: 12, color: colors.textMuted }}>{thresholdHint}</span>}
      </label>
      <label className="field" style={{ margin: 0, width: 140 }}>
        <span className="field-label">For (minutes)</span>
        <input type="number" min={1} className="input" disabled={!alert} value={minutes} onChange={e => onMinutes(Number(e.target.value) || 1)} />
      </label>
    </div>
  )
}

const styles: Record<string, CSSProperties> = {
  back: { color: colors.textMuted, textDecoration: 'none' },
  empty: { color: colors.textMuted, padding: 32, textAlign: 'center' },
  check: { display: 'inline-flex', alignItems: 'center', gap: 8, fontSize: 14 },
  metricRow: {
    display: 'flex',
    gap: 16,
    alignItems: 'center',
    flexWrap: 'wrap',
    padding: '12px 0',
    borderBottom: `1px solid ${colors.border}`,
  },
  barTrack: { height: 8, borderRadius: 99, background: colors.borderLight, overflow: 'hidden' },
  barFill: { height: '100%', borderRadius: 99 },
  dlRow: {
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    padding: '10px 0',
    borderBottom: `1px solid ${colors.border}`,
    fontSize: 14,
    minWidth: 0,
  },
  dt: { color: colors.textMuted, margin: 0, fontSize: 12, fontWeight: 600, letterSpacing: '0.04em', textTransform: 'uppercase' },
  dd: { margin: 0, fontWeight: 600, overflowWrap: 'anywhere', minWidth: 0 },
}
