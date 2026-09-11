export type MonitorType = 'http' | 'port' | 'ssl' | 'dns' | 'heartbeat'

export interface Monitor {
  id: string
  type: MonitorType
  name: string
  url: string
  port?: number
  config?: string
  method: string
  expected_status: number
  expected_status_min?: number
  expected_status_max?: number
  keyword_must_exist: string
  keyword_must_not_exist: string
  request_body: string
  request_headers: string
  http_username?: string
  http_password?: string
  http_auth_set?: boolean
  interval_seconds: number
  timeout_ms: number
  slow_threshold_ms: number
  follow_redirects: boolean
  alert_emails: string
  enabled: boolean
  notify_email?: boolean
  notify_slack?: boolean
  notify_webhooks?: boolean
  invert?: boolean
  tags?: string[]
  heartbeat_token?: string
  tenant_id?: string
  alert_after_failures?: number
  consecutive_failures?: number
  last_status: 'up' | 'down' | 'degraded' | 'unknown'
  last_checked_at?: string
  latest_response_time_ms?: number
}

export interface CheckResult {
  id: string
  monitor_id: string
  status: string
  status_code?: number
  response_time_ms: number
  dns_ms?: number
  tcp_ms?: number
  tls_ms?: number
  ttfb_ms?: number
  error?: string
  details?: string
  checked_at: string
}

export interface PaginatedResults<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface SSLDetails {
  expires_at: string
  days_remaining: number
  issuer: string
  subject: string
  sans?: string[]
  fingerprint: string
  issues?: string[]
}

export interface DNSDetails {
  records: Record<string, string[]>
  changes?: { type: string; before: string; after: string }[]
}

export interface PortDetails {
  host: string
  port: number
  open: boolean
}

export interface PerformanceMetrics {
  avg_ms: number
  min_ms: number
  max_ms: number
  p50_ms: number
  p95_ms: number
  p99_ms: number
  slow_count: number
  degraded_pct: number
}

export interface MonitorStats {
  monitor_id: string
  points: {
    timestamp: string
    response_time_ms: number
    status: string
    dns_ms?: number
    tcp_ms?: number
    tls_ms?: number
    ttfb_ms?: number
  }[]
  uptime_pct: number
  avg_response_ms: number
  performance: PerformanceMetrics
}

export interface MonitorRowStats {
  uptime_pct: number
  points: number[]
}

export type PerformanceHealth = 'good' | 'warning' | 'critical' | 'collecting' | 'failed' | 'paused'

export interface ServicePerformance {
  service_id: string
  name: string
  probe_type: string
  target: string
  latency_status: string
  slow_threshold_ms: number
  avg_ms: number
  p95_ms: number
  max_ms: number
  slow_count: number
  check_count: number
  has_data: boolean
  health: PerformanceHealth
}

export interface FleetTimelinePoint {
  timestamp: string
  avg_ms: number
  check_count: number
  slow_count: number
}

export interface PerformanceTarget {
  id: string
  name: string
  url: string
  method: string
  interval_seconds: number
  timeout_ms: number
  slow_threshold_ms: number
  follow_redirects: boolean
  enabled: boolean
  alert_emails?: string
  http_username?: string
  http_password?: string
  http_auth_set?: boolean
  tenant_id?: string
  alert_after_slow?: number
  consecutive_slow?: number
  last_status: 'up' | 'down' | 'degraded' | 'unknown'
  last_checked_at?: string
  latest_response_time_ms?: number
}

export interface PerformanceResult {
  id: string
  target_id: string
  status: string
  status_code?: number
  response_time_ms: number
  dns_ms?: number
  tcp_ms?: number
  tls_ms?: number
  ttfb_ms?: number
  error?: string
  checked_at: string
}

export interface PerformanceStats {
  target_id: string
  points: MonitorStats['points']
  avg_response_ms: number
  performance: PerformanceMetrics
}

export interface FleetPerformance {
  period: string
  timeline: FleetTimelinePoint[]
  services: ServicePerformance[]
  total_checks: number
  avg_ms: number
  p95_ms: number
  slow_checks: number
  service_count: number
  healthy_count: number
  warning_count: number
  critical_count: number
  collecting_count: number
}

