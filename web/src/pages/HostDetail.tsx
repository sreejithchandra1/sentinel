import { FormEvent, useCallback, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import { api, Host, HostDisk, HostStats } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import MetricCard from '../components/MetricCard'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import SegmentedTabs from '../components/SegmentedTabs'
import StatusBadge, { isPaused } from '../components/StatusBadge'
import { useAuth } from '../context/AuthContext'
import { chartGridStroke, chartTick, chartTooltipLabel, chartTooltipStyle } from '../chartTheme'
import { colors } from '../theme'
import { useAdaptivePoll } from '../utils/poll'

function samplesFromMinutes(minutes: number, interval: number): number {
  const sec = Math.max(60, Math.round(minutes * 60))
  return Math.max(1, Math.ceil(sec / Math.max(interval, 30)))
}

function minutesFromSamples(after: number, interval: number): number {
  return Math.max(1, Math.round((after || 1) * Math.max(interval, 30) / 60))
}

function fmt(n?: number, digits = 1): string {
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
    const [h, s] = await Promise.all([api.getHost(id), api.hostStats(id, period)])
    setHost(h)
    setStats(s)
    if (servicesHostId.current !== h.id) {
      servicesHostId.current = h.id
      setServicesText((h.services || []).join('\n'))
    }
    return {
      ...h,
      last_checked_at: h.last_seen_at,
    }
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
  const anyChart = host.collect_cpu || host.collect_memory || host.collect_disk || host.collect_load || host.collect_swap || host.collect_iowait

  function setHostField<K extends keyof Host>(key: K, value: Host[K]) {
    setHost(h => h ? { ...h, [key]: value } : h)
  }

  const loadLabel = latest?.load1 != null && ncpu
    ? `${fmt(latest.load1, 2)} / ${ncpu} cores (${fmt((latest.load1 / ncpu) * 100)}%)`
    : fmt(latest?.load1, 2)

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
        subtitle={
          <>
            <Link to="/hosts" style={styles.back}>← Hosts</Link>
            <span style={{ marginLeft: 10 }}>{host.hostname || 'Waiting for agent'}</span>
            {host.os && <span style={{ marginLeft: 10, color: colors.textMuted }}>{host.os_version || host.os}{host.arch ? ` / ${host.arch}` : ''}</span>}
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
            {isAdmin && (
              <>
                <button type="button" className="btn" disabled={toggling} onClick={togglePause}>
                  {isPaused(host) ? 'Resume' : 'Pause'}
                </button>
                <button type="button" className="btn" onClick={regenerate}>Install command</button>
                <button type="button" className="btn" onClick={() => setDeleteOpen(true)}>Delete</button>
              </>
            )}
          </>
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

      <div className="grid-4" style={{ marginBottom: 24 }}>
        <MetricCard label="CPU" value={`${fmt(latest?.cpu_percent)}%`} />
        <MetricCard label="Memory" value={`${fmt(latest?.mem_percent)}%`} />
        <MetricCard label="Swap" value={`${fmt(latest?.swap_percent)}%`} />
        <MetricCard label="Disk (worst)" value={`${fmt(latest?.disk_percent)}%`} />
        <MetricCard label="Load" value={loadLabel} />
        <MetricCard label="I/O wait" value={`${fmt(latest?.iowait_percent)}%`} />
      </div>

      {host.collect_disk && (
        <Panel style={{ marginBottom: 20 }}>
          <h3 className="panel-title">Disk usage per mount</h3>
          {disks.length === 0 ? (
            <div style={styles.empty}>Waiting for samples…</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {disks.slice().sort((a, b) => b.percent - a.percent).map(d => (
                <div key={d.mount}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 14, marginBottom: 6 }}>
                    <code style={{ color: colors.text }}>{d.mount}</code>
                    <span style={{ color: usageColor(d.percent, host.alert_disk_warning, host.alert_disk_threshold), fontWeight: 600 }}>
                      {fmt(d.percent)}%
                    </span>
                  </div>
                  <div style={styles.barTrack}>
                    <div style={{
                      ...styles.barFill,
                      width: `${Math.min(100, d.percent)}%`,
                      background: usageColor(d.percent, host.alert_disk_warning, host.alert_disk_threshold),
                    }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      )}

      {!anyChart ? (
        <Panel style={{ marginBottom: 20 }}>
          <div style={styles.empty}>Turn on collection below to see metric graphs.</div>
        </Panel>
      ) : (
        <div className="grid-2" style={{ marginBottom: 20, alignItems: 'stretch' }}>
          {host.collect_cpu && (
            <MetricChart
              title="CPU"
              data={chartData}
              dataKey="cpu"
              color={colors.brand}
              unit="%"
              domain={[0, 100]}
              warning={host.alert_cpu_enabled ? host.alert_cpu_warning : undefined}
              critical={host.alert_cpu_enabled ? host.alert_cpu_threshold : undefined}
            />
          )}
          {host.collect_memory && (
            <MetricChart
              title="Memory"
              data={chartData}
              dataKey="mem"
              color={colors.blue}
              unit="%"
              domain={[0, 100]}
              warning={host.alert_memory_enabled ? host.alert_memory_warning : undefined}
              critical={host.alert_memory_enabled ? host.alert_memory_threshold : undefined}
            />
          )}
          {host.collect_swap && (
            <MetricChart
              title="Swap"
              data={chartData}
              dataKey="swap"
              color="#bc8cff"
              unit="%"
              domain={[0, 100]}
              warning={host.alert_swap_enabled ? host.alert_swap_warning : undefined}
              critical={host.alert_swap_enabled ? host.alert_swap_threshold : undefined}
            />
          )}
          {host.collect_load && (
            <MetricChart
              title={ncpu ? `Load (% of ${ncpu} cores)` : 'Load'}
              data={chartData}
              dataKey="loadPct"
              color={colors.green}
              unit="%"
              warning={host.alert_load_enabled ? host.alert_load_warning : undefined}
              critical={host.alert_load_enabled
                ? (host.alert_load_threshold > 0 && ncpu
                  ? (host.alert_load_threshold / ncpu) * 100
                  : 90)
                : undefined}
            />
          )}
          {host.collect_iowait && (
            <MetricChart
              title="Disk I/O wait"
              data={chartData}
              dataKey="iowait"
              color={colors.yellow}
              unit="%"
              domain={[0, 100]}
              warning={host.alert_iowait_enabled ? host.alert_iowait_warning : undefined}
              critical={host.alert_iowait_enabled ? host.alert_iowait_threshold : undefined}
            />
          )}
        </div>
      )}

      <div className="grid-2" style={{ marginBottom: 20, alignItems: 'stretch' }}>
        <SecurityPanel host={host} />
        <ServicesPanel host={host} />
      </div>

      {isAdmin && (
        <Panel>
          <h3 className="panel-title">Monitoring</h3>
          <p style={{ color: colors.textMuted, fontSize: 14, marginTop: 0 }}>
            Warning defaults to 80, critical to 90. Load is compared to CPU cores (load 4 on 4 cores = 100%). Alerts fire only after the value stays high for the duration.
          </p>
          <form onSubmit={saveMonitoring}>
            <MetricToggle
              label="CPU"
              collect={host.collect_cpu}
              onCollect={v => setHostField('collect_cpu', v)}
              alert={host.alert_cpu_enabled}
              onAlert={v => setHostField('alert_cpu_enabled', v)}
              warning={host.alert_cpu_warning}
              onWarning={v => setHostField('alert_cpu_warning', v)}
              threshold={host.alert_cpu_threshold}
              onThreshold={v => setHostField('alert_cpu_threshold', v)}
              minutes={minutesFromSamples(host.alert_cpu_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_cpu_after', samplesFromMinutes(m, host.interval_seconds))}
            />
            <MetricToggle
              label="Memory"
              collect={host.collect_memory}
              onCollect={v => setHostField('collect_memory', v)}
              alert={host.alert_memory_enabled}
              onAlert={v => setHostField('alert_memory_enabled', v)}
              warning={host.alert_memory_warning}
              onWarning={v => setHostField('alert_memory_warning', v)}
              threshold={host.alert_memory_threshold}
              onThreshold={v => setHostField('alert_memory_threshold', v)}
              minutes={minutesFromSamples(host.alert_memory_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_memory_after', samplesFromMinutes(m, host.interval_seconds))}
            />
            <MetricToggle
              label="Swap"
              collect={host.collect_swap}
              onCollect={v => setHostField('collect_swap', v)}
              alert={host.alert_swap_enabled}
              onAlert={v => setHostField('alert_swap_enabled', v)}
              warning={host.alert_swap_warning}
              onWarning={v => setHostField('alert_swap_warning', v)}
              threshold={host.alert_swap_threshold}
              onThreshold={v => setHostField('alert_swap_threshold', v)}
              minutes={minutesFromSamples(host.alert_swap_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_swap_after', samplesFromMinutes(m, host.interval_seconds))}
            />
            <MetricToggle
              label="Disk"
              collect={host.collect_disk}
              onCollect={v => setHostField('collect_disk', v)}
              alert={host.alert_disk_enabled}
              onAlert={v => setHostField('alert_disk_enabled', v)}
              warning={host.alert_disk_warning}
              onWarning={v => setHostField('alert_disk_warning', v)}
              threshold={host.alert_disk_threshold}
              onThreshold={v => setHostField('alert_disk_threshold', v)}
              minutes={minutesFromSamples(host.alert_disk_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_disk_after', samplesFromMinutes(m, host.interval_seconds))}
            />
            <MetricToggle
              label="Load"
              collect={host.collect_load}
              onCollect={v => setHostField('collect_load', v)}
              alert={host.alert_load_enabled}
              onAlert={v => setHostField('alert_load_enabled', v)}
              warning={host.alert_load_warning}
              onWarning={v => setHostField('alert_load_warning', v)}
              threshold={host.alert_load_threshold}
              onThreshold={v => setHostField('alert_load_threshold', v)}
              minutes={minutesFromSamples(host.alert_load_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_load_after', samplesFromMinutes(m, host.interval_seconds))}
              thresholdHint="0 = 90% of CPU cores"
            />
            <MetricToggle
              label="I/O wait"
              collect={host.collect_iowait}
              onCollect={v => setHostField('collect_iowait', v)}
              alert={host.alert_iowait_enabled}
              onAlert={v => setHostField('alert_iowait_enabled', v)}
              warning={host.alert_iowait_warning}
              onWarning={v => setHostField('alert_iowait_warning', v)}
              threshold={host.alert_iowait_threshold}
              onThreshold={v => setHostField('alert_iowait_threshold', v)}
              minutes={minutesFromSamples(host.alert_iowait_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_iowait_after', samplesFromMinutes(m, host.interval_seconds))}
            />

            <h3 className="panel-title" style={{ marginTop: 24 }}>Security alerts</h3>
            <label style={styles.check}>
              <input type="checkbox" checked={host.collect_security} onChange={e => setHostField('collect_security', e.target.checked)} />
              Collect auth logs (read-only; agent is in the adm/systemd-journal groups)
            </label>
            <div style={styles.metricRow}>
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

            <h3 className="panel-title" style={{ marginTop: 24 }}>Services</h3>
            <p style={{ color: colors.textMuted, fontSize: 13, marginTop: 0 }}>One systemd unit per line (e.g. nginx, sshd, postgresql).</p>
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
              placeholder="nginx&#10;sshd"
              style={{ marginTop: 12, fontFamily: 'ui-monospace, monospace', fontSize: 13 }}
            />

            <div className="grid-2" style={{ marginTop: 16 }}>
              <label className="field">
                <span className="field-label">Report interval (seconds)</span>
                <input
                  type="number"
                  min={30}
                  max={300}
                  className="input"
                  value={host.interval_seconds}
                  onChange={e => setHostField('interval_seconds', Number(e.target.value) || 30)}
                />
              </label>
              <label className="field">
                <span className="field-label">Offline after missed reports</span>
                <input
                  type="number"
                  min={1}
                  className="input"
                  value={host.alert_after_failures}
                  onChange={e => setHostField('alert_after_failures', Number(e.target.value) || 2)}
                />
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
          </form>
        </Panel>
      )}
    </div>
  )
}

function SecurityPanel({ host }: { host: Host }) {
  const sec = host.security
  const rows: [string, string][] = [
    ['Last check-in', timeAgo(host.last_seen_at)],
    ['Agent version', host.agent_version || '—'],
    ['OS version', host.os_version || host.os || '—'],
    ['Kernel', host.kernel_version || '—'],
    ['Reboot required', host.reboot_required ? 'Yes' : 'No'],
    ['SSH failed (5m)', sec ? String(sec.ssh_failed_5m) : '—'],
    ['sudo failures (5m)', sec ? String(sec.sudo_failed_5m) : '—'],
    ['Auth failures (5m)', sec ? String(sec.auth_failed_5m) : '—'],
    ['Root logins (5m)', sec ? String(sec.root_logins_5m) : '—'],
  ]
  if (sec?.last_root_login) rows.push(['Last root login', new Date(sec.last_root_login).toLocaleString()])
  return (
    <Panel style={{ marginBottom: 0 }}>
      <h3 className="panel-title">Host security</h3>
      {sec && !sec.logs_readable && (
        <p style={{ color: colors.yellow, fontSize: 13, marginTop: 0 }}>Auth logs are not readable on this host yet. Re-run the installer so sentinel-agent is in the adm group.</p>
      )}
      <dl style={styles.dl}>
        {rows.map(([k, v]) => (
          <div key={k} style={styles.dlRow}>
            <dt style={styles.dt}>{k}</dt>
            <dd style={{
              ...styles.dd,
              color: (k === 'Reboot required' && host.reboot_required) || (k.startsWith('Root') && sec && sec.root_logins_5m > 0)
                ? colors.red
                : colors.text,
            }}>{v}</dd>
          </div>
        ))}
      </dl>
    </Panel>
  )
}

function ServicesPanel({ host }: { host: Host }) {
  const watched = host.services || []
  const status = host.service_status || []
  const byName = Object.fromEntries(status.map(s => [s.name, s]))
  return (
    <Panel style={{ marginBottom: 0 }}>
      <h3 className="panel-title">Services</h3>
      {watched.length === 0 ? (
        <div style={styles.empty}>Add systemd units under Monitoring to watch them.</div>
      ) : (
        <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
          {watched.map(name => {
            const st = byName[name]
            const active = st?.active || 'unknown'
            const ok = active === 'active' || active === 'activating'
            return (
              <li key={name} style={styles.svcRow}>
                <code>{name}</code>
                <span style={{ color: ok ? colors.green : colors.red, fontWeight: 600 }}>
                  {active}{st?.sub ? ` (${st.sub})` : ''}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </Panel>
  )
}

function MetricChart({
  title,
  data,
  dataKey,
  color,
  unit,
  warning,
  critical,
  domain,
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
              <YAxis
                domain={domain}
                unit={unit}
                tick={chartTick}
                axisLine={false}
                tickLine={false}
                width={unit ? 52 : 40}
              />
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
              <Area
                type="monotone"
                dataKey={dataKey}
                stroke={color}
                fill={`url(#${gradId})`}
                strokeWidth={2}
                name={title}
                connectNulls
              />
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
  label, collect, onCollect, alert, onAlert, warning, onWarning, threshold, onThreshold, minutes, onMinutes, thresholdHint,
}: {
  label: string
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
    <div style={styles.metricRow}>
      <div style={{ fontWeight: 600, width: 88 }}>{label}</div>
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
        <input
          type="number"
          min={1}
          className="input"
          disabled={!alert}
          value={minutes}
          onChange={e => onMinutes(Number(e.target.value) || 1)}
        />
      </label>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  back: { color: colors.textMuted, textDecoration: 'none' },
  empty: { color: colors.textMuted, padding: 40, textAlign: 'center' },
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
  dl: { margin: 0 },
  dlRow: { display: 'flex', justifyContent: 'space-between', gap: 16, padding: '8px 0', borderBottom: `1px solid ${colors.border}`, fontSize: 14 },
  dt: { color: colors.textMuted, margin: 0 },
  dd: { margin: 0, fontWeight: 600 },
  svcRow: { display: 'flex', justifyContent: 'space-between', gap: 12, padding: '8px 0', borderBottom: `1px solid ${colors.border}`, fontSize: 14 },
}
