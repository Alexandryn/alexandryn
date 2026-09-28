// State machine for Alexandryn's hosting lifecycle.
// 8 explicit states:
//   Not installed — server binary or prerequisites missing on disk
//   Initializing  — preflight environment, directories, configuration setup
//   Starting      — process spawned, waiting for port announcement and readiness
//   Healthy       — /healthz 200 AND /readyz 200; database connected and serving data
//   Degraded      — /healthz 200 alive, but /readyz non-200 (database connection lost or unready)
//   Stopped       — host process cleanly terminated/shutdown
//   Failed        — startup timeout, crash before ready, or recovery exhausted
//   Recovering    — post-readiness crash detected; respawning with backoff

import type { ChildProcess } from 'node:child_process'
import { existsSync } from 'node:fs'
import { ensureBinaryExecutable, resolveServerBinaryPath } from './serverBinary'
import { writeServerConfig } from './serverConfig'
import type { ServerConfigHandle } from './serverConfig'
import { spawnServer } from './serverProcess'
import { pollUntilReady, startHealthMonitor, type HealthMonitor } from './healthPoller'

export type HostingLifecycleState =
  | 'Not installed'
  | 'Initializing'
  | 'Starting'
  | 'Healthy'
  | 'Degraded'
  | 'Stopped'
  | 'Failed'
  | 'Recovering'

/** ServerState alias for HostingLifecycleState, supporting legacy 'Ready' alias where needed. */
export type ServerState = HostingLifecycleState | 'Ready'

export interface ServerEvent {
  state: ServerState
  /** Present when state is Healthy / Ready — the port the server is listening on. */
  port?: number
  /** Present when state is Recovering — which attempt this is (1-based). */
  attempt?: number
  /** Informational or error message explaining the state transition. */
  message?: string
  /** Error object if state transition was caused by an error. */
  error?: Error
}

/** Maximum automatic respawn attempts after a post-readiness crash. */
export const MAX_RESPAWN_ATTEMPTS = 3

/**
 * Returns the delay in milliseconds before respawn attempt `attempt` (1-based).
 *
 * 1s / 4s / 9s (= n² seconds). Pure function for testability.
 */
export function backoffDelayMs(attempt: number): number {
  return attempt * attempt * 1000
}

export interface LifecycleOptions {
  /** Config values passed to writeServerConfig. */
  configValues?: Record<string, string>
  /** Environment for the spawned server. Defaults to `process.env`. */
  env?: NodeJS.ProcessEnv
  /** Override the binary path resolver. Defaults to `resolveServerBinaryPath()`. */
  binaryPathResolver?: () => string
  /** Extra arguments appended to the spawn call, for testing only. */
  extraArgs?: string[]
  /** Override backoff delays (ms) for each attempt, for testing. Default: [1000, 4000, 9000]. */
  backoffDelaysMs?: readonly number[]
  /** Override poll options for testing (shorter interval/timeout). */
  pollOptions?: { intervalMs?: number; timeoutMs?: number }
  /** Override health monitor interval (ms). Default: 3000ms. */
  healthMonitorIntervalMs?: number
  /** Optional AbortSignal to cleanly terminate the lifecycle and child process. */
  signal?: AbortSignal
  /** Called when a child process is spawned (initial start and respawns). */
  onChildSpawned?: (child: ChildProcess) => void
  /** Called on every state transition. */
  onEvent: (event: ServerEvent) => void
}

/**
 * Runs the Go server lifecycle with explicit state transitions:
 * Not installed (if missing) -> Initializing -> Starting -> Healthy <-> Degraded
 * -> Recovering (on crash) -> Failed (exhausted) or Stopped (clean shutdown).
 */
export async function runServerLifecycle(options: LifecycleOptions): Promise<void> {
  const {
    configValues = {},
    env = process.env,
    binaryPathResolver = resolveServerBinaryPath,
    extraArgs = [],
    backoffDelaysMs = [1000, 4000, 9000],
    pollOptions,
    healthMonitorIntervalMs,
    signal,
    onChildSpawned,
    onEvent,
  } = options

  const binaryPath = binaryPathResolver()
  if (!existsSync(binaryPath)) {
    const errorMsg = `Server executable not found at ${binaryPath}`
    onEvent({ state: 'Not installed', message: errorMsg })
    onEvent({ state: 'Failed', message: errorMsg })
    throw new Error(errorMsg)
  }

  onEvent({ state: 'Initializing' })
  ensureBinaryExecutable(binaryPath)

  let activeChild: ChildProcess | undefined
  let healthMonitor: HealthMonitor | undefined

  // Attempt a single start: spawn -> announce port -> poll readyz/healthz.
  async function attempt(): Promise<{
    child: ChildProcess
    port: number
    config: ServerConfigHandle
  }> {
    onEvent({ state: 'Starting' })

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
      await config.cleanup()
      if (child !== undefined && child.exitCode === null && !child.killed) {
        child.kill('SIGKILL')
        await new Promise<void>((resolve) => child!.once('exit', resolve))
      }
      throw err
    }
  }

  // First start
  let child: ChildProcess
  let port: number
  let config: ServerConfigHandle
  try {
    const first = await attempt()
    child = first.child
    port = first.port
    config = first.config
  } catch (err) {
    onEvent({
      state: 'Failed',
      message: (err as Error).message,
      error: err as Error,
    })
    throw err
  }

  await config.cleanup()
  onEvent({ state: 'Healthy', port })

  function setupMonitor(currentPort: number): HealthMonitor {
    return startHealthMonitor(currentPort, {
      intervalMs: healthMonitorIntervalMs,
      onStateChange: (status) => {
        if (status.state === 'degraded') {
          onEvent({
            state: 'Degraded',
            port: currentPort,
            message: status.message || 'Database connection degraded',
          })
        } else if (status.state === 'healthy') {
          onEvent({ state: 'Healthy', port: currentPort })
        }
      },
    })
  }

  healthMonitor = setupMonitor(port)

  if (signal?.aborted) {
    healthMonitor.stop()
    if (child.exitCode === null && !child.killed) {
      child.kill('SIGTERM')
    }
    onEvent({ state: 'Stopped' })
    return
  }

  // Crash and recovery loop
  await new Promise<void>((resolve) => {
    let respawnCount = 0
    let respawnTimer: ReturnType<typeof setTimeout> | undefined
    let isTerminated = false

    function cleanupAndResolve(stopped = false): void {
      if (isTerminated) return
      isTerminated = true
      healthMonitor?.stop()
      if (respawnTimer !== undefined) clearTimeout(respawnTimer)
      if (activeChild !== undefined && activeChild.exitCode === null && !activeChild.killed) {
        activeChild.kill('SIGTERM')
      }
      if (stopped) {
        onEvent({ state: 'Stopped' })
      }
      resolve()
    }

    if (signal !== undefined) {
      signal.addEventListener(
        'abort',
        () => {
          cleanupAndResolve(true)
        },
        { once: true },
      )
    }

    function scheduleRespawn(): void {
      if (isTerminated) return
      healthMonitor?.stop()

      if (respawnCount >= MAX_RESPAWN_ATTEMPTS) {
        onEvent({
          state: 'Failed',
          message: `Server failed after ${MAX_RESPAWN_ATTEMPTS} recovery attempts`,
        })
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
            onEvent({ state: 'Healthy', port: newPort })
            healthMonitor = setupMonitor(newPort)
            watchChild(newChild)
          })
          .catch((err) => {
            if (isTerminated) return
            console.warn('[serverLifecycle] respawn attempt failed:', err)
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