export type UserRole = 'admin' | 'viewer'

export interface Profile {
  id: string
  username: string
  name: string
  email: string
  mfa_enabled: boolean
  role: UserRole
  tenant_id?: string
}

export interface LoginResponse {
  ok: boolean
  mfa_required?: boolean
  challenge_id?: string
  email_hint?: string
  message?: string
}

export interface Customer {
  id: string
  name: string
  monitor_quota: number
  monitor_count?: number
  alert_emails?: string
  created_at: string
}

export interface TeamMember {
  id: string
  username: string
  email: string
  role: UserRole
  tenant_id?: string
  locked?: boolean
  created_at: string
}

export interface CreateTeamMemberRequest {
  username: string
  email?: string
  password: string
  role: UserRole
  tenant_id?: string
}

export interface UpdateTeamMemberRequest {
  username?: string
  email?: string
  password?: string
  role?: UserRole
  tenant_id?: string
}

export interface UpdateProfileRequest {
  current_password: string
  username?: string
  name?: string
  email?: string
  new_password?: string
  mfa_enabled?: boolean
}

export interface OrgSettings {
  company_name: string
  tagline: string
  logo: string
}

export interface SMTPConfig {
  host: string
  port: number
  username: string
  password: string
  from: string
  alert_emails: string
  tls: boolean
  enabled: boolean
}

export interface EmailLogEntry {
  id: string
  status: 'sent' | 'fail' | 'skip' | 'pending' | string
  kind: string
  to_addr: string
  subject: string
  error?: string
  monitor_id?: string
  monitor_name?: string
  tenant_id?: string
  created_at: string
}

export interface WebhookConfig {
  url: string
  enabled: boolean
  events: string[]
}

export interface SlackConfig {
  webhook_url: string
  enabled: boolean
  events: string[]
}

export interface NotificationsSummary {
  slack: { enabled: boolean; configured: boolean; webhook_url?: string; events?: string[] }
  email?: { enabled: boolean; configured: boolean }
  webhooks?: { enabled: boolean; configured: boolean; count?: number }
}

export interface MaintenanceWindow {
  id: string
  name: string
  monitor_id?: string
  starts_at: string
  ends_at: string
  created_at: string
}

export interface ServerSettings {
  dashboard_url: string
  retention_days: number
  workers: number
}

export interface StatusPageConfig {
  enabled: boolean
  title: string
  monitor_ids: string[]
}

export interface PublicMonitorStatus {
  id: string
  name: string
  type: string
  status: string
  last_checked_at?: string
  url?: string
}

export interface PublicStatusResponse {
  title: string
  monitors: PublicMonitorStatus[]
}

export interface Incident {
  id: string
  monitor_id: string
  monitor_name: string
  type: string
  message?: string
  started_at: string
  resolved_at?: string
  acknowledged_at?: string
  acknowledged_by?: string
}

export type HostStatus = 'pending' | 'online' | 'offline'

export interface HostDisk {
  mount: string
  percent: number
}

export interface HostServiceStatus {
  name: string
  active: string
  sub?: string
}

export interface HostSecurity {
  logs_readable: boolean
  ssh_failed_5m: number
  sudo_failed_5m: number
  auth_failed_5m: number
  root_logins_5m: number
  last_root_login?: string
}

