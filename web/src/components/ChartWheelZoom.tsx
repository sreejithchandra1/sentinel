import { ReactNode, useEffect, useRef } from 'react'
import { zoomChartRange, chartRangesEqual, type ChartRange } from '../utils/period'

const WHEEL_MS = 70
const ZOOM_IN = 0.7
const ZOOM_OUT = 1.45

export default function ChartWheelZoom({
  range,
  onChange,
  children,
}: {
  range: ChartRange
  onChange: (range: ChartRange) => void
  children: ReactNode
}) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const rangeRef = useRef(range)
  rangeRef.current = range
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange
  const lastRef = useRef(0)

  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    function onWheel(e: WheelEvent) {
      const node = wrapRef.current
      if (!node) return
      e.preventDefault()
      const focused = document.activeElement
      if (focused instanceof HTMLElement && node.contains(focused) === false && (focused.tagName === 'BUTTON' || focused.tagName === 'SELECT')) {
        focused.blur()
      }
      const now = Date.now()
      if (now - lastRef.current < WHEEL_MS) return
      lastRef.current = now
      const rect = node.getBoundingClientRect()
      const ratio = rect.width > 0 ? (e.clientX - rect.left) / rect.width : 0.5
      const factor = e.deltaY < 0 ? ZOOM_IN : ZOOM_OUT
      const next = zoomChartRange(rangeRef.current, factor, now, ratio)
      if (chartRangesEqual(rangeRef.current, next)) return
      onChangeRef.current(next)
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [])

  return (
    <div ref={wrapRef} className="chart-wheel-zoom" title="Scroll to zoom">
      {children}
    </div>
  )
}
