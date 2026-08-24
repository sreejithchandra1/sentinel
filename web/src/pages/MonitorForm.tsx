import { FormEvent, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, Customer, Monitor, MonitorType, NotificationsSummary } from '../api'
import DeleteMonitorButton from '../components/DeleteMonitorButton'
import FormModal from '../components/FormModal'
import { useAuth } from '../context/AuthContext'
import { colors } from '../theme'
import { numberFieldValue, parseNumberInput } from '../utils/numberInput'

const PORT_PRESETS = [
  { label: 'SSH (22)', port: 22 },
  { label: 'SMTP (25)', port: 25 },
  { label: 'DNS (53)', port: 53 },
  { label: 'HTTP (80)', port: 80 },
  { label: 'HTTPS (443)', port: 443 },
  { label: 'MySQL (3306)', port: 3306 },
  { label: 'PostgreSQL (5432)', port: 5432 },
]

const defaults: Partial<Monitor> = {
  type: 'http',
  method: 'GET',
  expected_status: 200,
  interval_seconds: 60,
  timeout_ms: 10000,
  slow_threshold_ms: 3000,
  follow_redirects: true,
  enabled: true,
  invert: false,
  alert_after_failures: 2,
  notify_email: true,
  notify_slack: true,
  notify_webhooks: true,
}

