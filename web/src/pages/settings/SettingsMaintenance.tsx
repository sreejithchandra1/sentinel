import { FormEvent, useEffect, useRef, useState } from 'react'
import { api, MaintenanceWindow, Monitor } from '../../api'
import { ColGroup, ResizableTh, useColumnResize } from '../../components/ColumnResize'
import ConfirmDialog from '../../components/ConfirmDialog'
import KebabMenu from '../../components/KebabMenu'
import { colors } from '../../theme'

export default function SettingsMaintenance() {
  const [windows, setWindows] = useState<MaintenanceWindow[]>([])
  const [monitors, setMonitors] = useState<Monitor[]>([])
  const [name, setName] = useState('')
  const [monitorId, setMonitorId] = useState('')
  const [startsAt, setStartsAt] = useState('')
  const [endsAt, setEndsAt] = useState('')
  const [error, setError] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('maintenance', 4)

  async function load() {
    const [w, m] = await Promise.all([api.listMaintenance(), api.monitors()])
    setWindows(w)
    setMonitors(m)
  }

  useEffect(() => { load().catch(() => {}) }, [])

  async function handleCreate(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      await api.createMaintenance({
        name,
        monitor_id: monitorId || undefined,
        starts_at: new Date(startsAt).toISOString(),
        ends_at: new Date(endsAt).toISOString(),
      })
      setName('')
      setMonitorId('')
      setStartsAt('')
      setEndsAt('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Create failed')
    }
  }

  async function confirmDelete() {
    if (!deleteId) return
    setBusy(true)
    setError('')
    try {
      await api.deleteMaintenance(deleteId)
      setDeleteId(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setBusy(false)
    }
  }

  const deleteTarget = windows.find(w => w.id === deleteId)

  return (
    <>
      {error && <div style={styles.error} role="alert">{error}</div>}
      <form onSubmit={handleCreate} style={styles.card}>
        <h3 style={styles.title}>Schedule Maintenance</h3>
        <p style={styles.desc}>Suppress alerts during planned downtime. Leave monitor blank for global maintenance.</p>
        <div className="grid-2">
          <label className="field">
            <span className="field-label">Name</span>
            <input required className="input" value={name} onChange={e => setName(e.target.value)} />
          </label>
          <label className="field">
            <span className="field-label">Monitor (optional)</span>
            <select className="input" value={monitorId} onChange={e => setMonitorId(e.target.value)}>
              <option value="">All monitors</option>
              {monitors.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
            </select>
          </label>
          <label className="field">
            <span className="field-label">Starts</span>
            <input required type="datetime-local" className="input" value={startsAt} onChange={e => setStartsAt(e.target.value)} />
          </label>
          <label className="field">
            <span className="field-label">Ends</span>
            <input required type="datetime-local" className="input" value={endsAt} onChange={e => setEndsAt(e.target.value)} />
          </label>
        </div>
        <div style={styles.actions}>
          <button type="submit" className="btn btn-primary">Add window</button>
        </div>
      </form>

      <div style={{ ...styles.card, marginTop: 24 }}>
        <h3 style={styles.title}>Scheduled Windows</h3>
        {windows.length === 0 ? (
          <p style={styles.desc}>No maintenance windows scheduled.</p>
        ) : (
          <div className="data-table-wrap">
          <table ref={tableRef} className="data-table" style={styles.table}>
            <ColGroup widths={widths} />
            <thead><tr>
              <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Name</ResizableTh>
              <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Scope</ResizableTh>
              <ResizableTh index={2} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Period</ResizableTh>
              <ResizableTh index={3} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
            </tr></thead>
            <tbody>
              {windows.map(w => (
                <tr key={w.id}>
                  <td>{w.name}</td>
                  <td>{w.monitor_id ? monitors.find(m => m.id === w.monitor_id)?.name || w.monitor_id : 'All'}</td>
                  <td>{new Date(w.starts_at).toLocaleString()} – {new Date(w.ends_at).toLocaleString()}</td>
                  <td className="col-actions">
                    <KebabMenu>
                      {close => (
                        <button
                          type="button"
                          className="kebab-danger"
                          onClick={() => {
                            close()
                            setDeleteId(w.id)
                          }}
                        >
                          Delete
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
        open={!!deleteId}
        title="Delete maintenance window?"
        message={
          deleteTarget
            ? `Delete “${deleteTarget.name}”? Alerts will no longer be suppressed for this window.`
            : 'Delete this maintenance window?'
        }
        confirmLabel="Delete"
        danger
        busy={busy}
        onConfirm={confirmDelete}
        onCancel={() => { if (!busy) setDeleteId(null) }}
      />
    </>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: { background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10, padding: '28px 32px' },
  title: { margin: '0 0 8px', fontSize: 17, fontWeight: 600 },
  desc: { color: colors.textMuted, fontSize: 15, margin: '0 0 24px', lineHeight: 1.5 },
  actions: { display: 'flex', justifyContent: 'flex-start', marginTop: 24 },
  table: { width: '100%', borderCollapse: 'collapse', fontSize: 15 },
  error: { background: colors.redDim, color: colors.red, padding: 12, borderRadius: 8, marginBottom: 16 },
}
