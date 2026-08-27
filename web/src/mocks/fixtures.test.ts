import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// frontend-shell-and-routing.md FR-6: every hand-written fixture (tier b —
// endpoints the contract does not cover yet) MUST carry a grep-able
// marker so phase 06 has a concrete checklist, not a codebase to
// re-read. This test enforces the tier, it isn't just documented.
const MARKER = 'TODO(phase-06): replace with contract-generated fixture'

// vitest runs with cwd = web/; import.meta.url can be an http: URL under
// the vite transform, so resolve from cwd instead.
const handwrittenDir = join(process.cwd(), 'src/mocks/fixtures/handwritten')

describe('hand-written mock fixtures (FR-6 tier b)', () => {
  const files = readdirSync(handwrittenDir).filter((f) => /\.(ts|json)$/.test(f))

  it('there is at least one, so this test is not vacuous', () => {
    expect(files.length).toBeGreaterThan(0)
  })

  it.each(files)('%s carries the TODO(phase-06) marker', (file) => {
    const contents = readFileSync(`${handwrittenDir}/${file}`, 'utf8')
    expect(contents).toContain(MARKER)
  })
})
