export function displayIncidentMessage(message?: string) {
  if (!message) return ''
  return message.replace(
    / \((?:nginx error page|Shopware maintenance|PHP-FPM \/ upstream|Cloudflare|Unknown error page)\)$/,
    '',
  )
}
