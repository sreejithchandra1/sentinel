import { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

export type SegmentedTab = {
  id: string
  label: ReactNode
  count?: number
  to?: string
  end?: boolean
}

export default function SegmentedTabs({
  tabs,
  value,
  onChange,
  label,
}: {
  tabs: SegmentedTab[]
  value?: string
  onChange?: (id: string) => void
  label?: string
}) {
  const isNav = tabs.some(tab => tab.to)
  return (
    <div
      className="seg-tabs"
      role={isNav ? 'navigation' : 'tablist'}
      aria-label={label}
    >
      {tabs.map(tab => {
        const inner = (
          <>
            {tab.label}
            {typeof tab.count === 'number' && (
              <span className="seg-tab-count">{tab.count}</span>
            )}
          </>
        )
        if (tab.to) {
          return (
            <NavLink
              key={tab.id}
              to={tab.to}
              end={tab.end}
              className={({ isActive }) => 'seg-tab' + (isActive ? ' is-active' : '')}
            >
              {inner}
            </NavLink>
          )
        }
        const active = value === tab.id
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={active}
            className={'seg-tab' + (active ? ' is-active' : '')}
            onClick={() => onChange?.(tab.id)}
          >
            {inner}
          </button>
        )
      })}
    </div>
  )
}
