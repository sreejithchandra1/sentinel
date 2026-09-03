import defaultLogo from '../assets/logo.svg'

export { defaultLogo }

export function applyFavicon(src?: string | null) {
  if (typeof document === 'undefined') return
  const href = (src || '').trim() || defaultLogo
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.type = faviconType(href)
  link.href = href
}

function faviconType(href: string): string {
  if (href.startsWith('data:')) {
    const match = /^data:([^;,]+)/.exec(href)
    if (match?.[1]) return match[1]
  }
  if (/\.png(\?|$)/i.test(href)) return 'image/png'
  if (/\.jpe?g(\?|$)/i.test(href)) return 'image/jpeg'
  if (/\.ico(\?|$)/i.test(href)) return 'image/x-icon'
  if (/\.webp(\?|$)/i.test(href)) return 'image/webp'
  return 'image/svg+xml'
}

export default function AppLogo({
  src,
  size = 32,
  alt = 'Sentinel',
}: {
  src?: string | null
  size?: number
  alt?: string
}) {
  return (
    <img
      src={src || defaultLogo}
      alt={alt}
      width={size}
      height={size}
      style={{
        width: size,
        height: size,
        borderRadius: Math.max(4, Math.round(size * 0.2)),
        objectFit: 'contain',
        flexShrink: 0,
        display: 'block',
      }}
    />
  )
}
