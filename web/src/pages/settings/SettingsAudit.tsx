import { useCallback, useEffect, useRef, useState } from 'react'
import { api, AuditEntry, AuditMeta } from '../../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../../components/ColumnResize'
import DatePicker from '../../components/DatePicker'
import { colors } from '../../theme'

const PAGE_SIZE = 20

type Filters = {
  date: string
  actor: string
  action: string
  resource: string
}

const emptyFilters: Filters = {
  date: '',
  actor: '',
  action: '',
  resource: '',
}

export default function SettingsAudit() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [meta, setMeta] = useState<AuditMeta>({ actors: [], resources: [], actions: ['create', 'update', 'delete'] })
  const [filters, setFilters] = useState<Filters>(emptyFilters)
  const [page, setPage] = useState(0)
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('audit', 5)
  const sortValue = useCallback((e: AuditEntry, key: string) => {
    if (key === 'time') return e.created_at
    if (key === 'actor') return e.actor
    if (key === 'action') return e.action
    if (key === 'resource') return e.resource
    if (key === 'detail') return e.detail || ''
    return null
  }, [])
  const { sorted, header } = useTableSort(entries, sortValue)

  useEffect(() => {
    api.listAuditMeta().then(setMeta).catch(() => {})
  }, [])

  useEffect(() => {
    setPage(0)
  }, [filters.date, filters.actor, filters.action, filters.resource])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api.listAudit({
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      date: filters.date || undefined,
      actor: filters.actor || undefined,
      action: filters.action || undefined,
      resource: filters.resource || undefined,
    })
      .then(res => {
        if (cancelled) return
        setEntries(res.items)
        setTotal(res.total)
        setError('')
      })
      .catch(err => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [filters.date, filters.actor, filters.action, filters.resource, page])

  const set = <K extends keyof Filters>(key: K, value: Filters[K]) => {
    setFilters(prev => ({ ...prev, [key]: value }))
  }

  const hasFilters = !!(filters.date || filters.actor || filters.action || filters.resource)
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const from = total === 0 ? 0 : page * PAGE_SIZE + 1
  const to = Math.min(total, (page + 1) * PAGE_SIZE)

  return (
    <div style={styles.card}>
      <div style={styles.header}>
        <div>
          <h3 style={styles.title}>Audit Log</h3>
          <p style={styles.desc}>
            Recent administrative actions
            {total > 0 ? ` · Showing ${from}–${to} of ${total}` : ''}
          </p>
        </div>
      </div>

      <div style={styles.filterBar}>
        <label style={styles.field}>
          <span style={styles.label}>Actor</span>
          <select
            className="input"
            style={styles.select}
            value={filters.actor}
            onChange={e => set('actor', e.target.value)}
          >
            <option value="">All actors</option>
            {meta.actors.map(a => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
        </label>

        <label style={styles.field}>
          <span style={styles.label}>Action</span>
          <select
            className="input"
            style={styles.select}
            value={filters.action}
            onChange={e => set('action', e.target.value)}
          >
            <option value="">All actions</option>
            {(meta.actions.length ? meta.actions : ['create', 'update', 'delete']).map(a => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
        </label>

        <label style={styles.field}>
          <span style={styles.label}>Resource</span>
          <select
            className="input"
            style={styles.select}
            value={filters.resource}
            onChange={e => set('resource', e.target.value)}
          >
            <option value="">All resources</option>
            {meta.resources.map(r => (
              <option key={r} value={r}>{r}</option>
            ))}
          </select>
        </label>

        <DatePicker value={filters.date} onChange={d => set('date', d)} />

        {hasFilters && (
          <button type="button" className="btn" style={styles.reset} onClick={() => setFilters(emptyFilters)}>
            Reset filters
          </button>
        )}
      </div>

      {error && <div style={styles.error} role="alert">{error}</div>}

      {entries.length === 0 && !loading ? (
        <p style={styles.empty}>
          {hasFilters ? 'No audit entries match these filters.' : 'No audit entries yet.'}
        </p>
      ) : (
        <>
          <div className="data-table-wrap" style={styles.tableWrap}>
            <table ref={tableRef} className="data-table" style={styles.table}>
              <ColGroup widths={widths} />
              <thead>
                <tr>
                  <ResizableTh index={0} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('time')}>Time</ResizableTh>
                  <ResizableTh index={1} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('actor')}>Actor</ResizableTh>
                  <ResizableTh index={2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('action')}>Action</ResizableTh>
                  <ResizableTh index={3} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('resource')}>Resource</ResizableTh>
                  <ResizableTh index={4} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('detail')}>Detail</ResizableTh>
                </tr>
              </thead>
              <tbody>
                {loading && entries.length === 0 ? (
                  <tr>
                    <td colSpan={5} style={{ ...styles.td, color: colors.textMuted }}>Loading…</td>
                  </tr>
                ) : (
                  sorted.map(e => (
                    <tr key={e.id}>
                      <td style={styles.td}>{new Date(e.created_at).toLocaleString()}</td>
                      <td style={styles.td}>{e.actor}</td>
                      <td style={styles.td}>
                        <span style={styles.action}>{e.action}</span>
                      </td>
                      <td style={styles.td}>{e.resource}</td>
                      <td style={{ ...styles.td, color: colors.textMuted }}>{e.detail || '—'}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
          {total > PAGE_SIZE && (
            <div style={styles.pager}>
              <button
                type="button"
                className="btn"
                style={styles.pagerBtn}
                disabled={page <= 0 || loading}
                onClick={() => setPage(p => Math.max(0, p - 1))}
              >
                Previous
              </button>
              <span style={{ fontSize: 14, color: colors.textMuted }}>
                Page {page + 1} of {totalPages}
              </span>
              <button
                type="button"
                className="btn"
                style={styles.pagerBtn}
                disabled={page + 1 >= totalPages || loading}
                onClick={() => setPage(p => p + 1)}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card,
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    padding: 28,
  },
  header: { marginBottom: 16 },
  title: { margin: '0 0 8px', fontSize: 19, fontWeight: 600 },
  desc: { color: colors.textMuted, fontSize: 15, margin: 0 },
  filterBar: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    flexWrap: 'wrap',
    marginBottom: 20,
    padding: '14px 16px',
    background: colors.bg,
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
  },
  field: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 15,
    color: colors.textMuted,
  },
  label: { fontWeight: 500, flexShrink: 0 },
  select: {
    width: 'auto',
    minWidth: 130,
    padding: '0 12px',
    cursor: 'pointer',
  },
  reset: { padding: '8px 12px', fontSize: 14 },
  tableWrap: {},
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: 15,
  },
  th: {
    textAlign: 'left',
    padding: '12px 14px',
    borderBottom: `1px solid ${colors.border}`,
    color: colors.textMuted,
    fontWeight: 600,
    fontSize: 13,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    whiteSpace: 'nowrap',
  },
  td: {
    textAlign: 'left',
    padding: '12px 14px',
    borderBottom: `1px solid ${colors.border}`,
    verticalAlign: 'middle',
  },
  action: {
    textTransform: 'uppercase',
    fontSize: 12,
    fontWeight: 700,
    color: colors.textMuted,
    letterSpacing: '0.04em',
  },
  empty: { color: colors.textMuted, fontSize: 15, margin: '12px 0 0' },
  error: { background: colors.redDim, color: colors.red, padding: 12, borderRadius: 8, marginBottom: 16 },
  pager: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    paddingTop: 16,
    marginTop: 4,
  },
  pagerBtn: { padding: '8px 14px', fontSize: 14 },
}
