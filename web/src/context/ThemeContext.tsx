import { createContext, ReactNode, useContext, useEffect, useLayoutEffect, useMemo, useState } from 'react'

export type ThemeMode = 'light' | 'dark'
export type ThemePreference = ThemeMode | 'system'

const STORAGE_KEY = 'sentinel.theme'

type ThemeContextValue = {
  mode: ThemeMode
  preference: ThemePreference
  toggle: () => void
}

const ThemeContext = createContext<ThemeContextValue>({
  mode: 'dark',
  preference: 'system',
  toggle: () => {},
})

function systemTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark'
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

function readPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'light' || stored === 'dark') return stored
  } catch {
    /* ignore */
  }
  return 'system'
}

export function applyTheme(mode: ThemeMode) {
  const root = document.documentElement
  root.setAttribute('data-theme', mode)
  root.style.colorScheme = mode
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<ThemePreference>(readPreference)
  const [mode, setMode] = useState<ThemeMode>(() => (
    preference === 'system' ? systemTheme() : preference
  ))

  useLayoutEffect(() => {
    const next = preference === 'system' ? systemTheme() : preference
    setMode(next)
    applyTheme(next)
  }, [preference])

  useEffect(() => {
    if (preference !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: light)')
    const onChange = () => {
      const next = mq.matches ? 'light' : 'dark'
      setMode(next)
      applyTheme(next)
    }
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [preference])

  const value = useMemo<ThemeContextValue>(() => ({
    mode,
    preference,
    toggle: () => {
      const next: ThemeMode = mode === 'dark' ? 'light' : 'dark'
      setPreference(next)
      try {
        localStorage.setItem(STORAGE_KEY, next)
      } catch {
        /* ignore quota */
      }
    },
  }), [mode, preference])

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useTheme() {
  return useContext(ThemeContext)
}
