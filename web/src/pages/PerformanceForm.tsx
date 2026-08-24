import { FormEvent, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, Customer, PerformanceTarget } from '../api'
import ConfirmDialog from '../components/ConfirmDialog'
import FormModal from '../components/FormModal'
import { useAuth } from '../context/AuthContext'
import { colors } from '../theme'
import { numberFieldValue, parseNumberInput } from '../utils/numberInput'

const defaults: Partial<PerformanceTarget> = {
  method: 'GET',
  interval_seconds: 300,
  timeout_ms: 10000,
  slow_threshold_ms: 3000,
  follow_redirects: true,
  enabled: true,
  alert_after_slow: 1,
}

export default function PerformanceForm({
  targetId,
  onClose,
  onSaved,
}: {
  targetId?: string
  onClose?: () => void
  onSaved?: () => void
} = {}) {
  const params = useParams<{ id: string }>()
  const navigate = useNavigate()
  const id = targetId ?? params.id
  const { isPlatformAdmin } = useAuth()
  const [form, setForm] = useState<Partial<PerformanceTarget>>(defaults)
  const [error, setError] = useState('')
  const [customers, setCustomers] = useState<Customer[]>([])
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (id) api.getPerformanceTarget(id).then(setForm).catch(() => {})
  }, [id])

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(setCustomers).catch(() => {})
  }, [isPlatformAdmin])

  function set<K extends keyof PerformanceTarget>(key: K, value: PerformanceTarget[K] | undefined) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      if (id) {
        await api.updatePerformanceTarget(id, form)
        if (onSaved) onSaved()
        else navigate(`/performance/${id}`)
      } else {
        const created = await api.createPerformanceTarget(form)
        if (onSaved) onSaved()
        else navigate(`/performance/${created.id}`)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  async function confirmDelete() {
    if (!id) return
    setBusy(true)
    setError('')
    try {
      await api.deletePerformanceTarget(id)
      setDeleteOpen(false)
      if (onSaved) onSaved()
      else navigate('/performance')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
      setBusy(false)
    }
  }

  function handleClose() {
    if (onClose) onClose()
    else if (id) navigate(`/performance/${id}`)
    else navigate('/performance')
  }

  return (
    <FormModal
      title={id ? 'Edit Target' : 'Add Performance Target'}
      subtitle="Track response time and latency for a website — independent of uptime monitoring."
      onClose={handleClose}
    >
      {error && <div className="flash-error" role="alert">{error}</div>}
      <form onSubmit={handleSubmit} style={styles.form}>
        <Field label="Name">
          <input required className="input" value={form.name || ''} onChange={e => set('name', e.target.value)} />
        </Field>
        {isPlatformAdmin && (
          <Field label="Customer">
            <select
              className="input"
              value={form.tenant_id || ''}
              onChange={e => set('tenant_id', e.target.value)}
            >
              <option value="">Internal (unassigned)</option>
              {customers.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </Field>
        )}
        <Field label="URL">
          <input required type="url" className="input" value={form.url || ''} onChange={e => set('url', e.target.value)} placeholder="https://example.com" />
        </Field>
        <Field label="Method">
          <select className="input" value={form.method || 'GET'} onChange={e => set('method', e.target.value)}>
            <option value="GET">GET</option>
            <option value="HEAD">HEAD</option>
          </select>
        </Field>
        <div className="form-row">
          <Field label="Check Interval (seconds)">
            <input type="number" className="input" value={numberFieldValue(form.interval_seconds)} onChange={e => set('interval_seconds', parseNumberInput(e.target.value))} />
          </Field>
          <Field label="Slow Threshold (ms)">
            <input type="number" className="input" value={numberFieldValue(form.slow_threshold_ms)} onChange={e => set('slow_threshold_ms', parseNumberInput(e.target.value))} />
          </Field>
          <Field label="Timeout (ms)">
            <input type="number" className="input" value={numberFieldValue(form.timeout_ms)} onChange={e => set('timeout_ms', parseNumberInput(e.target.value))} />
          </Field>
          <Field label="Alert after consecutive slow checks">
            <input
              type="number"
              min={1}
              className="input"
              value={numberFieldValue(form.alert_after_slow)}
              onChange={e => set('alert_after_slow', parseNumberInput(e.target.value))}
            />
          </Field>
        </div>
        <p style={{ color: colors.textMuted, fontSize: 14, margin: '-8px 0 16px' }}>
          Send a SLOW alert only after this many slow checks in a row (default 1).
        </p>
        <label style={styles.checkbox}>
          <input type="checkbox" checked={form.follow_redirects ?? true} onChange={e => set('follow_redirects', e.target.checked)} />
          <span>Follow redirects</span>
        </label>
        <Field label="Alert emails (comma-separated)">
          <input className="input" value={form.alert_emails || ''} onChange={e => set('alert_emails', e.target.value)} />
        </Field>
        {id && (
          <label style={{ ...styles.checkbox, marginTop: 12 }}>
            <input type="checkbox" checked={form.enabled ?? true} onChange={e => set('enabled', e.target.checked)} />
            <span>Enabled</span>
          </label>
        )}
        <div className="form-modal-actions">
          <button type="button" className="btn" onClick={handleClose}>Cancel</button>
          <button type="submit" className="btn btn-primary">Save</button>
        </div>
        {id && (
          <div style={{ marginTop: 8 }}>
            <button type="button" className="btn" onClick={() => setDeleteOpen(true)} style={{ color: colors.red }}>Delete</button>
          </div>
        )}
      </form>

      <ConfirmDialog
        open={deleteOpen}
        title="Delete performance target?"
        message={`Delete “${form.name || 'this target'}”? Latency history for this target will be removed.`}
        confirmLabel="Delete"
        danger
        busy={busy}
        onConfirm={confirmDelete}
        onCancel={() => { if (!busy) setDeleteOpen(false) }}
      />
    </FormModal>
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
  form: {
    display: 'grid', gap: 4,
  },
  checkbox: { display: 'flex', gap: 10, alignItems: 'center', fontSize: 15, color: colors.textMuted },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(248,81,73,0.3)`,
  },
}
