import { useEffect } from 'react'
import { colors } from '../theme'
import ModalCloseButton from './ModalCloseButton'

export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !busy) onCancel()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, busy, onCancel])

  if (!open) return null

  return (
    <div
      style={styles.backdrop}
      role="presentation"
      onMouseDown={e => {
        if (e.target === e.currentTarget && !busy) onCancel()
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-message"
        style={styles.dialog}
      >
        <div style={styles.head}>
          <h3 id="confirm-dialog-title" style={styles.title}>{title}</h3>
          <ModalCloseButton onClick={onCancel} disabled={busy} />
        </div>
        <p id="confirm-dialog-message" style={styles.message}>{message}</p>
        <div style={styles.actions}>
          <button
            type="button"
            className="btn"
            disabled={busy}
            onClick={onCancel}
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            className={danger ? 'btn btn-danger' : 'btn btn-primary'}
            disabled={busy}
            onClick={onConfirm}
            autoFocus
          >
            {busy ? 'Please wait…' : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  backdrop: {
    position: 'fixed',
    inset: 0,
    zIndex: 1000,
    background: 'var(--overlay)',
    display: 'grid',
    placeItems: 'center',
    padding: 24,
  },
  dialog: {
    width: '100%',
    maxWidth: 420,
    background: colors.card,
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    padding: '24px 28px',
    boxShadow: 'var(--shadow)',
  },
  head: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: 12,
    marginBottom: 10,
  },
  title: {
    margin: 0,
    fontSize: 19,
    fontWeight: 700,
    letterSpacing: '-0.01em',
  },
  message: {
    margin: '0 0 24px',
    fontSize: 15,
    lineHeight: 1.5,
    color: colors.textMuted,
  },
  actions: {
    display: 'flex',
    justifyContent: 'flex-end',
    flexWrap: 'wrap',
    gap: 10,
  },
}
