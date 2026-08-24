import { colors, radius } from '../theme'

function labelFor(type?: string, url?: string): string {
  if (!type || type === 'http') {
    if (url?.startsWith('https://')) return 'HTTPS'
    if (url?.startsWith('http://')) return 'HTTP'
    return 'HTTP'
  }
  const labels: Record<string, string> = { port: 'PORT', ssl: 'SSL', dns: 'DNS', heartbeat: 'HEARTBEAT' }
  return labels[type] || type.toUpperCase()
}

export default function TypeBadge({ type, url }: { type?: string; url?: string }) {
  return (
    <span style={{
      background: colors.brandDim,
      color: colors.brand,
      padding: '3px 8px',
      borderRadius: radius.sm,
      fontSize: 13,
      fontWeight: 600,
      letterSpacing: '0.06em',
      border: `1px solid color-mix(in srgb, ${colors.brand} 32%, transparent)`,
    }}>
      {labelFor(type, url)}
    </span>
  )
}
