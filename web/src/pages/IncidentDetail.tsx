import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { api, Incident, incidentSubjectPath, isHostIncident } from '../api'
import { incidentLifecycleLabel } from '../components/IncidentStatus'
import PageHeader from '../components/PageHeader'
import Panel from '../components/Panel'
import { colors, fonts } from '../theme'
import { dnsChangeSummary, parseDNSChangeMessage } from '../utils/dnsChangeMessage'
import { formatDuration, incidentDurationSeconds } from '../utils/duration'
import { displayIncidentMessage } from '../utils/incidentMessage'

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
  const dnsTables = incident.type === 'dns_change' ? parseDNSChangeMessage(incident.message || '') : null
  const subtitle = dnsTables ? dnsChangeSummary(dnsTables) : (displayIncidentMessage(incident.message) || undefined)

  return (
    <div className="page">
      <PageHeader
        title={incident.monitor_name || incident.monitor_id}
        badges={<span style={{
          fontWeight: 600,
          fontSize: 15,
          color: lifecycle === 'Resolved' ? colors.green : lifecycle === 'Acknowledged' ? colors.blue : colors.red,
        }}>{lifecycle}</span>}
        subtitle={subtitle}
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

      <div style={{ display: 'grid', gap: 16 }}>
        <Panel>
        <dl style={styles.dl}>
          <Row label="Type" value={incident.type} />
          <Row label="Status" value={lifecycle} />
          {dnsTables ? (
            <div style={{ ...styles.row, display: 'block', textAlign: 'left' }}>
              <dt style={{ ...styles.dt, marginBottom: 10 }}>DNS records</dt>
              <dd style={{ ...styles.dd, textAlign: 'left', fontWeight: 400 }}>
                <DnsRecordTable title="Previous" rows={dnsTables.previous} />
                <DnsRecordTable title="Current" rows={dnsTables.current} />
              </dd>
            </div>
          ) : (
            <Row label="Message" value={displayIncidentMessage(incident.message) || '—'} />
          )}
          <Row label="Started" value={new Date(incident.started_at).toLocaleString()} />
          <Row label="Duration" value={duration} />
          <Row
            label="Resolved"
            value={incident.resolved_at ? new Date(incident.resolved_at).toLocaleString() : '—'}
          />
          <div style={styles.row}>
            <dt style={styles.dt}>{isHostIncident(incident.type) ? 'Host' : 'Monitor'}</dt>
            <dd style={styles.dd}>
              <Link to={incidentSubjectPath(incident)} style={styles.link}>
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

      {incident.error_page && (
        <Panel>
          <h3 className="panel-title" style={{ marginTop: 0 }}>Captured error page</h3>
          <dl style={styles.dl}>
            {incident.error_page.status_code ? (
              <Row label="Status code" value={String(incident.error_page.status_code)} />
            ) : null}
            {incident.error_page.view_url ? (
              <div style={styles.row}>
                <dt style={styles.dt}>Captured page</dt>
                <dd style={styles.dd}>
                  <a href={incident.error_page.view_url} target="_blank" rel="noreferrer" style={styles.link}>
                    Open captured page →
                  </a>
                </dd>
              </div>
            ) : null}
          </dl>
          {incident.error_page.headers && Object.keys(incident.error_page.headers).length > 0 && (
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, marginTop: 8 }}>
              <thead>
                <tr>
                  <th style={styles.th}>Header</th>
                  <th style={styles.th}>Value</th>
                </tr>
              </thead>
              <tbody>
                {Object.entries(incident.error_page.headers).map(([name, value]) => (
                  <tr key={name}>
                    <td style={{ ...styles.td, color: colors.textMuted, width: 140, whiteSpace: 'nowrap' }}>{name}</td>
                    <td style={{ ...styles.td, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', wordBreak: 'break-all' }}>{value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {incident.error_page.body_html ? (
            <iframe
              title="Captured error page"
              sandbox=""
              srcDoc={withBaseHref(incident.error_page.body_html, incident.error_page.page_url)}
              style={{
                width: '100%',
                height: 420,
                marginTop: 16,
                border: `1px solid ${colors.border}`,
                borderRadius: 8,
                background: '#fff',
              }}
            />
          ) : incident.error_page.excerpt ? (
            <pre style={{
              marginTop: 16,
              padding: 12,
              background: colors.bgElevated,
              borderRadius: 8,
              fontSize: 13,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}>{incident.error_page.excerpt}</pre>
          ) : null}
        </Panel>
      )}
      </div>
    </div>
  )
}

function DnsRecordTable({ title, rows }: { title: string; rows: { type: string; value: string }[] }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ fontSize: 13, fontWeight: 600, color: colors.text, marginBottom: 6 }}>{title}</div>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr>
            <th style={styles.th}>Type</th>
            <th style={styles.th}>Value</th>
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={2} style={{ ...styles.td, color: colors.textMuted }}>—</td>
            </tr>
          ) : rows.map((row, i) => (
            <tr key={`${row.type}-${row.value}-${i}`}>
              <td style={{ ...styles.td, color: colors.textMuted, width: 72, whiteSpace: 'nowrap' }}>{row.type}</td>
              <td style={{ ...styles.td, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', wordBreak: 'break-all' }}>{row.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
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

function withBaseHref(html: string, pageUrl?: string) {
  const baseUrl = (pageUrl || '').trim()
  if (!baseUrl) return html
  let href = baseUrl
  try {
    const u = new URL(baseUrl)
    if (u.pathname && !u.pathname.endsWith('/')) {
      u.pathname = u.pathname.slice(0, u.pathname.lastIndexOf('/') + 1)
      u.search = ''
      u.hash = ''
      href = u.toString()
    }
  } catch {
    href = baseUrl
  }
  const base = `<base href="${href.replace(/"/g, '&quot;')}">`
  const lower = html.toLowerCase()
  const headIdx = lower.indexOf('<head')
  if (headIdx >= 0) {
    const close = html.indexOf('>', headIdx)
    if (close >= 0) return html.slice(0, close + 1) + base + html.slice(close + 1)
  }
  const htmlIdx = lower.indexOf('<html')
  if (htmlIdx >= 0) {
    const close = html.indexOf('>', htmlIdx)
    if (close >= 0) return html.slice(0, close + 1) + `<head>${base}</head>` + html.slice(close + 1)
  }
  return `<head>${base}</head>${html}`
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
  th: {
    textAlign: 'left' as const,
    color: colors.textMuted,
    fontWeight: 600,
    fontSize: 12,
    padding: '6px 8px',
    borderBottom: `1px solid ${colors.border}`,
  },
  td: {
    padding: '7px 8px',
    borderBottom: `1px solid ${colors.border}`,
    color: colors.text,
    verticalAlign: 'top',
  },
}
