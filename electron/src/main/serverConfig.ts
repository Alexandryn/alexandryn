import { chmod, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

/**
 * `SHUTDOWN_GRACE_PERIOD`'s Go compiled default
 * (`backend-configuration.md`). Electron holds this in memory rather than
 * re-reading it from the config file at shutdown time — that file is
 * deleted the moment readiness succeeds (FR-5), long before shutdown
 * normally runs. If a future config ever overrides the key, the override
 * value is captured here when the file is authored.
 */
export const SERVER_SHUTDOWN_GRACE_MS = 10_000

export interface ServerConfigHandle {
  /** Pass this — and only this — as the child's `--config` argument. */
  readonly path: string
  /** Delete the file and its private directory. Idempotent. */
  cleanup(): Promise<void>
}

function tomlString(value: string): string {
  return `"${value
    .replace(/\\/g, '\\\\')
    .replace(/"/g, '\\"')
    .replace(/\n/g, '\\n')
    .replace(/\r/g, '\\r')
    .replace(/\t/g, '\\t')}"`
}


/**
 * Writes the Go server's `config.toml` (architecture-desktop-host.md
 * FR-5). Created under a fresh `mkdtemp` directory — an unpredictable
 * name, mode 0700 — with the file itself mode 0600, so no other local
 * user can read it and there is no fixed path to race a symlink onto.
 * The main process passes only `path` to the child and calls `cleanup()`
 * once the readiness check succeeds or the startup timeout expires.
 */
export async function writeServerConfig(
  values: Record<string, string>,
): Promise<ServerConfigHandle> {
  const dir = await mkdtemp(join(tmpdir(), 'alexandryn-'))
  // mkdtemp is 0700 by default, but umask can loosen it — pin it.
  await chmod(dir, 0o700)
  const path = join(dir, 'config.toml')

  const lines = ['# Alexandryn server config — written by the desktop host, deleted once ready.']
  for (const [key, value] of Object.entries(values)) {
    lines.push(`${key} = ${tomlString(value)}`)
  }
  await writeFile(path, lines.join('\n') + '\n', { mode: 0o600 })

  let cleaned = false
  return {
    path,
    async cleanup() {
      if (cleaned) return
      cleaned = true
      await rm(dir, { recursive: true, force: true })
    },
  }
}
