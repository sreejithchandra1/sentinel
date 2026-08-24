import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, NotificationsSummary, SlackConfig, SMTPConfig, WebhookConfig } from '../../api'
import { useAuth } from '../../context/AuthContext'
import { colors } from '../../theme'

export default function SettingsNotifications() {
  const { isPlatformAdmin } = useAuth()
  const [summary, setSummary] = useState<NotificationsSummary | null>(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      setSummary(await api.getNotificationsSummary())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load notifications')
    }
  }

  useEffect(() => { load() }, [])

  async function toggleSlack(enabled: boolean) {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const cfg = await api.getSlack()
      const next: SlackConfig = { ...cfg, enabled }
      await api.putSlack(next)
      setMessage(enabled ? 'Slack enabled' : 'Slack disabled')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update Slack')
    } finally {
      setBusy(false)
    }
  }

  async function toggleEmail(enabled: boolean) {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const cfg = await api.getSMTP()
      const next: SMTPConfig = { ...cfg, enabled }
      await api.putSMTP(next)
      setMessage(enabled ? 'Email enabled' : 'Email disabled')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update email')
    } finally {
      setBusy(false)
    }
  }

  async function toggleWebhooks(enabled: boolean) {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const hooks = await api.getWebhooks()
      const next: WebhookConfig[] = hooks.map(h => ({ ...h, enabled }))
      await api.putWebhooks(next.length ? next : [])
      setMessage(enabled ? 'Webhooks enabled' : 'Webhooks disabled')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update webhooks')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <div style={styles.card}>
        <h3 style={styles.title}>Notification endpoints</h3>
        <p style={styles.desc}>
          {isPlatformAdmin
            ? 'Enable Email, Slack, and webhooks independently. Alerts can go to multiple channels at once.'
            : 'Configure Slack for your customer account. Email uses the organization SMTP server and per-monitor recipients.'}
        </p>

        <div style={styles.grid}>
          {isPlatformAdmin && summary?.email && (
            <EndpointCard
              title="Email"
              description="SMTP delivery for down, recovery, and slow alerts."
              configured={summary.email.configured}
              enabled={summary.email.enabled}
              busy={busy}
              configureTo="/settings/notifications/email"
              onToggle={toggleEmail}
            />
          )}

          {summary?.slack && (
            <EndpointCard
              title="Slack"
              description={
                isPlatformAdmin
                  ? 'Incoming Webhook for platform-scoped monitors and targets.'
                  : 'Incoming Webhook for this customer’s monitors and targets.'
              }
              configured={summary.slack.configured}
              enabled={summary.slack.enabled}
              busy={busy}
              configureTo="/settings/notifications/slack"
              onToggle={toggleSlack}
            />
          )}

          {isPlatformAdmin && summary?.webhooks && (
            <EndpointCard
              title="Webhooks"
              description="Generic JSON webhooks for external systems."
              configured={summary.webhooks.configured}
              enabled={summary.webhooks.enabled}
              busy={busy}
              configureTo="/settings/notifications/webhooks"
              onToggle={toggleWebhooks}
            />
          )}
        </div>
      </div>
    </>
  )
}

function EndpointCard({
  title,
  description,
  configured,
  enabled,
  busy,
  configureTo,
  onToggle,
}: {
  title: string
  description: string
  configured: boolean
  enabled: boolean
  busy: boolean
  configureTo: string
  onToggle: (enabled: boolean) => void
}) {
  return (
    <div style={styles.endpoint}>
      <div style={styles.endpointTop}>
        <div>
          <h4 style={styles.endpointTitle}>{title}</h4>
          <p style={styles.endpointDesc}>{description}</p>
        </div>
        <label style={{
          ...styles.toggle,
          opacity: busy || !configured ? 0.5 : 1,
          cursor: busy || !configured ? 'not-allowed' : 'pointer',
        }}>
          <span style={styles.toggleLabel}>{enabled ? 'On' : 'Off'}</span>
          <span style={styles.switch}>
            <input
              type="checkbox"
              role="switch"
              aria-checked={enabled}
              checked={enabled}
              disabled={busy || !configured}
              onChange={e => onToggle(e.target.checked)}
              style={styles.switchInput}
            />
            <span
              aria-hidden
              style={{
                ...styles.switchTrack,
                background: enabled ? colors.brand : colors.borderLight,
              }}
            >
              <span style={{
                ...styles.switchThumb,
                transform: enabled ? 'translateX(18px)' : 'translateX(0)',
              }} />
            </span>
          </span>
        </label>
      </div>
      <div style={styles.endpointFooter}>
        <span style={configured ? styles.badgeOk : styles.badgeMuted}>
          {configured ? 'Configured' : 'Not configured'}
        </span>
        <Link to={configureTo} className="btn" style={styles.configureBtn}>
          Configure
        </Link>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px',
  },
  title: { margin: '0 0 8px', fontSize: 17, fontWeight: 600 },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 20px' },
  grid: { display: 'grid', gap: 14 },
  endpoint: {
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    padding: 16,
    background: colors.bgElevated || colors.card,
  },
  endpointTop: {
    display: 'flex',
    justifyContent: 'space-between',
    gap: 16,
    alignItems: 'flex-start',
    marginBottom: 14,
  },
  endpointTitle: { margin: '0 0 6px', fontSize: 16, fontWeight: 600 },
  endpointDesc: { margin: 0, color: colors.textMuted, fontSize: 14, lineHeight: 1.45 },
  toggle: { display: 'inline-flex', alignItems: 'center', gap: 10, fontSize: 14, whiteSpace: 'nowrap', userSelect: 'none' },
  toggleLabel: { color: colors.textMuted, fontWeight: 500, minWidth: 22 },
  switch: { position: 'relative', display: 'inline-flex', width: 42, height: 24, flexShrink: 0 },
  switchInput: {
    position: 'absolute',
    inset: 0,
    margin: 0,
    opacity: 0,
    width: '100%',
    height: '100%',
    cursor: 'inherit',
    zIndex: 1,
  },
  switchTrack: {
    display: 'block',
    width: 42,
    height: 24,
    borderRadius: 999,
    padding: 3,
    boxSizing: 'border-box',
    transition: 'background 0.15s ease',
  },
  switchThumb: {
    display: 'block',
    width: 18,
    height: 18,
    borderRadius: '50%',
    background: '#fff',
    boxShadow: '0 1px 2px rgba(0,0,0,0.35)',
    transition: 'transform 0.15s ease',
  },
  endpointFooter: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 },
  badgeOk: {
    fontSize: 12, fontWeight: 600, color: colors.green,
    background: colors.greenDim, padding: '3px 8px', borderRadius: 4,
  },
  badgeMuted: {
    fontSize: 12, fontWeight: 600, color: colors.textMuted,
    background: 'rgba(148,163,184,0.15)', padding: '3px 8px', borderRadius: 4,
  },
  configureBtn: { fontSize: 13, padding: '6px 12px', minHeight: 32, textDecoration: 'none' },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(34,197,94,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(239,68,68,0.3)`,
  },
}
