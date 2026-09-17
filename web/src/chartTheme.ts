import { colors, fonts, radius } from './theme'
import { chartTimeDomain, formatChartTick, type ChartRange, type ChartTimeZone } from './utils/period'

export const chartTick = {
  fill: colors.textMuted,
  fontSize: 12,
  fontFamily: fonts.mono,
}

export const chartTooltipStyle = {
  background: colors.card,
  border: `1px solid ${colors.border}`,
  borderRadius: radius.md,
  color: colors.text,
  fontSize: 14,
  fontFamily: fonts.sans,
}

export const chartTooltipLabel = {
  color: colors.textMuted,
}

export const chartGridStroke = colors.border

export function chartTimeXAxis(range: ChartRange, timeZone: ChartTimeZone = 'utc') {
  return {
    dataKey: 'ts' as const,
    type: 'number' as const,
    domain: chartTimeDomain(range),
    allowDataOverflow: true,
    tickCount: 5,
    tick: chartTick,
    axisLine: false as const,
    tickLine: false as const,
    tickFormatter: (ms: number) => formatChartTick(new Date(ms).toISOString(), range, timeZone),
  }
}

export function chartTimeTooltipLabel(range: ChartRange, timeZone: ChartTimeZone = 'utc') {
  return (ms: number) => formatChartTick(new Date(ms).toISOString(), range, timeZone)
}
