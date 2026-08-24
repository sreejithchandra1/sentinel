import { FormEvent, useEffect, useState } from 'react'
import AppLogo from '../components/AppLogo'
import { api, OrgSettings } from '../api'
import { colors } from '../theme'

type Mode = 'login' | 'forgot'
type LoginStep = 'credentials' | 'code'

export default function Login({ onLogin }: { onLogin: () => void }) {
  const [mode, setMode] = useState<Mode>('login')
  const [step, setStep] = useState<LoginStep>('credentials')
  const [username, setUsername] = useState('admin')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [challengeId, setChallengeId] = useState('')
  const [emailHint, setEmailHint] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [sent, setSent] = useState(false)
  const [brand, setBrand] = useState<OrgSettings | null>(null)

  useEffect(() => {
    api.publicBranding().then(setBrand).catch(() => {})
  }, [])

  async function handleLogin(e: FormEvent) {
    e.preventDefault()
    setError('')
    setMessage('')
    try {
      const res = await api.login(username, password)
      if (res.mfa_required && res.challenge_id) {
        setChallengeId(res.challenge_id)
        setEmailHint(res.email_hint || '')
        setCode('')
        setStep('code')
        setMessage(res.message || 'Check your email for a verification code.')
        return
      }
      onLogin()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    }
  }

  async function handleVerifyCode(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      const res = await api.verifyMFALogin(challengeId, code.trim())
      if (res.ok) {
        onLogin()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed')
    }
  }

  async function handleResendCode() {
    setError('')
    try {
      const res = await api.resendMFALogin(challengeId)
      if (res.challenge_id) setChallengeId(res.challenge_id)
      if (res.email_hint) setEmailHint(res.email_hint)
      setCode('')
      setMessage(res.message || 'A new verification code is on its way.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not resend code')
    }
  }

  async function handleForgot(e: FormEvent) {
    e.preventDefault()
    if (sent) return
    setError('')
    setSent(true)
    try {
      await api.forgotPassword(email.trim())
      setMessage('Password reset link sent to your email.')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Request failed')
      setSent(false)
    }
  }

  return (
    <div className="auth-split">
      <div className="auth-split-brand">
        <AppLogo src={brand?.logo} size={48} alt={brand?.company_name || 'Sentinel'} />
        <p style={styles.intro}>INTRODUCING</p>
        <h1 className="auth-hero">{brand?.company_name || 'Sentinel'}</h1>
        <p style={styles.tagline}>{brand?.tagline || 'Self-hosted infrastructure monitoring for websites, ports, SSL, and DNS.'}</p>
      </div>
      <form onSubmit={mode === 'login' ? (step === 'credentials' ? handleLogin : handleVerifyCode) : handleForgot} className="auth-split-form">
        <h2 style={{ margin: '0 0 4px', fontSize: 22 }}>
          {mode === 'login' ? (step === 'credentials' ? 'Sign in' : 'Verify sign-in') : 'Reset password'}
        </h2>
        <p style={{ color: colors.textMuted, margin: '0 0 24px', fontSize: 15 }}>
          {mode === 'login' && step === 'code'
            ? `Enter the verification code sent to ${emailHint || 'your email address'}.`
            : mode === 'login'
            ? 'Access your monitoring dashboard'
            : 'Enter your email address and we will send you a reset link.'}
        </p>
        {message && <div className="flash-ok" role="status">{message}</div>}
        {error && <div className="flash-error" role="alert">{error}</div>}
        {mode === 'login' ? (
          step === 'credentials' ? (
          <>
            <label className="field">
              <span className="field-label">Username</span>
              <input className="input" autoComplete="username" value={username} onChange={e => setUsername(e.target.value)} required />
            </label>
            <label className="field" style={{ marginTop: 16 }}>
              <span className="field-label">Password</span>
              <input className="input" type="password" autoComplete="current-password" value={password} onChange={e => setPassword(e.target.value)} required />
            </label>
            <button type="submit" className="btn btn-primary" style={{ marginTop: 24, width: '100%', justifyContent: 'center', padding: 12 }}>
              Sign in
            </button>
          </>
          ) : (
          <>
            <label className="field">
              <span className="field-label">Verification Code</span>
              <input
                className="input"
                value={code}
                onChange={e => setCode(e.target.value.toUpperCase())}
                required
                autoFocus
                autoComplete="one-time-code"
                inputMode="text"
                placeholder="Enter 8-character code"
              />
            </label>
            <button type="submit" className="btn btn-primary" style={{ marginTop: 24, width: '100%', justifyContent: 'center', padding: 12 }}>
              Verify and sign in
            </button>
            <button type="button" className="btn" style={{ marginTop: 12, width: '100%', justifyContent: 'center' }} onClick={handleResendCode}>
              Resend code
            </button>
            <button
              type="button"
              className="btn"
              style={{ marginTop: 12, width: '100%', justifyContent: 'center', fontSize: 14 }}
              onClick={() => {
                setStep('credentials')
                setChallengeId('')
                setCode('')
                setMessage('')
                setError('')
              }}
            >
              Back to password
            </button>
          </>
          )
        ) : message ? (
          <div className="flash-ok" role="status">{message}</div>
        ) : (
          <>
            <label className="field">
              <span className="field-label">Email</span>
              <input className="input" type="email" value={email} onChange={e => setEmail(e.target.value)} required autoFocus />
            </label>
            <button type="submit" className="btn btn-primary" disabled={sent} style={{ marginTop: 24, width: '100%', justifyContent: 'center', padding: 12 }}>
              Send reset link
            </button>
          </>
        )}
        <button
          type="button"
          className="btn"
          style={{ marginTop: 12, width: '100%', justifyContent: 'center', fontSize: 14 }}
          onClick={() => {
            setMode(mode === 'login' ? 'forgot' : 'login')
            setStep('credentials')
            setError('')
            setMessage('')
            setSent(false)
            setCode('')
            setChallengeId('')
          }}
        >
          {mode === 'login' ? 'Forgot password?' : 'Back to sign in'}
        </button>
      </form>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  intro: { margin: '16px 0 0', fontSize: 12, fontWeight: 600, letterSpacing: '0.2em', color: colors.brand, textTransform: 'uppercase' as const },
  tagline: { color: colors.textMuted, fontSize: 16, lineHeight: 1.6, maxWidth: 400 },
}
