import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// desktop-host-process-model.md FR-2 security requirement — structural tests.
// The spawn call must use child_process.spawn with an argument array, never
// exec or shell:true. This is a static/source check (same category as
// backend specs' no-globals structural rules): it proves the pattern by
// reading the source, not by executing the code, because there is no runtime
// observation that can distinguish argument-array spawn from exec after the
// fact in a unit harness (both call the same OS primitives once running).

// Read non-comment lines only so comment text mentioning "exec" or
// "shell:true" (as documentation) does not trigger false negatives.
const LINES = readFileSync(join(import.meta.dirname, 'serverProcess.ts'), 'utf8')
  .split('\n')
  .filter((l) => !l.trimStart().startsWith('//'))
  .join('\n')

describe('serverProcess structural security (FR-2)', () => {
  it('imports spawn from node:child_process, not exec', () => {
    expect(LINES).toMatch(/\bspawn\b/)
    // Must not import `exec` from child_process — only method calls like
    // regex.exec() are permitted, not child_process.exec or execFile.
    expect(LINES).not.toMatch(/from\s+['"]node:child_process['"]\S*.*\bexec\b/)
    expect(LINES).not.toMatch(/require\s*\(\s*['"]node:child_process['"]\s*\).*\bexec\b/)
  })

  it('spawn options: shell is false, never true', () => {
    expect(LINES).toMatch(/shell:\s*false/)
    expect(LINES).not.toMatch(/shell:\s*true/)
  })

  it('passes arguments as an array literal, not a concatenated string', () => {
    // The spawn call must have ['--config', configPath] — an array literal
    // containing '--config' as a separate element, not a string built by
    // concatenation. This assertion checks the source representation.
    expect(LINES).toMatch(/\[\s*'--config'/)
  })
})

// Integration tests (real spawn + port capture + prefixed stdio) live in
// lifecycle.test.ts alongside E10/E11, which share the same Go test binary.

