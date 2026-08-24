import { ReactNode } from 'react'
import ThemeToggle from './ThemeToggle'

export default function PageHeader({
  title,
  subtitle,
  actions,
  badges,
}: {
  title: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  badges?: ReactNode
}) {
  return (
    <header className="page-header">
      <div className="page-header-copy">
        <h1 className="page-title">{title}</h1>
        {badges && <div className="page-header-badges">{badges}</div>}
        {subtitle != null && subtitle !== '' && (
          <p className="page-subtitle">{subtitle}</p>
        )}
      </div>
      <div className="page-header-actions">
        {actions}
        <ThemeToggle />
      </div>
    </header>
  )
}
