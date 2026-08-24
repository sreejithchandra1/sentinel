import { FormEvent, useEffect, useState } from 'react'
import { api } from '../api'
import PageHeader from '../components/PageHeader'
import { roleLabel, useAuth } from '../context/AuthContext'
import { colors } from '../theme'

function initialsFrom(name: string, username: string): string {
  const source = (name || username || 'U').trim()
  const parts = source.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  return source.slice(0, 2).toUpperCase()
}

export default function Profile() {
  const { refresh } = useAuth()
  const [username, setUsername] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [mfaEnabled, setMFAEnabled] = useState(false)
  const [role, setRole] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    api.getProfile().then(p => {
      setUsername(p.username)
      setName(p.name || '')
      setEmail(p.email || '')
      setMFAEnabled(!!p.mfa_enabled)
      setRole(p.role)
    }).catch(() => {})
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')

    if (newPassword && newPassword !== confirmPassword) {
      setError('New passwords do not match')
      return
    }
    if (newPassword && (newPassword.length < 8 || !/[A-Za-z]/.test(newPassword) || !/[0-9]/.test(newPassword))) {
      setError('New password must be at least 8 characters and include a letter and a number')
      return
    }
    if (mfaEnabled && !email.trim()) {
      setError('Email is required to enable two-factor authentication')
      return
    }

    try {
      const updated = await api.updateProfile({
        current_password: currentPassword,
        username: username.trim(),
        name: name.trim(),
        email: email.trim(),
        new_password: newPassword || undefined,
        mfa_enabled: mfaEnabled,
      })
      setUsername(updated.username)
      setName(updated.name || '')
      setEmail(updated.email || '')
      setMFAEnabled(!!updated.mfa_enabled)
      setRole(updated.role)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setMessage('Profile updated successfully')
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  const display = name.trim() || username || 'admin'
  const initials = initialsFrom(name, username)

  return (
    <div className="page">
      <PageHeader
        title="Profile"
        subtitle="Manage your account credentials, recovery email, and sign-in security."
      />

      <div style={styles.headerCard}>
        <span style={styles.avatar}>{initials}</span>
        <div>
          <div style={{ fontWeight: 700, fontSize: 19 }}>{display}</div>
          <div style={{ color: colors.textMuted, fontSize: 15 }}>
            @{username || 'admin'} · {role ? roleLabel(role) : 'User'}
          </div>
        </div>
      </div>

      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <form onSubmit={handleSubmit} style={styles.form} autoComplete="off">
        <h3 style={styles.sectionTitle}>Account Details</h3>
        <div className="field">
          <label className="field-label" htmlFor="profile-name">Name</label>
          <input
            id="profile-name"
            className="input"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="Your display name"
            autoComplete="name"
          />
          <p style={{ color: colors.textMuted, fontSize: 13, margin: '8px 0 0' }}>
            Used in greetings like “Good morning, {name.trim() || '…'}”.
          </p>
        </div>
        <div className="field" style={{ marginTop: 16 }}>
          <label className="field-label" htmlFor="profile-username">Username</label>
          <input
            id="profile-username"
            className="input"
            value={username}
            onChange={e => setUsername(e.target.value)}
            required
            autoComplete="username"
          />
        </div>
        <div className="field" style={{ marginTop: 16 }}>
          <label className="field-label" htmlFor="profile-email">Email</label>
          <input
            id="profile-email"
            className="input"
            type="email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            placeholder="Used for password reset"
            autoComplete="email"
          />
          <p style={{ color: colors.textMuted, fontSize: 13, margin: '8px 0 0' }}>
            Required for password reset and email verification codes.
          </p>
        </div>

        <h3 style={{ ...styles.sectionTitle, marginTop: 28 }}>Two-Factor Authentication</h3>
        <label style={styles.toggleRow}>
          <input
            type="checkbox"
            checked={mfaEnabled}
            onChange={e => setMFAEnabled(e.target.checked)}
          />
          <span>
            <div style={{ fontWeight: 600 }}>Require an email verification code at sign-in</div>
            <div style={{ color: colors.textMuted, fontSize: 14, marginTop: 4 }}>
              Each login will require your password plus a one-time code sent to your account email.
            </div>
          </span>
        </label>

        <h3 style={{ ...styles.sectionTitle, marginTop: 28 }}>Change Password</h3>
        <p style={{ color: colors.textMuted, fontSize: 14, margin: '0 0 16px' }}>
          Enter your current password to confirm any changes.
        </p>

        <div className="field">
          <label className="field-label" htmlFor="profile-current-password">Current Password</label>
          <input
            id="profile-current-password"
            className="input"
            type="password"
            value={currentPassword}
            onChange={e => setCurrentPassword(e.target.value)}
            required
            autoComplete="current-password"
          />
        </div>
        <div className="field" style={{ marginTop: 16 }}>
          <label className="field-label" htmlFor="profile-new-password">New Password</label>
          <input
            id="profile-new-password"
            className="input"
            type="password"
            value={newPassword}
            onChange={e => setNewPassword(e.target.value)}
            placeholder="Leave blank to keep current"
            autoComplete="new-password"
          />
        </div>
        <div className="field" style={{ marginTop: 16 }}>
          <label className="field-label" htmlFor="profile-confirm-password">Confirm New Password</label>
          <input
            id="profile-confirm-password"
            className="input"
            type="password"
            value={confirmPassword}
            onChange={e => setConfirmPassword(e.target.value)}
            placeholder="Leave blank to keep current"
            autoComplete="new-password"
          />
        </div>

        <button type="submit" className="btn btn-primary" style={{ marginTop: 24 }}>Save Changes</button>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  headerCard: {
    display: 'flex', alignItems: 'center', gap: 16, padding: '20px 24px',
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, marginBottom: 24, maxWidth: 480,
  },
  avatar: {
    width: 56, height: 56, borderRadius: '50%',
    background: colors.brandDim,
    color: colors.brand, display: 'grid', placeItems: 'center',
    fontSize: 22, fontWeight: 700,
  },
  form: {
    maxWidth: 480, background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '28px 32px',
  },
  sectionTitle: { margin: '0 0 16px', fontSize: 16, fontWeight: 600 },
  toggleRow: {
    display: 'flex', alignItems: 'flex-start', gap: 12,
    padding: '14px 0 4px',
  },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, maxWidth: 480,
    border: `1px solid rgba(34,197,94,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, maxWidth: 480,
    border: `1px solid rgba(239,68,68,0.3)`,
  },
}
