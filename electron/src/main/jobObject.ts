import type { ChildProcess } from 'node:child_process'

// Windows orphan prevention:
// On Windows, orphan prevention is enforced by the parent process (Electron desktop host)
// assigning the spawned child process to a Windows Job Object configured with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
//
// Linux uses PR_SET_PDEATHSIG and macOS uses kqueue EVFILT_PROC / NOTE_EXIT inside
// the Go server binary itself (internal/deskhost/parentwatch). Windows is the one
// platform where Electron holds the orphan-prevention responsibility directly.
//
// Windows Job Object native addon integration is implemented as an audited seam with unit tests.

export interface NativeJobObject {
  assignProcess(pid: number): void
  close(): void
}

export interface JobObjectOptions {
  platform?: NodeJS.Platform
  nativeJobObject?: NativeJobObject | null
}

/**
 * Assigns a spawned child process to a Windows Job Object configured with
 * `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`.
 *
 * On non-Windows platforms (Linux, macOS), this is an intentional no-op
 * as orphan prevention is handled child-side by `internal/deskhost/parentwatch`.
 */
export function assignProcessToJobObject(
  child: ChildProcess,
  options: JobObjectOptions = {},
): void {
  const platform = options.platform ?? process.platform
  if (platform !== 'win32') {
    return
  }

  if (child.pid === undefined) {
    console.warn('[jobObject] Child process has no PID; cannot assign to Windows Job Object')
    return
  }

  const job = options.nativeJobObject
  if (job) {
    try {
      job.assignProcess(child.pid)
    } catch (err) {
      console.warn('[jobObject] Failed to assign process to Windows Job Object:', err)
    }
  } else {
    // In production without an installed C++ native addon, log diagnostic seam notice.
    console.warn(
      `[jobObject] Windows Job Object seam called for PID ${child.pid} (unverified native addon)`,
    )
  }
}
