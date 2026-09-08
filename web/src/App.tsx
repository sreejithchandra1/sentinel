import { Navigate, Route, Routes } from 'react-router-dom'
import { useCallback, useEffect, useState } from 'react'
import AdminRoute from './components/AdminRoute'
import PlatformAdminRoute from './components/PlatformAdminRoute'
import Layout from './components/Layout'
import Login from './pages/Login'
import ResetPassword from './pages/ResetPassword'
import Monitors from './pages/Monitors'
import Performance from './pages/Performance'
import PerformanceDetail from './pages/PerformanceDetail'
import PerformanceForm from './pages/PerformanceForm'
import MonitorDetail from './pages/MonitorDetail'
import MonitorForm from './pages/MonitorForm'
import Hosts from './pages/Hosts'
import HostDetail from './pages/HostDetail'
import SettingsLayout from './pages/settings/SettingsLayout'
import SettingsGeneral from './pages/settings/SettingsGeneral'
import SettingsSMTP from './pages/settings/SettingsSMTP'
import SettingsTeam from './pages/settings/SettingsTeam'
import SettingsWebhooks from './pages/settings/SettingsWebhooks'
import SettingsNotifications from './pages/settings/SettingsNotifications'
import SettingsSlack from './pages/settings/SettingsSlack'
import SettingsMaintenance from './pages/settings/SettingsMaintenance'
import SettingsServer from './pages/settings/SettingsServer'
import SettingsStatusPage from './pages/settings/SettingsStatusPage'
import SettingsTokens from './pages/settings/SettingsTokens'
import SettingsAudit from './pages/settings/SettingsAudit'
import SettingsEmailLog from './pages/settings/SettingsEmailLog'
import SettingsCustomers from './pages/settings/SettingsCustomers'
import Incidents from './pages/Incidents'
import IncidentDetail from './pages/IncidentDetail'
import SLAReportPage from './pages/SLAReport'
import StatusPage from './pages/StatusPage'
import Profile from './pages/Profile'
import { api, Profile as AuthProfile } from './api'
import { AuthProvider } from './context/AuthContext'
import { applyFavicon } from './components/AppLogo'
import { colors } from './theme'

export default function App() {
  const [user, setUser] = useState<AuthProfile | null>(null)
  const [authed, setAuthed] = useState<boolean | null>(null)

  const refreshUser = useCallback(async () => {
    const profile = await api.getProfile()
    setUser(profile)
    setAuthed(true)
  }, [])

  useEffect(() => {
    api.publicBranding()
      .then(b => applyFavicon(b.logo))
      .catch(() => applyFavicon(null))
  }, [])

  useEffect(() => {
    const t0 = performance.now()
    api.getProfile()
      .then(profile => {
        // #region agent log
        fetch('http://127.0.0.1:7473/ingest/49c06dfb-d2a4-42ad-83ca-3f7de467dc84',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'f7d7ab'},body:JSON.stringify({sessionId:'f7d7ab',runId:'pre-fix',hypothesisId:'D',location:'App.tsx:profile',message:'Auth gate complete',data:{ms:Math.round(performance.now()-t0),path:location.pathname},timestamp:Date.now()})}).catch(()=>{})
        // #endregion
        setUser(profile); setAuthed(true)
      })
      .catch(() => setAuthed(false))
  }, [])

  if (authed === null) {
    return (
      <div style={{ padding: 40, textAlign: 'center', color: colors.textMuted, minHeight: '100vh', background: colors.bg, display: 'grid', placeItems: 'center' }}>
        Loading…
      </div>
    )
  }

  if (!authed) {
    return (
      <Routes>
        <Route path="/login" element={<Login onLogin={refreshUser} />} />
        <Route path="/reset-password" element={<ResetPassword />} />
        <Route path="/status" element={<StatusPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    )
  }

  return (
    <AuthProvider user={user} refresh={refreshUser}>
      <Layout onLogout={async () => { await api.logout(); setUser(null); setAuthed(false) }}>
        <Routes>
          <Route path="/" element={<Monitors />} />
          <Route path="/incidents" element={<Incidents />} />
          <Route path="/incidents/:id" element={<IncidentDetail />} />
          <Route path="/sla" element={<SLAReportPage />} />
          <Route path="/status" element={<StatusPage />} />
          <Route path="/performance" element={<Performance />} />
          <Route path="/performance/targets/new" element={<AdminRoute><PerformanceForm /></AdminRoute>} />
          <Route path="/performance/targets/:id/edit" element={<AdminRoute><PerformanceForm /></AdminRoute>} />
          <Route path="/performance/:id" element={<PerformanceDetail />} />
          <Route path="/monitors/new" element={<AdminRoute><MonitorForm /></AdminRoute>} />
          <Route path="/monitors/:id" element={<MonitorDetail />} />
          <Route path="/monitors/:id/edit" element={<AdminRoute><MonitorForm /></AdminRoute>} />
          <Route path="/hosts" element={<Hosts />} />
          <Route path="/hosts/:id" element={<HostDetail />} />
          <Route path="/customers" element={<PlatformAdminRoute><SettingsCustomers /></PlatformAdminRoute>} />
          <Route path="/users" element={<AdminRoute><SettingsTeam /></AdminRoute>} />
          <Route path="/settings/customers" element={<Navigate to="/customers" replace />} />
          <Route path="/settings/team" element={<Navigate to="/users" replace />} />
          <Route path="/settings" element={<AdminRoute><SettingsLayout /></AdminRoute>}>
            <Route path="general" element={<SettingsGeneral />} />
            <Route path="notifications" element={<SettingsNotifications />} />
            <Route path="notifications/email" element={<PlatformAdminRoute><SettingsSMTP /></PlatformAdminRoute>} />
            <Route path="notifications/slack" element={<SettingsSlack />} />
            <Route path="notifications/webhooks" element={<PlatformAdminRoute><SettingsWebhooks /></PlatformAdminRoute>} />
            <Route path="smtp" element={<Navigate to="/settings/notifications/email" replace />} />
            <Route path="webhooks" element={<Navigate to="/settings/notifications/webhooks" replace />} />
            <Route path="maintenance" element={<SettingsMaintenance />} />
            <Route path="server" element={<SettingsServer />} />
            <Route path="status-page" element={<SettingsStatusPage />} />
            <Route path="tokens" element={<SettingsTokens />} />
            <Route path="audit" element={<SettingsAudit />} />
            <Route path="email-log" element={<PlatformAdminRoute><SettingsEmailLog /></PlatformAdminRoute>} />
          </Route>
          <Route path="/profile" element={<Profile />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </AuthProvider>
  )
}
