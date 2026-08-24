import { ReactNode, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export default function KebabMenu({
  label = 'Actions',
  children,
}: {
  label?: string
  children: (close: () => void) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const btnRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState({ top: 0, left: 0 })

  function close() {
    setOpen(false)
  }

  function place() {
    const btn = btnRef.current
    const menu = menuRef.current
    if (!btn || !menu) return
    const r = btn.getBoundingClientRect()
    const width = menu.offsetWidth || 160
    const height = menu.offsetHeight || 0
    let left = r.right - width
    left = Math.max(8, Math.min(left, window.innerWidth - width - 8))
    let top = r.bottom + 6
    if (height && top + height > window.innerHeight - 8) {
      top = Math.max(8, r.top - 6 - height)
    }
    setPos({ top, left })
  }

  useLayoutEffect(() => {
    if (!open) return
    place()
    const id = requestAnimationFrame(place)
    return () => cancelAnimationFrame(id)
  }, [open])

  useEffect(() => {
    if (!open) return
    function onDoc(e: MouseEvent) {
      const t = e.target as Node
      if (btnRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    function onReposition(e: Event) {
      if (menuRef.current?.contains(e.target as Node)) return
      setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    window.addEventListener('resize', onReposition)
    window.addEventListener('scroll', onReposition, true)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('resize', onReposition)
      window.removeEventListener('scroll', onReposition, true)
    }
  }, [open])

  return (
    <>
      <button
        ref={btnRef}
        type="button"
        className="kebab-btn"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(v => !v)}
      >
        <span className="kebab-dots" aria-hidden="true">
          <span />
          <span />
          <span />
        </span>
      </button>
      {open && createPortal(
        <div
          ref={menuRef}
          role="menu"
          className="kebab-menu"
          style={{ top: pos.top, left: pos.left }}
        >
          {children(close)}
        </div>,
        document.body,
      )}
    </>
  )
}
