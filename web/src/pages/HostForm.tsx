import { FormEvent, useEffect, useState } from 'react'
import { api, Customer, Host } from '../api'
import FormModal from '../components/FormModal'
import { useAuth } from '../context/AuthContext'
import { colors } from '../theme'

export default function HostForm({
  onClose,
  onSaved,
}: {
  onClose: () => void
  onSaved: () => void
}) {
  const { isPlatformAdmin } = useAuth()
  const [name, setName] = useState('')
  const [tenantID, setTenantID] = useState('')
  const [customers, setCustomers] = useState<Customer[]>([])
  const [host, setHost] = useState<Host | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(setCustomers).catch(() => {})
  }, [isPlatformAdmin])

  useEffect(() => {
    if (!host || host.status === 'online') return
    const id = setInterval(() => {
      api.getHost(host.id).then(h => {
        setHost(prev => ({ ...h, install_command: prev?.install_command, enroll_token: prev?.enroll_token }))
      }).catch(() => {})
    }, 3000)
    return () => clearInterval(id)
  }, [host?.id, host?.status])

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      const created = await api.createHost({
        name: name.trim() || 'New host',
        tenant_id: tenantID || undefined,
      })
      setHost(created)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not add host')
    } finally {
      setSaving(false)
    }
  }

  async function copyCommand() {
    if (!host?.install_command) return
    try {
      await navigator.clipboard.writeText(host.install_command)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      setError('Could not copy command')
    }
  }

  if (host) {
    const waiting = host.status !== 'online'
    return (
      <FormModal
        title={host.name || 'New host'}
        subtitle={waiting ? 'Run this command on the host. The agent reports outbound — no extra ports.' : 'Agent connected.'}
        onClose={() => { onSaved(); onClose() }}
      >
        {error && <div className="flash-error" role="alert">{error}</div>}
        <label className="field">
          <span className="field-label">Install command</span>
          <textarea className="input" readOnly rows={3} value={host.install_command || ''} style={{ fontFamily: 'ui-monospace, monospace', fontSize: 13 }} />
        </label>
        <p style={{ color: colors.textMuted, fontSize: 14, lineHeight: 1.5, margin: '8px 0 16px' }}>
          sudo is install-only. The service runs as the unprivileged <code>sentinel-agent</code> user.
          The token expires in 15 minutes and can only be used once.
        </p>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
          <button type="button" className="btn btn-primary" onClick={copyCommand}>
            {copied ? 'Copied' : 'Copy command'}
          </button>
          <span style={{ color: waiting ? colors.yellow : colors.green, fontSize: 14, fontWeight: 600 }}>
            {waiting ? 'Waiting for agent…' : 'Online'}
          </span>
          {!waiting && (
            <button type="button" className="btn" onClick={() => { onSaved(); onClose() }}>Done</button>
          )}
        </div>
      </FormModal>
    )
  }

  return (
    <FormModal title="Add host" subtitle="Install a read-only agent that pushes CPU, memory, disk, and load." onClose={onClose}>
      {error && <div className="flash-error" role="alert">{error}</div>}
      <form onSubmit={submit}>
        <label className="field">
          <span className="field-label">Name</span>
          <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="web-1" />
        </label>
        {isPlatformAdmin && customers.length > 0 && (
          <label className="field">
            <span className="field-label">Customer</span>
            <select className="input" value={tenantID} onChange={e => setTenantID(e.target.value)}>
              <option value="">Platform</option>
              {customers.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </label>
        )}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 20 }}>
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={saving}>{saving ? 'Creating…' : 'Create host'}</button>
        </div>
      </form>
    </FormModal>
  )
}
