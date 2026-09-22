#!/usr/bin/env node
// Stages the bundled PostgreSQL binaries (ADR 0007) into
// electron/resources/postgres/bin, where postgresBinaries.ts looks for them
// in a dev build and electron-builder's extraResources picks them up for a
// packaged one (electron-builder.yml). Run before `npm run dev` and before
// packaging, for Linux and Windows only — cmd/server has no macOS spawn
// implementation yet (spawn_darwin.go), so nothing is bundled for it.
//
// Source: io.zonky.test.postgres, a widely used (Go, Rust, Java, Node
// embedded-postgres libraries all depend on it) redistribution of the
// official PostgreSQL binaries built for exactly this purpose. Its own
// packaging is Apache-2.0; the PostgreSQL binaries themselves are under the
// PostgreSQL License (permissive, redistribution allowed). Matches the
// project's Docker target (postgres:16-alpine): same major version, so
// migrations behave identically on both deployment targets.
//
// Each artifact is a .jar (a zip file) holding one postgres-<os>-x86_64.txz.
// The zip layer is read here with only node:zlib (no new dependency); the
// inner .txz (tar + xz) is extracted by the system `tar`, present on both
// target CI runners (ubuntu-latest, windows-latest) and on any developer's
// machine recent enough to have Node 24.

import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { execFileSync } from 'node:child_process'
import { inflateRawSync } from 'node:zlib'

const POSTGRES_VERSION = '16.15.0'

// groupId artifact suffix, the .txz entry name inside the jar, and the
// published sha256 of the jar itself (pinned so a compromised or altered
// mirror is detected, not trusted) — one entry per platform this project
// bundles for.
const TARGETS = {
  linux: {
    artifact: 'embedded-postgres-binaries-linux-amd64',
    entry: 'postgres-linux-x86_64.txz',
    sha256: '653abc065c682b85d3da50168fb95dc524bd85426cdec9ce3695a550ef431df2',
  },
  win32: {
    artifact: 'embedded-postgres-binaries-windows-amd64',
    entry: 'postgres-windows-x86_64.txz',
    sha256: '51c7812dc1af47c9a2ccb64fe74efb88c515cff2da347ce72aab92b4cc8e1191',
  },
}

/** Reads one named entry out of a zip's central directory, without a zip dependency. */
export function readZipEntry(zip, entryName) {
  const nameBytes = Buffer.from(entryName, 'utf8')
  const eocdSig = Buffer.from([0x50, 0x4b, 0x05, 0x06])
  const eocd = zip.lastIndexOf(eocdSig)
  if (eocd < 0) throw new Error('not a zip file (no end-of-central-directory record)')
  const cdOffset = zip.readUInt32LE(eocd + 16)
  const cdCount = zip.readUInt16LE(eocd + 10)

  let p = cdOffset
  for (let i = 0; i < cdCount; i++) {
    if (zip.readUInt32LE(p) !== 0x02014b50)
      throw new Error(`corrupt central directory entry at ${p}`)
    const method = zip.readUInt16LE(p + 10)
    const compSize = zip.readUInt32LE(p + 20)
    const nameLen = zip.readUInt16LE(p + 28)
    const extraLen = zip.readUInt16LE(p + 30)
    const commentLen = zip.readUInt16LE(p + 32)
    const localHeaderOffset = zip.readUInt32LE(p + 42)
    const name = zip.subarray(p + 46, p + 46 + nameLen)

    if (name.equals(nameBytes)) {
      const lh = localHeaderOffset
      if (zip.readUInt32LE(lh) !== 0x04034b50) throw new Error('corrupt local file header')
      const lhNameLen = zip.readUInt16LE(lh + 26)
      const lhExtraLen = zip.readUInt16LE(lh + 28)
      const dataStart = lh + 30 + lhNameLen + lhExtraLen
      const data = zip.subarray(dataStart, dataStart + compSize)
      if (method === 0) return Buffer.from(data) // stored, no compression
      if (method === 8) return inflateRawSync(data) // deflate
      throw new Error(`unsupported zip compression method ${method} for ${entryName}`)
    }
    p += 46 + nameLen + extraLen + commentLen
  }
  throw new Error(`entry ${entryName} not found in zip`)
}

async function fetchWithRetry(url, attempts = 3) {
  let lastErr
  for (let i = 0; i < attempts; i++) {
    try {
      const res = await fetch(url)
      if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`)
      return Buffer.from(await res.arrayBuffer())
    } catch (err) {
      lastErr = err
      if (i < attempts - 1) await new Promise((r) => setTimeout(r, 1000 * (i + 1)))
    }
  }
  throw lastErr
}

async function stagePostgres(platform) {
  const target = TARGETS[platform]
  if (!target) {
    console.log(
      `fetch-postgres-binaries: no bundled PostgreSQL for platform "${platform}" — skipping`,
    )
    return
  }

  const destDir = join(import.meta.dirname, '..', 'resources', 'postgres')
  if (existsSync(destDir)) {
    console.log(
      `fetch-postgres-binaries: ${destDir} already exists — skipping (delete it to re-fetch)`,
    )
    return
  }

  const url = `https://repo1.maven.org/maven2/io/zonky/test/postgres/${target.artifact}/${POSTGRES_VERSION}/${target.artifact}-${POSTGRES_VERSION}.jar`
  console.log(`fetch-postgres-binaries: downloading ${url}`)
  const jar = await fetchWithRetry(url)

  const gotSha = createHash('sha256').update(jar).digest('hex')
  if (gotSha !== target.sha256) {
    throw new Error(
      `fetch-postgres-binaries: checksum mismatch for ${url}\n  got:  ${gotSha}\n  want: ${target.sha256}`,
    )
  }

  const txz = readZipEntry(jar, target.entry)
  const tmp = mkdtempSync(join(tmpdir(), 'pg-fetch-'))
  const txzPath = join(tmp, target.entry)
  writeFileSync(txzPath, txz)

  mkdirSync(destDir, { recursive: true })
  execFileSync('tar', ['xJf', txzPath, '-C', destDir], { stdio: 'inherit' })
  rmSync(tmp, { recursive: true, force: true })

  const binDir = join(destDir, 'bin')
  if (!existsSync(binDir)) {
    throw new Error(`fetch-postgres-binaries: extraction did not produce ${binDir}`)
  }
  console.log(`fetch-postgres-binaries: staged PostgreSQL ${POSTGRES_VERSION} into ${destDir}`)
}

if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  const platform = process.argv.includes('--platform')
    ? process.argv[process.argv.indexOf('--platform') + 1]
    : process.platform
  await stagePostgres(platform)
}
