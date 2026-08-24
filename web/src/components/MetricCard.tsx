import { colors, fonts, radius } from '../theme'

export default function MetricCard({
  label,
  value,
  sub,
  accent = 'default',
}: {
  label: string
  value: string
  sub?: string
  accent?: 'default' | 'green' | 'blue' | 'yellow' | 'red'
}) {
  const accentColor = {
    default: colors.text,
    green: colors.green,
    blue: colors.blue,
    yellow: colors.yellow,
    red: colors.red,
  }[accent]

  return (
    <div style={{
      background: colors.card,
      border: `1px solid ${colors.border}`,
      borderRadius: radius.md,
      padding: '16px 18px',
    }}>
      <div style={{
        fontSize: 13,
        fontWeight: 600,
        color: colors.textMuted,
        marginBottom: 8,
        letterSpacing: '0.04em',
        textTransform: 'uppercase',
      }}>
        {label}
      </div>
      <div style={{
        fontSize: 26,
        fontWeight: 600,
        color: accentColor,
        lineHeight: 1.15,
        letterSpacing: '-0.02em',
        fontFamily: fonts.mono,
        fontVariantNumeric: 'tabular-nums',
      }}>
        {value}
      </div>
      {sub && (
        <div style={{ fontSize: 14, color: colors.textMuted, marginTop: 6 }}>
          {sub}
        </div>
      )}
    </div>
  )
}
