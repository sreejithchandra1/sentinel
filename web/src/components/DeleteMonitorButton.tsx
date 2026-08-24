import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { colors } from '../theme'

export default function DeleteMonitorButton({
  id,
  name,
  variant = 'default',
  onDeleted,
}: {
  id: string
  name: string
  variant?: 'default' | 'danger'
  onDeleted?: () => void
}) {
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState('')

  async function handleDelete() {
    setDeleting(true)
    setError('')
    try {
      await api.deleteMonitor(id)
      onDeleted ? onDeleted() : navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
      setDeleting(false)
    }
  }

  if (confirming) {
    return (
      <div style={styles.confirmBox}>
        <span style={{ fontSize: 15, color: colors.text }}>Delete <strong>{name}</strong>?</span>
        {error && <div style={{ color: colors.red, fontSize: 14 }}>{error}</div>}
        <div style={{ display: 'flex', gap: 8 }}>
          <button onClick={handleDelete} disabled={deleting} className="btn btn-danger" style={{ fontSize: 15 }}>
            {deleting ? 'Deleting…' : 'Confirm'}
          </button>
          <button onClick={() => setConfirming(false)} disabled={deleting} className="btn" style={{ fontSize: 15 }}>Cancel</button>
        </div>
      </div>
    )
  }

  return (
    <button
      type="button"
      onClick={() => setConfirming(true)}
      className={variant === 'danger' ? 'btn btn-danger' : 'btn btn-danger'}
      style={{ fontSize: 15, padding: variant === 'danger' ? '8px 16px' : '6px 12px' }}
    >
      Delete
    </button>
  )
}

const styles: Record<string, React.CSSProperties> = {
  confirmBox: {
    display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-end',
    background: colors.redDim, padding: 12, borderRadius: 8,
    border: `1px solid rgba(248,81,73,0.3)`,
  },
}
