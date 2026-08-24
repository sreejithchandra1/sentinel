import { CSSProperties, ReactNode } from 'react'

export default function Panel({
  children,
  padded = true,
  className = '',
  style,
}: {
  children: ReactNode
  padded?: boolean
  className?: string
  style?: CSSProperties
}) {
  const cls = ['panel', padded ? '' : 'panel--flush', className].filter(Boolean).join(' ')
  return (
    <div className={cls} style={style}>
      {children}
    </div>
  )
}
