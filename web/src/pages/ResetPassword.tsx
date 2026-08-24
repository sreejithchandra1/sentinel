import { FormEvent, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { colors } from '../theme'

export default function ResetPassword() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get('token') || ''
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')

    if (!token) {
      setError('Invalid reset link')
      return
    }
    if (password.length < 8 || !/[A-Za-z]/.test(password) || !/[0-9]/.test(password)) {
      setError('Password must be at least 8 characters and include a letter and a number')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }

    if (submitting) return
    setSubmitting(true)
    try {
      await api.resetPassword(token, password)
      setMessage('Password updated. You can sign in with your new password.')
      setPassword('')
      setConfirm('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed')
      setSubmitting(false)
    }
  }

  return (
    <div style={styles.wrap}>
      <form onSubmit={handleSubmit} style={styles.card}>
        <h2 style={{ margin: '0 0 4px', fontSize: 22 }}>Reset password</h2>
        <p style={{ color: colors.textMuted, margin: '0 0 24px', fontSize: 15 }}>
          Enter a new password for your account.
        </p>
        {message && <div className="flash-ok" role="status">{message}</div>}
        {error && <div className="flash-error" role="alert">{error}</div>}
        {!token && !message && (
          <div className="flash-error" role="alert">This reset link is invalid or has expired.</div>
        )}
        {token && !message && (
          <>
            <label className="field">
              <span className="field-label">New Password</span>
              <input className="input" type="password" autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} required disabled={submitting} />
            </label>
            <label className="field" style={{ marginTop: 16 }}>
              <span className="field-label">Confirm Password</span>
              <input className="input" type="password" autoComplete="new-password" value={confirm} onChange={e => setConfirm(e.target.value)} required disabled={submitting} />
            </label>
            <button type="submit" className="btn btn-primary" disabled={submitting} style={{ marginTop: 24, width: '100%', justifyContent: 'center', padding: 12 }}>
              {submitting ? 'Updating…' : 'Update Password'}
            </button>
          </>
        )}
        <Link to="/login" style={styles.backLink}>Back to sign in</Link>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: {
    minHeight: '100vh', display: 'grid', placeItems: 'center',
    background: colors.bg, padding: 24,
  },
  card: {
    width: '100%', maxWidth: 400, background: colors.card,
    border: `1px solid ${colors.border}`, borderRadius: 10, padding: '40px 32px',
  },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 10,
    borderRadius: 8, marginBottom: 16, fontSize: 15,
    border: `1px solid rgba(63,185,80,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 10,
    borderRadius: 8, marginBottom: 16, fontSize: 15,
    border: `1px solid rgba(248,81,73,0.3)`,
  },
  backLink: {
    display: 'inline-block', marginTop: 20, fontSize: 15,
    color: colors.brand, textDecoration: 'none',
  },
}
