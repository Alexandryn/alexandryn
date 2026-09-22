import { describe, expect, it } from 'vitest'
import { pathWithPostgresBinFirst } from './postgresBinaries'

// index.ts builds the env it passes to runServerLifecycle from exactly this
// function — this is that composition, isolated from Electron's app module
// (which resolvePostgresBinDir needs and index.ts is otherwise untestable
// without a full Electron runtime).
function envWithPostgresBin(
  base: NodeJS.ProcessEnv,
  pgBinDir: string | undefined,
  delimiter: string,
): NodeJS.ProcessEnv {
  if (!pgBinDir) return base
  return { ...base, PATH: pathWithPostgresBinFirst(pgBinDir, base.PATH, delimiter) }
}

describe('envWithPostgresBin (the composition index.ts uses)', () => {
  it('leaves the environment untouched when there is nothing bundled', () => {
    const base = { PATH: '/usr/bin', OTHER: 'x' }
    expect(envWithPostgresBin(base, undefined, ':')).toBe(base)
  })

  it('puts the bundled bin dir first on PATH, keeping every other variable', () => {
    const base = { PATH: '/usr/bin', OTHER: 'x' }
    const result = envWithPostgresBin(base, '/pg/bin', ':')
    expect(result.PATH).toBe('/pg/bin:/usr/bin')
    expect(result.OTHER).toBe('x')
    expect(result).not.toBe(base)
  })
})
