// desktop-host-ipc-surface.md FR-1, FR-2.
// The single source of truth for all declared IPC operations.
// Hand-maintained descriptor array (FR-1, constitution §9).
// Preload script iterates this array with zero external dependencies.

export interface OperationDescriptor {
  readonly name: string
  readonly namespace: string
  readonly method: string
}

// ── Operations ─────────────────────────────────────────────────────────────

/**
 * system.getAppVersion (FR-5) — Returns the packaged app version string from package.json.
 * Takes no arguments.
 */
export const SYSTEM_GET_APP_VERSION = {
  name: 'system.getAppVersion',
  namespace: 'system',
  method: 'getAppVersion',
} as const satisfies OperationDescriptor

/**
 * source.pickLocalFolder (FR-6) — Opens the OS native folder-selection dialog
 * and returns the chosen absolute path, or null on cancellation.
 * Takes no arguments (main process does not accept arbitrary paths from renderer).
 */
export const SOURCE_PICK_LOCAL_FOLDER = {
  name: 'source.pickLocalFolder',
  namespace: 'source',
  method: 'pickLocalFolder',
} as const satisfies OperationDescriptor

/**
 * Enumerated array of all declared IPC operations.
 * Main process and preload iterate this array directly at startup (FR-2).
 */
export const OPERATIONS = [
  SYSTEM_GET_APP_VERSION,
  SOURCE_PICK_LOCAL_FOLDER,
] as const

// ── Global TypeScript contract for window.alexandryn ────────────────────────

export interface AlexandrynDesktopBridge {
  system: {
    getAppVersion: () => Promise<string>
  }
  source: {
    pickLocalFolder: () => Promise<{ path: string } | null>
  }
}

declare global {
  interface Window {
    alexandryn?: AlexandrynDesktopBridge
  }
}
