import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// Hand-written fixtures (tier b — endpoints the contract does not cover yet)
// carry a marker to indicate they should eventually be replaced by
// contract-generated fixtures.
const MARKER = 'TODO: replace with contract-generated fixture'

// vitest runs with cwd = web/; import.meta.url can be an http: URL under
// the vite transform, so resolve from cwd instead.
const handwrittenDir = join(process.cwd(), 'src/mocks/fixtures/handwritten')

describe('hand-written mock fixtures (tier b)', () => {
  const files = readdirSync(handwrittenDir).filter((f) => /\.(ts|json)$/.test(f))

  it('there is at least one, so this test is not vacuous', () => {
    expect(files.length).toBeGreaterThan(0)
  })

  it.each(files)('%s carries the replacement marker', (file) => {
    const contents = readFileSync(`${handwrittenDir}/${file}`, 'utf8')
    expect(contents).toContain(MARKER)
  })
})
