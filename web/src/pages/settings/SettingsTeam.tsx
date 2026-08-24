import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, Customer, TeamMember, UserRole } from '../../api'
import { ColGroup, ResizableTh, useColumnResize, useTableSort } from '../../components/ColumnResize'
import ConfirmDialog from '../../components/ConfirmDialog'
import KebabMenu from '../../components/KebabMenu'
import ModalCloseButton from '../../components/ModalCloseButton'
import PageHeader from '../../components/PageHeader'
import { useAuth } from '../../context/AuthContext'
import { colors } from '../../theme'

type RoleFilter = 'all' | UserRole
type CustomerFilterId = 'all' | 'platform' | string
type StatusFilter = 'all' | 'locked' | 'active'

export default function SettingsTeam() {
  const { user: currentUser, isPlatformAdmin } = useAuth()
  const colCount = isPlatformAdmin ? 5 : 4
  const tableRef = useRef<HTMLTableElement>(null)
  const { widths, startResize, autoFit } = useColumnResize(isPlatformAdmin ? 'team-5' : 'team-4', colCount)
  const [members, setMembers] = useState<TeamMember[]>([])
  const [customers, setCustomers] = useState<Customer[]>([])
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<UserRole>('viewer')
  const [tenantId, setTenantId] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [roleFilter, setRoleFilter] = useState<RoleFilter>('all')
  const [customerFilter, setCustomerFilter] = useState<CustomerFilterId>('all')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [deleteId, setDeleteId] = useState<string | null>(null)
  const [resetId, setResetId] = useState<string | null>(null)
  const [resetPassword, setResetPassword] = useState('')
  const [resetPassword2, setResetPassword2] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  async function load() {
    try {
      setMembers(await api.listTeam())
      if (isPlatformAdmin) {
        setCustomers(await api.listCustomers())
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users')
    }
  }

  useEffect(() => { load() }, [isPlatformAdmin])

  function customerName(id?: string) {
    if (!id) return 'Platform'
    return customers.find(c => c.id === id)?.name || id.slice(0, 8)
  }

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return members.filter(m => {
      if (roleFilter !== 'all' && m.role !== roleFilter) return false
      if (statusFilter === 'locked' && !m.locked) return false
      if (statusFilter === 'active' && m.locked) return false
      if (isPlatformAdmin) {
        if (customerFilter === 'platform' && m.tenant_id) return false
        if (customerFilter !== 'all' && customerFilter !== 'platform' && m.tenant_id !== customerFilter) {
          return false
        }
      }
      if (!q) return true
      const hay = [m.username, m.email || '', customerName(m.tenant_id)].join(' ').toLowerCase()
      return hay.includes(q)
    })
  }, [members, search, roleFilter, customerFilter, statusFilter, isPlatformAdmin, customers])

  const sortValue = useCallback((m: TeamMember, key: string) => {
    if (key === 'username') return m.username
    if (key === 'role') return m.role
    if (key === 'customer') return customerName(m.tenant_id)
    if (key === 'status') return m.locked ? 'locked' : 'active'
    return null
  }, [customers])
  const { sorted, header } = useTableSort(filtered, sortValue)

  function openAdd() {
    setUsername('')
    setEmail('')
    setPassword('')
    setRole('viewer')
    setTenantId('')
    setFormError('')
    setShowAdd(true)
  }

  function closeAdd() {
    if (busy) return
    setShowAdd(false)
    setFormError('')
  }

  async function handleAdd(e: FormEvent) {
    e.preventDefault()
    setFormError('')
    setMessage('')
    setBusy(true)
    try {
      await api.createTeamMember({
        username,
        email: email.trim() || undefined,
        password,
        role,
        tenant_id: isPlatformAdmin ? (tenantId || undefined) : undefined,
      })
      setShowAdd(false)
      setUsername('')
      setEmail('')
      setPassword('')
      setRole('viewer')
      setTenantId('')
      setMessage('User added')
      await load()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to add user')
    } finally {
      setBusy(false)
    }
  }

  async function handleRoleChange(id: string, newRole: UserRole) {
    setError('')
    setMessage('')
    try {
      await api.updateTeamMember(id, { role: newRole })
      setMessage('User updated')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  async function handleTenantChange(id: string, newTenant: string) {
    setError('')
    setMessage('')
    try {
      await api.updateTeamMember(id, { tenant_id: newTenant })
      setMessage('User updated')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Update failed')
    }
  }

  async function handleUnlock(id: string) {
    setError('')
    setMessage('')
    try {
      await api.unlockTeamMember(id)
      setMessage('User unlocked')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unlock failed')
    }
  }

  async function confirmDelete() {
    if (!deleteId) return
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await api.deleteTeamMember(deleteId)
      setDeleteId(null)
      setMessage('User removed')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    } finally {
      setBusy(false)
    }
  }

  async function confirmResetPassword() {
    if (!resetId) return
    if (resetPassword !== resetPassword2) {
      setError('Passwords do not match')
      return
    }
    setBusy(true)
    setError('')
    setMessage('')
    try {
      await api.resetTeamMemberPassword(resetId, resetPassword)
      setResetId(null)
      setResetPassword('')
      setResetPassword2('')
      setMessage('Password reset')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Password reset failed')
    } finally {
      setBusy(false)
    }
  }

  const deleteTarget = members.find(m => m.id === deleteId)
  const resetTarget = members.find(m => m.id === resetId)

  const adminRoleLabel = isPlatformAdmin ? 'Admin - Full Access' : 'Admin - Customer Access'
  const viewerRoleLabel = 'User - Read Only Access'

  return (
    <div className="page">
      <PageHeader
        title="Users"
        subtitle={
          isPlatformAdmin
            ? 'Manage platform and customer users. Unlock lockouts and reset passwords when needed.'
            : 'Manage users for your customer account. Unlock lockouts and reset passwords when needed.'
        }
      />

      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <div style={styles.card}>
        <div style={styles.cardHeader}>
          <div>
            <h3 style={{ ...styles.cardTitle, marginBottom: 6 }}>All users</h3>
            <p style={{ color: colors.textMuted, fontSize: 15, margin: 0 }}>
              {isPlatformAdmin
                ? 'Platform admins have full access. Assign a customer for customer admins and users.'
                : 'Admins can manage this customer’s monitors. Users can view only.'}
            </p>
          </div>
          <button type="button" className="btn btn-primary" onClick={openAdd}>
            Add user
          </button>
        </div>

        {members.length > 0 && (
          <div style={styles.filters}>
            <input
              className="input"
              style={styles.search}
              value={search}
              onChange={e => setSearch(e.target.value)}
              placeholder="Search username or email…"
              aria-label="Search username or email"
              autoComplete="off"
            />
            <select
              className="input"
              style={styles.filterSelect}
              value={roleFilter}
              onChange={e => setRoleFilter(e.target.value as RoleFilter)}
              aria-label="Filter by role"
            >
              <option value="all">All roles</option>
              <option value="admin">Admin</option>
              <option value="viewer">User</option>
            </select>
            <select
              className="input"
              style={styles.filterSelect}
              value={statusFilter}
              onChange={e => setStatusFilter(e.target.value as StatusFilter)}
              aria-label="Filter by status"
            >
              <option value="all">All statuses</option>
              <option value="active">Active</option>
              <option value="locked">Locked</option>
            </select>
            {isPlatformAdmin && (
              <select
                className="input"
                style={styles.filterSelect}
                value={customerFilter}
                onChange={e => setCustomerFilter(e.target.value)}
                aria-label="Filter by customer"
              >
                <option value="all">All customers</option>
                <option value="platform">Platform</option>
                {customers.map(c => (
                  <option key={c.id} value={c.id}>{c.name}</option>
                ))}
              </select>
            )}
          </div>
        )}

        {members.length === 0 ? (
          <p style={{ color: colors.textMuted }}>No users yet. Click “Add user” to create one.</p>
        ) : filtered.length === 0 ? (
          <p style={{ color: colors.textMuted }}>No users match the current filters.</p>
        ) : (
          <div className="data-table-wrap" style={styles.tableWrap}>
            <table ref={tableRef} className="data-table" style={styles.table}>
              <ColGroup widths={widths} />
              <thead>
                <tr>
                  <ResizableTh index={0} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('username')}>Username</ResizableTh>
                  <ResizableTh index={1} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('role')}>Role</ResizableTh>
                  {isPlatformAdmin && (
                    <ResizableTh index={2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('customer')}>Customer</ResizableTh>
                  )}
                  <ResizableTh index={isPlatformAdmin ? 3 : 2} style={styles.th} startResize={startResize} autoFit={autoFit} tableRef={tableRef} {...header('status')}>Status</ResizableTh>
                  <ResizableTh index={isPlatformAdmin ? 4 : 3} className="col-actions" resize={false} startResize={startResize} autoFit={autoFit} tableRef={tableRef} />
                </tr>
              </thead>
              <tbody>
                {sorted.map(m => (
                  <tr key={m.id}>
                    <td style={styles.td}>
                      <span style={styles.username}>
                        {m.username}
                        {m.id === currentUser?.id && <span style={styles.youBadge}>You</span>}
                      </span>
                    </td>
                    <td style={styles.td}>
                      <select
                        className="input input-compact"
                        style={styles.tableSelect}
                        value={m.role}
                        disabled={m.id === currentUser?.id}
                        onChange={e => handleRoleChange(m.id, e.target.value as UserRole)}
                        title={m.role === 'admin' ? adminRoleLabel : viewerRoleLabel}
                      >
                        <option value="admin">Admin</option>
                        <option value="viewer">User</option>
                      </select>
                    </td>
                    {isPlatformAdmin && (
                      <td style={styles.td}>
                        <select
                          className="input input-compact"
                          style={styles.tableSelect}
                          value={m.tenant_id || ''}
                          disabled={m.id === currentUser?.id}
                          onChange={e => handleTenantChange(m.id, e.target.value)}
                        >
                          <option value="">Platform</option>
                          {customers.map(c => (
                            <option key={c.id} value={c.id}>{c.name}</option>
                          ))}
                        </select>
                      </td>
                    )}
                    <td style={styles.td}>
                      {m.locked ? (
                        <span style={styles.lockedBadge}>Locked</span>
                      ) : (
                        <span style={styles.activeBadge}>Active</span>
                      )}
                    </td>
                    <td className="col-actions">
                      {(m.locked || m.id !== currentUser?.id) && (
                        <KebabMenu>
                          {close => (
                            <>
                              {m.locked && (
                                <button
                                  type="button"
                                  onClick={() => {
                                    close()
                                    handleUnlock(m.id)
                                  }}
                                >
                                  Unlock
                                </button>
                              )}
                              {m.id !== currentUser?.id && (
                                <>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      close()
                                      setResetId(m.id)
                                      setResetPassword('')
                                      setResetPassword2('')
                                      setError('')
                                    }}
                                  >
                                    Reset password
                                  </button>
                                  <button
                                    type="button"
                                    className="kebab-danger"
                                    onClick={() => {
                                      close()
                                      setDeleteId(m.id)
                                    }}
                                  >
                                    Remove
                                  </button>
                                </>
                              )}
                            </>
                          )}
                        </KebabMenu>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {members.length > 0 && (
          <p style={{ color: colors.textDim, fontSize: 13, margin: '14px 0 0' }}>
            Showing {filtered.length} of {members.length} users
          </p>
        )}
      </div>

      <ConfirmDialog
        open={!!deleteId}
        title="Remove user?"
        message={
          deleteTarget
            ? `Remove “${deleteTarget.username}”? They will lose access immediately.`
            : 'Remove this user? They will lose access immediately.'
        }
        confirmLabel="Remove"
        danger
        busy={busy}
        onConfirm={confirmDelete}
        onCancel={() => { if (!busy) setDeleteId(null) }}
      />

      {showAdd && (
        <div style={styles.modalBackdrop} onClick={closeAdd}>
          <form
            onSubmit={handleAdd}
            style={styles.modal}
            onClick={e => e.stopPropagation()}
            autoComplete="off"
            data-lpignore="true"
            data-1p-ignore="true"
            data-bwignore="true"
          >
            <input type="text" name="username" autoComplete="username" tabIndex={-1} aria-hidden style={styles.honeypot} />
            <input type="password" name="password" autoComplete="current-password" tabIndex={-1} aria-hidden style={styles.honeypot} />

            <div className="modal-head">
              <div className="modal-head-copy">
                <h3 className="form-modal-title">Add user</h3>
                <p className="form-modal-sub">
                  Create a new account
                  {isPlatformAdmin ? ' for the platform or a customer.' : ' for your customer account.'}
                </p>
              </div>
              <ModalCloseButton onClick={closeAdd} disabled={busy} />
            </div>
            {formError && <div style={{ ...styles.error, marginBottom: 16 }}>{formError}</div>}
            <Field label="Username" htmlFor="user-username">
              <CleanInput
                id="user-username"
                name="sentinel_username_new"
                value={username}
                onChange={setUsername}
                required
                autoComplete="off"
              />
            </Field>
            <Field label="Email (optional)" htmlFor="user-email">
              <CleanInput
                id="user-email"
                type="email"
                name="sentinel_email_new"
                value={email}
                onChange={setEmail}
                placeholder="For password reset"
                autoComplete="off"
              />
            </Field>
            <Field label="Password" htmlFor="user-password">
              <CleanInput
                id="user-password"
                type="password"
                name="sentinel_password_new"
                value={password}
                onChange={setPassword}
                required
                minLength={8}
                autoComplete="new-password"
              />
              <p style={{ color: colors.textMuted, fontSize: 13, margin: '8px 0 0' }}>
                At least 8 characters, with a letter and a number.
              </p>
            </Field>
            <Field label="Role" htmlFor="user-role">
              <select id="user-role" className="input" value={role} onChange={e => setRole(e.target.value as UserRole)}>
                <option value="admin">{adminRoleLabel}</option>
                <option value="viewer">{viewerRoleLabel}</option>
              </select>
            </Field>
            {isPlatformAdmin && (
              <Field label="Customer" htmlFor="user-customer">
                <select id="user-customer" className="input" value={tenantId} onChange={e => setTenantId(e.target.value)}>
                  <option value="">Platform (no customer)</option>
                  {customers.map(c => (
                    <option key={c.id} value={c.id}>{c.name}</option>
                  ))}
                </select>
              </Field>
            )}
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 8 }}>
              <button type="button" className="btn" disabled={busy} onClick={closeAdd}>Cancel</button>
              <button type="submit" className="btn btn-primary" disabled={busy}>
                {busy ? 'Adding…' : 'Add user'}
              </button>
            </div>
          </form>
        </div>
      )}

      {resetId && (
        <div style={styles.modalBackdrop} onClick={() => { if (!busy) setResetId(null) }}>
          <div style={styles.modal} onClick={e => e.stopPropagation()}>
            <div className="modal-head">
              <div className="modal-head-copy">
                <h3 className="form-modal-title">Reset password</h3>
                <p className="form-modal-sub">
                  Set a new password for {resetTarget?.username || 'this user'}. This also clears any login lockout.
                </p>
              </div>
              <ModalCloseButton onClick={() => { if (!busy) setResetId(null) }} disabled={busy} />
            </div>
            <Field label="New password" htmlFor="reset-password">
              <input
                id="reset-password"
                className="input"
                type="password"
                value={resetPassword}
                onChange={e => setResetPassword(e.target.value)}
                minLength={8}
                autoComplete="new-password"
              />
            </Field>
            <Field label="Confirm password" htmlFor="reset-password2">
              <input
                id="reset-password2"
                className="input"
                type="password"
                value={resetPassword2}
                onChange={e => setResetPassword2(e.target.value)}
                minLength={8}
                autoComplete="new-password"
              />
            </Field>
            <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 8 }}>
              <button type="button" className="btn" disabled={busy} onClick={() => setResetId(null)}>Cancel</button>
              <button
                type="button"
                className="btn btn-primary"
                disabled={busy || resetPassword.length < 8 || resetPassword !== resetPassword2}
                onClick={confirmResetPassword}
              >
                {busy ? 'Saving…' : 'Reset password'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function CleanInput({
  id,
  name,
  value,
  onChange,
  type = 'text',
  placeholder,
  required,
  minLength,
  autoComplete = 'off',
}: {
  id: string
  name: string
  value: string
  onChange: (v: string) => void
  type?: string
  placeholder?: string
  required?: boolean
  minLength?: number
  autoComplete?: string
}) {
  const [unlocked, setUnlocked] = useState(false)
  return (
    <div className="input-shell">
      <input
        id={id}
        className="input-shell-control"
        type={type}
        name={name}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
        required={required}
        minLength={minLength}
        autoComplete={autoComplete}
        autoCorrect="off"
        spellCheck={false}
        readOnly={!unlocked}
        onFocus={() => setUnlocked(true)}
        data-lpignore="true"
        data-1p-ignore="true"
        data-bwignore="true"
        data-form-type="other"
      />
    </div>
  )
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string
  htmlFor: string
  children: React.ReactNode
}) {
  return (
    <div className="field" style={{ marginBottom: 18 }}>
      <label className="field-label" htmlFor={htmlFor}>{label}</label>
      {children}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px', minWidth: 0,
  },
  cardHeader: {
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: 16,
    marginBottom: 20,
  },
  cardTitle: { margin: '0 0 20px', fontSize: 17, fontWeight: 600 },
  filters: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 10,
    marginBottom: 16,
  },
  search: {
    flex: '1 1 220px',
    minWidth: 180,
  },
  filterSelect: {
    flex: '0 1 150px',
    minWidth: 130,
  },
  tableWrap: { width: '100%' },
  table: {
    width: '100%',
    minWidth: 760,
    borderCollapse: 'separate',
    borderSpacing: 0,
    fontSize: 15,
  },
  th: {
    textAlign: 'left',
    fontSize: 12,
    fontWeight: 600,
    color: colors.textMuted,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
    padding: '0 12px 12px',
    borderBottom: `1px solid ${colors.border}`,
    whiteSpace: 'nowrap',
  },
  td: {
    padding: '14px 12px',
    borderBottom: `1px solid ${colors.border}`,
    verticalAlign: 'middle',
  },
  tableSelect: {
    width: '100%',
    maxWidth: '100%',
    minWidth: 0,
    boxSizing: 'border-box',
  },
  username: { fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 8, minWidth: 0 },
  youBadge: {
    fontSize: 11, fontWeight: 600, color: colors.brand,
    background: colors.brandDim, padding: '2px 6px', borderRadius: 4,
    flexShrink: 0,
  },
  lockedBadge: {
    display: 'inline-block',
    fontSize: 12, fontWeight: 600, color: colors.red,
    background: colors.redDim, padding: '3px 8px', borderRadius: 4,
    whiteSpace: 'nowrap',
  },
  activeBadge: {
    display: 'inline-block',
    fontSize: 12, fontWeight: 600, color: colors.green,
    background: colors.greenDim, padding: '3px 8px', borderRadius: 4,
    whiteSpace: 'nowrap',
  },
  roleBadge: { fontSize: 13, color: colors.textMuted },
  actionRow: {
    display: 'inline-flex',
    flexWrap: 'nowrap',
    gap: 8,
    justifyContent: 'flex-end',
    alignItems: 'center',
  },
  actionBtn: { fontSize: 13, padding: '6px 10px', minHeight: 32, whiteSpace: 'nowrap' },
  deleteBtn: { fontSize: 13, padding: '6px 10px', minHeight: 32, color: colors.red, whiteSpace: 'nowrap' },
  honeypot: {
    position: 'absolute',
    left: -9999,
    width: 1,
    height: 1,
    opacity: 0,
    pointerEvents: 'none',
  },
  modalBackdrop: {
    position: 'fixed', inset: 0, background: 'var(--overlay)',
    display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 50, padding: 16,
  },
  modal: {
    background: colors.card, border: `1px solid ${colors.border}`, borderRadius: 10,
    padding: 24, width: '100%', maxWidth: 440, maxHeight: '90vh', overflowY: 'auto',
    boxShadow: 'var(--shadow)',
  },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(34,197,94,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(239,68,68,0.3)`,
  },
}
