// State machine for the Go server's lifecycle after first startup.
// Distinct states:
//   Starting   — initial spawn, waiting for first readiness
//   Ready      — /healthz 200 received; real UI is loaded
//   Recovering — Go process exited post-readiness; respawning
//   Failed     — all attempts exhausted or first-start timeout; manual retry only
//
// Degraded state (Go alive, PostgreSQL unreachable) is surfaced by the
// web UI's own API-error handling, not by this lifecycle module.

import type { ChildProcess } from 'node:child_process'
import { resolveServerBinaryPath } from './serverBinary'
import { writeServerConfig } from './serverConfig'
import type { ServerConfigHandle } from './serverConfig'
import { spawnServer } from './serverProcess'
import { pollUntilReady } from './healthPoller'

export type ServerState = 'Starting' | 'Ready' | 'Recovering' | 'Failed'

export interface ServerEvent {
  state: ServerState
  /** Present when state is `Ready` — the port the server is listening on. */
  port?: number
  /** Present when state is `Recovering` — which attempt this is (1-based). */
  attempt?: number
}

/** Maximum automatic respawn attempts after a post-readiness crash. */
export const MAX_RESPAWN_ATTEMPTS = 3

/**
 * Returns the delay in milliseconds before respawn attempt `attempt` (1-based).
 *
 * 1s / 4s / 9s (= n² seconds). This is a pure function — no I/O,
 * no timers — so the unit test verifies the schedule in isolation from any
 * real process timing.
 */
export function backoffDelayMs(attempt: number): number {
  return attempt * attempt * 1000
}

export interface LifecycleOptions {
  /** Config values passed to writeServerConfig. */
  configValues?: Record<string, string>
  /**
   * Environment for the spawned server. Defaults to `process.env`. The one
   * production use is putting a bundled PostgreSQL's bin directory first on
   * `PATH` (postgresBinaries.ts).
   */
  env?: NodeJS.ProcessEnv
  /**
   * Override the binary path resolver. Defaults to `resolveServerBinaryPath()`
   * (production). Tests inject the test binary path directly to avoid
   * needing a mocked `app.isPackaged`.
   */
  binaryPathResolver?: () => string
  /**
   * Extra arguments appended to the spawn call, for testing only.
   * Production callers pass nothing.
   */
  extraArgs?: string[]
  /**
   * Override backoff delays (ms) for each attempt, for testing.
   * Default: [1000, 4000, 9000].
   */
  backoffDelaysMs?: readonly number[]
  /**
   * Override poll options for testing (shorter interval/timeout).
   */
  pollOptions?: { intervalMs?: number; timeoutMs?: number }
  /**
   * Optional AbortSignal to cleanly terminate the lifecycle and its child process.
   */
  signal?: AbortSignal
  /**
   * Called when a child process is spawned (initial start and respawns).
   */
  onChildSpawned?: (child: ChildProcess) => void
  /**
   * Called on every state transition, including the initial `Starting`.
   */
  onEvent: (event: ServerEvent) => void
}

/**
 * Runs the Go server lifecycle: spawn → poll → Ready → crash detection →
 * bounded respawn → Recovering or Failed.
 *
 * Returns the live child process once the server is ready (for the shutdown
 * handler in index.ts). Rejects if the server never becomes ready or all
 * respawn attempts are exhausted.
 *
 * The returned child may later be replaced by a respawn — callers that hold
 * the reference for shutdown must listen to `onEvent` to track the current
 * one, or use `setServerChild` (index.ts) which is updated by this function.
 */
export async function runServerLifecycle(options: LifecycleOptions): Promise<void> {
  const {
    configValues = {},
    env = process.env,
    binaryPathResolver = resolveServerBinaryPath,
    extraArgs = [],
    backoffDelaysMs = [1000, 4000, 9000],
    pollOptions,
    signal,
    onChildSpawned,
    onEvent,
  } = options

  onEvent({ state: 'Starting' })

  let activeChild: ChildProcess | undefined

  // Attempt a single start: resolve → config → spawn → poll.
  // Returns the active child and port on success.
  // Throws on failure (binary missing, poll timeout).
  async function attempt(): Promise<{ child: ChildProcess; port: number; config: ServerConfigHandle }> {
    const binaryPath = binaryPathResolver()
    // Pass Electron's PID for child-side orphan monitoring
    const config = await writeServerConfig({
      DESKTOP_PARENT_PID: String(process.pid),
      ...configValues,
    })



    let child: ChildProcess | undefined
    let port: number | undefined

    try {
      const spawned = spawnServer(binaryPath, config.path, extraArgs, env)
      child = spawned.child
      activeChild = child
      onChildSpawned?.(child)
      port = await spawned.portPromise
      await pollUntilReady(port, pollOptions)
      return { child, port, config }
    } catch (err) {
      // Clean up config file on any failure path.
      await config.cleanup()
      // Kill child if it's still running (e.g. poll timeout with binary alive).
      if (child !== undefined && child.exitCode === null && !child.killed) {
        child.kill('SIGKILL')
        await new Promise<void>((resolve) => child!.once('exit', resolve))
      }
      throw err
    }
  }

  // First start.
  let child: ChildProcess
  let port: number
  let config: ServerConfigHandle
  try {
    const first = await attempt()
    child = first.child
    port = first.port
    config = first.config
  } catch (err) {
    onEvent({ state: 'Failed' })
    throw err
  }

  await config.cleanup() // delete config once ready
  onEvent({ state: 'Ready', port })

  if (signal?.aborted) {
    if (child.exitCode === null && !child.killed) {
      child.kill('SIGTERM')
    }
    return
  }

  // Crash loop: watch for post-readiness exits and respawn up to MAX_RESPAWN_ATTEMPTS.
  await new Promise<void>((resolve) => {
    let respawnCount = 0
    let respawnTimer: ReturnType<typeof setTimeout> | undefined
    let isTerminated = false

    function cleanupAndResolve(): void {
      if (isTerminated) return
      isTerminated = true
      if (respawnTimer !== undefined) clearTimeout(respawnTimer)
      if (activeChild !== undefined && activeChild.exitCode === null && !activeChild.killed) {
        activeChild.kill('SIGTERM')
      }
      resolve()
    }

    if (signal !== undefined) {
      signal.addEventListener('abort', () => {
        cleanupAndResolve()
      }, { once: true })
    }

    function scheduleRespawn(): void {
      if (isTerminated) return
      if (respawnCount >= MAX_RESPAWN_ATTEMPTS) {
        onEvent({ state: 'Failed' })
        cleanupAndResolve()
        return
      }

      respawnCount++
      onEvent({ state: 'Recovering', attempt: respawnCount })

      const delay = backoffDelaysMs[respawnCount - 1] ?? backoffDelayMs(respawnCount)

      respawnTimer = setTimeout(() => {
        if (isTerminated) return
        void attempt()
          .then(({ child: newChild, port: newPort, config: newConfig }) => {
            if (isTerminated) {
              void newConfig.cleanup()
              if (newChild.exitCode === null && !newChild.killed) newChild.kill('SIGTERM')
              return
            }
            void newConfig.cleanup()
            onEvent({ state: 'Ready', port: newPort })
            watchChild(newChild)
          })
          .catch(() => {
            scheduleRespawn()
          })
      }, delay)
    }

    function watchChild(currentChild: ChildProcess): void {
      if (isTerminated) return
      if (currentChild.exitCode !== null || currentChild.killed) {
        scheduleRespawn()
        return
      }
      currentChild.once('exit', () => {
        if (isTerminated) return
        scheduleRespawn()
      })
    }

    watchChild(child)
  })
}
