import { FormEvent, useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from 'recharts'
import { api, Host, HostStats } from '../api'
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

  const load = useCallback(async () => {
    if (!id) return null
    const [h, s] = await Promise.all([api.getHost(id), api.hostStats(id, period)])
    setHost(h)
    setStats(s)
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
      const updated = await api.updateHost(host.id, host)
      setHost(updated)
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
  const chartData = (stats?.points || []).map(p => ({
    time: new Date(p.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    cpu: p.cpu_percent ?? null,
    mem: p.mem_percent ?? null,
    disk: p.disk_percent ?? null,
    load: p.load1 ?? null,
  }))

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
        subtitle={
          <>
            <Link to="/hosts" style={styles.back}>← Hosts</Link>
            <span style={{ marginLeft: 10 }}>{host.hostname || 'Waiting for agent'}</span>
            {host.os && <span style={{ marginLeft: 10, color: colors.textMuted }}>{host.os}/{host.arch}</span>}
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
        <MetricCard label="Disk" value={`${fmt(latest?.disk_percent)}%`} />
        <MetricCard label="Load 1" value={fmt(latest?.load1, 2)} />
      </div>

      <Panel style={{ marginBottom: 20 }}>
        <h3 className="panel-title">Metrics</h3>
        <div style={{ height: 300 }}>
          {chartData.length > 0 ? (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData}>
                <CartesianGrid stroke={chartGridStroke} vertical={false} />
                <XAxis dataKey="time" tick={chartTick} axisLine={false} tickLine={false} />
                <YAxis tick={chartTick} axisLine={false} tickLine={false} />
                <Tooltip contentStyle={chartTooltipStyle} labelStyle={chartTooltipLabel} />
                <Area type="monotone" dataKey="cpu" stroke={colors.brand} fill={colors.brand} fillOpacity={0.15} name="CPU %" />
                <Area type="monotone" dataKey="mem" stroke={colors.blue} fill={colors.blue} fillOpacity={0.12} name="Memory %" />
                <Area type="monotone" dataKey="disk" stroke={colors.yellow} fill={colors.yellow} fillOpacity={0.12} name="Disk %" />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div style={styles.empty}>Waiting for samples…</div>
          )}
        </div>
      </Panel>

      {isAdmin && (
        <Panel>
          <h3 className="panel-title">Monitoring</h3>
          <p style={{ color: colors.textMuted, fontSize: 14, marginTop: 0 }}>
            Metrics are collected by default. Alerts fire only after the value stays above the threshold for the duration — a single spike will not page.
          </p>
          <form onSubmit={saveMonitoring}>
            <MetricToggle
              label="CPU"
              collect={host.collect_cpu}
              onCollect={v => setHostField('collect_cpu', v)}
              alert={host.alert_cpu_enabled}
              onAlert={v => setHostField('alert_cpu_enabled', v)}
              threshold={host.alert_cpu_threshold}
              onThreshold={v => setHostField('alert_cpu_threshold', v)}
              minutes={minutesFromSamples(host.alert_cpu_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_cpu_after', samplesFromMinutes(m, host.interval_seconds))}
              unit="%"
            />
            <MetricToggle
              label="Memory"
              collect={host.collect_memory}
              onCollect={v => setHostField('collect_memory', v)}
              alert={host.alert_memory_enabled}
              onAlert={v => setHostField('alert_memory_enabled', v)}
              threshold={host.alert_memory_threshold}
              onThreshold={v => setHostField('alert_memory_threshold', v)}
              minutes={minutesFromSamples(host.alert_memory_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_memory_after', samplesFromMinutes(m, host.interval_seconds))}
              unit="%"
            />
            <MetricToggle
              label="Disk"
              collect={host.collect_disk}
              onCollect={v => setHostField('collect_disk', v)}
              alert={host.alert_disk_enabled}
              onAlert={v => setHostField('alert_disk_enabled', v)}
              threshold={host.alert_disk_threshold}
              onThreshold={v => setHostField('alert_disk_threshold', v)}
              minutes={minutesFromSamples(host.alert_disk_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_disk_after', samplesFromMinutes(m, host.interval_seconds))}
              unit="%"
            />
            <MetricToggle
              label="Load"
              collect={host.collect_load}
              onCollect={v => setHostField('collect_load', v)}
              alert={host.alert_load_enabled}
              onAlert={v => setHostField('alert_load_enabled', v)}
              threshold={host.alert_load_threshold}
              onThreshold={v => setHostField('alert_load_threshold', v)}
              minutes={minutesFromSamples(host.alert_load_after, host.interval_seconds)}
              onMinutes={m => setHostField('alert_load_after', samplesFromMinutes(m, host.interval_seconds))}
              unit=""
              thresholdHint="0 = 2 × CPU count"
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

function MetricToggle({
  label, collect, onCollect, alert, onAlert, threshold, onThreshold, minutes, onMinutes, unit, thresholdHint,
}: {
  label: string
  collect: boolean
  onCollect: (v: boolean) => void
  alert: boolean
  onAlert: (v: boolean) => void
  threshold: number
  onThreshold: (v: number) => void
  minutes: number
  onMinutes: (v: number) => void
  unit: string
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
        <span className="field-label">Threshold{unit ? ` (${unit})` : ''}</span>
        <input
          type="number"
          className="input"
          disabled={!alert}
          value={threshold}
          onChange={e => onThreshold(Number(e.target.value))}
        />
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
}
