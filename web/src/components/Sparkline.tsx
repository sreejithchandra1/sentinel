import { colors } from '../theme'

function downsample(values: Array<number | null>, n: number): Array<number | null> {
  if (values.length <= n) return values
  const out: Array<number | null> = []
  for (let i = 0; i < n; i++) {
    const start = Math.floor((i * values.length) / n)
    const end = Math.max(start + 1, Math.floor(((i + 1) * values.length) / n))
    let sum = 0
    let count = 0
    for (let j = start; j < end; j++) {
      const v = values[j]
      if (v != null && Number.isFinite(v)) {
        sum += v
        count++
      }
    }
    out.push(count ? sum / count : null)
  }
  return out
}

export function SparkBars({
  values,
  width = 72,
  height = 28,
  color = colors.green,
}: {
  values: Array<number | null>
  width?: number
  height?: number
  color?: string
}) {
  const n = 18
  const slice = downsample(values, n)
  const nums = slice.filter((v): v is number => v != null && Number.isFinite(v))
  if (nums.length < 2) {
    return <span style={{ display: 'inline-block', width, height }} />
  }
  const min = Math.min(...nums)
  const max = Math.max(...nums)
  const range = max - min
  const gap = 1.6
  const barW = Math.max(2.2, (width - gap * (slice.length - 1)) / slice.length)
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden>
      {slice.map((v, i) => {
        const x = i * (barW + gap)
        const t = v == null || !Number.isFinite(v)
          ? 0
          : range === 0
            ? 0.42
            : 0.18 + 0.82 * ((v - min) / range)
        const h = Math.max(v == null ? 0 : 3, t * height)
        return (
          <rect
            key={i}
            x={x}
            y={height - h}
            width={barW}
            height={h}
            rx={1.2}
            fill={color}
            opacity={0.88}
          />
        )
      })}
    </svg>
  )
}

export default function Sparkline({
  values,
  width = 64,
  height = 22,
  color = colors.green,
}: {
  values: number[]
  width?: number
  height?: number
  color?: string
}) {
  if (values.length < 2) {
    return <span style={{ display: 'inline-block', width, height }} />
  }
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  const pts = values.map((v, i) => {
    const x = (i / (values.length - 1)) * width
    const y = height - ((v - min) / range) * (height - 2) - 1
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  const line = pts.join(' ')
  const fill = `${pts[0]} ${line} ${width.toFixed(1)},${height} 0,${height}`

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden>
      <polygon points={fill} fill={color} opacity={0.18} />
      <polyline
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
        points={line}
      />
    </svg>
  )
}
