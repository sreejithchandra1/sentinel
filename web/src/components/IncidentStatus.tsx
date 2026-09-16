import StatusBadge from './StatusBadge'
import { colors } from '../theme'
import type { Incident } from '../api'

function sslExpiryDays(message?: string): number | null {
  if (!message) return null
  const m = message.match(/expires in (\d+) days/i)
  if (!m) return null
  return Number(m[1])
}

/** Open / Acknowledged / Resolved — used on the incident page. */
export function incidentLifecycleLabel(incident: Incident): string {
  if (incident.resolved_at) return 'Resolved'
  if (incident.acknowledged_at) return 'Acknowledged'
  return 'Open'
}

/** Gauge alerts: "Disk warning: 83% (warning 80.0 / critical 90.0)" vs "Disk critical: …". */
function hostGaugeIsCritical(message?: string): boolean {
  return /\bcritical:/.test((message || '').toLowerCase())
}

export function incidentStatusLabel(incident: Incident): string {
  const type = (incident.type || '').toLowerCase()
  if (type === 'cert_change' || type === 'dns_change') return 'Notice'
  if (incident.resolved_at) return 'Resolved'
  if (incident.acknowledged_at) return 'Acknowledged'

  if (type === 'ssl_expiry') {
    const days = sslExpiryDays(incident.message)
    if (days != null && days <= 7) return 'Critical'
    return 'Warning'
  }
  if (type === 'slow') return 'Warning'

  if (type.startsWith('host_') && type !== 'host_offline') {
    if (type === 'host_root_login' || type === 'host_auth' || type === 'host_service') {
      return 'Critical'
    }
    if (hostGaugeIsCritical(incident.message)) return 'Critical'
    return 'Warning'
  }

  return 'Open'
}

export function incidentRowClass(incident: Incident): string | undefined {
  if (incident.resolved_at) return undefined
  const label = incidentStatusLabel(incident)
  if (label === 'Notice' || label === 'Warning') return 'row-warn'
  return 'row-down'
}

/** Status label for incident rows — cert/DNS changes are notices, not downtime. */
export default function IncidentStatus({ incident }: { incident: Incident }) {
  const label = incidentStatusLabel(incident)
  if (label === 'Notice') return <span style={styles.notice}>Notice</span>
  if (label === 'Resolved') return <span style={styles.resolved}>Resolved</span>
  if (label === 'Acknowledged') return <span style={styles.acked}>Acknowledged</span>
  if (label === 'Critical') return <span style={styles.critical}>Critical</span>
  if (label === 'Warning') return <span style={styles.warning}>Warning</span>
  return <StatusBadge status="down" />
}

const styles: Record<string, React.CSSProperties> = {
  resolved: { color: colors.green, fontSize: 15, fontWeight: 600 },
  acked: { color: colors.blue, fontSize: 15, fontWeight: 600 },
  notice: { color: colors.blue, fontSize: 15, fontWeight: 600 },
  warning: { color: colors.yellow, fontSize: 15, fontWeight: 600 },
  critical: { color: colors.red, fontSize: 15, fontWeight: 600 },
}
