import { shell } from 'electron'
import type { WebContents } from 'electron'

// desktop-host-window-and-serving.md FR-4, Constitution §4 — External-link interception.
// All external links and window.open calls are denied within Electron and forwarded
// to the OS default browser via shell.openExternal, strictly validated for http/https.

/**
 * Validates whether a URL has a safe http: or https: scheme.
 * Constitution §4: shell.openExternal on unvalidated schemes (e.g. file:, javascript:)
 * can be an execution vector on various OS platforms.
 */
export function isSafeExternalUrl(rawUrl: string): boolean {
  try {
    const parsed = new URL(rawUrl)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

/**
 * Validates and opens an external URL using the system's default browser.
 * Returns true if the URL was valid and opened, false otherwise.
 */
export function openExternalIfSafe(
  rawUrl: string,
  opener: (url: string) => Promise<void> = shell.openExternal,
): boolean {
  if (!isSafeExternalUrl(rawUrl)) {
    console.warn('[navigation] Blocked external open attempt for unsafe URL:', rawUrl)
    return false
  }
  opener(rawUrl).catch((err) => {
    console.warn('[navigation] Failed to open external URL in default browser:', rawUrl, err)
  })
  return true
}

export interface NavigationOptions {
  /** Static allowed server origin, e.g. "http://127.0.0.1:34851" */
  allowedOrigin?: string
  /** Dynamic getter for allowed origin, used when the port is discovered at runtime */
  getAllowedOrigin?: () => string | undefined
  opener?: (url: string) => Promise<void>
}

/**
 * Configures external-link interception and origin navigation locking on a WebContents instance.
 * desktop-host-window-and-serving.md FR-4.
 */
export function setupWindowNavigation(
  webContents: WebContents,
  options?: NavigationOptions,
): void {
  const opener = options?.opener ?? shell.openExternal

  // Intercept window.open and <a target="_blank">
  webContents.setWindowOpenHandler(({ url }) => {
    openExternalIfSafe(url, opener)
    return { action: 'deny' }
  })

  // Intercept in-window navigations to external origins
  webContents.on('will-navigate', (event, url) => {
    const currentAllowedOrigin = options?.getAllowedOrigin?.() ?? options?.allowedOrigin
    if (currentAllowedOrigin !== undefined) {
      try {
        const parsed = new URL(url)
        if (parsed.origin === currentAllowedOrigin) {
          return
        }
      } catch {
        // Invalid URL -> block
      }
    }

    event.preventDefault()
    openExternalIfSafe(url, opener)
  })
}
