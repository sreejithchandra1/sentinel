import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, Incident } from '../api'
import { incidentLifecycleLabel } from '../components/IncidentStatus'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import { colors, fonts } from '../theme'
import { formatDuration, incidentDurationSeconds } from '../utils/duration'

export default function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const [incident, setIncident] = useState<Incident | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [acking, setAcking] = useState(false)
  const [now, setNow] = useState(Date.now())

  async function load() {
    if (!id) return
    try {
      setError('')
      const item = await api.getIncident(id)
      setIncident(item)
    } catch (err) {
      setIncident(null)
      setError(err instanceof Error ? err.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setLoading(true)
    load()
    const poll = setInterval(load, 30000)
    return () => clearInterval(poll)
  }, [id])

  useEffect(() => {
    if (!incident || incident.resolved_at) return
    const tick = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(tick)
  }, [incident?.id, incident?.resolved_at])

  async function acknowledge() {
    if (!id) return
    setAcking(true)
    setError('')
    try {
      const item = await api.acknowledgeIncident(id)
      setIncident(item)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to acknowledge')
    } finally {
      setAcking(false)
    }
  }

  if (loading && !incident) {
    return <div className="page" style={{ color: colors.textMuted }}>Loading…</div>
  }

  if (!incident) {
    return (
      <div className="page">
        <PageHeader title="Incident" subtitle="Not found" />
        {error && <div className="flash-error" role="alert">{error}</div>}
      </div>
    )
  }

  const open = !incident.resolved_at
  const canAck = open && !incident.acknowledged_at
  const duration = formatDuration(
    incidentDurationSeconds(incident.started_at, incident.resolved_at, now),
  )
  const lifecycle = incidentLifecycleLabel(incident)

  return (
    <div className="page">
      <PageHeader
        title={incident.monitor_name || incident.monitor_id}
        badges={<span style={{
          fontWeight: 600,
          fontSize: 15,
          color: lifecycle === 'Resolved' ? colors.green : lifecycle === 'Acknowledged' ? colors.blue : colors.red,
        }}>{lifecycle}</span>}
        subtitle={incident.message || undefined}
        actions={canAck ? (
          <button
            type="button"
            className="btn btn-primary"
            disabled={acking}
            onClick={acknowledge}
          >
            {acking ? 'Acknowledging…' : 'Acknowledge'}
          </button>
        ) : undefined}
      />

      {error && <div className="flash-error" role="alert">{error}</div>}

      <Panel>
        <dl style={styles.dl}>
          <Row label="Type" value={incident.type} />
          <Row label="Status" value={lifecycle} />
          <Row label="Message" value={incident.message || '—'} />
          <Row label="Started" value={new Date(incident.started_at).toLocaleString()} />
          <Row label="Duration" value={duration} />
          <Row
            label="Resolved"
            value={incident.resolved_at ? new Date(incident.resolved_at).toLocaleString() : '—'}
          />
          <div style={styles.row}>
            <dt style={styles.dt}>Monitor</dt>
            <dd style={styles.dd}>
              <Link to={`/monitors/${incident.monitor_id}`} style={styles.link}>
                {incident.monitor_name || incident.monitor_id}
              </Link>
            </dd>
          </div>
          <Row
            label="Acknowledged"
            value={
              incident.acknowledged_at
                ? `${incident.acknowledged_by || 'Someone'} · ${new Date(incident.acknowledged_at).toLocaleString()}`
                : '—'
            }
          />
        </dl>
      </Panel>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={styles.row}>
      <dt style={styles.dt}>{label}</dt>
      <dd style={styles.dd}>{value}</dd>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  dl: { margin: 0 },
  row: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: 16,
    padding: '10px 0',
    borderBottom: `1px solid ${colors.border}`,
    fontSize: 14,
  },
  dt: { color: colors.textMuted, flexShrink: 0 },
  dd: {
    margin: 0,
    fontWeight: 500,
    textAlign: 'right',
    fontFamily: fonts.sans,
    wordBreak: 'break-word',
  },
  link: { color: colors.brand, textDecoration: 'none', fontWeight: 600 },
}
