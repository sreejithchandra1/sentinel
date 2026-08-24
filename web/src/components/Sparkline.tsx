import { colors } from '../theme'

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
