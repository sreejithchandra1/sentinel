import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { api, Profile } from '../api'
import { roleLabel, useAuth } from '../context/AuthContext'
import { colors } from '../theme'

function initialsFrom(name: string, username: string): string {
  const source = (name || username || 'U').trim()
  const parts = source.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  return source.slice(0, 2).toUpperCase()
}

export default function ProfileMenu({ onLogout, collapsed }: { onLogout: () => void; collapsed?: boolean }) {
  const { user } = useAuth()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [open, setOpen] = useState(false)
  const location = useLocation()

  useEffect(() => {
    api.getProfile().then(setProfile).catch(() => {})
  }, [location.pathname])

  useEffect(() => {
    setOpen(false)
  }, [collapsed, location.pathname])

  const username = profile?.username || user?.username || 'admin'
  const name = (profile?.name || user?.name || '').trim()
  const displayName = name || username
  const initials = initialsFrom(name, username)

  return (
    <div style={{ ...styles.wrap, padding: collapsed ? 0 : '0 4px' }}>
      <button
        type="button"
        onClick={() => setOpen(v => !v)}
        style={{
          ...styles.profileBtn,
          ...(collapsed ? styles.profileBtnCollapsed : {}),
        }}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={displayName}
      >
        <span style={{ ...styles.avatar, ...(collapsed ? styles.avatarCollapsed : {}) }}>{initials}</span>
        {!collapsed && (
          <>
            <span style={styles.profileInfo}>
              <span style={styles.profileName}>{displayName}</span>
              <span style={styles.roleChip}>{profile ? roleLabel(profile.role) : 'Admin'}</span>
            </span>
            <span style={{ ...styles.chevron, transform: open ? 'rotate(180deg)' : 'none' }}>▾</span>
          </>
        )}
      </button>

      {open && (
        <div style={{ ...styles.menu, ...(collapsed ? styles.menuCollapsed : {}) }}>
          <div style={styles.signedIn}>Signed in as @{username}</div>
          <Link to="/profile" style={styles.menuItem} onClick={() => setOpen(false)}>
            Profile Settings
          </Link>
          <button type="button" onClick={() => { setOpen(false); onLogout() }} style={styles.menuItemBtn}>
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  wrap: { position: 'relative', padding: '0 4px' },
  profileBtn: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    width: '100%',
    padding: '10px 12px',
    borderRadius: 8,
    border: 'none',
    background: 'transparent',
    color: colors.text,
    textAlign: 'left',
  },
  profileBtnCollapsed: {
    justifyContent: 'center',
    padding: '8px 0',
    gap: 0,
  },
  avatar: {
    width: 40,
    height: 40,
    borderRadius: '50%',
    flexShrink: 0,
    background: colors.brandDim,
    color: colors.brand,
    display: 'grid',
    placeItems: 'center',
    fontSize: 14,
    fontWeight: 700,
    border: `1px solid color-mix(in srgb, ${colors.brand} 35%, transparent)`,
  },
  avatarCollapsed: {
    width: 36,
    height: 36,
    fontSize: 13,
  },
  profileInfo: { display: 'flex', flexDirection: 'column', flex: 1, minWidth: 0 },
  profileName: {
    fontSize: 15,
    fontWeight: 600,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  roleChip: {
    display: 'inline-flex',
    alignSelf: 'flex-start',
    marginTop: 4,
    padding: '1px 7px',
    borderRadius: 999,
    fontSize: 12,
    fontWeight: 600,
    letterSpacing: '0.02em',
    color: colors.textMuted,
    background: colors.bgElevated,
    border: `1px solid ${colors.border}`,
    lineHeight: 1.4,
  },
  chevron: { fontSize: 11, color: colors.textMuted, transition: 'transform 0.15s', flexShrink: 0 },
  menu: {
    position: 'absolute',
    bottom: '100%',
    left: 4,
    right: 4,
    marginBottom: 8,
    background: colors.card,
    border: `1px solid ${colors.border}`,
    borderRadius: 10,
    overflow: 'hidden',
    boxShadow: 'var(--shadow)',
    zIndex: 20,
  },
  menuCollapsed: {
    left: 'calc(100% + 8px)',
    right: 'auto',
    bottom: 0,
    width: 220,
    marginBottom: 0,
  },
  signedIn: {
    padding: '10px 14px 8px',
    fontSize: 13,
    color: colors.textMuted,
    borderBottom: `1px solid ${colors.border}`,
  },
  menuItem: {
    display: 'block',
    padding: '12px 14px',
    fontSize: 14,
    fontWeight: 500,
    color: colors.text,
    textDecoration: 'none',
    borderBottom: `1px solid ${colors.border}`,
  },
  menuItemBtn: {
    display: 'block',
    width: '100%',
    padding: '12px 14px',
    fontSize: 14,
    fontWeight: 500,
    color: colors.red,
    background: 'transparent',
    border: 'none',
    textAlign: 'left',
  },
}
