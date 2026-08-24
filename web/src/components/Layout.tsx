import { Link, useLocation } from 'react-router-dom'
import { ReactNode, useEffect, useState } from 'react'
import AppLogo from './AppLogo'
import NavIcon from './NavIcon'
import ProfileMenu from './ProfileMenu'
import TableCellTooltip from './TableCellTooltip'
import ThemeToggle from './ThemeToggle'
import { api, OrgSettings } from '../api'
import { useAuth } from '../context/AuthContext'
import { iconSizes, icons, NavIconKey } from '../icons'

const COLLAPSE_KEY = 'sentinel.sidebar.collapsed'
const MOBILE_MQ = '(max-width: 900px)'

function useMobile() {
  const [mobile, setMobile] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(MOBILE_MQ).matches : false,
  )
  useEffect(() => {
    const mq = window.matchMedia(MOBILE_MQ)
    const onChange = () => setMobile(mq.matches)
    onChange()
    mq.addEventListener('change', onChange)
    window.addEventListener('resize', onChange)
    return () => {
      mq.removeEventListener('change', onChange)
      window.removeEventListener('resize', onChange)
    }
  }, [])
  return mobile
}

function CollapseIcon({ collapsed }: { collapsed: boolean }) {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path
        d={collapsed ? 'M6 3.5 11 8l-5 4.5' : 'M10 3.5 5 8l5 4.5'}
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function MenuIcon({ open }: { open: boolean }) {
  return (
    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
      {open ? (
        <path d="M4.5 4.5 13.5 13.5M13.5 4.5 4.5 13.5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      ) : (
        <path d="M3 5h12M3 9h12M3 13h12" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
      )}
    </svg>
  )
}

type NavItem = {
  path: string
  label: string
  icon: NavIconKey
  adminOnly?: boolean
  platformOnly?: boolean
}

const allNavItems: NavItem[] = [
  { path: '/', label: 'Monitors', icon: 'monitors' },
  { path: '/incidents', label: 'Incidents', icon: 'incidents' },
  { path: '/performance', label: 'Performance', icon: 'performance' },
  { path: '/customers', label: 'Customers', icon: 'customers', platformOnly: true },
  { path: '/users', label: 'Users', icon: 'users', adminOnly: true },
  { path: '/settings', label: 'Settings', icon: 'settings', adminOnly: true },
]

function isActivePath(pathname: string, path: string) {
  if (path === '/') {
    return pathname === '/' || (pathname.startsWith('/monitors/') && !pathname.endsWith('/new'))
  }
  if (path === '/performance') return pathname.startsWith('/performance')
  if (path === '/incidents') return pathname.startsWith('/incidents')
  if (path === '/customers') return pathname.startsWith('/customers')
  if (path === '/users') return pathname.startsWith('/users')
  return pathname.startsWith(path)
}

export default function Layout({ children, onLogout }: { children: ReactNode; onLogout: () => void }) {
  const location = useLocation()
  const { isAdmin, isPlatformAdmin } = useAuth()
  const mobile = useMobile()
  const [org, setOrg] = useState<OrgSettings | null>(null)
  const [navOpen, setNavOpen] = useState(false)
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(COLLAPSE_KEY) === '1'
    } catch {
      return false
    }
  })

  function toggleCollapsed() {
    setCollapsed(v => {
      const next = !v
      try {
        localStorage.setItem(COLLAPSE_KEY, next ? '1' : '0')
      } catch {
        /* ignore quota */
      }
      return next
    })
  }

  const navItems = allNavItems.filter(item => {
    if (item.platformOnly && !isPlatformAdmin) return false
    if (item.adminOnly && !isAdmin) return false
    return true
  })

  useEffect(() => {
    api.getGeneral().then(setOrg).catch(() => {})
  }, [location.pathname])

  useEffect(() => {
    setNavOpen(false)
  }, [location.pathname, mobile])

  useEffect(() => {
    if (!navOpen) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setNavOpen(false)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [navOpen])

  useEffect(() => {
    if (!mobile || !navOpen) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [mobile, navOpen])

  const brandName = org?.company_name || 'Sentinel'
  const tagline = org?.tagline || 'Infrastructure Monitoring'
  const sidebarOpen = !mobile || navOpen

  return (
    <div className="app-shell">
      <a href="#main-content" className="skip-link">Skip to main content</a>

      <header className="app-topbar">
        <button
          type="button"
          className="app-menu-btn"
          aria-label={navOpen ? 'Close navigation' : 'Open navigation'}
          aria-expanded={navOpen}
          aria-controls="app-sidebar"
          onClick={() => setNavOpen(v => !v)}
        >
          <MenuIcon open={navOpen} />
        </button>
        <Link to="/" className="app-topbar-brand">
          <AppLogo src={org?.logo} size={iconSizes.nav} alt="" />
          <span>{brandName}</span>
        </Link>
        <div className="app-topbar-actions">
          <ThemeToggle />
        </div>
      </header>

      {navOpen && (
        <button
          type="button"
          className="app-backdrop"
          aria-label="Close navigation"
          onClick={() => setNavOpen(false)}
        />
      )}

      <aside
        id="app-sidebar"
        className={'app-sidebar' + (collapsed ? ' is-collapsed' : '') + (navOpen ? ' is-open' : '')}
        aria-label="Primary"
        aria-hidden={mobile && !navOpen ? true : undefined}
        ref={el => {
          if (!el) return
          if (mobile && !navOpen) el.setAttribute('inert', '')
          else el.removeAttribute('inert')
        }}
      >
        <div className={'app-brand' + (collapsed ? ' is-collapsed' : '')}>
          <Link to="/" className="app-logo" title={brandName}>
            <AppLogo src={org?.logo} size={iconSizes.brandLogo} alt={brandName} />
            <span className="app-logo-text">
              <span className="app-logo-name">{brandName}</span>
              <span className="app-logo-tagline">{tagline}</span>
            </span>
          </Link>
          <button
            type="button"
            className="app-collapse-btn"
            onClick={toggleCollapsed}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            aria-pressed={collapsed}
            title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <CollapseIcon collapsed={collapsed} />
          </button>
        </div>

        <nav className="app-nav" aria-label="Application">
          {navItems.map(item => {
            const active = isActivePath(location.pathname, item.path)
            return (
              <Link
                key={item.path}
                to={item.path}
                title={item.label}
                aria-label={item.label}
                aria-current={active ? 'page' : undefined}
                className={'app-nav-item' + (active ? ' is-active' : '') + (collapsed ? ' is-icon' : '')}
                tabIndex={sidebarOpen ? undefined : -1}
              >
                {active && <span className="app-nav-accent" aria-hidden="true" />}
                <NavIcon src={icons[item.icon]} size={iconSizes.nav} alt="" />
                <span className="app-nav-label">{item.label}</span>
              </Link>
            )
          })}
        </nav>

        <div className="app-sidebar-footer">
          <ProfileMenu onLogout={onLogout} collapsed={collapsed && !mobile} />
        </div>
      </aside>

      <div className="app-main">
        <TableCellTooltip />
        <main id="main-content" className="app-content" tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  )
}
