import DatePicker from './DatePicker'
import { colors } from '../theme'

export type IncidentStatusFilter = '' | 'open' | 'resolved'

const TYPE_OPTIONS = [
  { value: '', label: 'All types' },
  { value: 'down', label: 'Down' },
  { value: 'ssl_expiry', label: 'SSL expiry' },
  { value: 'cert_change', label: 'Cert change' },
  { value: 'dns_change', label: 'DNS change' },
  { value: 'slow', label: 'Slow' },
]

export type IncidentFilterValues = {
  date: string
  status: IncidentStatusFilter
  type: string
  monitorId: string
}

type MonitorOption = { id: string; name: string }

export default function IncidentFilters({
  value,
  onChange,
  monitors,
  showMonitor = true,
}: {
  value: IncidentFilterValues
  onChange: (next: IncidentFilterValues) => void
  monitors?: MonitorOption[]
  showMonitor?: boolean
}) {
  const set = <K extends keyof IncidentFilterValues>(key: K, v: IncidentFilterValues[K]) => {
    onChange({ ...value, [key]: v })
  }

  const hasFilters = !!(value.date || value.status || value.type || value.monitorId)

  return (
    <div style={styles.bar}>
      {showMonitor && monitors && monitors.length > 0 && (
        <label style={styles.field}>
          <span style={styles.label}>Site</span>
          <select
            className="input"
            style={styles.select}
            value={value.monitorId}
            onChange={e => set('monitorId', e.target.value)}
          >
            <option value="">All sites</option>
            {monitors.map(m => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
        </label>
      )}

      <label style={styles.field}>
        <span style={styles.label}>Status</span>
        <select
          className="input"
          style={styles.select}
          value={value.status}
          onChange={e => set('status', e.target.value as IncidentStatusFilter)}
        >
          <option value="">All statuses</option>
          <option value="open">Down / Open</option>
          <option value="resolved">Resolved</option>
        </select>
      </label>

      <label style={styles.field}>
        <span style={styles.label}>Type</span>
        <select
          className="input"
          style={styles.select}
          value={value.type}
          onChange={e => set('type', e.target.value)}
        >
          {TYPE_OPTIONS.map(o => (
            <option key={o.value || 'all'} value={o.value}>{o.label}</option>
          ))}
        </select>
      </label>

      <DatePicker value={value.date} onChange={d => set('date', d)} />

      {hasFilters && (
        <button
          type="button"
          className="btn"
          style={styles.reset}
          onClick={() => onChange({ date: '', status: '', type: '', monitorId: '' })}
        >
          Reset filters
        </button>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  bar: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    flexWrap: 'wrap',
  },
  field: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 16,
    color: colors.textMuted,
  },
  label: { fontWeight: 500, flexShrink: 0 },
  select: {
    width: 'auto',
    minWidth: 140,
    padding: '0 12px',
    cursor: 'pointer',
  },
  reset: {
    padding: '8px 12px',
    fontSize: 15,
  },
}