export default function MonitorForm({
  monitorId,
  onClose,
  onSaved,
}: {
  monitorId?: string
  onClose?: () => void
  onSaved?: () => void
} = {}) {
  const params = useParams<{ id: string }>()
  const navigate = useNavigate()
  const id = monitorId ?? params.id
  const { isPlatformAdmin } = useAuth()
  const [form, setForm] = useState<Partial<Monitor>>(defaults)
  const [error, setError] = useState('')
  const [statusRange, setStatusRange] = useState(false)
  const [dnsRecords, setDnsRecords] = useState('A,AAAA,MX,TXT,NS,CNAME')
  const [graceSeconds, setGraceSeconds] = useState<number | undefined>(60)
  const [tagsInput, setTagsInput] = useState('')
  const [enablePerformance, setEnablePerformance] = useState(false)
  const [customers, setCustomers] = useState<Customer[]>([])
  const [summary, setSummary] = useState<NotificationsSummary | null>(null)

  const monitorType = (form.type || 'http') as MonitorType
  const showPerformanceToggle = !id && monitorType === 'http'

  useEffect(() => {
    if (monitorType !== 'http') setEnablePerformance(false)
  }, [monitorType])

  useEffect(() => {
    if (!isPlatformAdmin) return
    api.listCustomers().then(setCustomers).catch(() => {})
  }, [isPlatformAdmin])

  useEffect(() => {
    api.getNotificationsSummary().then(s => {
      setSummary(s)
      if (!id) {
        setForm(prev => ({
          ...prev,
          notify_email: !!(s.email?.configured && s.email?.enabled),
          notify_slack: !!(s.slack?.configured && s.slack?.enabled),
          notify_webhooks: !!(s.webhooks?.configured && s.webhooks?.enabled),
        }))
      }
    }).catch(() => setSummary(null))
  }, [id])

  useEffect(() => {
    if (id) {
      api.getMonitor(id).then(m => {
        setForm(m)
        setTagsInput((m.tags || []).join(', '))
        setStatusRange(m.expected_status_min != null && m.expected_status_max != null)
        if (m.config) {
          try {
            const cfg = JSON.parse(m.config)
            if (cfg.dns_records) setDnsRecords(cfg.dns_records.join(','))
            if (cfg.grace_seconds) setGraceSeconds(cfg.grace_seconds)
          } catch { /* ignore */ }
        }
      })
    }
  }, [id])

  function set<K extends keyof Monitor>(key: K, value: Monitor[K] | undefined) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  const emailConfigured = !!summary?.email?.configured
  const emailGlobalOn = !!(summary?.email?.configured && summary?.email?.enabled)
  const slackConfigured = !!summary?.slack?.configured
  const slackGlobalOn = !!(summary?.slack?.configured && summary?.slack?.enabled)
  const webhooksConfigured = !!summary?.webhooks?.configured
  const webhooksGlobalOn = !!(summary?.webhooks?.configured && summary?.webhooks?.enabled)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    const payload: Partial<Monitor> = {
      ...form,
      notify_email: !!form.notify_email,
      notify_slack: !!form.notify_slack,
    }
    if (isPlatformAdmin) {
      payload.notify_webhooks = !!form.notify_webhooks
    } else {
      delete payload.notify_webhooks
    }
    payload.tags = tagsInput.split(',').map(s => s.trim()).filter(Boolean)
    if (monitorType === 'dns') {
      payload.config = JSON.stringify({
        dns_records: dnsRecords.split(',').map(s => s.trim().toUpperCase()).filter(Boolean),
      })
    }
    if (monitorType === 'heartbeat') {
      payload.config = JSON.stringify({ grace_seconds: graceSeconds ?? 60 })
      payload.url = ''
    }
    try {
      if (id) {
        await api.updateMonitor(id, payload)
        if (onSaved) onSaved()
        else navigate(`/monitors/${id}`)
        return
      }

      const created = await api.createMonitor(payload)
      if (enablePerformance && created.url?.startsWith('http')) {
        try {
          await api.createPerformanceTarget({
            name: created.name,
            url: created.url,
            method: created.method || 'GET',
            interval_seconds: Math.max(created.interval_seconds || 60, 60),
            timeout_ms: created.timeout_ms || 10000,
            slow_threshold_ms: created.slow_threshold_ms || 3000,
            follow_redirects: created.follow_redirects ?? true,
            alert_emails: created.alert_emails || '',
            tenant_id: created.tenant_id || '',
            enabled: true,
          })
        } catch (perfErr) {
          const msg = perfErr instanceof Error
            ? `Monitor created, but performance target failed: ${perfErr.message}`
            : 'Monitor created, but performance target failed'
          setError(msg)
          setTimeout(() => {
            if (onSaved) onSaved()
            else navigate(`/monitors/${created.id}`)
          }, 2500)
          return
        }
      }
      if (onSaved) onSaved()
      else navigate(`/monitors/${created.id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  function handleClose() {
    if (onClose) onClose()
    else if (id) navigate(`/monitors/${id}`)
    else navigate('/')
  }

  return (
    <FormModal
      title={id ? 'Edit Monitor' : 'Add Monitor'}
      subtitle="Configure check type, target, and alert settings."
      onClose={handleClose}
      wide
    >
      {error && <div className="flash-error" role="alert">{error}</div>}
      <form onSubmit={handleSubmit} style={styles.form}>
        <Field label="Monitor Type">
          <select value={monitorType} onChange={e => set('type', e.target.value as MonitorType)} className="input">
            <option value="http">HTTP / HTTPS</option>
            <option value="port">Port (TCP)</option>
            <option value="ssl">SSL Certificate</option>
            <option value="dns">DNS Records</option>
            <option value="heartbeat">Heartbeat (Cron)</option>
          </select>
        </Field>
        <Field label="Name">
          <input required value={form.name || ''} onChange={e => set('name', e.target.value)} className="input" />
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
                <option key={c.id} value={c.id}>
                  {c.name} ({c.monitor_count ?? 0}/{c.monitor_quota})
                </option>
              ))}
            </select>
          </Field>
        )}

        {monitorType === 'http' && (
          <>
            <Field label="URL">
              <input required type="url" value={form.url || ''} onChange={e => set('url', e.target.value)} className="input" placeholder="https://example.com" />
            </Field>
            <Field label="Method">
              <select value={form.method || 'GET'} onChange={e => set('method', e.target.value)} className="input">
                <option>GET</option><option>POST</option><option>HEAD</option>
              </select>
            </Field>
            <Field label="Status Code Validation">
              <label style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 8 }}>
                <input type="checkbox" checked={statusRange} onChange={e => setStatusRange(e.target.checked)} />
                Use range (e.g. 200-299)
              </label>
              {statusRange ? (
                <div style={{ display: 'flex', gap: 8 }}>
                  <input type="number" value={numberFieldValue(form.expected_status_min)} onChange={e => set('expected_status_min', parseNumberInput(e.target.value))} className="input" />
                  <input type="number" value={numberFieldValue(form.expected_status_max)} onChange={e => set('expected_status_max', parseNumberInput(e.target.value))} className="input" />
                </div>
              ) : (
                <input type="number" value={numberFieldValue(form.expected_status)} onChange={e => set('expected_status', parseNumberInput(e.target.value))} className="input" />
              )}
            </Field>
            <Field label="Keyword must exist">
              <input value={form.keyword_must_exist || ''} onChange={e => set('keyword_must_exist', e.target.value)} className="input" />
            </Field>
            <Field label="Keyword must not exist">
              <input value={form.keyword_must_not_exist || ''} onChange={e => set('keyword_must_not_exist', e.target.value)} className="input" />
            </Field>
            <div className="form-row">
              <Field label="HTTP Basic Auth username">
                <input
                  autoComplete="off"
                  value={form.http_username || ''}
                  onChange={e => set('http_username', e.target.value)}
                  className="input"
                  placeholder="Optional — htpasswd user"
                />
              </Field>
              <Field label="HTTP Basic Auth password">
                <input
                  type="password"
                  autoComplete="new-password"
                  value={form.http_password || ''}
                  onChange={e => set('http_password', e.target.value)}
                  className="input"
                  placeholder={form.http_auth_set ? 'Leave blank to keep current password' : 'Optional'}
                />
              </Field>
            </div>
            <p style={{ color: colors.textMuted, fontSize: 14, margin: '-8px 0 12px' }}>
              Sent as an Authorization header for htpasswd-protected sites. Clear the username to disable.
            </p>
            <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <input type="checkbox" checked={form.follow_redirects ?? true} onChange={e => set('follow_redirects', e.target.checked)} />
              Follow redirects
            </label>
          </>
        )}

        {monitorType === 'port' && (
          <>
            <Field label="Host">
              <input required value={form.url || ''} onChange={e => set('url', e.target.value)} className="input" placeholder="example.com" />
            </Field>
            <Field label="Port">
              <input required type="number" value={numberFieldValue(form.port)} onChange={e => set('port', parseNumberInput(e.target.value))} className="input" />
            </Field>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {PORT_PRESETS.map(p => (
                <button key={p.port} type="button" onClick={() => set('port', p.port)} style={styles.preset}>
                  {p.label}
                </button>
              ))}
            </div>
          </>
        )}

        {monitorType === 'ssl' && (
          <>
            <Field label="Hostname">
              <input required value={form.url || ''} onChange={e => set('url', e.target.value)} className="input" placeholder="example.com" />
            </Field>
            <Field label="Port (default 443)">
              <input type="number" value={numberFieldValue(form.port)} onChange={e => set('port', parseNumberInput(e.target.value))} className="input" />
            </Field>
          </>
        )}

        {monitorType === 'dns' && (
          <>
            <Field label="Domain">
              <input required value={form.url || ''} onChange={e => set('url', e.target.value)} className="input" placeholder="example.com" />
            </Field>
            <Field label="Record types (comma-separated)">
              <input value={dnsRecords} onChange={e => setDnsRecords(e.target.value)} className="input" placeholder="A,AAAA,MX,TXT,NS,CNAME" />
            </Field>
          </>
        )}

        {monitorType === 'heartbeat' && (
          <>
            <Field label="Grace period (seconds)">
              <input type="number" min={30} className="input" value={numberFieldValue(graceSeconds)}
                onChange={e => setGraceSeconds(parseNumberInput(e.target.value))} />
              <p style={{ color: colors.textMuted, fontSize: 14, margin: '8px 0 0' }}>
                Alert if no ping received within this window after the last heartbeat.
              </p>
            </Field>
            {form.heartbeat_token && (
              <Field label="Ping URL">
                <input readOnly className="input" value={`${window.location.origin}/api/heartbeat/${form.heartbeat_token}`} />
                <p style={{ color: colors.textMuted, fontSize: 14, margin: '8px 0 0' }}>
                  Call this URL (GET or POST) from your cron job or script on schedule.
                </p>
              </Field>
            )}
          </>
        )}

        <Field label="Tags (comma-separated)">
          <input className="input" value={tagsInput} onChange={e => setTagsInput(e.target.value)} placeholder="production, api" />
        </Field>

        <div className="form-row">
          <Field label="Interval (seconds)">
            <input type="number" min={30} value={numberFieldValue(form.interval_seconds)} onChange={e => set('interval_seconds', parseNumberInput(e.target.value))} className="input" />
          </Field>
          <Field label="Timeout (ms)">
            <input type="number" value={numberFieldValue(form.timeout_ms)} onChange={e => set('timeout_ms', parseNumberInput(e.target.value))} className="input" />
          </Field>
        </div>
        <Field label="Alert after consecutive failures">
          <input
            type="number"
            min={1}
            className="input"
            value={numberFieldValue(form.alert_after_failures)}
            onChange={e => set('alert_after_failures', parseNumberInput(e.target.value))}
          />
          <p style={{ color: colors.textMuted, fontSize: 14, margin: '8px 0 0' }}>
            Send a DOWN alert after this many failed checks in a row, and wait for the same number of successful checks before RECOVERY (default 2). Only one DOWN email per outage.
          </p>
        </Field>

        <div style={styles.notifyBox}>
          <div style={styles.notifyTitle}>Notify via</div>
          <p style={{ color: colors.textMuted, fontSize: 14, margin: '0 0 12px' }}>
            Defaults follow Settings → Notifications. Turn a channel off to skip it for this monitor only.
          </p>
          <NotifyToggle
            label="Email"
            checked={!!form.notify_email}
            disabled={!emailConfigured || !emailGlobalOn}
            hint={!emailConfigured ? 'Not configured' : !emailGlobalOn ? 'Disabled globally' : undefined}
            onChange={v => set('notify_email', v)}
          />
          {!!form.notify_email && emailGlobalOn && (
            <Field label="Alert emails (comma-separated, optional)">
              <input
                value={form.alert_emails || ''}
                onChange={e => set('alert_emails', e.target.value)}
                className="input"
                placeholder="Leave blank to use default recipients"
              />
            </Field>
          )}
          <NotifyToggle
            label="Slack"
            checked={!!form.notify_slack}
            disabled={!slackConfigured || !slackGlobalOn}
            hint={!slackConfigured ? 'Not configured' : !slackGlobalOn ? 'Disabled globally' : undefined}
            onChange={v => set('notify_slack', v)}
          />
          {isPlatformAdmin && (
            <NotifyToggle
              label="Webhooks"
              checked={!!form.notify_webhooks}
              disabled={!webhooksConfigured || !webhooksGlobalOn}
              hint={!webhooksConfigured ? 'Not configured' : !webhooksGlobalOn ? 'Disabled globally' : undefined}
              onChange={v => set('notify_webhooks', v)}
            />
          )}
        </div>

        <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input type="checkbox" checked={form.invert ?? false} onChange={e => set('invert', e.target.checked)} />
          Invert status (treat DOWN as UP — for monitoring unreachable hosts)
        </label>
        {showPerformanceToggle && (
          <label style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
            <input
              type="checkbox"
              checked={enablePerformance}
              onChange={e => setEnablePerformance(e.target.checked)}
              style={{ marginTop: 3 }}
            />
            <span>
              Also enable performance monitoring
              <span style={{ display: 'block', color: colors.textMuted, fontSize: 14, marginTop: 4 }}>
                Creates a separate performance target for latency tracking on this URL.
              </span>
            </span>
          </label>
        )}
        <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <input type="checkbox" checked={form.enabled ?? true} onChange={e => set('enabled', e.target.checked)} />
          Enabled
        </label>
        <div className="form-modal-actions">
          <button type="button" className="btn" onClick={handleClose}>Cancel</button>
          <button type="submit" className="btn btn-primary">{id ? 'Save Changes' : 'Create Monitor'}</button>
        </div>
        {id && form.name && (
          <div style={{ marginTop: 8 }}>
            <DeleteMonitorButton id={id} name={form.name} variant="danger" />
          </div>
        )}
      </form>
    </FormModal>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span className="field-label">{label}</span>
      {children}
    </label>
  )
}

function NotifyToggle({
  label,
  checked,
  disabled,
  hint,
  onChange,
}: {
  label: string
  checked: boolean
  disabled?: boolean
  hint?: string
  onChange: (v: boolean) => void
}) {
  return (
    <label style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: 12,
      marginBottom: 10,
      opacity: disabled ? 0.55 : 1,
      cursor: disabled ? 'not-allowed' : 'pointer',
    }}>
      <span style={{ fontSize: 15 }}>
        {label}
        {hint && <span style={{ color: colors.textMuted, marginLeft: 8, fontSize: 13 }}>{hint}</span>}
      </span>
      <span style={styles.switch}>
        <input
          type="checkbox"
          role="switch"
          aria-checked={checked}
          checked={checked}
          disabled={disabled}
          onChange={e => onChange(e.target.checked)}
          style={styles.switchInput}
        />
        <span
          aria-hidden
          style={{
            ...styles.switchTrack,
            background: checked && !disabled ? colors.brand : colors.borderLight,
          }}
        >
          <span style={{
            ...styles.switchThumb,
            transform: checked ? 'translateX(18px)' : 'translateX(0)',
          }} />
        </span>
      </span>
    </label>
  )
}

const styles: Record<string, React.CSSProperties> = {
  form: {
    display: 'grid', gap: 16,
  },
  error: {
    background: colors.redDim, color: colors.red,
    padding: 12, borderRadius: 8, marginBottom: 16,
    border: `1px solid rgba(248,81,73,0.3)`,
  },
  preset: {
    padding: '6px 14px', borderRadius: 20, border: `1px solid ${colors.border}`,
    background: colors.bgElevated, color: colors.brand, fontSize: 14,
  },
  notifyBox: {
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    padding: 14,
    background: colors.bgElevated || colors.card,
  },
  notifyTitle: { fontSize: 15, fontWeight: 600, marginBottom: 4 },
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
}
