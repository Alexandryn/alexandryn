// The single source of truth for all declared IPC operations.
// Hand-maintained descriptor array.
// Preload script iterates this array with zero external dependencies.

export interface OperationDescriptor {
  readonly name: string
  readonly namespace: string
  readonly method: string
}

// ── Operations ─────────────────────────────────────────────────────────────

/**
 * system.getAppVersion — Returns the packaged app version string from package.json.
 * Takes no arguments.
 */
export const SYSTEM_GET_APP_VERSION = {
  name: 'system.getAppVersion',
  namespace: 'system',
  method: 'getAppVersion',
} as const satisfies OperationDescriptor

/**
 * system.retryStartup — Re-triggers desktop server startup sequence after cold-start failure.
 * Takes no arguments.
 */
export const SYSTEM_RETRY_STARTUP = {
  name: 'system.retryStartup',
  namespace: 'system',
  method: 'retryStartup',
} as const satisfies OperationDescriptor

/**
 * source.pickLocalFolder — Opens the OS native folder-selection dialog
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
 * Main process and preload iterate this array directly at startup.
 */
export const OPERATIONS = [
  SYSTEM_GET_APP_VERSION,
  SYSTEM_RETRY_STARTUP,
  SOURCE_PICK_LOCAL_FOLDER,
] as const

// ── Global TypeScript contract for window.alexandryn ────────────────────────

export interface AlexandrynDesktopBridge {
  system: {
    getAppVersion: () => Promise<string>
    retryStartup: () => Promise<void>
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
