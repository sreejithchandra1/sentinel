import { Outlet, Navigate, useLocation } from 'react-router-dom'
import PageHeader from '../../components/PageHeader'
import SegmentedTabs from '../../components/SegmentedTabs'
import { useAuth } from '../../context/AuthContext'

const allTabs = [
  { path: '/settings/general', label: 'General', platformOnly: true },
  { path: '/settings/notifications', label: 'Notifications', platformOnly: false, adminOnly: true },
  { path: '/settings/maintenance', label: 'Maintenance', platformOnly: true },
  { path: '/settings/server', label: 'Server', platformOnly: true },
  { path: '/settings/status-page', label: 'Status Page', platformOnly: true },
  { path: '/settings/tokens', label: 'API Tokens', platformOnly: false },
  { path: '/settings/audit', label: 'Audit', platformOnly: true },
]

export default function SettingsLayout() {
  const location = useLocation()
  const { isPlatformAdmin, isAdmin } = useAuth()
  const tabs = allTabs.filter(t => {
    if (t.platformOnly && !isPlatformAdmin) return false
    if (t.adminOnly && !isAdmin) return false
    return true
  })
  const defaultPath = isPlatformAdmin ? '/settings/general' : (isAdmin ? '/settings/notifications' : '/settings/tokens')

  if (location.pathname === '/settings') {
    return <Navigate to={defaultPath} replace />
  }

  if (location.pathname === '/settings/smtp') {
    return <Navigate to="/settings/notifications/email" replace />
  }
  if (location.pathname === '/settings/webhooks') {
    return <Navigate to="/settings/notifications/webhooks" replace />
  }

  const allowed = tabs.some(t => location.pathname === t.path || location.pathname.startsWith(t.path + '/'))
  if (!allowed) {
    return <Navigate to={defaultPath} replace />
  }

  return (
    <div className="page">
      <div className="page-sticky">
        <PageHeader
          title="Settings"
          subtitle={
            isPlatformAdmin
              ? 'Configure organization details and alert delivery.'
              : isAdmin
                ? 'Manage notifications and API tokens for your account.'
                : 'Manage API tokens for this account.'
          }
        />
        <div className="page-sticky-extra">
          <SegmentedTabs
            label="Settings sections"
            tabs={tabs.map(tab => ({
              id: tab.path,
              label: tab.label,
              to: tab.path,
              end: tab.path !== '/settings/notifications',
            }))}
          />
        </div>
      </div>

      <Outlet />
    </div>
  )
}
