import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'

function isTruncated(el: HTMLElement): boolean {
  if (el.scrollWidth > el.clientWidth + 1 || el.scrollHeight > el.clientHeight + 1) return true
  const nodes = el.querySelectorAll<HTMLElement>('*')
  for (const node of nodes) {
    if (node.scrollWidth > node.clientWidth + 1 || node.scrollHeight > node.clientHeight + 1) return true
  }
  return false
}

export default function TableCellTooltip() {
  const [tip, setTip] = useState<{ text: string; top: number; left: number } | null>(null)

  useEffect(() => {
    function hide() {
      setTip(null)
    }

    function onOver(e: MouseEvent) {
      const target = e.target as HTMLElement | null
      const td = target?.closest?.('.data-table td') as HTMLElement | null
      if (!td || td.classList.contains('col-actions')) {
        hide()
        return
      }
      if (td.querySelector('input, select, textarea, .kebab-btn')) {
        hide()
        return
      }
      if (!isTruncated(td)) {
        hide()
        return
      }
      const text = (td.innerText || '').replace(/\s+\n/g, '\n').replace(/[ \t]+/g, ' ').trim()
      if (!text) {
        hide()
        return
      }
      const r = td.getBoundingClientRect()
      const left = Math.min(r.left, window.innerWidth - 280)
      setTip({ text, top: r.bottom + 6, left: Math.max(8, left) })
    }

    document.addEventListener('mouseover', onOver)
    window.addEventListener('scroll', hide, true)
    window.addEventListener('resize', hide)
    return () => {
      document.removeEventListener('mouseover', onOver)
      window.removeEventListener('scroll', hide, true)
      window.removeEventListener('resize', hide)
    }
  }, [])

  if (!tip) return null

  return createPortal(
    <div className="cell-tooltip" role="tooltip" style={{ top: tip.top, left: tip.left }}>
      {tip.text}
    </div>,
    document.body,
  )
}
