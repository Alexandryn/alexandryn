import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'

// afterPack.cjs is a plain CommonJS hook that electron-builder loads by path.
const require = createRequire(import.meta.url)
const { resolveBinaryPath } = require('../../scripts/afterPack.cjs') as {
  resolveBinaryPath: (options: {
    appOutDir: string
    platform: string
    names: Array<string | undefined>
  }) => string
}

const dirs: string[] = []
afterEach(() => {
  for (const dir of dirs.splice(0)) rmSync(dir, { recursive: true, force: true })
})

/** A fake unpacked app directory holding the given relative files. */
function unpacked(...files: string[]): string {
  const dir = mkdtempSync(join(tmpdir(), 'afterpack-'))
  dirs.push(dir)
  for (const file of files) {
    mkdirSync(dirname(join(dir, file)), { recursive: true })
    writeFileSync(join(dir, file), '')
  }
  return dir
}

describe('resolveBinaryPath', () => {
  it('finds the Linux binary under the configured executable name', () => {
    const dir = unpacked('alexandryn', 'chrome-sandbox')
    expect(resolveBinaryPath({ appOutDir: dir, platform: 'linux', names: ['alexandryn'] })).toBe(
      join(dir, 'alexandryn'),
    )
  })

  it('finds the Windows binary when the packager exposes no executableName (the release failure)', () => {
    // On Windows electron-builder names the file after productName, and the
    // packager's own executableName is undefined: this was `undefined.exe`.
    const dir = unpacked('Alexandryn.exe', 'resources/app.asar')
    expect(
      resolveBinaryPath({ appOutDir: dir, platform: 'win32', names: [undefined, 'Alexandryn'] }),
    ).toBe(join(dir, 'Alexandryn.exe'))
  })

  it('finds the macOS binary inside the app bundle when executableName is undefined', () => {
    const dir = unpacked(
      'Alexandryn.app/Contents/MacOS/Alexandryn',
      'Alexandryn.app/Contents/Info.plist',
    )
    expect(
      resolveBinaryPath({ appOutDir: dir, platform: 'darwin', names: [undefined, 'Alexandryn'] }),
    ).toBe(join(dir, 'Alexandryn.app', 'Contents', 'MacOS', 'Alexandryn'))
  })

  it('also finds a macOS binary whose name differs from the bundle name', () => {
    const dir = unpacked('Alexandryn.app/Contents/MacOS/alexandryn')
    expect(
      resolveBinaryPath({
        appOutDir: dir,
        platform: 'darwin',
        names: ['alexandryn', 'Alexandryn'],
      }),
    ).toBe(join(dir, 'Alexandryn.app', 'Contents', 'MacOS', 'alexandryn'))
  })

  it('prefers the first name that exists and ignores undefined and repeated names', () => {
    const dir = unpacked('Alexandryn.exe', 'alexandryn.exe')
    expect(
      resolveBinaryPath({
        appOutDir: dir,
        platform: 'win32',
        names: [undefined, 'alexandryn', 'alexandryn', 'Alexandryn'],
      }),
    ).toBe(join(dir, 'alexandryn.exe'))
  })

  it('throws, naming every path it tried, when nothing matches, instead of using a wrong file', () => {
    const dir = unpacked('something-else.exe')
    let message = ''
    try {
      resolveBinaryPath({ appOutDir: dir, platform: 'win32', names: [undefined, 'Alexandryn'] })
    } catch (error) {
      message = (error as Error).message
    }
    expect(message).toMatch(/Alexandryn\.exe/)
    expect(message).toMatch(/win32/)
  })

  it('throws when there are no usable names at all', () => {
    expect(() =>
      resolveBinaryPath({ appOutDir: unpacked(), platform: 'linux', names: [undefined] }),
    ).toThrow(/no executable name/i)
  })
})