export interface Host {
  id: string
  name: string
  hostname: string
  tenant_id?: string
  os?: string
  os_version?: string
  kernel_version?: string
  arch?: string
  agent_version?: string
  num_cpu?: number
  uptime_seconds?: number
  reboot_required?: boolean
  last_seen_at?: string
  status: HostStatus
  enabled: boolean
  interval_seconds: number
  alert_after_failures: number
  consecutive_misses: number
  collect_cpu: boolean
  collect_memory: boolean
  collect_disk: boolean
  collect_load: boolean
  collect_swap: boolean
  collect_iowait: boolean
  collect_security: boolean
  collect_services: boolean
  alert_cpu_enabled: boolean
  alert_cpu_warning: number
  alert_cpu_threshold: number
  alert_cpu_after: number
  alert_memory_enabled: boolean
  alert_memory_warning: number
  alert_memory_threshold: number
  alert_memory_after: number
  alert_disk_enabled: boolean
  alert_disk_warning: number
  alert_disk_threshold: number
  alert_disk_after: number
  alert_load_enabled: boolean
  alert_load_warning: number
  alert_load_threshold: number
  alert_load_after: number
  alert_swap_enabled: boolean
  alert_swap_warning: number
  alert_swap_threshold: number
  alert_swap_after: number
  alert_iowait_enabled: boolean
  alert_iowait_warning: number
  alert_iowait_threshold: number
  alert_iowait_after: number
  alert_auth_enabled: boolean
  alert_auth_threshold: number
  alert_root_login_enabled: boolean
  alert_reboot_enabled: boolean
  alert_service_enabled: boolean
  services?: string[]
  disks?: HostDisk[]
  service_status?: HostServiceStatus[]
  security?: HostSecurity
  alert_emails: string
  notify_email: boolean
  notify_slack: boolean
  notify_webhooks: boolean
  created_at: string
  updated_at: string
  enroll_token?: string
  install_command?: string
  enroll_expires_at?: string
}

export interface HostStatsPoint {
  timestamp: string
  cpu_percent?: number
  mem_percent?: number
  swap_percent?: number
  iowait_percent?: number
  load1?: number
  disk_percent?: number
  disks?: HostDisk[]
  num_cpu?: number
}

export interface HostStats {
  host_id: string
  points: HostStatsPoint[]
}

export function isHostIncident(type?: string): boolean {
  return (type || '').startsWith('host_')
}

export function incidentSubjectPath(inc: { type: string; monitor_id: string }): string {
  return isHostIncident(inc.type) ? `/hosts/${inc.monitor_id}` : `/monitors/${inc.monitor_id}`
}

export interface SLAMonitorRow {
  monitor_id: string
  name: string
  tenant_id?: string
  incident_count: number
  downtime_seconds: number
  mttr_seconds?: number
  availability_pct: number
}

export interface SLAReport {
  period_start: string
  period_end: string
  monitor_count: number
  incident_count: number
  downtime_seconds: number
  mttr_seconds?: number
  availability_pct: number
  window_seconds: number
  monitors: SLAMonitorRow[]
}

/** Local-calendar day → ISO from/to for incident filters. */
export function localDayBounds(date: string): { from: string; to: string } {
  const [y, m, d] = date.split('-').map(Number)
  const start = new Date(y, m - 1, d, 0, 0, 0, 0)
  const end = new Date(y, m - 1, d + 1, 0, 0, 0, 0)
  return { from: start.toISOString(), to: end.toISOString() }
}

export interface AuditEntry {
  id: string
  actor: string
  action: string
  resource: string
  detail?: string
  created_at: string
}

export interface AuditMeta {
  actors: string[]
  resources: string[]
  actions: string[]
}

export interface APIToken {
  id: string
  user_id: string
  name: string
  prefix: string
  created_at: string
  last_used_at?: string
}

