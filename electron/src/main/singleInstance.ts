import { app, BrowserWindow } from 'electron'

// Single-instance lock must be acquired BEFORE any window is created
// and BEFORE the server spawn call, so a second process can never start
// a second Go server against the same PostgreSQL data directory.

/**
 * Acquires the Electron single-instance lock and wires the `second-instance`
 * handler so an existing window is focused when a second launch is attempted.
 *
 * Returns `true` if this process is the primary instance (lock acquired).
 * Returns `false` if another instance already holds the lock; the caller
 * must call `app.quit()` immediately — no window, no spawn.
 */
export function acquireSingleInstanceLock(): boolean {
  const isPrimary = app.requestSingleInstanceLock()

  if (isPrimary) {
    // When a second instance is launched, Electron fires this event on the
    // primary. Focus (or restore) the existing window rather than silently
    // ignoring the second launch.
    app.on('second-instance', () => {
      const windows = BrowserWindow.getAllWindows()
      const win = windows[0]
      if (win !== undefined) {
        if (win.isMinimized()) win.restore()
        win.focus()
      }
    })
  }

  return isPrimary
}
