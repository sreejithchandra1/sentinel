import { FormEvent, useEffect, useState } from 'react'
import { api, ServerSettings } from '../../api'
import { colors } from '../../theme'

export default function SettingsServer() {
  const [cfg, setCfg] = useState<ServerSettings>({ dashboard_url: '', retention_days: 30, workers: 10 })
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => { api.getServerSettings().then(setCfg).catch(() => {}) }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const saved = await api.putServerSettings(cfg)
      setCfg(saved)
      setMessage('Server settings saved (restart required for worker/retention changes)')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  return (
    <>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}
      <form onSubmit={handleSave} style={styles.card}>
        <h3 style={styles.title}>Server Settings</h3>
        <p style={styles.desc}>Stored in the database. Some values require a process restart to take effect.</p>
        <div style={styles.stack}>
          <label className="field">
            <span className="field-label">Dashboard URL</span>
            <input className="input" value={cfg.dashboard_url} onChange={e => setCfg(c => ({ ...c, dashboard_url: e.target.value }))} />
          </label>
          <div className="grid-2">
            <label className="field">
              <span className="field-label">Retention (days)</span>
              <input
                type="number"
                min={30}
                className="input"
                value={cfg.retention_days}
                onChange={e => setCfg(c => ({ ...c, retention_days: Math.max(30, +e.target.value || 30) }))}
              />
              <span style={styles.hint}>Minimum 30 days — check history is kept for this period.</span>
            </label>
            <label className="field">
              <span className="field-label">Workers</span>
              <input type="number" className="input" value={cfg.workers} onChange={e => setCfg(c => ({ ...c, workers: +e.target.value }))} />
            </label>
          </div>
        </div>
        <div style={styles.actions}>
          <button type="submit" className="btn btn-primary">Save</button>
        </div>
      </form>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: { background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10, padding: '32px' },
  title: { margin: '0 0 8px', fontSize: 17, fontWeight: 600 },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 24px', lineHeight: 1.5 },
  stack: { display: 'flex', flexDirection: 'column', gap: 20 },
  hint: { fontSize: 13, color: colors.textMuted, lineHeight: 1.45 },
  actions: { display: 'flex', justifyContent: 'flex-start', marginTop: 24 },
  ok: { background: 'rgba(34,197,94,0.15)', color: colors.green, padding: 12, borderRadius: 8, marginBottom: 16 },
  error: { background: colors.redDim, color: colors.red, padding: 12, borderRadius: 8, marginBottom: 16 },
}
