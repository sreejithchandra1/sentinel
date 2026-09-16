import { ReactNode } from 'react'
import { colors, fonts } from '../theme'
import { SparkBars } from './Sparkline'

export default function HostMetricCard({
  icon,
  label,
  value,
  band,
  hint,
  color,
  series,
}: {
  icon: ReactNode
  label: string
  value: string
  band: 'Normal' | 'Warning' | 'Critical'
  hint?: string
  color: string
  series: Array<number | null>
}) {
  const bandColor = band === 'Critical' ? colors.red : band === 'Warning' ? colors.yellow : colors.green
  return (
    <div style={{
      position: 'relative',
      background: colors.card,
      border: `1px solid ${colors.border}`,
      borderRadius: 16,
      padding: '14px 14px 12px',
      minHeight: 128,
      overflow: 'hidden',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
        <span style={{
          width: 28,
          height: 28,
          borderRadius: 8,
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: `color-mix(in srgb, ${color} 16%, transparent)`,
          color,
          flexShrink: 0,
        }}>
          {icon}
        </span>
        <span style={{
          fontSize: 11,
          fontWeight: 600,
          color: colors.textMuted,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}>
          {label}
        </span>
      </div>
      <div style={{
        fontSize: 28,
        fontWeight: 600,
        color: colors.text,
        lineHeight: 1.15,
        letterSpacing: '-0.03em',
        fontFamily: fonts.mono,
        fontVariantNumeric: 'tabular-nums',
        marginTop: 12,
      }}>
        {value}
      </div>
      <div style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 6,
        marginTop: 10,
        paddingRight: 76,
        minHeight: 28,
      }}>
        <span style={{
          width: 7,
          height: 7,
          borderRadius: 99,
          background: bandColor,
          marginTop: 4,
          flexShrink: 0,
        }} />
        <span style={{
          fontSize: 12,
          lineHeight: 1.35,
          color: colors.textMuted,
        }}>
          {hint ? `${band} · ${hint}` : band}
        </span>
      </div>
      <div style={{ position: 'absolute', right: 10, bottom: 10, pointerEvents: 'none' }}>
        <SparkBars values={series} color={color} />
      </div>
    </div>
  )
}

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden>
      {children}
    </svg>
  )
}

const stroke = {
  stroke: 'currentColor',
  strokeWidth: 1.8,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
}

export const hostMetricIcons = {
  cpu: (
    <Icon>
      <rect x="5" y="5" width="14" height="14" rx="2" {...stroke} />
      <rect x="9" y="9" width="6" height="6" rx="0.5" {...stroke} />
      <path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" {...stroke} />
    </Icon>
  ),
  memory: (
    <Icon>
      <rect x="4" y="7" width="16" height="10" rx="2" {...stroke} />
      <path d="M8 7V5M12 7V5M16 7V5M8 17v2M12 17v2M16 17v2" {...stroke} />
    </Icon>
  ),
  swap: (
    <Icon>
      <rect x="8" y="8" width="12" height="12" rx="2" {...stroke} />
      <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" {...stroke} />
    </Icon>
  ),
  disk: (
    <Icon>
      <path d="M4 6h16v12H4z" {...stroke} />
      <path d="M4 12h16M8 16h.01M12 16h.01" {...stroke} />
    </Icon>
  ),
  load: (
    <Icon>
      <path d="M22 12h-4l-3 7L9 5l-3 7H2" {...stroke} />
    </Icon>
  ),
  iowait: (
    <Icon>
      <path d="M13 2 3 14h9l-1 8 10-12h-9z" {...stroke} />
    </Icon>
  ),
}
