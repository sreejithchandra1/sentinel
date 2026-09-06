import { FormEvent, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, Customer, Monitor, MonitorType, NotificationsSummary, PerformanceTarget } from '../api'
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

const MONITOR_CHECKS: { key: MonitorType; label: string }[] = [
  { key: 'http', label: 'HTTP / HTTPS' },
  { key: 'ssl', label: 'SSL' },
  { key: 'dns', label: 'DNS' },
  { key: 'port', label: 'Port' },
  { key: 'heartbeat', label: 'Heartbeat' },
]

type CheckSet = Record<MonitorType, boolean> & { performance: boolean }

const emptyChecks = (httpOn: boolean): CheckSet => ({
  http: httpOn,
  ssl: false,
  dns: false,
  port: false,
  heartbeat: false,
  performance: false,
})

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

export function normalizeHost(raw: string): string {
  const trimmed = raw.trim().toLowerCase()
  if (!trimmed) return ''
  const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
  try {
    return new URL(withScheme).hostname.replace(/\.$/, '')
  } catch {
    return trimmed.replace(/^https?:\/\//, '').split('/')[0].split(':')[0]
  }
}

export function httpUrlFrom(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  return `https://${trimmed}`
}

function sameTenant(a?: string, b?: string): boolean {
  return (a || '') === (b || '')
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
  const [target, setTarget] = useState('')
  const [checks, setChecks] = useState<CheckSet>(() => emptyChecks(true))
  const [siblings, setSiblings] = useState<Partial<Record<MonitorType, Monitor>>>({})
  const [perfSibling, setPerfSibling] = useState<PerformanceTarget | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [statusRange, setStatusRange] = useState(false)
  const [dnsRecords, setDnsRecords] = useState('A,AAAA,MX,TXT,NS,CNAME')
  const [sslPort, setSslPort] = useState<number | undefined>(443)
  const [graceSeconds, setGraceSeconds] = useState<number | undefined>(60)
  const [tagsInput, setTagsInput] = useState('')
  const [customers, setCustomers] = useState<Customer[]>([])
  const [summary, setSummary] = useState<NotificationsSummary | null>(null)

  const canPerformance = /^https?:\/\//i.test(httpUrlFrom(target))
  const anyCheck = MONITOR_CHECKS.some(c => checks[c.key]) || checks.performance
  const needsTarget = checks.http || checks.ssl || checks.dns || checks.port || checks.performance

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
    if (!id) return
    let cancelled = false
    ;(async () => {
      try {
        const [m, mons, perfs] = await Promise.all([
          api.getMonitor(id),
          api.monitors(),
          api.performanceTargets().catch(() => [] as PerformanceTarget[]),
        ])
        if (cancelled) return
        const tenant = m.tenant_id || ''
        const host = normalizeHost(m.url || '')
        const next: Partial<Record<MonitorType, Monitor>> = {}
        for (const item of mons) {
          if (!sameTenant(item.tenant_id, tenant)) continue
          if (item.type === 'heartbeat') {
            if (m.type === 'heartbeat' && item.id === m.id) next.heartbeat = item
            continue
          }
          if (!host || normalizeHost(item.url || '') !== host) continue
          const t = (item.type || 'http') as MonitorType
          if (!next[t] || item.id === m.id) next[t] = item
        }
        if (m.type === 'heartbeat') next.heartbeat = m

        const preferredUrl = next.http?.url || m.url || ''
        const perf = perfs.find(p =>
          sameTenant(p.tenant_id, tenant) && (
            normalizeHost(p.url) === host
            || (preferredUrl && p.url === httpUrlFrom(preferredUrl))
          ),
        ) || null

        setForm({
          ...m,
          url: preferredUrl || m.url,
          port: next.port?.port ?? m.port,
          method: next.http?.method || m.method,
          expected_status: next.http?.expected_status ?? m.expected_status,
          expected_status_min: next.http?.expected_status_min ?? m.expected_status_min,
          expected_status_max: next.http?.expected_status_max ?? m.expected_status_max,
          keyword_must_exist: next.http?.keyword_must_exist ?? m.keyword_must_exist,
          keyword_must_not_exist: next.http?.keyword_must_not_exist ?? m.keyword_must_not_exist,
          request_body: next.http?.request_body ?? m.request_body,
          request_headers: next.http?.request_headers ?? m.request_headers,
          http_username: next.http?.http_username ?? m.http_username,
          http_password: next.http?.http_password ?? m.http_password,
          http_auth_set: next.http?.http_auth_set ?? m.http_auth_set,
          follow_redirects: next.http?.follow_redirects ?? m.follow_redirects,
          heartbeat_token: next.heartbeat?.heartbeat_token || m.heartbeat_token,
        })
        setTarget(m.type === 'heartbeat' ? '' : (preferredUrl || m.url || ''))
        setSslPort(next.ssl?.port ?? 443)
        setTagsInput((m.tags || []).join(', '))
        const http = next.http || (m.type === 'http' ? m : undefined)
        setStatusRange(!!(http?.expected_status_min != null && http?.expected_status_max != null))
        const cfgSource = next.dns?.config || next.heartbeat?.config || m.config
        if (cfgSource) {
          try {
            const cfg = JSON.parse(cfgSource)
            if (cfg.dns_records) setDnsRecords(cfg.dns_records.join(','))
            if (cfg.grace_seconds) setGraceSeconds(cfg.grace_seconds)
          } catch { /* ignore */ }
        }
        setSiblings(next)
        setPerfSibling(perf)
        setChecks({
          http: !!next.http?.enabled,
          ssl: !!next.ssl?.enabled,
          dns: !!next.dns?.enabled,
          port: !!next.port?.enabled,
          heartbeat: !!next.heartbeat?.enabled,
          performance: !!perf?.enabled,
        })
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load monitor')
      }
    })()
    return () => { cancelled = true }
  }, [id])

  useEffect(() => {
    if (checks.performance && !canPerformance) {
      setChecks(prev => ({ ...prev, performance: false }))
    }
  }, [canPerformance, checks.performance])

  function set<K extends keyof Monitor>(key: K, value: Monitor[K] | undefined) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function setCheck(key: keyof CheckSet, value: boolean) {
    setChecks(prev => ({ ...prev, [key]: value }))
  }

  function selectAll() {
    setChecks({
      http: true,
      ssl: true,
      dns: true,
      port: true,
      heartbeat: true,
      performance: canPerformance,
    })
  }

  const emailConfigured = !!summary?.email?.configured
  const emailGlobalOn = !!(summary?.email?.configured && summary?.email?.enabled)
  const slackConfigured = !!summary?.slack?.configured
  const slackGlobalOn = !!(summary?.slack?.configured && summary?.slack?.enabled)
  const webhooksConfigured = !!summary?.webhooks?.configured
  const webhooksGlobalOn = !!(summary?.webhooks?.configured && summary?.webhooks?.enabled)

  const sharedFields = useMemo(() => {
    const tags = tagsInput.split(',').map(s => s.trim()).filter(Boolean)
    const payload: Partial<Monitor> = {
      name: form.name,
      tenant_id: form.tenant_id || '',
      interval_seconds: form.interval_seconds,
      timeout_ms: form.timeout_ms,
      slow_threshold_ms: form.slow_threshold_ms,
      alert_after_failures: form.alert_after_failures,
      alert_emails: form.alert_emails || '',
      notify_email: !!form.notify_email,
      notify_slack: !!form.notify_slack,
      invert: !!form.invert,
      tags,
      enabled: true,
    }
    if (isPlatformAdmin) payload.notify_webhooks = !!form.notify_webhooks
    return payload
  }, [form, tagsInput, isPlatformAdmin])

  function payloadForType(type: MonitorType, existing?: Monitor): Partial<Monitor> {
    const host = normalizeHost(target)
    const httpUrl = httpUrlFrom(target)
    const base: Partial<Monitor> = {
      ...(existing || {}),
      ...sharedFields,
      type,
      enabled: true,
    }
    if (type === 'http') {
      return {
        ...base,
        url: httpUrl,
        method: form.method || 'GET',
        expected_status: form.expected_status ?? 200,
        expected_status_min: statusRange ? form.expected_status_min : undefined,
        expected_status_max: statusRange ? form.expected_status_max : undefined,
        keyword_must_exist: form.keyword_must_exist || '',
        keyword_must_not_exist: form.keyword_must_not_exist || '',
        request_body: form.request_body || '',
        request_headers: form.request_headers || '',
        http_username: form.http_username || '',
        http_password: form.http_password || '',
        follow_redirects: form.follow_redirects ?? true,
      }
    }
    if (type === 'port') {
      return { ...base, url: host, port: form.port }
    }
    if (type === 'ssl') {
      return { ...base, url: host, port: sslPort || 443 }
    }
    if (type === 'dns') {
      return {
        ...base,
        url: host,
        config: JSON.stringify({
          dns_records: dnsRecords.split(',').map(s => s.trim().toUpperCase()).filter(Boolean),
        }),
      }
    }
    return {
      ...base,
      url: '',
      config: JSON.stringify({ grace_seconds: graceSeconds ?? 60 }),
    }
  }

  async function saveType(type: MonitorType, enabled: boolean): Promise<string | undefined> {
    const existing = siblings[type]
    if (enabled) {
      if (existing) {
        await api.updateMonitor(existing.id, payloadForType(type, existing))
        if (existing.enabled === false) await api.setMonitorEnabled(existing.id, true)
        return existing.id
      }
      const created = await api.createMonitor(payloadForType(type))
      return created.id
    }
    if (existing?.enabled !== false && existing) {
      await api.setMonitorEnabled(existing.id, false)
    }
    return existing?.id
  }

  async function savePerformance(enabled: boolean) {
    const url = httpUrlFrom(target)
    if (enabled) {
      const body = {
        name: form.name || url,
        url,
        interval_seconds: Math.max(form.interval_seconds || 60, 60),
        timeout_ms: form.timeout_ms || 10000,
        slow_threshold_ms: form.slow_threshold_ms || 3000,
        follow_redirects: form.follow_redirects ?? true,
        alert_emails: form.alert_emails || '',
        http_username: form.http_username || '',
        http_password: form.http_password || '',
        tenant_id: form.tenant_id || '',
        enabled: true,
        method: form.method === 'HEAD' ? 'HEAD' : 'GET',
      }
      if (perfSibling) {
        await api.updatePerformanceTarget(perfSibling.id, { ...perfSibling, ...body })
        if (perfSibling.enabled === false) await api.setPerformanceTargetEnabled(perfSibling.id, true)
        return
      }
      await api.createPerformanceTarget(body)
      return
    }
    if (perfSibling?.enabled !== false && perfSibling) {
      await api.setPerformanceTargetEnabled(perfSibling.id, false)
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (!anyCheck) {
      setError('Enable at least one check')
      return
    }
    if (needsTarget && !target.trim()) {
      setError('Enter a website URL or host')
      return
    }
    if (checks.port && (form.port == null || form.port < 1)) {
      setError('Enter a TCP port')
      return
    }
    setSaving(true)
    const errors: string[] = []
    let firstId = id
    const types: MonitorType[] = ['http', 'ssl', 'dns', 'port', 'heartbeat']
    try {
      for (const type of types) {
        try {
          const savedId = await saveType(type, checks[type])
          if (!firstId && savedId) firstId = savedId
        } catch (err) {
          errors.push(`${labelFor(type)}: ${err instanceof Error ? err.message : 'failed'}`)
        }
      }
      if (checks.performance || perfSibling) {
        try {
          await savePerformance(checks.performance)
        } catch (err) {
          errors.push(`Performance: ${err instanceof Error ? err.message : 'failed'}`)
        }
      }
      if (errors.length) {
        setError(errors.join(' · '))
        if (firstId && firstId !== id) {
          setTimeout(() => {
            if (onSaved) onSaved()
            else navigate(`/monitors/${firstId}`)
          }, 2500)
        }
        return
      }
      if (onSaved) onSaved()
      else navigate(firstId ? `/monitors/${firstId}` : '/')
    } finally {
      setSaving(false)
    }
  }

  function handleClose() {
    if (onClose) onClose()
    else if (id) navigate(`/monitors/${id}`)
    else navigate('/')
  }

  const checkToggles = (
    <>
      {MONITOR_CHECKS.map(c => (
        <NotifyToggle
          key={c.key}
          label={c.label}
          checked={checks[c.key]}
          onChange={v => setCheck(c.key, v)}
        />
      ))}
      <NotifyToggle
        label="Performance"
        checked={checks.performance}
        disabled={!canPerformance}
        hint={!canPerformance ? 'Needs an HTTP URL' : undefined}
        onChange={v => setCheck('performance', v)}
      />
    </>
  )

  return (
    <FormModal
      title={id ? 'Edit Monitor' : 'Add Monitor'}
      subtitle="Enter a target and enable the checks you need."
      onClose={handleClose}
      wide
    >
      {error && <div className="flash-error" role="alert">{error}</div>}
      <form onSubmit={handleSubmit} style={styles.form}>
        <Field label="Website URL">
          <input
            className="input"
            value={target}
            onChange={e => setTarget(e.target.value)}
            placeholder="https://example.com"
            required={needsTarget}
          />
        </Field>

        <div style={styles.checksBox}>
          {id ? (
            <>
              <div style={styles.notifyTitle}>Checks</div>
              <p style={{ color: colors.textMuted, fontSize: 14, margin: '0 0 12px' }}>
                Enable the checks to run for this target. Turning a check off pauses it; history is kept.
              </p>
              <div>{checkToggles}</div>
            </>
          ) : (
            <>
              <div style={styles.checksHeader}>
                <div style={styles.notifyTitle}>Checks</div>
                <button type="button" className="btn" style={styles.selectAll} onClick={selectAll}>
                  Select all
                </button>
              </div>
              <div style={styles.checksGrid}>{checkToggles}</div>
            </>
          )}
        </div>

        {checks.http && (
          <>
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

        {checks.port && (
          <>
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

        {checks.ssl && (
          <Field label="SSL port (default 443)">
            <input type="number" value={numberFieldValue(sslPort)} onChange={e => setSslPort(parseNumberInput(e.target.value))} className="input" />
          </Field>
        )}

        {checks.dns && (
          <Field label="DNS record types (comma-separated)">
            <input value={dnsRecords} onChange={e => setDnsRecords(e.target.value)} className="input" placeholder="A,AAAA,MX,TXT,NS,CNAME" />
          </Field>
        )}

        {checks.heartbeat && (
          <>
            <Field label="Heartbeat grace period (seconds)">
              <input type="number" min={30} className="input" value={numberFieldValue(graceSeconds)}
                onChange={e => setGraceSeconds(parseNumberInput(e.target.value))} />
              <p style={{ color: colors.textMuted, fontSize: 14, margin: '8px 0 0' }}>
                Alert if no ping received within this window after the last heartbeat.
              </p>
            </Field>
            {(siblings.heartbeat?.heartbeat_token || form.heartbeat_token) && (
              <Field label="Ping URL">
                <input readOnly className="input" value={`${window.location.origin}/api/heartbeat/${siblings.heartbeat?.heartbeat_token || form.heartbeat_token}`} />
                <p style={{ color: colors.textMuted, fontSize: 14, margin: '8px 0 0' }}>
                  Call this URL (GET or POST) from your cron job or script on schedule.
                </p>
              </Field>
            )}
          </>
        )}

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
                placeholder="Leave blank to use this customer’s notification recipients"
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
        <div className="form-modal-actions">
          {id && form.name && (
            <DeleteMonitorButton id={id} name={form.name} variant="danger" />
          )}
          <div className="form-modal-actions-end">
            <button type="button" className="btn" onClick={handleClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : id ? 'Save Changes' : 'Create Monitor'}
            </button>
          </div>
        </div>
      </form>
    </FormModal>
  )
}

function labelFor(type: MonitorType): string {
  return MONITOR_CHECKS.find(c => c.key === type)?.label || type
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
  checksBox: {
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    padding: 14,
    background: colors.bgElevated || colors.card,
  },
  checksHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    marginBottom: 8,
  },
  checksGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
    columnGap: 16,
    rowGap: 4,
  },
  selectAll: {
    fontSize: 13,
    minHeight: 32,
    padding: '4px 10px',
    color: colors.brand,
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
