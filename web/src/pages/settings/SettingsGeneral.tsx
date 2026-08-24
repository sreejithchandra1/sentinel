import { ChangeEvent, FormEvent, useEffect, useState } from 'react'
import AppLogo from '../../components/AppLogo'
import ConfirmDialog from '../../components/ConfirmDialog'
import { api, OrgSettings } from '../../api'
import { colors } from '../../theme'

const MAX_LOGO_BYTES = 512 * 1024

export default function SettingsGeneral() {
  const [cfg, setCfg] = useState<OrgSettings>({ company_name: '', tagline: '', logo: '' })
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [resetOpen, setResetOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => { api.getGeneral().then(setCfg).catch(() => {}) }, [])

  function set<K extends keyof OrgSettings>(key: K, value: OrgSettings[K]) {
    setCfg(prev => ({ ...prev, [key]: value }))
  }

  async function handleLogoChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) {
      setError('Please select an image file')
      return
    }
    if (file.size > MAX_LOGO_BYTES) {
      setError('Logo must be 512KB or smaller')
      return
    }
    setError('')
    const reader = new FileReader()
    reader.onload = () => set('logo', reader.result as string)
    reader.readAsDataURL(file)
    e.target.value = ''
  }

  async function confirmReset() {
    setBusy(true)
    setError('')
    setMessage('')
    try {
      const reset = await api.resetGeneral()
      setCfg(reset)
      setResetOpen(false)
      setMessage('Organization settings reset to defaults')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Reset failed')
    } finally {
      setBusy(false)
    }
  }

  async function handleSave(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const saved = await api.putGeneral(cfg)
      setCfg(saved)
      setMessage('Organization settings saved')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    }
  }

  const displayName = cfg.company_name || 'Sentinel'

  return (
    <>
      {message && <div style={styles.ok}>{message}</div>}
      {error && <div style={styles.error} role="alert">{error}</div>}

      <div className="split-panels">
        <form onSubmit={handleSave} style={styles.card}>
          <h3 style={styles.cardTitle}>Organization</h3>
          <Field label="Company Name">
            <input
              className="input"
              value={cfg.company_name}
              onChange={e => set('company_name', e.target.value)}
              placeholder="Acme Corp"
            />
          </Field>
          <Field label="Tagline">
            <input
              className="input"
              value={cfg.tagline}
              onChange={e => set('tagline', e.target.value)}
              placeholder="Infrastructure monitoring"
            />
          </Field>

          <div style={{ marginTop: 8 }}>
            <span className="field-label" style={{ display: 'block', marginBottom: 10 }}>Logo</span>
            <div style={styles.logoRow}>
              <AppLogo src={cfg.logo || null} size={56} alt="Company logo" />
              <div style={styles.logoActions}>
                <label className="btn" style={{ cursor: 'pointer' }}>
                  Upload
                  <input type="file" accept="image/*" onChange={handleLogoChange} style={{ display: 'none' }} />
                </label>
                {cfg.logo && (
                  <button type="button" className="btn" onClick={() => set('logo', '')}>Remove</button>
                )}
              </div>
            </div>
            <p style={styles.hint}>PNG, JPG, or SVG. Max 512KB. Shown in the sidebar. Empty uses the default Sentinel logo.</p>
          </div>

          <div style={{ display: 'flex', gap: 10, marginTop: 20 }}>
            <button type="submit" className="btn btn-primary">Save Changes</button>
            <button type="button" className="btn" onClick={() => setResetOpen(true)}>Reset to Default</button>
          </div>
        </form>

        <div style={styles.card}>
          <h3 style={styles.cardTitle}>Preview</h3>
          <p style={{ color: colors.textMuted, fontSize: 15, margin: '0 0 20px' }}>
            How your branding appears in the sidebar.
          </p>
          <div style={styles.preview}>
            <AppLogo src={cfg.logo || null} size={32} />
            <div>
              <div style={{ fontWeight: 700, fontSize: 17 }}>{displayName}</div>
              {cfg.tagline && <div style={{ color: colors.textMuted, fontSize: 13, marginTop: 2 }}>{cfg.tagline}</div>}
            </div>
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={resetOpen}
        title="Reset branding?"
        message="Reset organization branding to defaults? This removes your company name, tagline, and logo."
        confirmLabel="Reset"
        danger
        busy={busy}
        onConfirm={confirmReset}
        onCancel={() => { if (!busy) setResetOpen(false) }}
      />
    </>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="field" style={{ marginBottom: 16 }}>
      <span className="field-label">{label}</span>
      {children}
    </label>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    background: colors.card, border: `1px solid ${colors.border}`,
    borderRadius: 10, padding: '24px 28px', minWidth: 0,
  },
  cardTitle: { margin: '0 0 20px', fontSize: 17, fontWeight: 600 },
  logoRow: { display: 'flex', alignItems: 'center', gap: 16 },
  logoPreview: {
    width: 56, height: 56, borderRadius: 10, objectFit: 'contain',
    background: colors.bg, border: `1px solid ${colors.border}`,
  },
  logoPlaceholder: {
    width: 56, height: 56, borderRadius: 10,
    background: `linear-gradient(135deg, ${colors.brand}, ${colors.brandDeep})`,
    color: colors.bg, display: 'grid', placeItems: 'center',
    fontSize: 24, fontWeight: 700,
  },
  logoActions: { display: 'flex', gap: 8, flexWrap: 'wrap' },
  hint: { color: colors.textMuted, fontSize: 13, margin: '10px 0 0' },
  preview: {
    display: 'flex', alignItems: 'center', gap: 12,
    padding: '12px 14px', borderRadius: 10,
    background: colors.sidebar, border: `1px solid ${colors.border}`,
  },
  previewLogo: { width: 32, height: 32, borderRadius: 6, objectFit: 'contain' },
  previewIcon: { color: colors.brand, fontSize: 24, width: 32, textAlign: 'center' },
  ok: {
    background: colors.greenDim, color: colors.green, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(63,185,80,0.3)`,
  },
  error: {
    background: colors.redDim, color: colors.red, padding: 12,
    borderRadius: 8, marginBottom: 16, border: `1px solid rgba(248,81,73,0.3)`,
  },
}
