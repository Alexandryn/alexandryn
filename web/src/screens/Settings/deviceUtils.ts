/**
 * Utilities for formatting device information in Settings -> Devices.
 */

export function formatRelativeTime(
  isoString: string | null | undefined,
  now?: Date,
  prefix = 'Active',
): string {
  if (!isoString) {
    return 'Never synced'
  }

  const date = new Date(isoString)
  if (isNaN(date.getTime())) {
    return 'Never synced'
  }

  const currentTime = now ? now.getTime() : Date.now()
  const diffMs = currentTime - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHours = Math.floor(diffMin / 60)

  if (diffMin < 5) {
    return `${prefix} now`
  }
  if (diffMin < 60) {
    return `${prefix} ${diffMin} minute${diffMin === 1 ? '' : 's'} ago`
  }
  if (diffHours < 24) {
    return `${prefix} ${diffHours} hour${diffHours === 1 ? '' : 's'} ago`
  }

  // Format as date string for 24h or older
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function isActiveDot(lastSeenAt: string | null | undefined, now?: Date): boolean {
  if (!lastSeenAt) {
    return false
  }
  const date = new Date(lastSeenAt)
  if (isNaN(date.getTime())) {
    return false
  }

  const currentTime = now ? now.getTime() : Date.now()
  const diffMs = currentTime - date.getTime()
  return diffMs >= 0 && diffMs < 5 * 60 * 1000
}

export function buildKindLine(deviceClass: string, enrolledVia: string): string {
  let classStr: string
  switch (deviceClass.toLowerCase()) {
    case 'phone':
      classStr = 'Phone'
      break
    case 'tablet':
      classStr = 'Tablet'
      break
    case 'desktop':
      classStr = 'Desktop'
      break
    case 'tv':
      classStr = 'TV'
      break
    case 'unknown':
      classStr = 'Device'
      break
    default:
      classStr = deviceClass ? deviceClass.charAt(0).toUpperCase() + deviceClass.slice(1) : 'Device'
  }

  let enrolledStr: string
  switch (enrolledVia.toLowerCase()) {
    case 'pairing_code':
      enrolledStr = 'paired by code'
      break
    case 'password_login':
      enrolledStr = 'direct login'
      break
    default:
      enrolledStr = enrolledVia ? enrolledVia.replace(/_/g, ' ') : 'paired'
  }

  return `${classStr} · ${enrolledStr}`
}
