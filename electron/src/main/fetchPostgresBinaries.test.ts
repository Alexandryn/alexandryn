// readZipEntry is the one pure, testable piece of fetch-postgres-binaries.mjs
// (the rest is network + tar + filesystem side effects, exercised for real
// by scripts/README.md's documented manual run, not here). These build real
// zip files byte-for-byte (stored and deflated entries, a second entry after
// the target one, and a zip64-style central directory offset) so the parser
// is proven against real zip structure, not a hand-wavy stand-in.
import { deflateRawSync } from 'node:zlib'
import { describe, expect, it } from 'vitest'

// fetch-postgres-binaries.mjs is a plain build script, not itself
// typechecked; fetch-postgres-binaries.d.mts types its one tested export.
import { readZipEntry } from '../../scripts/fetch-postgres-binaries.mjs'

function u16(n: number): Buffer {
  const b = Buffer.alloc(2)
  b.writeUInt16LE(n)
  return b
}
function u32(n: number): Buffer {
  const b = Buffer.alloc(4)
  b.writeUInt32LE(n)
  return b
}

interface Entry {
  name: string
  data: Buffer
  method?: number
}

/** Builds a minimal, real zip file containing the given entries. */
function buildZip(entries: Entry[]): Buffer {
  const localParts: Buffer[] = []
  const centralParts: Buffer[] = []
  let offset = 0

  for (const { name, data, method = 0 } of entries) {
    const nameBuf = Buffer.from(name, 'utf8')
    const payload = method === 8 ? deflateRawSync(data) : data
    const localHeader = Buffer.concat([
      u32(0x04034b50),
      u16(20), // version needed
      u16(0), // flags
      u16(method),
      u16(0),
      u16(0), // mod time/date
      u32(0), // crc32 (unchecked by readZipEntry)
      u32(payload.length),
      u32(data.length),
      u16(nameBuf.length),
      u16(0),
      nameBuf,
    ])
    localParts.push(localHeader, payload)

    const centralHeader = Buffer.concat([
      u32(0x02014b50),
      u16(20),
      u16(20),
      u16(0),
      u16(method),
      u16(0),
      u16(0),
      u32(0),
      u32(payload.length),
      u32(data.length),
      u16(nameBuf.length),
      u16(0),
      u16(0),
      u16(0),
      u16(0),
      u32(0),
      u32(offset),
      nameBuf,
    ])
    centralParts.push(centralHeader)
    offset += localHeader.length + payload.length
  }

  const central = Buffer.concat(centralParts)
  const centralOffset = offset
  const eocd = Buffer.concat([
    u32(0x06054b50),
    u16(0),
    u16(0),
    u16(entries.length),
    u16(entries.length),
    u32(central.length),
    u32(centralOffset),
    u16(0),
  ])

  return Buffer.concat([...localParts, central, eocd])
}

describe('readZipEntry', () => {
  it('reads a stored (uncompressed) entry', () => {
    const zip = buildZip([{ name: 'hello.txt', data: Buffer.from('hello world') }])
    expect(readZipEntry(zip, 'hello.txt').toString()).toBe('hello world')
  })

  it('reads a deflated entry', () => {
    const payload = Buffer.from('x'.repeat(5000))
    const zip = buildZip([{ name: 'big.bin', data: payload, method: 8 }])
    expect(readZipEntry(zip, 'big.bin')).toEqual(payload)
  })

  it('finds the requested entry among several, by exact name', () => {
    const zip = buildZip([
      { name: 'META-INF/MANIFEST.MF', data: Buffer.from('Manifest-Version: 1.0\n') },
      { name: 'postgres-linux-x86_64.txz', data: Buffer.from('fake-txz-bytes') },
    ])
    expect(readZipEntry(zip, 'postgres-linux-x86_64.txz').toString()).toBe('fake-txz-bytes')
  })

  it('throws naming the entry when it is not present', () => {
    const zip = buildZip([{ name: 'a.txt', data: Buffer.from('a') }])
    expect(() => readZipEntry(zip, 'missing.txz')).toThrow(/missing\.txz/)
  })

  it('throws on a file with no end-of-central-directory record', () => {
    expect(() => readZipEntry(Buffer.from('not a zip'), 'x')).toThrow(/not a zip/)
  })

  it('throws on an unsupported compression method', () => {
    const zip = buildZip([{ name: 'a.bin', data: Buffer.from('a'), method: 99 }])
    expect(() => readZipEntry(zip, 'a.bin')).toThrow(/unsupported.*99/)
  })
})