export interface APITokenCreated extends APIToken {
  token: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error((body as { error?: string }).error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  verifyMFALogin: (challenge_id: string, code: string) =>
    request<LoginResponse>('/api/auth/mfa/verify', {
      method: 'POST',
      body: JSON.stringify({ challenge_id, code }),
    }),
  resendMFALogin: (challenge_id: string) =>
    request<LoginResponse>('/api/auth/mfa/resend', {
      method: 'POST',
      body: JSON.stringify({ challenge_id }),
    }),
  logout: () => request('/api/auth/logout', { method: 'POST' }),
  forgotPassword: (email: string) =>
    request<{ message: string }>('/api/auth/forgot-password', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),
  resetPassword: (token: string, new_password: string) =>
    request<{ ok: boolean }>('/api/auth/reset-password', {
      method: 'POST',
      body: JSON.stringify({ token, new_password }),
    }),
  getProfile: () => request<Profile>('/api/profile'),
  updateProfile: (data: UpdateProfileRequest) =>
    request<Profile>('/api/profile', { method: 'PUT', body: JSON.stringify(data) }),
  monitors: (opts?: { tag?: string; customer?: string }) => {
    const params = new URLSearchParams()
    if (opts?.tag) params.set('tag', opts.tag)
    if (opts?.customer) params.set('customer', opts.customer)
    const q = params.toString()
    return request<Monitor[]>(q ? `/api/monitors?${q}` : '/api/monitors')
  },
  getMonitor: (id: string) => request<Monitor>(`/api/monitors/${id}`),
  createMonitor: (data: Partial<Monitor>) =>
    request<Monitor>('/api/monitors', { method: 'POST', body: JSON.stringify(data) }),
  updateMonitor: (id: string, data: Partial<Monitor>) =>
    request<Monitor>(`/api/monitors/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  setMonitorEnabled: (id: string, enabled: boolean) =>
    request<Monitor>(`/api/monitors/${id}/enabled`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
  deleteMonitor: (id: string) => request(`/api/monitors/${id}`, { method: 'DELETE' }),
  results: (id: string, opts?: { limit?: number; offset?: number }) => {
    const limit = opts?.limit ?? 10
    const offset = opts?.offset ?? 0
    return request<PaginatedResults<CheckResult>>(
      `/api/monitors/${id}/results?limit=${limit}&offset=${offset}`,
    )
  },
  monitorIncidents: (id: string, opts?: {
    limit?: number
    offset?: number
    date?: string
    status?: string
    type?: string
  }) => {
    const limit = opts?.limit ?? 20
    const offset = opts?.offset ?? 0
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    if (opts?.date) {
      const { from, to } = localDayBounds(opts.date)
      params.set('from', from)
      params.set('to', to)
    }
    if (opts?.status) params.set('status', opts.status)
    if (opts?.type) params.set('type', opts.type)
    return request<PaginatedResults<Incident>>(
      `/api/monitors/${id}/incidents?${params}`,
    )
  },
  stats: (id: string, period = '24h') =>
    request<MonitorStats>(`/api/monitors/${id}/stats?period=${period}`),
  monitorStatsSummary: (period = '24h', ids: string[] = []) =>
    request<Record<string, MonitorRowStats>>('/api/monitors/stats', {
      method: 'POST',
      body: JSON.stringify({ period, ids }),
    }),
  performance: (period = '24h', customer?: string) => {
    const params = new URLSearchParams({ period })
    if (customer) params.set('customer', customer)
    return request<FleetPerformance>(`/api/performance?${params}`)
  },
  performanceTargets: (customer?: string) =>
    request<PerformanceTarget[]>(
      customer ? `/api/performance/targets?customer=${encodeURIComponent(customer)}` : '/api/performance/targets',
    ),
  getPerformanceTarget: (id: string) => request<PerformanceTarget>(`/api/performance/targets/${id}`),
  createPerformanceTarget: (data: Partial<PerformanceTarget>) =>
    request<PerformanceTarget>('/api/performance/targets', { method: 'POST', body: JSON.stringify(data) }),
  updatePerformanceTarget: (id: string, data: Partial<PerformanceTarget>) =>
    request<PerformanceTarget>(`/api/performance/targets/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  setPerformanceTargetEnabled: (id: string, enabled: boolean) =>
    request<PerformanceTarget>(`/api/performance/targets/${id}/enabled`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),
  deletePerformanceTarget: (id: string) => request(`/api/performance/targets/${id}`, { method: 'DELETE' }),
  performanceResults: (id: string, opts?: {
    limit?: number
    offset?: number
    date?: string
    breaches?: boolean
  }) => {
    const limit = opts?.limit ?? 20
    const offset = opts?.offset ?? 0
    const params = new URLSearchParams({
      limit: String(limit),
      offset: String(offset),
      breaches: opts?.breaches === false ? '0' : '1',
    })
    if (opts?.date) {
      const { from, to } = localDayBounds(opts.date)
      params.set('from', from)
      params.set('to', to)
    }
    return request<PaginatedResults<PerformanceResult>>(
      `/api/performance/targets/${id}/results?${params}`,
    )
  },
  performanceStats: (id: string, period = '24h') =>
    request<PerformanceStats>(`/api/performance/targets/${id}/stats?period=${period}`),
  hosts: (customer?: string) =>
    request<Host[]>(customer ? `/api/hosts?customer=${encodeURIComponent(customer)}` : '/api/hosts'),
  getHost: (id: string) => request<Host>(`/api/hosts/${id}`),
  createHost: (data: Partial<Host>) =>
    request<Host>('/api/hosts', { method: 'POST', body: JSON.stringify(data) }),
  updateHost: (id: string, data: Partial<Host>) =>
    request<Host>(`/api/hosts/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  setHostEnabled: (id: string, enabled: boolean) =>
    request<Host>(`/api/hosts/${id}/enabled`, { method: 'PUT', body: JSON.stringify({ enabled }) }),
  deleteHost: (id: string) => request(`/api/hosts/${id}`, { method: 'DELETE' }),
  enrollHost: (id: string) =>
    request<Host>(`/api/hosts/${id}/enroll`, { method: 'POST' }),
  hostStats: (id: string, period = '24h') =>
    request<HostStats>(`/api/hosts/${id}/stats?period=${period}`),
  listCustomers: () => request<Customer[]>('/api/settings/customers'),
  createCustomer: (data: { name: string; monitor_quota?: number; alert_emails?: string }) =>
    request<Customer>('/api/settings/customers', { method: 'POST', body: JSON.stringify(data) }),
  updateCustomer: (id: string, data: { name: string; monitor_quota: number; alert_emails?: string }) =>
    request<Customer>(`/api/settings/customers/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCustomer: (id: string) => request(`/api/settings/customers/${id}`, { method: 'DELETE' }),
  getAlertRecipients: () => request<{ alert_emails: string }>('/api/settings/alert-recipients'),
  putAlertRecipients: (data: { alert_emails: string }) =>
    request<{ alert_emails: string }>('/api/settings/alert-recipients', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  testAlertRecipients: (alertEmails: string) =>
    request('/api/settings/alert-recipients/test', {
      method: 'POST',
      body: JSON.stringify({ alert_emails: alertEmails }),
    }),
  testCustomerEmails: (id: string, alertEmails: string) =>
    request(`/api/settings/customers/${id}/test-email`, {
      method: 'POST',
      body: JSON.stringify({ alert_emails: alertEmails }),
    }),
  listTeam: () => request<TeamMember[]>('/api/settings/team'),
  createTeamMember: (data: CreateTeamMemberRequest) =>
    request<TeamMember>('/api/settings/team', { method: 'POST', body: JSON.stringify(data) }),
  updateTeamMember: (id: string, data: UpdateTeamMemberRequest) =>
    request<TeamMember>(`/api/settings/team/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  unlockTeamMember: (id: string) =>
    request<TeamMember>(`/api/settings/team/${id}/unlock`, { method: 'POST' }),
  resetTeamMemberPassword: (id: string, password: string) =>
    request<TeamMember>(`/api/settings/team/${id}/reset-password`, {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),
  deleteTeamMember: (id: string) => request(`/api/settings/team/${id}`, { method: 'DELETE' }),
  getGeneral: () => request<OrgSettings>('/api/settings/general'),
  publicBranding: () => request<OrgSettings>('/api/public/branding'),
  putGeneral: (cfg: OrgSettings) =>
    request<OrgSettings>('/api/settings/general', { method: 'PUT', body: JSON.stringify(cfg) }),
  resetGeneral: () => request<OrgSettings>('/api/settings/general/reset', { method: 'POST' }),
  getSMTP: () => request<SMTPConfig>('/api/settings/smtp'),
  putSMTP: (cfg: SMTPConfig) =>
    request<SMTPConfig>('/api/settings/smtp', { method: 'PUT', body: JSON.stringify(cfg) }),
  testSMTP: (to: string) =>
    request('/api/settings/smtp/test', { method: 'POST', body: JSON.stringify({ to }) }),
  listEmailLog: (opts?: {
    limit?: number
    offset?: number
    status?: string
    kind?: string
    date?: string
  }) => {
    const params = new URLSearchParams()
    params.set('limit', String(opts?.limit ?? 20))
    params.set('offset', String(opts?.offset ?? 0))
    if (opts?.status) params.set('status', opts.status)
    if (opts?.kind) params.set('kind', opts.kind)
    if (opts?.date) {
      const { from, to } = localDayBounds(opts.date)
      params.set('from', from)
      params.set('to', to)
    }
    return request<PaginatedResults<EmailLogEntry>>(`/api/settings/smtp/log?${params}`)
  },
  getNotificationsSummary: () => request<NotificationsSummary>('/api/settings/notifications'),
  getSlack: () => request<SlackConfig>('/api/settings/slack'),
  putSlack: (cfg: SlackConfig) =>
    request<SlackConfig>('/api/settings/slack', { method: 'PUT', body: JSON.stringify(cfg) }),
  testSlack: () => request('/api/settings/slack/test', { method: 'POST', body: '{}' }),
  incidents: (opts?: {
    openOnly?: boolean
    date?: string
    limit?: number
    offset?: number
    status?: string
    type?: string
    monitorId?: string
  }) => {
    const params = new URLSearchParams()
    params.set('limit', String(opts?.limit ?? 20))
    params.set('offset', String(opts?.offset ?? 0))
    if (opts?.openOnly) params.set('open', '1')
    if (opts?.status) params.set('status', opts.status)
    if (opts?.type) params.set('type', opts.type)
    if (opts?.monitorId) params.set('monitor_id', opts.monitorId)
    if (opts?.date) {
      const { from, to } = localDayBounds(opts.date)
      params.set('from', from)
      params.set('to', to)
    }
    return request<PaginatedResults<Incident>>(`/api/incidents?${params}`)
  },
  getIncident: (id: string) => request<Incident>(`/api/incidents/${id}`),
  acknowledgeIncident: (id: string) =>
    request<Incident>(`/api/incidents/${id}/acknowledge`, { method: 'POST' }),
  slaReport: (month: string, customer?: string) => {
    const params = new URLSearchParams({ month })
    if (customer) params.set('customer', customer)
    return request<SLAReport>(`/api/reports/sla?${params}`)
  },
  getWebhooks: () => request<WebhookConfig[]>('/api/settings/webhooks'),
  putWebhooks: (hooks: WebhookConfig[]) =>
    request<WebhookConfig[]>('/api/settings/webhooks', { method: 'PUT', body: JSON.stringify(hooks) }),
  listMaintenance: () => request<MaintenanceWindow[]>('/api/settings/maintenance'),
  createMaintenance: (data: Partial<MaintenanceWindow>) =>
    request<MaintenanceWindow>('/api/settings/maintenance', { method: 'POST', body: JSON.stringify(data) }),
  deleteMaintenance: (id: string) => request(`/api/settings/maintenance/${id}`, { method: 'DELETE' }),
  getServerSettings: () => request<ServerSettings>('/api/settings/server'),
  putServerSettings: (cfg: ServerSettings) =>
    request<ServerSettings>('/api/settings/server', { method: 'PUT', body: JSON.stringify(cfg) }),
  getStatusPageConfig: () => request<StatusPageConfig>('/api/settings/status-page'),
  putStatusPageConfig: (cfg: StatusPageConfig) =>
    request<StatusPageConfig>('/api/settings/status-page', { method: 'PUT', body: JSON.stringify(cfg) }),
  listAudit: (opts?: {
    limit?: number
    offset?: number
    actor?: string
    action?: string
    resource?: string
    date?: string
  }) => {
    const params = new URLSearchParams()
    params.set('limit', String(opts?.limit ?? 20))
    params.set('offset', String(opts?.offset ?? 0))
    if (opts?.actor) params.set('actor', opts.actor)
    if (opts?.action) params.set('action', opts.action)
    if (opts?.resource) params.set('resource', opts.resource)
    if (opts?.date) {
      const { from, to } = localDayBounds(opts.date)
      params.set('from', from)
      params.set('to', to)
    }
    return request<PaginatedResults<AuditEntry>>(`/api/settings/audit?${params}`)
  },
  listAuditMeta: () => request<AuditMeta>('/api/settings/audit/meta'),
  listTokens: () => request<APIToken[]>('/api/settings/tokens'),
  createToken: (name: string) =>
    request<APITokenCreated>('/api/settings/tokens', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteToken: (id: string) => request(`/api/settings/tokens/${id}`, { method: 'DELETE' }),
  publicStatus: () => request<PublicStatusResponse>('/api/public/status'),
}
