import { useCallback, useEffect, useRef, useState } from 'react'
import { api, EmailLogEntry } from '../../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../../components/ColumnResize'
import DatePicker from '../../components/DatePicker'
import { colors } from '../../theme'

const PAGE_SIZE = 20

export default function SettingsEmailLog() {
  const [entries, setEntries] = useState<EmailLogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [status, setStatus] = useState('')
  const [date, setDate] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize('email-log', 5)
  const sortValue = useCallback((e: EmailLogEntry, key: string) => {
    if (key === 'time') return e.created_at
    if (key === 'status') return e.status
    if (key === 'to') return e.to_addr || ''
    if (key === 'subject') return e.subject || e.monitor_name || ''
    if (key === 'error') return e.error || ''
    return null
  }, [])
  const { sorted, header } = useTableSort(entries, sortValue)

  useEffect(() => { setPage(0) }, [status, date])

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    api.listEmailLog({
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      status: status || undefined,
      date: date || undefined,
    })
      .then(res => {
        if (cancelled) return
        setEntries(res.items || [])
        setTotal(res.total)
        setError('')
      })
      .catch(err => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load email log')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [page, status, date])

  const hasFilters = !!(status || date)
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const from = total === 0 ? 0 : page * PAGE_SIZE + 1
  const to = Math.min(total, (page + 1) * PAGE_SIZE)

  return (
    <div style={styles.card}>
      <div style={styles.header}>
        <div>
          <h3 style={styles.title}>Email log</h3>
          <p style={styles.desc}>
            Sent, failed, skipped, and pending alert emails
            {total > 0 ? ` · Showing ${from}–${to} of ${total}` : ''}
          </p>
        </div>
      </div>

      <div style={styles.filterBar}>
        <label style={styles.field}>
          <span style={styles.label}>Status</span>
          <select
            className="input"
            style={styles.select}
            value={status}
            onChange={e => setStatus(e.target.value)}
          >
            <option value="">All</option>
            <option value="sent">Sent</option>
            <option value="fail">Fail</option>
            <option value="skip">Skip</option>
            <option value="pending">Pending</option>
          </select>
        </label>
        <DatePicker value={date} onChange={setDate} />
        {hasFilters && (
          <button type="button" className="btn" style={styles.reset} onClick={() => { setStatus(''); setDate('') }}>
            Reset filters
          </button>
        )}
      </div>

      {error && <div style={styles.error} role="alert">{error}</div>}

      {entries.length === 0 && !loading ? (
        <p style={styles.empty}>
          {hasFilters
            ? 'No email attempts match these filters.'
            : 'No email attempts yet — Send test or wait for an alert.'}
        </p>
      ) : (
        <>
          <div className="data-table-wrap" style={styles.tableWrap}>
            <table ref={tableRef} className="data-table" style={styles.table}>
              <ColGroup widths={widths} />
              <thead>
                <tr>
                  <ResizableTh index={0} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('time')}>Time</ResizableTh>
                  <ResizableTh index={1} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
                  <ResizableTh index={2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('to')}>To</ResizableTh>
                  <ResizableTh index={3} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('subject')}>Subject</ResizableTh>
                  <ResizableTh index={4} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('error')}>Error</ResizableTh>
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
                      <td style={styles.td}><StatusBadge status={e.status} /></td>
                      <td style={{ ...styles.td, wordBreak: 'break-all' }}>{e.to_addr || '—'}</td>
                      <td style={styles.td}>{e.subject || e.monitor_name || '—'}</td>
                      <td style={{ ...styles.td, color: colors.textMuted }}>{e.error || '—'}</td>
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

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { color: string; bg: string; label: string }> = {
    sent: { color: colors.green, bg: colors.greenDim, label: 'Sent' },
    fail: { color: colors.red, bg: colors.redDim, label: 'Fail' },
    skip: { color: colors.textMuted, bg: 'rgba(148,163,184,0.15)', label: 'Skip' },
    pending: { color: colors.yellow, bg: colors.yellowDim, label: 'Pending' },
  }
  const s = map[status] || { color: colors.textMuted, bg: 'rgba(148,163,184,0.15)', label: status }
  return (
    <span style={{
      fontSize: 12, fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase',
      color: s.color, background: s.bg, padding: '3px 8px', borderRadius: 4,
    }}>
      {s.label}
    </span>
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
