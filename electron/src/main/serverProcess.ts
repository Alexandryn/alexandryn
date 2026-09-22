import { spawn } from 'node:child_process'
import type { ChildProcess } from 'node:child_process'
import { createInterface } from 'node:readline'
import { assignProcessToJobObject } from './jobObject'

// Spawn the Go server via child_process.spawn with an argument array,
// never exec or shell:true.
// The binary path and arguments are internal (no user/renderer input),
// but argument-array spawn is the correct default regardless: defense in
// depth and no shell-interpretation risk, ever.

const PORT_PATTERN = /\bPORT=(\d+)\b/

/**
 * Spawns the Go server binary and captures the port it announces on stdout.
 *
 * Returns a `SpawnedServer` once the binary is running. Port resolution is
 * asynchronous — `portPromise` resolves when the child emits `PORT=<n>` on
 * stdout, or rejects if the process exits before announcing a port.
 */
export interface SpawnedServer {
  /** The underlying child process. */
  readonly child: ChildProcess
  /**
   * Resolves with the port number once the child announces `PORT=<n>` on stdout.
   * Rejects if the child exits without announcing a port.
   */
  readonly portPromise: Promise<number>
}

/**
 * Spawns the Go server.
 *
 * @param binaryPath - Absolute path from `resolveServerBinaryPath()`.
 * @param configPath - Absolute path from `writeServerConfig()`.
 * @param extraArgs  - Additional arguments appended after `['--config', configPath]`.
 *                     Only used by integration tests; production callers pass nothing.
 * @param env        - Environment for the child. Defaults to `process.env` (today's
 *                     behaviour). The one production use is putting a bundled
 *                     PostgreSQL's bin directory first on `PATH`
 *                     (postgresBinaries.ts) — the Go server locates `postgres`
 *                     and `initdb` with a plain PATH lookup.
 */
export function spawnServer(
  binaryPath: string,
  configPath: string,
  extraArgs: string[] = [],
  env: NodeJS.ProcessEnv = process.env,
): SpawnedServer {
  // Argument array, never exec/shell:true — the binary path and the
  // --config flag/value are all internal; there is no user or renderer input
  // in this call, and the spawn form keeps it that way unconditionally.
  const child = spawn(binaryPath, ['--config', configPath, ...extraArgs], {
    stdio: ['ignore', 'pipe', 'pipe'],
    // Explicitly no shell. Stated for reviewers; false is the default.
    shell: false,
    env,
  })

  // On Windows, assign child to a Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
  assignProcessToJobObject(child)

  let portResolve: (port: number) => void

  let portReject: (err: Error) => void
  const portPromise = new Promise<number>((resolve, reject) => {
    portResolve = resolve
    portReject = reject
  })

  // Stdout: scan each line for PORT=<n>, prefix all output to distinguish
  // the child's logs from Electron's own.
  // The server announces its bound port on stdout as
  // "PORT=<n>" before it starts serving.
  let portAnnounced = false
  const stdout = createInterface({ input: child.stdout! })
  stdout.on('line', (line) => {
    process.stdout.write(`[server] ${line}\n`)
    if (!portAnnounced) {
      const match = PORT_PATTERN.exec(line)
      if (match !== null) {
        portAnnounced = true
        const port = parseInt(match[1]!, 10)
        portResolve(port)
      }
    }
  })

  // Stderr: prefix and forward; no PORT scanning needed here.
  const stderr = createInterface({ input: child.stderr! })
  stderr.on('line', (line) => {
    process.stderr.write(`[server] ${line}\n`)
  })

  // If spawn fails (e.g. ENOENT), reject the portPromise immediately.
  child.once('error', (err) => {
    portReject(err)
  })

  // If the child exits before announcing a port, reject the promise so the
  // caller (health poller) can surface a `Failed` state rather than
  // hanging indefinitely.
  child.once('exit', (code) => {
    portReject(new Error(`Server process exited (code ${code ?? 'null'}) before announcing a port`))
  })

  return { child, portPromise }
}
