import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { app, dialog, ipcMain } from 'electron'
import { z } from 'zod'
import {
  OPERATIONS,
  SOURCE_PICK_LOCAL_FOLDER,
  SYSTEM_GET_APP_VERSION,
  SYSTEM_RETRY_STARTUP,
} from '../shared/operations'

// desktop-host-ipc-surface.md FR-2, FR-3, FR-4, FR-5, FR-6.
// Main process IPC registration: iterates OPERATIONS array directly.
// Line 1 of each handler runs Zod schema validation; rejects before business logic.

let cachedPackageVersion: string | undefined

/**
 * Reads the application version string from package.json once at main process startup (FR-5).
 */
export function getPackageVersion(): string {
  if (cachedPackageVersion !== undefined) {
    return cachedPackageVersion
  }
  try {
    const pkgPath = app.isPackaged
      ? join(app.getAppPath(), 'package.json')
      : join(import.meta.dirname, '../../package.json')
    const pkg = JSON.parse(readFileSync(pkgPath, 'utf8')) as { version?: string }
    cachedPackageVersion = pkg.version ?? app.getVersion()
  } catch {
    cachedPackageVersion = app.getVersion()
  }
  return cachedPackageVersion
}

export interface IpcHandlerDependencies {
  getAppVersion?: () => string
  showOpenDialog?: typeof dialog.showOpenDialog
  onRetryStartup?: () => Promise<void> | void
}

export const OPERATION_SCHEMAS: Record<string, z.ZodType> = {
  [SYSTEM_GET_APP_VERSION.name]: z.void().or(z.undefined()),
  [SYSTEM_RETRY_STARTUP.name]: z.void().or(z.undefined()),
  [SOURCE_PICK_LOCAL_FOLDER.name]: z.void().or(z.undefined()),
}

/**
 * Registers all declared IPC handlers from OPERATIONS array on ipcMain.
 */
export function registerIpcHandlers(deps: IpcHandlerDependencies = {}): void {
  const getAppVersion = deps.getAppVersion ?? getPackageVersion
  const showOpenDialog = deps.showOpenDialog ?? ((options) => dialog.showOpenDialog(options))

  const handlers: Record<string, (args: unknown) => Promise<unknown> | unknown> = {
    [SYSTEM_GET_APP_VERSION.name]: () => getAppVersion(),
    [SYSTEM_RETRY_STARTUP.name]: async () => {
      await deps.onRetryStartup?.()
    },
    [SOURCE_PICK_LOCAL_FOLDER.name]: async () => {
      const result = await showOpenDialog({
        properties: ['openDirectory'],
      })
      if (result.canceled || result.filePaths.length === 0 || !result.filePaths[0]) {
        return null
      }
      return { path: result.filePaths[0] }
    },
  }


  for (const op of OPERATIONS) {
    const handler = handlers[op.name]
    const schema = OPERATION_SCHEMAS[op.name]

    if (!handler) {
      throw new Error(`Missing handler for declared operation: ${op.name}`)
    }
    if (!schema) {
      throw new Error(`Missing Zod schema for declared operation: ${op.name}`)
    }

    ipcMain.removeHandler(op.name)

    ipcMain.handle(op.name, async (_event, rawArgs) => {
      // desktop-host-ipc-surface.md FR-3: Schema validation is handler's line 1
      const parsed = schema.safeParse(rawArgs)
      if (!parsed.success) {
        // Observability: Log failure reason internally, throw generic error to caller
        console.error(`[ipc] Validation failed for ${op.name}:`, parsed.error.issues)
        throw new Error('Invalid arguments')
      }

      return handler(parsed.data)
    })
  }
}

/**
 * Cleans up registered IPC handlers (used in test teardown).
 */
export function unregisterIpcHandlers(): void {
  for (const op of OPERATIONS) {
    ipcMain.removeHandler(op.name)
  }
}
