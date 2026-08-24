import { colors, radius } from '../theme'

const labels: Record<string, string> = {
  up: 'Healthy',
  down: 'Down',
  degraded: 'Warning',
  unknown: 'Unknown',
  critical: 'Critical',
}

const statusStyles: Record<string, { bg: string; text: string; dot: string }> = {
  up: { bg: colors.greenDim, text: colors.green, dot: colors.green },
  down: { bg: colors.redDim, text: colors.red, dot: colors.red },
  degraded: { bg: colors.yellowDim, text: colors.yellow, dot: colors.yellow },
  critical: { bg: colors.redDim, text: colors.red, dot: colors.red },
  unknown: { bg: 'rgba(156,163,175,0.12)', text: colors.textMuted, dot: colors.textMuted },
}

/** Map stored monitor status to badge status (SSL down = critical expiry). */
export function badgeStatusFor(monitorType: string | undefined, status: string): string {
  if ((monitorType || 'http') === 'ssl' && status === 'down') return 'critical'
  return status
}

export default function StatusBadge({ status }: { status: string }) {
  const c = statusStyles[status] || statusStyles.unknown
  return (
    <span style={{
      display: 'inline-flex',
      alignItems: 'center',
      gap: 6,
      background: c.bg,
      color: c.text,
      padding: '4px 8px',
      borderRadius: radius.sm,
      fontSize: 13,
      fontWeight: 600,
      letterSpacing: '0.02em',
    }}>
      <span style={{ width: 6, height: 6, borderRadius: 1, background: c.dot }} />
      {labels[status] || status}
    </span>
  )
}
