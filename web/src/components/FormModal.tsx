import { ReactNode, useEffect } from 'react'
import ModalCloseButton from './ModalCloseButton'

export default function FormModal({
  title,
  subtitle,
  onClose,
  children,
  wide = false,
}: {
  title: string
  subtitle?: string
  onClose: () => void
  children: ReactNode
  wide?: boolean
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [onClose])

  return (
    <div
      className="form-modal-backdrop"
      onMouseDown={e => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className={'form-modal' + (wide ? ' form-modal--wide' : '')}
        role="dialog"
        aria-modal="true"
        aria-labelledby="form-modal-title"
        onMouseDown={e => e.stopPropagation()}
      >
        <div className="modal-head">
          <div className="modal-head-copy">
            <h3 id="form-modal-title" className="form-modal-title">{title}</h3>
            {subtitle && <p className="form-modal-sub">{subtitle}</p>}
          </div>
          <ModalCloseButton onClick={onClose} />
        </div>
        {children}
      </div>
    </div>
  )
}
