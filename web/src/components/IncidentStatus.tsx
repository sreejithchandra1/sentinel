import StatusBadge from './StatusBadge'
import { colors } from '../theme'
import type { Incident } from '../api'

function sslExpiryDays(message?: string): number | null {
  if (!message) return null
  const m = message.match(/expires in (\d+) days/i)
  if (!m) return null
  return Number(m[1])
}

/** Status label for incident rows — cert/DNS changes are notices, not downtime. */
export default function IncidentStatus({ incident }: { incident: Incident }) {
  const type = (incident.type || '').toLowerCase()

  if (type === 'cert_change' || type === 'dns_change') {
    return <span style={styles.notice}>Notice</span>
  }

  if (type === 'ssl_expiry' || type === 'slow') {
    if (incident.resolved_at) {
      return <span style={styles.resolved}>Resolved</span>
    }
    if (type === 'ssl_expiry') {
      const days = sslExpiryDays(incident.message)
      if (days != null && days <= 7) {
        return <span style={styles.critical}>Critical</span>
      }
    }
    return <span style={styles.warning}>Warning</span>
  }

  if (incident.resolved_at) {
    return <span style={styles.resolved}>Resolved</span>
  }

  return <StatusBadge status="down" />
}

export function incidentStatusLabel(incident: Incident): string {
  const type = (incident.type || '').toLowerCase()
  if (type === 'cert_change' || type === 'dns_change') return 'Notice'
  if (incident.resolved_at) return 'Resolved'
  if (type === 'ssl_expiry') {
    const days = sslExpiryDays(incident.message)
    if (days != null && days <= 7) return 'Critical'
    return 'Warning'
  }
  if (type === 'slow') return 'Warning'
  return 'Open'
}

const styles: Record<string, React.CSSProperties> = {
  resolved: { color: colors.green, fontSize: 15, fontWeight: 600 },
  notice: { color: colors.blue, fontSize: 15, fontWeight: 600 },
  warning: { color: colors.yellow, fontSize: 15, fontWeight: 600 },
  critical: { color: colors.red, fontSize: 15, fontWeight: 600 },
}
