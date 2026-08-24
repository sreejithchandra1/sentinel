import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, SlackConfig } from '../../api'
import { useAuth } from '../../context/AuthContext'
import { colors } from '../../theme'

const defaultCfg = (): SlackConfig => ({
  webhook_url: '',
  enabled: false,
  events: ['all'],
})

export default function SettingsSlack() {
  const { isPlatformAdmin } = useAuth()
  const [cfg, setCfg] = useState<SlackConfig>(defaultCfg())
  const [eventsText, setEventsText] = useState('all')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api.getSlack().then(c => {
      setCfg(c)
      setEventsText((c.events || ['all']).join(', '))
    }).catch(() => {})
  }, [])

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const events = eventsText.split(',').map(s => s.trim()).filter(Boolean)
      const saved = await api.putSlack({
        ...cfg,
        events: events.length ? events : ['all'],
      })
      setCfg(saved)
      setEventsText((saved.events || ['all']).join(', '))
      setMessage('Slack settings saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setBusy(false)
    }
  }

  async function handleTest() {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await api.testSlack()
      setMessage('Test message sent to Slack')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Link to="/settings/notifications" style={styles.back}>← Notifications</Link>
      </div>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <form onSubmit={handleSave} style={styles.card}>
        <h3 style={styles.title}>Slack</h3>
        <p style={styles.desc}>
          {isPlatformAdmin
            ? 'Post alerts for platform monitors to a Slack Incoming Webhook.'
            : 'Post alerts for your customer’s monitors to a Slack Incoming Webhook.'}
          {' '}Create a webhook in Slack (Apps → Incoming Webhooks), then paste the URL below.
        </p>

        <label className="field" style={{ marginBottom: 16 }}>
          <span className="field-label">Webhook URL</span>
          <input
            className="input"
            value={cfg.webhook_url}
            onChange={e => setCfg(prev => ({ ...prev, webhook_url: e.target.value }))}
            placeholder="https://hooks.slack.com/services/..."
          />
        </label>

        <label className="field" style={{ marginBottom: 16 }}>
          <span className="field-label">Events</span>
          <input
            className="input"
            value={eventsText}
            onChange={e => setEventsText(e.target.value)}
            placeholder="all, DOWN, RECOVERY, SLOW, NORMAL"
          />
          <p style={{ color: colors.textMuted, fontSize: 13, margin: '8px 0 0' }}>
            Comma-separated. Use <code>all</code> or specific events like DOWN, RECOVERY, SLOW.
          </p>
        </label>

        <label style={styles.check}>
          <input
            type="checkbox"
            checked={cfg.enabled}
            onChange={e => setCfg(prev => ({ ...prev, enabled: e.target.checked }))}
          />
          <span>Enable Slack alerts</span>
        </label>

        <div style={{ display: 'flex', gap: 10, marginTop: 18 }}>
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? 'Saving…' : 'Save'}
          </button>
          <button type="button" className="btn" disabled={busy || !cfg.webhook_url.trim()} onClick={handleTest}>
            Send test
          </button>
        </div>
      </form>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  back: { color: colors.textMuted, fontSize: 14, textDecoration: 'none' },
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px', maxWidth: 640,
  },
  title: { margin: '0 0 8px', fontSize: 17, fontWeight: 600 },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 20px', lineHeight: 1.45 },
  check: { display: 'flex', gap: 10, alignItems: 'center', fontSize: 15, color: colors.textMuted },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(34,197,94,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(239,68,68,0.3)`,
  },
}
