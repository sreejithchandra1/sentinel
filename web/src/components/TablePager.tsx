export const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const

export type PageSize = (typeof PAGE_SIZE_OPTIONS)[number]

export function isPageSize(n: number): n is PageSize {
  return (PAGE_SIZE_OPTIONS as readonly number[]).includes(n)
}

export default function TablePager({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  disabled = false,
}: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
  onPageSizeChange: (size: PageSize) => void
  disabled?: boolean
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : page * pageSize + 1
  const to = Math.min(total, (page + 1) * pageSize)

  return (
    <div className="table-pager">
      <label className="table-pager-size">
        <span>Per page</span>
        <select
          className="input input-compact table-pager-select"
          value={pageSize}
          aria-label="Rows per page"
          disabled={disabled}
          onChange={e => {
            const next = Number(e.target.value)
            if (isPageSize(next)) onPageSizeChange(next)
          }}
        >
          {PAGE_SIZE_OPTIONS.map(n => (
            <option key={n} value={n}>{n}</option>
          ))}
        </select>
      </label>
      <span className="num table-pager-range">
        {from}–{to} of {total}
      </span>
      <div className="table-pager-nav">
        <button
          type="button"
          className="btn btn-sm"
          disabled={disabled || page <= 0}
          onClick={() => onPageChange(Math.max(0, page - 1))}
        >
          Previous
        </button>
        <span className="num table-pager-page">
          Page {page + 1} of {totalPages}
        </span>
        <button
          type="button"
          className="btn btn-sm"
          disabled={disabled || page + 1 >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
        </button>
      </div>
    </div>
  )
}
