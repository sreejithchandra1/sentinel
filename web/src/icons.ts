/**
 * Main UI icons — replace files under /public/icons/ anytime.
 *
 * Sizes (display pixels, 1x):
 *   Brand logo (sidebar)     36×36   → /logo.svg  (or org logo from Settings)
 *   Nav icons                18×18   → /icons/*.svg
 *
 * Recommended asset specs:
 *   Nav SVG viewBox: 0 0 24 24, single-color strokes/fills (icons are tinted via CSS mask)
 *   Logo: square PNG/SVG, transparent background, at least 72×72 for retina
 */

export const iconSizes = {
  brandLogo: 36,
  nav: 18,
} as const

/** Public URL paths — drop replacement files in web/public/icons/ */
export const icons = {
  monitors: '/icons/monitors.svg',
  incidents: '/icons/incidents.svg',
  sla: '/icons/sla.svg',
  performance: '/icons/performance.svg',
  customers: '/icons/customers.svg',
  users: '/icons/users.svg',
  settings: '/icons/settings.svg',
} as const

export type NavIconKey = keyof typeof icons
