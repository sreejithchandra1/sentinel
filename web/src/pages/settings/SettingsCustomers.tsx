import { FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { api, Customer } from '../../api'
import { ColGroup, ResizableTh, useColumnResize } from '../../components/ColumnResize'
import ConfirmDialog from '../../components/ConfirmDialog'
import KebabMenu from '../../components/KebabMenu'
import PageHeader from '../../components/PageHeader'
import { colors } from '../../theme'

export default function SettingsCustomers() {
  const [customers, setCustomers] = useState<Customer[]>([])
  const [name, setName] = useState('')
  const [quota, setQuota] = useState(1)
  const [editing, setEditing] = useState<Customer | null>(null)
  const [editName, setEditName] = useState('')
  const [editQuota, setEditQuota] = useState(1)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('customers', 3)

  async function load() {
    try {
      setCustomers(await api.listCustomers())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load customers')
    }
  }

  useEffect(() => { load() }, [])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return customers
    return customers.filter(c => c.name.toLowerCase().includes(q))
  }, [customers, search])

  async function handleAdd(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      await api.createCustomer({ name, monitor_quota: quota })
      setName('')
      setQuota(1)
      setMessage('Customer created')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create customer')
    }
  }

  function startEdit(c: Customer) {
    setEditing(c)
    setEditName(c.name)
    setEditQuota(c.monitor_quota)
  }

  async function handleSaveEdit(e: FormEvent) {
    e.preventDefault()
    if (!editing) return
    setError('')
    setMessage('')
    try {
      await api.updateCustomer(editing.id, { name: editName, monitor_quota: editQuota })
      setEditing(null)
      setMessage('Customer updated')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  async function confirmDelete() {
    if (!deleteId) return
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await api.deleteCustomer(deleteId)
      setDeleteId(null)
      setMessage('Customer deleted')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setBusy(false)
    }
  }

  const deleteTarget = customers.find(c => c.id === deleteId)

  return (
    <div className="page">
      <PageHeader
        title="Customers"
        subtitle="Group monitors by customer and set monitor quotas."
      />

      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <div className="split-panels">
        <div style={styles.card}>
          <h3 style={styles.cardTitle}>All customers</h3>
          <p style={{ color: colors.textMuted, fontSize: 15, margin: '0 0 16px' }}>
            Default quota is 1 monitor per customer.
          </p>

          {customers.length > 0 && (
            <input
              className="input"
              style={styles.search}
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search customers…"
              aria-label="Search customers"
              autoComplete="off"
            />
          )}

          {customers.length === 0 ? (
            <p style={{ color: colors.textMuted }}>No customers yet.</p>
          ) : filtered.length === 0 ? (
            <p style={{ color: colors.textMuted }}>No customers match “{search.trim()}”.</p>
          ) : (
            <div className="data-table-wrap">
              <table ref={tableRef} className="data-table">
                <ColGroup widths={widths} />
                <thead>
                  <tr>
                    <ResizableTh index={0} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Name</ResizableTh>
                    <ResizableTh index={1} startResize={startResize} autoFit={autoFit} tableRef={tableRef}>Usage</ResizableTh>
                    <ResizableTh index={2} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(c => (
                    <tr key={c.id}>
                      {editing?.id === c.id ? (
                        <td colSpan={3} style={{ overflow: 'visible', whiteSpace: 'normal' }}>
                          <form onSubmit={handleSaveEdit} className="customer-edit-row">
                            <input className="input input-compact" value={editName} onChange={e => setEditName(e.target.value)} required />
                            <input
                              className="input input-compact"
                              type="number"
                              min={1}
                              value={editQuota}
                              onChange={e => setEditQuota(+e.target.value)}
                              style={{ width: 88 }}
                            />
                            <button type="submit" className="btn btn-primary" style={styles.rowBtn}>Save</button>
                            <button type="button" className="btn" style={styles.rowBtn} onClick={() => setEditing(null)}>Cancel</button>
                          </form>
                        </td>
                      ) : (
                        <>
                          <td style={{ fontWeight: 600 }}>{c.name}</td>
                          <td style={{ color: colors.textMuted }}>
                            {c.monitor_count ?? 0} / {c.monitor_quota}
                          </td>
                          <td className="col-actions">
                            <KebabMenu>
                              {close => (
                                <>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      close()
                                      startEdit(c)
                                    }}
                                  >
                                    Edit
                                  </button>
                                  <button
                                    type="button"
                                    className="kebab-danger"
                                    onClick={() => {
                                      close()
                                      setDeleteId(c.id)
                                    }}
                                  >
                                    Delete
                                  </button>
                                </>
                              )}
                            </KebabMenu>
                          </td>
                        </>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {customers.length > 0 && (
            <p style={{ color: colors.textDim, fontSize: 13, margin: '14px 0 0' }}>
              Showing {filtered.length} of {customers.length} customers
            </p>
          )}
        </div>

        <form onSubmit={handleAdd} style={styles.card} autoComplete="off">
          <h3 style={styles.cardTitle}>Add Customer</h3>
          <div className="field" style={{ marginBottom: 16 }}>
            <label className="field-label" htmlFor="customer-name">Name</label>
            <input id="customer-name" className="input" value={name} onChange={e => setName(e.target.value)} required autoComplete="off" />
          </div>
          <div className="field" style={{ marginBottom: 16 }}>
            <label className="field-label" htmlFor="customer-quota">Monitor quota</label>
            <input id="customer-quota" className="input" type="number" min={1} value={quota} onChange={e => setQuota(+e.target.value)} />
          </div>
          <button type="submit" className="btn btn-primary" style={{ marginTop: 8 }}>Add Customer</button>
        </form>
      </div>

      <ConfirmDialog
        open={!!deleteId}
        title="Delete customer?"
        message={
          deleteTarget
            ? `Delete “${deleteTarget.name}”? Monitors will be unassigned, not deleted.`
            : 'Delete this customer? Monitors will be unassigned, not deleted.'
        }
        confirmLabel="Delete"
        danger
        busy={busy}
        onConfirm={confirmDelete}
        onCancel={() => { if (!busy) setDeleteId(null) }}
      />
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px', minWidth: 0,
  },
  cardTitle: { margin: '0 0 20px', fontSize: 17, fontWeight: 600 },
  search: { maxWidth: 360, width: '100%', marginBottom: 16 },
  rowBtn: { fontSize: 13, padding: '6px 10px', minHeight: 32 },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(34,197,94,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(239,68,68,0.3)`,
  },
}
