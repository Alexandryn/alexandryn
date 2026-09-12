import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { OPERATIONS } from './operations'
import { OPERATION_SCHEMAS } from '../main/ipc'

// IPC surface structural security tests:
// (a) No ipcMain.handle anywhere outside src/main/ipc.ts
// (b) Every operation has an entry in operations.ts and a valid Zod schema in ipc.ts
// (c) Every operation rejects invalid argument shapes
// (d) No operation accepts arbitrary paths from renderer

function getAllTypeScriptFiles(dir: string): string[] {
  const files: string[] = []
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    const stat = statSync(fullPath)
    if (stat.isDirectory()) {
      if (entry !== 'node_modules' && entry !== 'out' && entry !== 'dist' && entry !== 'test-helpers') {
        files.push(...getAllTypeScriptFiles(fullPath))
      }
    } else if (entry.endsWith('.ts') && !entry.endsWith('.test.ts') && !entry.endsWith('.spec.ts')) {
      files.push(fullPath)
    }
  }
  return files
}

describe('IPC surface structural security tests', () => {
  it('no ipcMain.handle call sites exist anywhere outside src/main/ipc.ts', () => {
    const srcDir = join(import.meta.dirname, '..')
    const tsFiles = getAllTypeScriptFiles(srcDir)

    const unauthorizedCallSites: { file: string; line: number }[] = []

    for (const file of tsFiles) {
      if (file.endsWith('/src/main/ipc.ts')) {
        continue
      }
      const content = readFileSync(file, 'utf8')
      const lines = content.split('\n')
      lines.forEach((line, index) => {
        const trimmed = line.trimStart()
        if (trimmed.startsWith('//') || trimmed.startsWith('*')) return
        if (/\bipcMain\.handle\b/.test(line)) {
          unauthorizedCallSites.push({ file, line: index + 1 })
        }
      })
    }

    expect(unauthorizedCallSites).toEqual([])
  })

  it('all declared operations follow <namespace>.<method> naming format', () => {
    expect(OPERATIONS.length).toBeGreaterThan(0)
    for (const op of OPERATIONS) {
      expect(op.name).toBe(`${op.namespace}.${op.method}`)
      expect(op.namespace).toMatch(/^[a-z]+$/)
      expect(op.method).toMatch(/^[a-zA-Z]+$/)
    }
  })

  it('every declared operation has an active Zod schema', () => {
    for (const op of OPERATIONS) {
      const schema = OPERATION_SCHEMAS[op.name]
      expect(schema).toBeDefined()
      expect(typeof schema?.safeParse).toBe('function')
    }
  })

  it('every declared operation rejects malformed or hostile argument types', () => {
    const hostileInputs = [
      { extraField: 'evil', payload: '/etc/passwd' },
      '__proto__',
      123456,
      ['nested', 'array'],
      { path: '../../malicious' },
    ]

    for (const op of OPERATIONS) {
      const schema = OPERATION_SCHEMAS[op.name]
      for (const input of hostileInputs) {
        const result = schema!.safeParse(input)
        expect(result.success).toBe(false)
      }
    }
  })

  it('no declared operation accepts arbitrary path strings', () => {
    for (const op of OPERATIONS) {
      const schema = OPERATION_SCHEMAS[op.name]
      // All operations take void/undefined (no arbitrary renderer path parameter)
      const parseWithPath = schema!.safeParse({ path: '/some/path' })
      expect(parseWithPath.success).toBe(false)
    }
  })
})
