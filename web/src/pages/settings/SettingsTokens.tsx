import { FormEvent, useEffect, useRef, useState } from 'react'
import { api, APIToken, APITokenCreated } from '../../api'
import { ColGroup, ResizableTh, useColumnResize } from '../../components/ColumnResize'
import ConfirmDialog from '../../components/ConfirmDialog'
import KebabMenu from '../../components/KebabMenu'
import { colors } from '../../theme'

export default function SettingsTokens() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [name, setName] = useState('')
  const [created, setCreated] = useState<APITokenCreated | null>(null)
  const [error, setError] = useState('')
  const [revokeId, setRevokeId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('tokens', 5)

  async function load() {
    setTokens(await api.listTokens())
  }

  useEffect(() => { load().catch(() => {}) }, [])

  async function handleCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    setCreated(null)
    try {
      const t = await api.createToken(name)
      setCreated(t)
      setName('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  async function confirmRevoke() {
    if (!revokeId) return
    setBusy(true)
    setError('')
    try {
      await api.deleteToken(revokeId)
      setRevokeId(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Revoke failed')
    } finally {
      setBusy(false)
    }
  }

  const revokeTarget = tokens.find(t => t.id === revokeId)

  return (
    <>
      {error && <div style={styles.error} role="alert">{error}</div>}
      {created && (
        <div style={styles.tokenBox}>
          <strong>Token created — copy now, it won&apos;t be shown again:</strong>
          <code style={styles.token}>{created.token}</code>
        </div>
      )}
      <form onSubmit={handleCreate} style={styles.card}>
        <h3 style={styles.title}>API Tokens</h3>
        <p style={styles.desc}>Use Bearer tokens for programmatic API access.</p>
        <div style={{ display: 'flex', gap: 12 }}>
          <input required className="input" placeholder="Token name" value={name} onChange={e => setName(e.target.value)} style={{ flex: 1 }} />
          <button type="submit" className="btn btn-primary">Create token</button>
        </div>
      </form>
      <div style={{ ...styles.card, marginTop: 24 }}>
        {tokens.length === 0 ? (
          <p style={{ ...styles.desc, marginBottom: 0 }}>No API tokens yet.</p>
        ) : (
          <div className="data-table-wrap" style={styles.tableWrap}>
            <table ref={tableRef} className="data-table" style={styles.table}>
              <ColGroup widths={widths} />
              <thead>
                <tr>
                  <ResizableTh index={0} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Name</ResizableTh>
                  <ResizableTh index={1} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Prefix</ResizableTh>
                  <ResizableTh index={2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Created</ResizableTh>
                  <ResizableTh index={3} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Last used</ResizableTh>
                  <ResizableTh index={4} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                </tr>
              </thead>
              <tbody>
                {tokens.map(t => (
                  <tr key={t.id}>
                    <td style={styles.td}>{t.name}</td>
                    <td style={{ ...styles.td, fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 14, color: colors.textMuted }}>
                      {t.prefix}…
                    </td>
                    <td style={styles.td}>{new Date(t.created_at).toLocaleString()}</td>
                    <td style={styles.td}>{t.last_used_at ? new Date(t.last_used_at).toLocaleString() : '—'}</td>
                    <td className="col-actions">
                      <KebabMenu>
                        {close => (
                          <button
                            type="button"
                            className="kebab-danger"
                            onClick={() => {
                              close()
                              setRevokeId(t.id)
                            }}
                          >
                            Revoke
                          </button>
                        )}
                      </KebabMenu>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={!!revokeId}
        title="Revoke API token?"
        message={
          revokeTarget
            ? `Revoke “${revokeTarget.name}”? Apps using this token will lose access immediately.`
            : 'Revoke this API token? Apps using it will lose access immediately.'
        }
        confirmLabel="Revoke"
        danger
        busy={busy}
        onConfirm={confirmRevoke}
        onCancel={() => { if (!busy) setRevokeId(null) }}
      />
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: { background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10, padding: 28 },
  title: { margin: '0 0 8px' },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 20px' },
  tableWrap: {},
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 15 },
  th: {
    textAlign: 'left',
    padding: '0 12px 12px',
    fontSize: 12,
    fontWeight: 600,
    color: colors.textMuted,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    borderBottom: `1px solid ${colors.border}`,
  },
  td: {
    padding: '14px 12px',
    borderBottom: `1px solid ${colors.border}`,
    verticalAlign: 'middle',
  },
  revokeBtn: {
    minHeight: 36,
    padding: '0 14px',
    fontSize: 14,
  },
  tokenBox: { background: colors.bgElevated, border: `1px solid ${colors.border}`, borderRadius: 8, padding: 16, marginBottom: 16 },
  token: { display: 'block', marginTop: 8, wordBreak: 'break-all', fontSize: 14 },
  error: { background: colors.redDim, color: colors.red, padding: 12, borderRadius: 8, marginBottom: 16 },
}
