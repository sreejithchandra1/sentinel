import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, SMTPConfig } from '../../api'
import { colors } from '../../theme'

export default function SettingsSMTP() {
  const [cfg, setCfg] = useState<SMTPConfig>({
    host: '', port: 587, username: '', password: '', from: '', alert_emails: '', tls: true, enabled: true,
  })
  const [testTo, setTestTo] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => { api.getSMTP().then(setCfg).catch(() => {}) }, [])

  function set<K extends keyof SMTPConfig>(key: K, value: SMTPConfig[K]) {
    setCfg(prev => ({ ...prev, [key]: value }))
  }

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const saved = await api.putSMTP(cfg)
      setCfg(saved)
      setMessage('Email settings saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  async function handleTest() {
    setError('')
    setMessage('')
    try {
      await api.testSMTP(testTo)
      setMessage('Test email sent')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Test failed'
      setError(/too many requests/i.test(msg) ? 'Too many requests — wait a minute and try again.' : msg)
    }
  }

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Link to="/settings/notifications" style={{ color: colors.textMuted, fontSize: 14, textDecoration: 'none' }}>
          ← Notifications
        </Link>
      </div>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <div className="split-panels">
        <form onSubmit={handleSave} style={styles.card}>
          <h3 style={styles.cardTitle}>Email (SMTP)</h3>
          <p style={{ color: colors.textMuted, fontSize: 15, margin: '0 0 20px' }}>
            Configure email delivery for down, slow, and recovery alerts.
          </p>
          <Field label="SMTP Host"><input className="input" value={cfg.host} onChange={e => set('host', e.target.value)} /></Field>
          <Field label="Port"><input className="input" type="number" value={cfg.port} onChange={e => set('port', +e.target.value)} /></Field>
          <Field label="Username"><input className="input" value={cfg.username} onChange={e => set('username', e.target.value)} /></Field>
          <Field label="Password"><input className="input" type="password" value={cfg.password} onChange={e => set('password', e.target.value)} placeholder="Leave blank to keep existing" /></Field>
          <Field label="From Address"><input className="input" value={cfg.from} onChange={e => set('from', e.target.value)} /></Field>
          <Field label="Alert Recipients">
            <input className="input" value={cfg.alert_emails || ''} onChange={e => set('alert_emails', e.target.value)} placeholder="you@example.com, team@example.com" />
          </Field>
          <p style={{ color: colors.textMuted, fontSize: 14, margin: '-8px 0 16px' }}>
            Monitoring alerts (down, recovery, slow) are sent here. If empty, alerts go to admin profile emails.
            From address is only used as the sender, not the recipient.
          </p>
          <label style={styles.checkbox}>
            <input type="checkbox" checked={cfg.tls} onChange={e => set('tls', e.target.checked)} />
            <span>Use TLS</span>
          </label>
          <label style={{ ...styles.checkbox, marginTop: 10 }}>
            <input type="checkbox" checked={!!cfg.enabled} onChange={e => set('enabled', e.target.checked)} />
            <span>Enable email alerts</span>
          </label>
          <button type="submit" className="btn btn-primary" style={{ marginTop: 16 }}>Save Settings</button>
        </form>

        <div style={styles.card}>
          <h3 style={styles.cardTitle}>Send Test Email</h3>
          <p style={{ color: colors.textMuted, fontSize: 15, margin: '0 0 16px' }}>Verify your SMTP configuration.</p>
          <Field label="Recipient">
            <input className="input" value={testTo} onChange={e => setTestTo(e.target.value)} placeholder="recipient@example.com" />
          </Field>
          <button type="button" onClick={handleTest} className="btn" style={{ marginTop: 8 }}>Send Test</button>
        </div>
      </div>
    </>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="field" style={{ marginBottom: 16 }}>
      <span className="field-label">{label}</span>
      {children}
    </label>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px', minWidth: 0,
  },
  cardTitle: { margin: '0 0 20px', fontSize: 17, fontWeight: 600 },
  checkbox: { display: 'flex', gap: 10, alignItems: 'center', fontSize: 15, color: colors.textMuted },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(63,185,80,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(248,81,73,0.3)`,
  },
}
