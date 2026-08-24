import { FormEvent, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, Monitor, StatusPageConfig } from '../../api'
import { colors } from '../../theme'

function normalizeConfig(c: Partial<StatusPageConfig> | null | undefined): StatusPageConfig {
  return {
    enabled: !!c?.enabled,
    title: c?.title?.trim() || 'System Status',
    monitor_ids: Array.isArray(c?.monitor_ids) ? c!.monitor_ids : [],
  }
}

export default function SettingsStatusPage() {
  const [cfg, setCfg] = useState<StatusPageConfig>({ enabled: false, title: 'System Status', monitor_ids: [] })
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    Promise.all([api.getStatusPageConfig(), api.monitors()])
      .then(([c, m]) => {
        if (cancelled) return
        setCfg(normalizeConfig(c))
        setMonitors(m)
        setError('')
      })
      .catch(err => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load status page settings')
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [])

  function toggleMonitor(id: string) {
    setCfg(c => {
      const ids = c.monitor_ids || []
      return {
        ...c,
        monitor_ids: ids.includes(id) ? ids.filter(x => x !== id) : [...ids, id],
      }
    })
  }

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const saved = await api.putStatusPageConfig(normalizeConfig(cfg))
      setCfg(normalizeConfig(saved))
      setMessage('Status page settings saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  if (loading) {
    return <div style={{ color: colors.textMuted, padding: '12px 0' }}>Loading status page settings…</div>
  }

  return (
    <>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}
      <form onSubmit={handleSave} style={styles.card}>
        <h3 style={styles.title}>Public Status Page</h3>
        <p style={styles.desc}>
          Available at{' '}
          <Link to="/status" target="_blank" rel="noreferrer" style={{ color: colors.brand }}>
            /status
          </Link>
          {' '}when enabled. No login required.
        </p>
        <label style={styles.check}>
          <input
            type="checkbox"
            checked={cfg.enabled}
            onChange={e => setCfg(c => ({ ...c, enabled: e.target.checked }))}
          />
          Enable public status page
        </label>
        <label className="field" style={{ marginTop: 16 }}>
          <span className="field-label">Page title</span>
          <input
            className="input"
            value={cfg.title}
            onChange={e => setCfg(c => ({ ...c, title: e.target.value }))}
          />
        </label>
        <div style={{ marginTop: 20 }}>
          <div className="field-label" style={{ marginBottom: 12 }}>Monitors to display</div>
          {monitors.length === 0 ? (
            <p style={{ color: colors.textMuted, fontSize: 15, margin: 0 }}>
              No monitors yet. Add monitors first, then choose which ones appear on the status page.
            </p>
          ) : (
            <div style={styles.list}>
              {monitors.map(m => (
                <label key={m.id} style={styles.item}>
                  <input
                    type="checkbox"
                    checked={(cfg.monitor_ids || []).includes(m.id)}
                    onChange={() => toggleMonitor(m.id)}
                  />
                  <span>
                    <span style={{ fontWeight: 500 }}>{m.name}</span>
                    <span style={{ color: colors.textMuted, marginLeft: 8, fontSize: 13 }}>{m.type}</span>
                  </span>
                </label>
              ))}
            </div>
          )}
        </div>
        <button type="submit" className="btn btn-primary" style={{ marginTop: 20 }}>Save</button>
      </form>
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: { background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10, padding: 28, maxWidth: 640 },
  title: { margin: '0 0 8px' },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 20px' },
  check: { display: 'flex', gap: 8, alignItems: 'center', fontSize: 15 },
  list: { display: 'grid', gap: 8, maxHeight: 280, overflow: 'auto' },
  item: { display: 'flex', gap: 10, alignItems: 'center', fontSize: 15 },
  ok: { background: 'rgba(34,197,94,0.15)', color: colors.green, padding: 12, borderRadius: 8, marginBottom: 16 },
  error: { background: colors.redDim, color: colors.red, padding: 12, borderRadius: 8, marginBottom: 16 },
}
