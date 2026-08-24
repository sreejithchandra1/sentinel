import { useEffect, useState } from 'react'
import { api, PublicStatusResponse } from '../api'
import StatusBadge from '../components/StatusBadge'
import { colors } from '../theme'

export default function StatusPage() {
  const [data, setData] = useState<PublicStatusResponse | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api.publicStatus()
      .then(setData)
      .catch(err => setError(err instanceof Error ? err.message : 'Status page unavailable'))
    const id = setInterval(() => {
      api.publicStatus().then(setData).catch(() => {})
    }, 30000)
    return () => clearInterval(id)
  }, [])

  if (error) {
    return (
      <div style={styles.shell}>
        <div style={styles.center}>{error}</div>
      </div>
    )
  }

  if (!data) {
    return <div style={styles.shell}><div style={styles.center}>Loading…</div></div>
  }

  const up = (data.monitors || []).filter(m => m.status === 'up').length

  return (
    <div style={styles.shell}>
      <div style={styles.container}>
        <header style={styles.header}>
          <h1 style={styles.title}>{data.title}</h1>
          <p style={styles.subtitle}>
            {up} of {(data.monitors || []).length} services operational
          </p>
        </header>
        <div style={styles.list} role="list">
          {(data.monitors || []).length === 0 ? (
            <div style={{ ...styles.row, justifyContent: 'center', color: colors.textMuted }}>
              No monitors are published on this status page yet.
            </div>
          ) : (
            (data.monitors || []).map(m => (
              <div key={m.id} className="status-row" style={styles.row} role="listitem">
                <div>
                  <div style={styles.name}>{m.name}</div>
                  {m.url && <div style={styles.url}>{m.url}</div>}
                  {m.last_checked_at && (
                    <div style={styles.meta}>Last check {new Date(m.last_checked_at).toLocaleString()}</div>
                  )}
                </div>
                <StatusBadge status={m.status as 'up' | 'down' | 'degraded' | 'unknown'} />
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  shell: { minHeight: '100vh', background: colors.bg, padding: '48px 24px' },
  center: { textAlign: 'center', color: colors.textMuted, paddingTop: 80 },
  container: { maxWidth: 720, margin: '0 auto' },
  header: { textAlign: 'center', marginBottom: 40 },
  title: { fontSize: 24, fontWeight: 600, margin: '0 0 8px', letterSpacing: '-0.02em' },
  subtitle: { color: colors.textMuted, margin: 0 },
  list: { display: 'grid', gap: 12 },
  row: {
    background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10, padding: '18px 20px',
  },
  name: { fontWeight: 600, fontSize: 17 },
  url: { color: colors.textMuted, fontSize: 14, marginTop: 4 },
  meta: { color: colors.textDim, fontSize: 13, marginTop: 6 },
}
