export type DNSChangeRow = { type: string; value: string }

export type DNSChangeTables = {
  previous: DNSChangeRow[]
  current: DNSChangeRow[]
}

const segmentRe = /([A-Za-z0-9]+)\s+record changed from\s+(.+?)\s+to\s+(.+?)(?:;|$)/gi

export function parseDNSChangeMessage(msg: string): DNSChangeTables | null {
  const raw = msg.trim()
  if (!raw) return null
  const previous: DNSChangeRow[] = []
  const current: DNSChangeRow[] = []
  for (const m of raw.matchAll(segmentRe)) {
    const recType = m[1].trim().toUpperCase()
    for (const v of splitDNSValues(m[2])) previous.push({ type: recType, value: v })
    for (const v of splitDNSValues(m[3])) current.push({ type: recType, value: v })
  }
  if (!previous.length && !current.length) return null
  return { previous, current }
}

export function dnsChangeSummary(tables: DNSChangeTables | null): string {
  if (!tables) return ''
  const types: string[] = []
  const seen = new Set<string>()
  for (const row of [...tables.previous, ...tables.current]) {
    if (!row.type || seen.has(row.type)) continue
    seen.add(row.type)
    types.push(row.type)
  }
  if (types.length === 0) return 'DNS records changed'
  if (types.length === 1) return `${types[0]} record changed`
  if (types.length === 2) return `${types[0]} and ${types[1]} records changed`
  return `${types.slice(0, -1).join(', ')}, and ${types[types.length - 1]} records changed`
}

function splitDNSValues(raw: string): string[] {
  return raw.split(',').map(s => s.trim().replace(/^["']|["']$/g, '')).filter(Boolean)
}
